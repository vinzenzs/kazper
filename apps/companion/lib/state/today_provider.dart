import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../domain/models.dart';
import 'app_providers.dart';

/// Stale-while-revalidate daily summary. On build it returns the cached
/// summary immediately (if present) and kicks off a background revalidation;
/// otherwise it awaits the network. There is no offline banner — staleness is
/// implicit (per the spec).
class TodayNotifier extends AsyncNotifier<DailySummary?> {
  @override
  Future<DailySummary?> build() async {
    final repo = ref.watch(repositoryProvider);
    final date = todayDate();
    final cached = await repo.cachedDailySummary(date);
    if (cached != null) {
      // Render cache now; revalidate without blocking the first frame.
      _revalidate(date);
      return cached;
    }
    return repo.fetchDailySummary(date);
  }

  Future<void> _revalidate(String date) async {
    try {
      final fresh = await ref.read(repositoryProvider).fetchDailySummary(date);
      state = AsyncData(fresh);
    } catch (_) {
      // Keep the stale cache visible — no banner, no error surface.
    }
  }

  /// Pull-to-refresh / post-write refresh.
  Future<void> refresh() async {
    final date = todayDate();
    state = await AsyncValue.guard(
      () => ref.read(repositoryProvider).fetchDailySummary(date),
    );
  }
}

final todayProvider =
    AsyncNotifierProvider<TodayNotifier, DailySummary?>(TodayNotifier.new);

/// Today's hydration total + entries. Supports an optimistic bump so a logged
/// glass shows instantly — the write is enqueued in the outbox and only lands
/// on the backend asynchronously, so a naive immediate re-fetch would race the
/// POST and read the stale total. The optimistic value is reconciled on the
/// next refresh (pull-to-refresh / foreground).
class HydrationTodayNotifier extends AsyncNotifier<HydrationDaily> {
  @override
  Future<HydrationDaily> build() {
    return ref.watch(repositoryProvider).fetchHydrationDaily(todayDate());
  }

  /// Immediately reflect [ml] added to today's total (no network round-trip).
  void addOptimistic(double ml) {
    final current = state.valueOrNull;
    if (current == null) return;
    state = AsyncData(HydrationDaily(
      date: current.date,
      tz: current.tz,
      totalMl: current.totalMl + ml,
      entries: current.entries,
    ));
  }

  /// Re-fetch from the backend, reconciling any optimistic bumps.
  Future<void> refresh() async {
    state = await AsyncValue.guard(
      () => ref.read(repositoryProvider).fetchHydrationDaily(todayDate()),
    );
  }
}

final hydrationDailyProvider =
    AsyncNotifierProvider<HydrationTodayNotifier, HydrationDaily>(
        HydrationTodayNotifier.new);
