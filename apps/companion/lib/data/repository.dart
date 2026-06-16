import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:drift/drift.dart' show Value;

import '../domain/models.dart';
import '../domain/planning.dart';
import '../domain/training.dart';
import 'db/app_database.dart';
import 'net/api_client.dart';
import 'net/idempotency.dart';
import 'sync/outbox_worker.dart';

/// The data surface the providers depend on. Reads go straight to the network
/// (writing through to the Drift cache where the screens need stale-while-
/// revalidate); writes are enqueued into the outbox and flushed by the worker.
///
/// It is an interface so provider unit tests can supply a fake without booting
/// Drift or Dio (see `test/state/`).
abstract class Repository {
  /// Cached daily summary for [date], or null if nothing is cached yet.
  Future<DailySummary?> cachedDailySummary(String date);

  /// Fetches `GET /summary/daily`, writes through to the cache, returns it.
  /// The backend defaults the timezone when the app omits it.
  Future<DailySummary> fetchDailySummary(String date);

  /// Fetches `GET /summary/hydration/daily`.
  Future<HydrationDaily> fetchHydrationDaily(String date);

  /// Fetches `GET /goals`. Returns null when goals are unset (`{"goals":null}`).
  Future<Goals?> fetchGoals();

  /// Cached product row by id, or null.
  Future<Product?> cachedProduct(String id);

  /// `POST /products/lookup/{barcode}`, write-through to the products cache.
  /// Throws [ProductNotFound] on 404.
  Future<Product> lookupProduct(String barcode);

  /// `GET /products/search?q=`, recency-ranked, write-through to the cache.
  Future<List<Product>> searchProducts(String q);

  /// `GET /products`, most-recently-used first, write-through to the cache.
  Future<List<Product>> recentProducts({int limit, int offset});

  /// Previously-used foods from the local cache, most-recently-used first.
  /// Used for the picker's instant (stale-while-revalidate) first paint and as
  /// the offline source when the network is unreachable.
  Future<List<Product>> cachedRecentProducts(int limit);

  /// Offline search fallback: a case-insensitive substring match over the
  /// cached foods, recency-first.
  Future<List<Product>> cachedSearchProducts(String q);

  /// Multipart `POST /meals/from_photo`. Returns the committed meal + the
  /// inference confidence. Not routed through the outbox — the caller needs
  /// the response synchronously and the image bytes are large.
  Future<PhotoMealResult> logMealFromPhoto({
    required Uint8List jpegBytes,
    required double quantityG,
    required String mealType,
    required DateTime loggedAt,
  });

  /// Cached assembled training day for [date], or null if nothing is cached.
  Future<TrainingDay?> cachedTrainingDay(String date);

  /// Assembles today's prescribed session(s) + their fuel from
  /// `GET /context/training`, `GET /workouts/{id}/program`, and
  /// `GET /race-prep/recommend-workout-fuel`; writes through to the cache.
  Future<TrainingDay> fetchTrainingDay(String date);

  // --- Write path (enqueued in the outbox) ----------------------------------

  Future<void> enqueueMeal({
    required String productId,
    required double quantityG,
    required String mealType,
    required DateTime loggedAt,
  });

  Future<void> enqueueFreeformMeal({
    required String name,
    required double quantityG,
    required String mealType,
    required DateTime loggedAt,
    double? kcal,
    double? proteinG,
    double? carbsG,
    double? fatG,
    Map<String, double>? micros,
    bool saveAsProduct,
  });

  Future<void> enqueuePatchMeal(
    String id, {
    double? quantityG,
    String? mealType,
  });

  Future<void> enqueueDeleteMeal(String id);

  Future<void> enqueueHydration({required double quantityMl, DateTime? loggedAt});

  Future<void> enqueueDeleteHydration(String id);

  /// Drains the outbox now (used after optimistic writes).
  // --- Planning surfaces (chat-produced) ------------------------------------

  /// Cached planned meals for [date] (stale-while-revalidate), or empty.
  Future<List<PlannedMeal>> cachedPlan(String date);

  /// Fetches `GET /plan?from=date&to=date`, write-through to the cache.
  Future<List<PlannedMeal>> fetchPlan(String date);

