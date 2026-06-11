import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:drift/drift.dart' show Value;

import '../domain/models.dart';
import '../domain/planning.dart';
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
