import 'package:drift/native.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:kazper/data/db/app_database.dart';

void main() {
  late AppDatabase db;

  setUp(() => db = AppDatabase.forTesting(NativeDatabase.memory()));
  tearDown(() => db.close());

  Future<void> seed(String id, String name,
      {String? brand, String? lastLoggedAt}) {
    return db.productsCacheDao.upsertFromApi({
      'id': id,
      'name': name,
      'brand': brand,
      'source': 'manual',
      'nutriments_per_100g': {'kcal': 100},
      'last_logged_at': lastLoggedAt,
    });
  }

  test('recentlyUsed orders by last_logged_at DESC NULLS LAST, then name',
      () async {
    await seed('a', 'Apple', lastLoggedAt: '2026-06-10T12:00:00Z');
    await seed('b', 'Banana', lastLoggedAt: '2026-06-01T12:00:00Z');
    await seed('c', 'Never logged', lastLoggedAt: null);
    await seed('d', 'Also never', lastLoggedAt: null);

    final rows = await db.productsCacheDao.recentlyUsed(50);
    expect(rows.map((r) => r.id).toList(), ['a', 'b', 'd', 'c']);
    // 'a' (most recent) first, 'b' next, then the two nulls by name ASC
    // ('Also never' before 'Never logged').
  });

  test('searchCached matches name/brand case-insensitively, recency-first',
      () async {
    await seed('g1', 'Homemade granola', lastLoggedAt: '2026-06-10T12:00:00Z');
    await seed('g2', 'Granny Smith apple', lastLoggedAt: null);
    await seed('x', 'Banana', brand: 'Chiquita');

    final gran = await db.productsCacheDao.searchCached('GRAN');
    expect(gran.map((r) => r.id).toList(), ['g1', 'g2']);

    final byBrand = await db.productsCacheDao.searchCached('chiq');
    expect(byBrand.single.id, 'x');

    final none = await db.productsCacheDao.searchCached('zzz');
    expect(none, isEmpty);
  });
}
