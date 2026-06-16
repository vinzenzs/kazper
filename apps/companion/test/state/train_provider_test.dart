import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:kazper/domain/training.dart';
import 'package:kazper/state/app_providers.dart';
import 'package:kazper/state/train_provider.dart';

import '../support/fake_repository.dart';

ProviderContainer _container(FakeRepository repo) {
  final c = ProviderContainer(
    overrides: [repositoryProvider.overrideWithValue(repo)],
  );
  addTearDown(c.dispose);
  return c;
}

TrainingDay _day({List<TrainSession>? sessions}) => TrainingDay(
      date: '2026-06-16',
      sessions: sessions ??
          [
            TrainSession(
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
                notes: [],
              ),
            ),
          ],
    );

void main() {
  group('trainProvider', () {
    test('fetches from network when no cache', () async {
      final repo = FakeRepository()..freshTraining = _day();
      final day = await _container(repo).read(trainProvider.future);
      expect(day!.sessions.single.sport, 'bike');
      expect(day.sessions.single.fuel!.intra.single, contains('90 g/h'));
    });

    test('returns cache first, then revalidates to fresh', () async {
      final repo = FakeRepository()
        ..cachedTraining = _day()
        ..freshTraining = _day();
      final c = _container(repo);
      expect(await c.read(trainProvider.future), isNotNull);
      await Future<void>.delayed(Duration.zero);
      expect(c.read(trainProvider).value, isNotNull);
    });

    test('empty day is a rest day (no sessions)', () async {
      final repo = FakeRepository()
        ..freshTraining = const TrainingDay(date: '2026-06-16', sessions: []);
      final day = await _container(repo).read(trainProvider.future);
      expect(day!.sessions, isEmpty);
    });

    test('round-trips through toJson/fromJson (cache shape)', () {
      final j = _day().toJson();
      final back = TrainingDay.fromJson(j);
      expect(back.sessions.single.targets.single, 'Z4 · 230–268 W');
      expect(back.sessions.single.fuel!.post.single, contains('protein'));
    });
  });
}
