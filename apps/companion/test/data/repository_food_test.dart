import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:drift/native.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';
import 'package:kazper/data/db/app_database.dart';
import 'package:kazper/data/net/api_client.dart';
import 'package:kazper/data/repository.dart';
import 'package:kazper/data/sync/outbox_worker.dart';

class _MockApiClient extends Mock implements ApiClient {}

class _MockDio extends Mock implements Dio {}

class _MockOutbox extends Mock implements OutboxWorker {}

Response<Map<String, dynamic>> _resp(String path, Map<String, dynamic> data) =>
    Response(requestOptions: RequestOptions(path: path), data: data);

void main() {
  late AppDatabase db;
  late _MockDio dio;
  late _MockApiClient api;
  late _MockOutbox outbox;
  late ApiRepository repo;

  setUp(() {
    db = AppDatabase.forTesting(NativeDatabase.memory());
    dio = _MockDio();
    api = _MockApiClient();
    outbox = _MockOutbox();
    when(() => api.dio).thenReturn(dio);
    when(() => outbox.drain()).thenAnswer((_) async => DrainOutcome.empty);
    repo = ApiRepository(db: db, api: api, outbox: outbox);
  });

  tearDown(() => db.close());

  test('searchProducts parses {results} and writes through the cache',
      () async {
    when(() => dio.get<Map<String, dynamic>>('/products/search',
        queryParameters: any(named: 'queryParameters'))).thenAnswer(
      (_) async => _resp('/products/search', {
        'results': [
          {
            'id': 'p1',
            'name': 'Yogurt',
            'source': 'manual',
            'nutriments_per_100g': {'kcal': 60},
            'last_logged_at': '2026-06-10T12:00:00Z',
          }
        ]
      }),
    );

    final results = await repo.searchProducts('yog');
    expect(results.single.name, 'Yogurt');
    // Write-through: it now lives in the local cache.
    final cached = await db.productsCacheDao.getById('p1');
    expect(cached, isNotNull);
    expect(cached!.lastLoggedAt, isNotNull);
  });

  test('recentProducts parses {products} and writes through the cache',
      () async {
    when(() => dio.get<Map<String, dynamic>>('/products',
        queryParameters: any(named: 'queryParameters'))).thenAnswer(
      (_) async => _resp('/products', {
        'products': [
          {
            'id': 'p2',
            'name': 'Banana',
            'source': 'off',
            'nutriments_per_100g': {'kcal': 89},
          }
        ],
        'total': 1,
        'limit': 50,
        'offset': 0,
      }),
    );

    final products = await repo.recentProducts();
    expect(products.single.name, 'Banana');
    expect(await db.productsCacheDao.getById('p2'), isNotNull);
  });

  test('enqueueFreeformMeal(saveAsProduct: true) writes save_as_product in the '
      'pending row body', () async {
    await repo.enqueueFreeformMeal(
      name: 'Homemade granola',
      quantityG: 60,
      mealType: 'breakfast',
      loggedAt: DateTime.utc(2026, 6, 11, 8),
      kcal: 420,
      saveAsProduct: true,
    );

    final rows = await db.select(db.pendingWrites).get();
    expect(rows, hasLength(1));
    expect(rows.single.path, '/meals/freeform');
    final body = jsonDecode(utf8.decode(rows.single.body)) as Map;
    expect(body['save_as_product'], true);
    expect(body['name'], 'Homemade granola');
  });

  test('enqueueFreeformMeal without saveAsProduct omits the flag', () async {
    await repo.enqueueFreeformMeal(
      name: 'banana',
      quantityG: 120,
      mealType: 'snack',
      loggedAt: DateTime.utc(2026, 6, 11, 8),
    );
    final rows = await db.select(db.pendingWrites).get();
    final body = jsonDecode(utf8.decode(rows.single.body)) as Map;
    expect(body.containsKey('save_as_product'), isFalse);
  });
}
