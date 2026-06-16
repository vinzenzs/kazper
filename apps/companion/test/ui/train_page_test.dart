import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:kazper/domain/training.dart';
import 'package:kazper/state/app_providers.dart';
import 'package:kazper/ui/train/train_page.dart';

import '../support/fake_repository.dart';

Widget _harness(FakeRepository repo) => ProviderScope(
      overrides: [repositoryProvider.overrideWithValue(repo)],
      child: const MaterialApp(home: TrainPage()),
    );

TrainSession _bike() => TrainSession(
      workoutId: 'w1',
      sport: 'bike',
      name: 'Endurance Ride',
      startedAt: DateTime.parse('2026-06-16T15:00:00Z'),
      durationMin: 180,
      targets: const ['Z4 · 230–268 W'],
      fuel: const SessionFuel(
        pre: ['80 g carb · 120 min before'],
        intra: ['90 g/h carb · 700 mg/h Na'],
        post: ['60 g carb · 30 g protein'],
        notes: ['Intensity defaulted to Z2 (no TSS yet).'],
      ),
    );

void main() {
  testWidgets('renders the session header, resolved target, and its fuel',
      (tester) async {
    final repo = FakeRepository()
      ..freshTraining = TrainingDay(date: '2026-06-16', sessions: [_bike()]);
    await tester.pumpWidget(_harness(repo));
    await tester.pumpAndSettle();

    expect(find.text('Endurance Ride'), findsOneWidget);
    expect(find.text('Z4 · 230–268 W'), findsOneWidget);
    expect(find.text('FUEL THIS SESSION'), findsOneWidget);
    expect(find.textContaining('90 g/h'), findsOneWidget);
    expect(find.textContaining('Z2'), findsOneWidget); // disclosure note
  });

  testWidgets('renders a rest-day state when there is no session',
      (tester) async {
    final repo = FakeRepository()
      ..freshTraining = const TrainingDay(date: '2026-06-16', sessions: []);
    await tester.pumpWidget(_harness(repo));
    await tester.pumpAndSettle();

    expect(find.text('Rest day'), findsOneWidget);
    expect(find.text('FUEL THIS SESSION'), findsNothing);
  });

  testWidgets('guardrail: read-only — no write affordances', (tester) async {
    final repo = FakeRepository()
      ..freshTraining = TrainingDay(date: '2026-06-16', sessions: [_bike()]);
    await tester.pumpWidget(_harness(repo));
    await tester.pumpAndSettle();

    // The Train screen is a fueling lens, not a tracker: it exposes no control
    // that mutates state (no FAB, no buttons, no toggles).
    expect(find.byType(FloatingActionButton), findsNothing);
    expect(find.byType(ElevatedButton), findsNothing);
    expect(find.byType(TextButton), findsNothing);
    expect(find.byType(Switch), findsNothing);
    expect(find.byType(Checkbox), findsNothing);
  });
}