  /// Cached shopping list, or empty.
  Future<List<ShoppingItem>> cachedShopping();

  /// Fetches `GET /shopping/items?include_checked=true`, write-through.
  Future<List<ShoppingItem>> fetchShopping();

  Future<void> enqueueMarkEaten(String planId);
  Future<void> enqueuePlanStatus(String planId, String status);
  Future<void> enqueueShoppingChecked(String itemId, bool checked);
  Future<void> enqueueAddShoppingItem(String name);
  Future<void> enqueueClearCheckedShopping();

  Future<void> flush();
}

/// Thrown by [Repository.lookupProduct] on a 404 from the backend.
class ProductNotFound implements Exception {
  final String barcode;
  ProductNotFound(this.barcode);
  @override
  String toString() => 'product_not_found: $barcode';
}

/// Thrown when the vision endpoint is not configured (`503 vision_unavailable`).
class VisionUnavailable implements Exception {
  @override
  String toString() => 'vision_unavailable';
}

class ApiRepository implements Repository {
  final AppDatabase db;
  final ApiClient api;
  final OutboxWorker outbox;

  ApiRepository({required this.db, required this.api, required this.outbox});

  @override
  Future<DailySummary?> cachedDailySummary(String date) async {
    final cached = await db.recentSummaryDao.getAnyForDate(date);
    if (cached == null) return null;
    return DailySummary.fromJson({
      'date': cached.date,
      'tz': cached.tz,
      'totals': cached.totals,
      'entries': cached.entries,
      // totals/entries are cached; adherence/goal_source live alongside.
      ...?cached.totals['__envelope__'] as Map<String, dynamic>?,
    });
  }

  @override
  Future<DailySummary> fetchDailySummary(String date) async {
    final resp = await api.dio.get<Map<String, dynamic>>(
      '/summary/daily',
      queryParameters: {'date': date},
    );
    final json = resp.data ?? const {};
    final summary = DailySummary.fromJson(json);
    // Write-through: cache the whole envelope so adherence survives a reload.
    final totals = (json['totals'] as Map?)?.cast<String, dynamic>() ?? {};
    final envelope = <String, dynamic>{
      'adherence': ?json['adherence'],
      'goal_source': ?json['goal_source'],
      'phase_name': ?json['phase_name'],
    };
    await db.recentSummaryDao.upsertForDate(
      date: date,
      tz: summary.tz,
      totals: {...totals, '__envelope__': envelope},
      entries: ((json['entries'] as List?) ?? const [])
          .map((e) => (e as Map).cast<String, dynamic>())
          .toList(),
    );
    return summary;
  }

  @override
  Future<TrainingDay?> cachedTrainingDay(String date) async {
    final payload = await db.trainingDayDao.getForDate(date);
    if (payload == null) return null;
    return TrainingDay.fromJson(payload);
  }

  @override
  Future<TrainingDay> fetchTrainingDay(String date) async {
    // 1. The day's prescribed sessions = upcoming planned workouts whose LOCAL
    //    start date is today. /context/training returns a lookahead window; we
    //    keep only today's.
    final ctx = await api.dio.get<Map<String, dynamic>>(
      '/context/training',
      queryParameters: {'date': date},
    );
    final upcoming = ((ctx.data?['upcoming_workouts'] as List?) ?? const [])
        .map((e) => (e as Map).cast<String, dynamic>())
        .where((w) => _localYmd(DateTime.parse(w['started_at'] as String)) == date)
        .toList();

    // 2. Per session: resolved targets + the fuel it demands (best-effort).
    final sessions = <TrainSession>[];
    for (final w in upcoming) {
      final id = w['id'] as String;
      sessions.add(TrainSession(
        workoutId: id,
        sport: w['sport'] as String,
        name: w['name'] as String?,
        startedAt: DateTime.parse(w['started_at'] as String),
        durationMin: (w['duration_min'] as num).toDouble(),
        targets: await _sessionTargets(id),
        fuel: await _sessionFuel(id),
      ));
    }
    final day = TrainingDay(date: date, sessions: sessions);
    await db.trainingDayDao.upsertForDate(date: date, payload: day.toJson());
    return day;
  }

