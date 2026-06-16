import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../domain/training.dart';
import 'app_providers.dart';

/// Stale-while-revalidate training day for the Train screen. On build it
/// returns the cached assembled day immediately (if present) and revalidates in
/// the background; otherwise it awaits the network fan-out. No offline banner —
/// staleness is implicit, matching the Today screen (mobile-companion spec).
class TrainNotifier extends AsyncNotifier<TrainingDay?> {
  @override
  Future<TrainingDay?> build() async {
    final repo = ref.watch(repositoryProvider);
    final date = todayDate();
    final cached = await repo.cachedTrainingDay(date);
    if (cached != null) {
      _revalidate(date);
      return cached;
    }
    return repo.fetchTrainingDay(date);
  }

  Future<void> _revalidate(String date) async {
    try {
      state = AsyncData(await ref.read(repositoryProvider).fetchTrainingDay(date));
    } catch (_) {
      // Keep the stale cache visible — no banner, no error surface.
    }
  }

  /// Pull-to-refresh / tab-reselect refresh.
  Future<void> refresh() async {
    final date = todayDate();
    state = await AsyncValue.guard(
      () => ref.read(repositoryProvider).fetchTrainingDay(date),
    );
  }
}

final trainProvider =
    AsyncNotifierProvider<TrainNotifier, TrainingDay?>(TrainNotifier.new);
