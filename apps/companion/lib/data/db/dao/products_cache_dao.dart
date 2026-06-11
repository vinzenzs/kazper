import 'dart:convert';

import 'package:drift/drift.dart';

import '../app_database.dart';

part 'products_cache_dao.g.dart';

@DriftAccessor(tables: [ProductsCache])
class ProductsCacheDao extends DatabaseAccessor<AppDatabase>
    with _$ProductsCacheDaoMixin {
  ProductsCacheDao(super.db);

  Future<void> upsertFromApi(Map<String, dynamic> product) {
    final loggedAt = product['last_logged_at'] as String?;
    return into(productsCache).insertOnConflictUpdate(
      ProductsCacheCompanion.insert(
        id: product['id'] as String,
        name: product['name'] as String,
        brand: Value(product['brand'] as String?),
        source: (product['source'] as String?) ?? 'unknown',
        nutrimentsPer100gJson:
            jsonEncode(product['nutriments_per_100g'] ?? const {}),
        servingSizeG: Value((product['serving_size_g'] as num?)?.toDouble()),
        lastLoggedQuantityG: Value(
            (product['last_logged_quantity_g'] as num?)?.toDouble()),
        lastLoggedAt:
            Value(loggedAt == null ? null : DateTime.parse(loggedAt).toUtc()),
        refreshedAt: DateTime.now(),
      ),
    );
  }

  Future<ProductsCacheData?> getById(String id) {
    return (select(productsCache)..where((p) => p.id.equals(id)))
        .getSingleOrNull();
  }

  Future<List<ProductsCacheData>> recentlyScanned(int limit) {
    return (select(productsCache)
          ..orderBy([(p) => OrderingTerm.desc(p.refreshedAt)])
          ..limit(limit))
        .get();
  }

  /// Previously-used foods, most-recently-*used* first. Mirrors the backend's
  /// `last_logged_at DESC NULLS LAST, name ASC` so the offline picker matches
  /// `GET /products`. Distinct from [recentlyScanned], which orders by fetch
  /// time, not use.
  Future<List<ProductsCacheData>> recentlyUsed(int limit) {
    return (select(productsCache)
          ..orderBy([
            (p) => OrderingTerm(
                  expression: p.lastLoggedAt,
                  mode: OrderingMode.desc,
                  nulls: NullsOrder.last,
                ),
            (p) => OrderingTerm.asc(p.name),
          ])
          ..limit(limit))
        .get();
  }

  /// Offline fallback for search: case-insensitive substring match on name or
  /// brand, recency-first. Used when the network is unreachable.
  Future<List<ProductsCacheData>> searchCached(String q, {int limit = 50}) {
    final pattern = '%${q.toLowerCase()}%';
    return (select(productsCache)
          ..where((p) =>
              p.name.lower().like(pattern) | p.brand.lower().like(pattern))
          ..orderBy([
            (p) => OrderingTerm(
                  expression: p.lastLoggedAt,
                  mode: OrderingMode.desc,
                  nulls: NullsOrder.last,
                ),
            (p) => OrderingTerm.asc(p.name),
          ])
          ..limit(limit))
        .get();
  }
}