  String _localYmd(DateTime utc) {
    final d = utc.toLocal();
    final mm = d.month.toString().padLeft(2, '0');
    final dd = d.day.toString().padLeft(2, '0');
    return '${d.year.toString().padLeft(4, '0')}-$mm-$dd';
  }

  /// Compact resolved-target lines for a session's program (per-segment for
  /// multisport). Best-effort: returns empty on any failure — the fuel is the
  /// hero, targets are context.
  Future<List<String>> _sessionTargets(String workoutId) async {
    try {
      final resp = await api.dio
          .get<Map<String, dynamic>>('/workouts/$workoutId/program');
      final prog = resp.data ?? const {};
      final out = <String>[];
      final segments = prog['segments'] as List?;
      if (segments != null && segments.isNotEmpty) {
        for (final raw in segments) {
          final seg = (raw as Map).cast<String, dynamic>();
          final sport = seg['sport'] as String?;
          for (final t in _targetsFromSteps(seg['steps'] as List?)) {
            out.add(sport != null ? '$sport · $t' : t);
          }
        }
      } else {
        out.addAll(_targetsFromSteps(prog['steps'] as List?));
      }
      return out.toSet().toList();
    } catch (_) {
      return const [];
    }
  }

  List<String> _targetsFromSteps(List? steps) {
    final out = <String>[];
    for (final raw in steps ?? const []) {
      final s = (raw as Map).cast<String, dynamic>();
      if (s['type'] == 'repeat') {
        out.addAll(_targetsFromSteps(s['steps'] as List?));
        continue;
      }
      final f = _formatTarget((s['target'] as Map?)?.cast<String, dynamic>());
      if (f != null) out.add(f);
    }
    return out;
  }

  String? _formatTarget(Map<String, dynamic>? t) {
    if (t == null) return null;
    final lo = t['low'] as int?;
    final hi = t['high'] as int?;
    final body = switch (t['kind'] as String?) {
      'power_w' => _range(lo, hi, 'W'),
      'hr_bpm' => _range(lo, hi, 'bpm'),
      'cadence' => _range(lo, hi, 'rpm'),
      'rpe' => _range(lo, hi, 'RPE'),
      'power_zone' => 'power Z${_zoneRange(lo, hi)}',
      'hr_zone' => 'HR Z${_zoneRange(lo, hi)}',
      'pace' => _pace(t['low_sec_per_km'] as int?, t['high_sec_per_km'] as int?, '/km'),
      'swim_pace' =>
        _pace(t['low_sec_per_100m'] as int?, t['high_sec_per_100m'] as int?, '/100m'),
      _ => null,
    };
    if (body == null) return null;
    final origin = t['origin'] as String?;
    return (origin != null && origin.isNotEmpty) ? '$origin · $body' : body;
  }

  String? _range(int? lo, int? hi, String unit) {
    if (lo == null && hi == null) return null;
    if (lo != null && hi != null && lo != hi) return '$lo–$hi $unit';
    return '${hi ?? lo} $unit';
  }

  String _zoneRange(int? lo, int? hi) =>
      (lo != null && hi != null && lo != hi) ? '$lo–$hi' : '${hi ?? lo}';

  String? _pace(int? lo, int? hi, String suffix) {
    String fmt(int s) => '${s ~/ 60}:${(s % 60).toString().padLeft(2, '0')}';
    if (lo == null && hi == null) return null;
    if (lo != null && hi != null && lo != hi) return '${fmt(lo)}–${fmt(hi)}$suffix';
    return '${fmt(hi ?? lo!)}$suffix';
  }

  Future<SessionFuel?> _sessionFuel(String workoutId) async {
    try {
      final resp = await api.dio.get<Map<String, dynamic>>(
        '/race-prep/recommend-workout-fuel',
        queryParameters: {'workout_id': workoutId},
      );
      return _fuelFromJson(resp.data ?? const {});
    } catch (_) {
      return null;
    }
  }

