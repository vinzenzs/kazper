import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../domain/training.dart';
import '../../state/train_provider.dart';

/// Band 1 of the Train screen (add-companion-train-screen): today's prescribed
/// session(s) and the fuel each demands. Read-only — a fueling lens, not a
/// training tracker: it exposes no control that mutates a session's schedule,
/// status, or structure (the watch/Garmin owns execution).
class TrainPage extends ConsumerWidget {
  const TrainPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final day = ref.watch(trainProvider);
    return Scaffold(
      appBar: AppBar(title: const Text('Train')),
      body: RefreshIndicator(
        onRefresh: () => ref.read(trainProvider.notifier).refresh(),
        child: day.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (e, _) => _Scrollable(child: _Message('Couldn’t load training. $e')),
          data: (d) => (d == null || d.sessions.isEmpty)
              ? const _RestDay()
              : _SessionList(day: d),
        ),
      ),
    );
  }
}

class _SessionList extends StatelessWidget {
  final TrainingDay day;
  const _SessionList({required this.day});

  @override
  Widget build(BuildContext context) {
    return ListView(
      padding: const EdgeInsets.all(12),
      children: [
        for (final s in day.sessions) _SessionCard(session: s),
      ],
    );
  }
}

class _SessionCard extends StatelessWidget {
  final TrainSession session;
  const _SessionCard({required this.session});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final time = TimeOfDay.fromDateTime(session.startedAt.toLocal()).format(context);
    return Card(
      margin: const EdgeInsets.only(bottom: 12),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Header: sport · duration · time
            Row(
              children: [
                Text(_sportEmoji(session.sport), style: const TextStyle(fontSize: 22)),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    session.name ?? _sportLabel(session.sport),
                    style: theme.textTheme.titleMedium,
                  ),
                ),
                Text(time, style: theme.textTheme.labelLarge),
              ],
            ),
            const SizedBox(height: 4),
            Text(
              '${_sportLabel(session.sport)} · ${_duration(session.durationMin)}',
              style: theme.textTheme.bodySmall?.copyWith(color: theme.hintColor),
            ),
            if (session.targets.isNotEmpty) ...[
              const SizedBox(height: 6),
              Wrap(
                spacing: 6,
                runSpacing: 6,
                children: [for (final t in session.targets) _Chip(t)],
              ),
            ],
            if (session.fuel != null) ...[
              const Divider(height: 24),
              _FuelBlock(fuel: session.fuel!),
            ],
          ],
        ),
      ),
    );
  }
}

class _FuelBlock extends StatelessWidget {
  final SessionFuel fuel;
  const _FuelBlock({required this.fuel});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('FUEL THIS SESSION',
            style: theme.textTheme.labelSmall?.copyWith(
                letterSpacing: 0.8, color: theme.colorScheme.primary)),
        const SizedBox(height: 6),
        _phase(context, 'Pre', fuel.pre),
        _phase(context, 'During', fuel.intra),
        _phase(context, 'Post', fuel.post),
        for (final n in fuel.notes) ...[
          const SizedBox(height: 6),
          Text(n, style: theme.textTheme.bodySmall?.copyWith(
              fontStyle: FontStyle.italic, color: theme.hintColor)),
        ],
      ],
    );
  }

  Widget _phase(BuildContext context, String label, List<String> lines) {
    if (lines.isEmpty) return const SizedBox.shrink();
    final theme = Theme.of(context);
    return Padding(
      padding: const EdgeInsets.only(bottom: 4),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 56,
            child: Text(label, style: theme.textTheme.labelMedium),
          ),
          Expanded(
            child: Text(lines.join('\n'), style: theme.textTheme.bodyMedium),
          ),
        ],
      ),
    );
  }
}

class _Chip extends StatelessWidget {
  final String label;
  const _Chip(this.label);
  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
      decoration: BoxDecoration(
        color: theme.colorScheme.secondaryContainer,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Text(label,
          style: theme.textTheme.labelSmall
              ?.copyWith(color: theme.colorScheme.onSecondaryContainer)),
    );
  }
}

class _RestDay extends StatelessWidget {
  const _RestDay();
  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return _Scrollable(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          const SizedBox(height: 80),
          Icon(Icons.self_improvement_outlined, size: 48, color: theme.hintColor),
          const SizedBox(height: 12),
          Text('Rest day', style: theme.textTheme.titleMedium),
          const SizedBox(height: 4),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 32),
            child: Text(
              'No session today — fuel for recovery: protein across the day and enough carbs to top up glycogen.',
              textAlign: TextAlign.center,
              style: theme.textTheme.bodySmall?.copyWith(color: theme.hintColor),
            ),
          ),
        ],
      ),
    );
  }
}

class _Message extends StatelessWidget {
  final String text;
  const _Message(this.text);
  @override
  Widget build(BuildContext context) => Padding(
        padding: const EdgeInsets.all(32),
        child: Center(child: Text(text, textAlign: TextAlign.center)),
      );
}

/// Wraps content so RefreshIndicator + empty/error states remain pull-able.
class _Scrollable extends StatelessWidget {
  final Widget child;
  const _Scrollable({required this.child});
  @override
  Widget build(BuildContext context) => ListView(
        physics: const AlwaysScrollableScrollPhysics(),
        children: [child],
      );
}

String _duration(double minutes) {
  final m = minutes.round();
  if (m < 60) return '$m min';
  final h = m ~/ 60;
  final rem = m % 60;
  return rem == 0 ? '${h}h' : '${h}h ${rem}m';
}

String _sportEmoji(String sport) => switch (sport) {
      'swim' => '🏊',
      'bike' => '🚴',
      'run' => '🏃',
      'strength' => '🏋️',
      'yoga' => '🧘',
      'mobility' => '🤸',
      'multisport' => '🔁',
      _ => '🏅',
    };

String _sportLabel(String sport) =>
    sport.isEmpty ? sport : '${sport[0].toUpperCase()}${sport.substring(1)}';
