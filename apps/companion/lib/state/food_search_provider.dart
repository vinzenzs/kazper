import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../data/repository.dart';
import '../domain/models.dart';
import 'app_providers.dart';

/// State of the Camera screen's food-search mode: the current query plus the
/// resolved food list. Empty query → previously-used foods (recency-first);
/// non-empty → name search.
class FoodSearchState {
  final String query;
  final AsyncValue<List<Product>> results;

  const FoodSearchState({
    this.query = '',
    this.results = const AsyncValue.loading(),
  });

  FoodSearchState copyWith({
    String? query,
    AsyncValue<List<Product>>? results,
  }) {
    return FoodSearchState(
      query: query ?? this.query,
      results: results ?? this.results,
    );
  }
}

/// Drives the food picker: holds the query, debounces input (~250ms), and
/// resolves results stale-while-revalidate — the local cache paints first, then
/// the network revalidates. When the network is unreachable it stays on the
/// cache (offline substring match for a query) with no error surfaced unless the
/// cache is also empty.
class FoodSearchNotifier extends Notifier<FoodSearchState> {
  Timer? _debounce;
  int _seq = 0;

  @override
  FoodSearchState build() {
    ref.onDispose(() => _debounce?.cancel());
    // Load recent foods as soon as the picker opens (after build returns).
    Future.microtask(() => _run(''));
    return const FoodSearchState();
  }

  Repository get _repo => ref.read(repositoryProvider);

  /// Called on every keystroke; debounces before resolving.
  void setQuery(String q) {
    state = state.copyWith(query: q);
    _debounce?.cancel();
    _debounce = Timer(const Duration(milliseconds: 250), () => _run(q));
  }

  /// Force a re-resolve of the current query (pull-to-refresh, post-create).
  Future<void> refresh() => _run(state.query);

  Future<void> _run(String q) async {
    final seq = ++_seq;
    final trimmed = q.trim();

    // 1) Paint from cache immediately (no spinner if we have something).
    try {
      final cached = trimmed.isEmpty
          ? await _repo.cachedRecentProducts(50)
          : await _repo.cachedSearchProducts(trimmed);
      if (seq == _seq && cached.isNotEmpty) {
        state = state.copyWith(results: AsyncData(cached));
      }
    } catch (_) {
      // Cache read failures are non-fatal; the network pass below decides.
    }

    // 2) Revalidate over the network.
    try {
      final fresh = trimmed.isEmpty
          ? await _repo.recentProducts()
          : await _repo.searchProducts(trimmed);
      if (seq == _seq) state = state.copyWith(results: AsyncData(fresh));
    } catch (e, st) {
      if (seq != _seq) return;
      // Offline: keep whatever the cache gave us; only error if we have nothing.
      if (state.results is! AsyncData<List<Product>>) {
        final cached = trimmed.isEmpty
            ? await _repo.cachedRecentProducts(50)
            : await _repo.cachedSearchProducts(trimmed);
        state = state.copyWith(
          results:
              cached.isEmpty ? AsyncError(e, st) : AsyncData(cached),
        );
      }
    }
  }
}

final foodSearchProvider =
    NotifierProvider<FoodSearchNotifier, FoodSearchState>(
        FoodSearchNotifier.new);