  SessionFuel _fuelFromJson(Map<String, dynamic> j) {
    final pre = <String>[], intra = <String>[], post = <String>[];

    final p = (j['pre_workout'] as Map?)?.cast<String, dynamic>();
    if (p != null) {
      final carbs = (p['carbs_g'] as num?)?.round();
      final w = (p['window_minutes_before'] as List?)?.cast<num>();
      if (carbs != null) {
        final when = (w != null && w.length == 2) ? '${w[0]}–${w[1]} min before' : 'before';
        pre.add('$carbs g carb · $when');
      }
    }

    final i = (j['intra_workout'] as Map?)?.cast<String, dynamic>();
    if (i != null && (i['applicable'] as bool? ?? false)) {
      final parts = <String>[];
      final c = (i['carbs_g_per_hour'] as num?)?.round();
      final na = (i['sodium_mg_per_hour'] as num?)?.round();
      final fl = (i['fluid_ml_per_hour'] as num?)?.round();
      if (c != null) parts.add('$c g/h carb');
      if (na != null) parts.add('$na mg/h Na');
      if (fl != null) parts.add('$fl ml/h fluid');
      if (parts.isNotEmpty) intra.add(parts.join(' · '));
    }

    final po = (j['post_workout'] as Map?)?.cast<String, dynamic>();
    if (po != null) {
      final parts = <String>[];
      final c = (po['carbs_g'] as num?)?.round();
      final pr = (po['protein_g'] as num?)?.round();
      final w = (po['window_minutes_after'] as List?)?.cast<num>();
      if (c != null) parts.add('$c g carb');
      if (pr != null) parts.add('$pr g protein');
      if (parts.isNotEmpty) {
        final when = (w != null && w.length == 2) ? ' · within ${w[1]} min' : '';
        post.add('${parts.join(' · ')}$when');
      }
    }

    return SessionFuel(
      pre: pre,
      intra: intra,
      post: post,
      notes: ((j['notes'] as List?) ?? const []).cast<String>(),
    );
  }

  @override
  Future<HydrationDaily> fetchHydrationDaily(String date) async {
    final resp = await api.dio.get<Map<String, dynamic>>(
      '/summary/hydration/daily',
      queryParameters: {'date': date},
    );
    return HydrationDaily.fromJson(resp.data ?? const {});
  }

  @override
  Future<Goals?> fetchGoals() async {
    final resp = await api.dio.get<Map<String, dynamic>>('/goals');
    final goals = resp.data?['goals'];
    if (goals == null) return null;
    return Goals.fromJson((goals as Map).cast<String, dynamic>());
  }

  @override
  Future<Product?> cachedProduct(String id) async {
    final row = await db.productsCacheDao.getById(id);
    if (row == null) return null;
    return _rowToProduct(row);
  }

  Product _rowToProduct(ProductsCacheData row) => Product.fromJson({
        'id': row.id,
        'name': row.name,
        'brand': row.brand,
        'source': row.source,
        'nutriments_per_100g': jsonDecode(row.nutrimentsPer100gJson),
        'serving_size_g': row.servingSizeG,
        'last_logged_quantity_g': row.lastLoggedQuantityG,
        'last_logged_at': row.lastLoggedAt?.toUtc().toIso8601String(),
      });

  @override
  Future<List<Product>> searchProducts(String q) async {
    final resp = await api.dio.get<Map<String, dynamic>>(
      '/products/search',
      queryParameters: {'q': q},
    );
    final results = (resp.data?['results'] as List?) ?? const [];
    final products = <Product>[];
    for (final r in results) {
      final json = (r as Map).cast<String, dynamic>();
      await db.productsCacheDao.upsertFromApi(json);
      products.add(Product.fromJson(json));
    }
    return products;
  }

  @override
  Future<List<Product>> recentProducts({int limit = 50, int offset = 0}) async {
    final resp = await api.dio.get<Map<String, dynamic>>(
      '/products',
      queryParameters: {'limit': limit, 'offset': offset},
    );
    final rows = (resp.data?['products'] as List?) ?? const [];
    final products = <Product>[];
    for (final r in rows) {
      final json = (r as Map).cast<String, dynamic>();
      await db.productsCacheDao.upsertFromApi(json);
      products.add(Product.fromJson(json));
    }
    return products;
  }

