import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../domain/chat.dart';
import '../../state/chat_provider.dart';
import '../../state/sessions_provider.dart';

/// Chat history — past conversations newest-first. Tap to reopen and continue;
/// swipe to delete; overflow menu to rename. Online-only (no offline cache).
class SessionsPage extends ConsumerWidget {
  const SessionsPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final sessions = ref.watch(sessionsProvider);

    return Scaffold(
      appBar: AppBar(title: const Text('History')),
      body: RefreshIndicator(
        onRefresh: () => ref.read(sessionsProvider.notifier).refresh(),
        child: _body(context, ref, sessions),
      ),
    );
  }

  Widget _body(BuildContext context, WidgetRef ref, SessionsState s) {
    if (s.loading && s.sessions.isEmpty) {
      return const Center(child: CircularProgressIndicator());
    }
    if (s.error != null && s.sessions.isEmpty) {
      return ListView(children: [
        const SizedBox(height: 120),
        const Center(child: Text("Couldn't load conversations.")),
        const SizedBox(height: 8),
        Center(
          child: TextButton(
            onPressed: () => ref.read(sessionsProvider.notifier).refresh(),
            child: const Text('Retry'),
          ),
        ),
      ]);
    }
    if (s.sessions.isEmpty) {
      return ListView(children: const [
        SizedBox(height: 120),
        Center(child: Text('No conversations yet.')),
      ]);
    }
    return ListView.builder(
      itemCount: s.sessions.length,
      itemBuilder: (context, i) => _SessionTile(session: s.sessions[i]),
    );
  }
}

class _SessionTile extends ConsumerWidget {
  final ChatSessionSummary session;
  const _SessionTile({required this.session});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final title = (session.title?.trim().isNotEmpty ?? false)
        ? session.title!.trim()
        : 'Untitled';
    return Dismissible(
      key: ValueKey(session.id),
      direction: DismissDirection.endToStart,
      background: Container(
        color: Theme.of(context).colorScheme.errorContainer,
        alignment: Alignment.centerRight,
        padding: const EdgeInsets.only(right: 20),
        child: Icon(Icons.delete_outline,
            color: Theme.of(context).colorScheme.onErrorContainer),
      ),
      confirmDismiss: (_) => _confirmDelete(context),
      onDismissed: (_) =>
          ref.read(sessionsProvider.notifier).delete(session.id),
      child: ListTile(
        title: Text(title, maxLines: 1, overflow: TextOverflow.ellipsis),
        subtitle: Row(
          children: [
            Text(_relativeTime(session.lastMessageAt)),
            if (session.awaitingConfirmation) ...[
              const SizedBox(width: 8),
              Icon(Icons.fact_check_outlined,
                  size: 14, color: Theme.of(context).colorScheme.primary),
              const SizedBox(width: 2),
              Text('Awaiting confirmation',
                  style: TextStyle(
                      fontSize: 12, color: Theme.of(context).colorScheme.primary)),
            ],
          ],
        ),
        onTap: () => _open(context, ref),
        trailing: PopupMenuButton<String>(
          onSelected: (v) {
            if (v == 'rename') _rename(context, ref, title);
            if (v == 'delete') _deleteWithConfirm(context, ref);
          },
          itemBuilder: (_) => const [
            PopupMenuItem(value: 'rename', child: Text('Rename')),
            PopupMenuItem(value: 'delete', child: Text('Delete')),
          ],
        ),
      ),
    );
  }

  Future<void> _open(BuildContext context, WidgetRef ref) async {
    final ok = await ref.read(chatProvider.notifier).openSession(session);
    if (!context.mounted) return;
    if (ok) {
      Navigator.pop(context);
    } else {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text("Couldn't open that conversation.")),
      );
    }
  }

  Future<void> _deleteWithConfirm(BuildContext context, WidgetRef ref) async {
    if (await _confirmDelete(context) == true && context.mounted) {
      await ref.read(sessionsProvider.notifier).delete(session.id);
    }
  }

  Future<bool?> _confirmDelete(BuildContext context) => showDialog<bool>(
        context: context,
        builder: (ctx) => AlertDialog(
          title: const Text('Delete conversation?'),
          content: const Text('This permanently removes it from the server.'),
          actions: [
            TextButton(
                onPressed: () => Navigator.pop(ctx, false),
                child: const Text('Cancel')),
            FilledButton(
                onPressed: () => Navigator.pop(ctx, true),
                child: const Text('Delete')),
          ],
        ),
      );

  Future<void> _rename(
      BuildContext context, WidgetRef ref, String current) async {
    final controller = TextEditingController(
        text: (session.title?.trim().isNotEmpty ?? false) ? session.title : '');
    final title = await showDialog<String>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Rename conversation'),
        content: TextField(
          controller: controller,
          autofocus: true,
          decoration: const InputDecoration(hintText: 'Title'),
          onSubmitted: (v) => Navigator.pop(ctx, v.trim()),
        ),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(ctx), child: const Text('Cancel')),
          FilledButton(
              onPressed: () => Navigator.pop(ctx, controller.text.trim()),
              child: const Text('Save')),
        ],
      ),
    );
    if (title != null && context.mounted) {
      await ref.read(sessionsProvider.notifier).rename(session.id, title);
    }
  }
}

/// A compact "time ago" for the list (minutes / hours / days, else a date).
String _relativeTime(DateTime t) {
  final d = DateTime.now().difference(t);
  if (d.inMinutes < 1) return 'just now';
  if (d.inMinutes < 60) return '${d.inMinutes}m ago';
  if (d.inHours < 24) return '${d.inHours}h ago';
  if (d.inDays < 7) return '${d.inDays}d ago';
  return '${t.year}-${t.month.toString().padLeft(2, '0')}-${t.day.toString().padLeft(2, '0')}';
}
