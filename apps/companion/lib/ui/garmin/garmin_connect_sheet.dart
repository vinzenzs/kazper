import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../domain/garmin.dart';
import '../../state/garmin_provider.dart';

/// Opens the Garmin connection sheet: sync-status at a glance plus a
/// (re)connect flow that drives the login/MFA proxy. The bridge holds the
/// credentials, so this never collects an email or password.
Future<void> showGarminSheet(BuildContext context) {
  return showModalBottomSheet<void>(
    context: context,
    isScrollControlled: true,
    showDragHandle: true,
    builder: (_) => const _GarminSheet(),
  );
}

class _GarminSheet extends ConsumerStatefulWidget {
  const _GarminSheet();

  @override
  ConsumerState<_GarminSheet> createState() => _GarminSheetState();
}

class _GarminSheetState extends ConsumerState<_GarminSheet> {
  final _codeController = TextEditingController();

  @override
  void dispose() {
    _codeController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final connect = ref.watch(garminConnectProvider);
    final sync = ref.watch(garminSyncProvider);

    return Padding(
      padding: EdgeInsets.fromLTRB(
        16,
        0,
        16,
        16 + MediaQuery.of(context).viewInsets.bottom,
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('Garmin', style: Theme.of(context).textTheme.titleLarge),
          const SizedBox(height: 12),
          _SyncStatusTile(
            status: sync,
            onRefresh: () => ref.read(garminSyncProvider.notifier).refresh(),
          ),
          const Divider(height: 24),
          ..._connectSection(context, connect),
        ],
      ),
    );
  }

  List<Widget> _connectSection(BuildContext context, GarminConnectState s) {
    final notifier = ref.read(garminConnectProvider.notifier);
    switch (s.phase) {
      case GarminConnectPhase.disabled:
        return const [
          ListTile(
            contentPadding: EdgeInsets.zero,
            leading: Icon(Icons.cloud_off_outlined),
            title: Text('Garmin not configured'),
            subtitle: Text('This server has no Garmin integration set up.'),
          ),
        ];
      case GarminConnectPhase.connected:
        return [
          ListTile(
            contentPadding: EdgeInsets.zero,
            leading: Icon(Icons.check_circle_outline,
                color: Theme.of(context).colorScheme.primary),
            title: const Text('Garmin connected'),
            subtitle: const Text('Your Garmin account is re-linked.'),
          ),
        ];
      case GarminConnectPhase.awaitingMfa:
      case GarminConnectPhase.submittingMfa:
        final busy = s.phase == GarminConnectPhase.submittingMfa;
        return [
          const Text('Enter the 6-digit code from your authenticator app.'),
          const SizedBox(height: 8),
          TextField(
            controller: _codeController,
            keyboardType: TextInputType.number,
            maxLength: 6,
            enabled: !busy,
            decoration: const InputDecoration(
              labelText: 'MFA code',
              counterText: '',
            ),
          ),
          if (s.error != null) _ErrorText(s.error!),
          const SizedBox(height: 8),
          FilledButton(
            onPressed: busy
                ? null
                : () => notifier.submitMfa(_codeController.text),
            child: busy
                ? const _Spinner()
                : const Text('Submit code'),
          ),
        ];
      case GarminConnectPhase.idle:
      case GarminConnectPhase.triggering:
      case GarminConnectPhase.error:
        final busy = s.phase == GarminConnectPhase.triggering;
        return [
          const Text(
            'Re-link your Garmin account when the connection expires. '
            'Credentials stay on the server — you only confirm the login here.',
          ),
          if (s.error != null) _ErrorText(s.error!),
          const SizedBox(height: 8),
          FilledButton.icon(
            onPressed: busy ? null : notifier.connect,
            icon: busy
                ? const _Spinner()
                : const Icon(Icons.link),
            label: Text(busy ? 'Connecting…' : 'Connect Garmin'),
          ),
        ];
    }
  }
}

/// The sync-status line: last successful sync, with in-progress / failed /
/// not-configured variants.
class _SyncStatusTile extends StatelessWidget {
  final AsyncValue<GarminSyncStatus?> status;
  final VoidCallback onRefresh;

  const _SyncStatusTile({required this.status, required this.onRefresh});

  @override
  Widget build(BuildContext context) {
    return ListTile(
      contentPadding: EdgeInsets.zero,
      leading: const Icon(Icons.sync),
      title: const Text('Last sync'),
      subtitle: Text(_subtitle()),
      trailing: IconButton(
        icon: const Icon(Icons.refresh),
        tooltip: 'Refresh',
        onPressed: onRefresh,
      ),
    );
  }

  String _subtitle() {
    return status.when(
      loading: () => 'Checking…',
      error: (_, _) => 'Sync status unavailable',
      data: (s) {
        if (s == null) return 'Garmin not configured';
        final latest = s.latest;
        if (latest != null && latest.isRunning) return 'Syncing…';
        if (s.lastSuccessfulAt != null) {
          final rel = _relativeTime(s.lastSuccessfulAt!);
          if (latest != null && latest.isError) {
            return 'Last sync failed · last good $rel';
          }
          return 'Last synced $rel';
        }
        if (latest != null && latest.isError) return 'Last sync failed';
        return 'Never synced';
      },
    );
  }
}

class _ErrorText extends StatelessWidget {
  final String text;
  const _ErrorText(this.text);

  @override
  Widget build(BuildContext context) => Padding(
        padding: const EdgeInsets.only(top: 8),
        child: Text(
          text,
          style: TextStyle(color: Theme.of(context).colorScheme.error),
        ),
      );
}

class _Spinner extends StatelessWidget {
  const _Spinner();
  @override
  Widget build(BuildContext context) => const SizedBox(
        width: 18,
        height: 18,
        child: CircularProgressIndicator(strokeWidth: 2),
      );
}

/// Coarse relative time ("just now" / "5 min ago" / "2 h ago" / "3 d ago").
String _relativeTime(DateTime when) {
  final d = DateTime.now().difference(when);
  if (d.inMinutes < 1) return 'just now';
  if (d.inMinutes < 60) return '${d.inMinutes} min ago';
  if (d.inHours < 24) return '${d.inHours} h ago';
  return '${d.inDays} d ago';
}