  @override
  Future<List<Product>> cachedRecentProducts(int limit) async {
    final rows = await db.productsCacheDao.recentlyUsed(limit);
    return rows.map(_rowToProduct).toList();
  }

  @override
  Future<List<Product>> cachedSearchProducts(String q) async {
    final rows = await db.productsCacheDao.searchCached(q);
    return rows.map(_rowToProduct).toList();
  }

  @override
  Future<Product> lookupProduct(String barcode) async {
    final resp = await api.dio.post<Map<String, dynamic>>(
      '/products/lookup/$barcode',
      options: Options(validateStatus: (_) => true),
    );
    if (resp.statusCode == 404) throw ProductNotFound(barcode);
    if (resp.statusCode == null ||
        resp.statusCode! < 200 ||
        resp.statusCode! >= 300) {
      throw DioException(
        requestOptions: resp.requestOptions,
        response: resp,
        message: 'lookup failed: ${resp.statusCode}',
      );
    }
    final product = (resp.data?['product'] as Map?)?.cast<String, dynamic>() ??
        resp.data ??
        const {};
    await db.productsCacheDao.upsertFromApi(product);
    return Product.fromJson(product);
  }

  @override
  Future<PhotoMealResult> logMealFromPhoto({
    required Uint8List jpegBytes,
    required double quantityG,
    required String mealType,
    required DateTime loggedAt,
  }) async {
    final form = FormData.fromMap({
      'image': MultipartFile.fromBytes(jpegBytes, filename: 'meal.jpg'),
      'quantity_g': quantityG,
      'meal_type': mealType,
      'logged_at': loggedAt.toUtc().toIso8601String(),
    });
    final resp = await api.dio.post<Map<String, dynamic>>(
      '/meals/from_photo',
      data: form,
      options: Options(
        validateStatus: (_) => true,
        headers: {'Idempotency-Key': newIdempotencyKey()},
      ),
    );
    if (resp.statusCode == 503) throw VisionUnavailable();
    if (resp.statusCode == null ||
        resp.statusCode! < 200 ||
        resp.statusCode! >= 300) {
      throw DioException(
        requestOptions: resp.requestOptions,
        response: resp,
        message: 'from_photo failed: ${resp.statusCode}',
      );
    }
    return PhotoMealResult.fromJson(resp.data ?? const {});
  }

  @override
  Future<void> enqueueMeal({
    required String productId,
    required double quantityG,
    required String mealType,
    required DateTime loggedAt,
  }) {
    return _enqueue('POST', '/meals', {
      'product_id': productId,
      'quantity_g': quantityG,
      'meal_type': mealType,
      'logged_at': loggedAt.toUtc().toIso8601String(),
    });
  }

  @override
  Future<void> enqueueFreeformMeal({
    required String name,
    required double quantityG,
    required String mealType,
    required DateTime loggedAt,
    double? kcal,
    double? proteinG,
    double? carbsG,
    double? fatG,
    Map<String, double>? micros,
    bool saveAsProduct = false,
  }) {
    return _enqueue('POST', '/meals/freeform', {
      'name': name,
      'quantity_g': quantityG,
      'meal_type': mealType,
      'logged_at': loggedAt.toUtc().toIso8601String(),
      'nutriments_per_100g': {
        'kcal': ?kcal,
        'protein_g': ?proteinG,
        'carbs_g': ?carbsG,
        'fat_g': ?fatG,
        ...?micros,
      },
      // Quick-create logs the meal AND persists a reusable product server-side
      // (one idempotent call, replays through the outbox). Omitted when false
      // so the plain "describe it" escape hatch stays a pure freeform log.
      if (saveAsProduct) 'save_as_product': true,
    });
  }

  @override
  Future<void> enqueuePatchMeal(String id, {double? quantityG, String? mealType}) {
    return _enqueue('PATCH', '/meals/$id', {
      'quantity_g': ?quantityG,
      'meal_type': ?mealType,
    });
  }

  @override
  Future<void> enqueueDeleteMeal(String id) =>
      _enqueue('DELETE', '/meals/$id', null);

  @override
  Future<void> enqueueHydration({required double quantityMl, DateTime? loggedAt}) {
    return _enqueue('POST', '/hydration', {
      'quantity_ml': quantityMl,
      'logged_at': (loggedAt ?? DateTime.now()).toUtc().toIso8601String(),
    });
  }

  @override
  Future<void> enqueueDeleteHydration(String id) =>
      _enqueue('DELETE', '/hydration/$id', null);

  @override
  Future<void> flush() => outbox.drain();

  // --- Planning surfaces ----------------------------------------------------

  @override
  Future<List<PlannedMeal>> cachedPlan(String date) async {
    final rows = await db.planCacheDao.forDate(date);
    return rows
        .map((r) => PlannedMeal(
              id: r.id,
              planDate: r.planDate,
              slot: r.slot,
              status: r.status,
              productId: r.productId,
              productName: r.productName,
              quantityG: r.quantityG,
            ))
        .toList();
  }

  @override
  Future<List<PlannedMeal>> fetchPlan(String date) async {
    final resp = await api.dio.get<Map<String, dynamic>>(
      '/plan',
      queryParameters: {'from': date, 'to': date},
    );
    final list = ((resp.data?['planned_meals'] as List?) ?? const [])
        .map((e) => PlannedMeal.fromJson((e as Map).cast<String, dynamic>()))
        .toList();
    await db.planCacheDao.replaceForDate(date, [
      for (final p in list)
        PlanCacheCompanion.insert(
          id: p.id,
          planDate: p.planDate,
          slot: p.slot,
          status: p.status,
          productId: Value(p.productId),
          productName: Value(p.productName),
          quantityG: Value(p.quantityG),
          refreshedAt: DateTime.now(),
        ),
    ]);
    return list;
  }

  @override
  Future<List<ShoppingItem>> cachedShopping() async {
    final rows = await db.shoppingCacheDao.all();
    return rows
        .map((r) => ShoppingItem(
              id: r.id,
              name: r.name,
              checked: r.checked,
              quantityText: r.quantityText,
            ))
        .toList();
  }

  @override
  Future<List<ShoppingItem>> fetchShopping() async {
    final resp = await api.dio.get<Map<String, dynamic>>(
      '/shopping/items',
      queryParameters: {'include_checked': 'true'},
    );
    final list = ((resp.data?['items'] as List?) ?? const [])
        .map((e) => ShoppingItem.fromJson((e as Map).cast<String, dynamic>()))
        .toList();
    await db.shoppingCacheDao.replaceAll([
      for (var i = 0; i < list.length; i++)
        ShoppingCacheCompanion.insert(
          id: list[i].id,
          name: list[i].name,
          quantityText: Value(list[i].quantityText),
          checked: Value(list[i].checked),
          seq: i,
          refreshedAt: DateTime.now(),
        ),
    ]);
    return list;
  }

  @override
  Future<void> enqueueMarkEaten(String planId) =>
      _enqueue('POST', '/plan/$planId/eaten', {});

  @override
  Future<void> enqueuePlanStatus(String planId, String status) =>
      _enqueue('PATCH', '/plan/$planId', {'status': status});

  @override
  Future<void> enqueueShoppingChecked(String itemId, bool checked) =>
      _enqueue('PATCH', '/shopping/items/$itemId', {'checked': checked});

  @override
  Future<void> enqueueAddShoppingItem(String name) => _enqueue(
        'POST',
        '/shopping/items',
        {
          'items': [
            {'name': name},
          ],
        },
      );

  @override
  Future<void> enqueueClearCheckedShopping() =>
      _enqueue('DELETE', '/shopping/items?checked=true', null);

  Future<void> _enqueue(
    String method,
    String path,
    Map<String, dynamic>? body,
  ) async {
    final bytes = body == null
        ? Uint8List(0)
        : Uint8List.fromList(utf8.encode(jsonEncode(body)));
    await db.pendingWritesDao.enqueue(
      id: newIdempotencyKey(),
      method: method,
      path: path,
      body: bytes,
      idemKey: newIdempotencyKey(),
    );
    unawaited(outbox.drain());
  }
}
