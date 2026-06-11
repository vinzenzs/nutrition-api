import 'package:drift/drift.dart';

import '../app_database.dart';

part 'shopping_cache_dao.g.dart';

@DriftAccessor(tables: [ShoppingCache])
class ShoppingCacheDao extends DatabaseAccessor<AppDatabase>
    with _$ShoppingCacheDaoMixin {
  ShoppingCacheDao(super.db);

  /// Replace the whole cached shopping list (stale-while-revalidate).
  Future<void> replaceAll(List<ShoppingCacheCompanion> items) {
    return transaction(() async {
      await delete(shoppingCache).go();
      for (final c in items) {
        await into(shoppingCache).insert(c);
      }
    });
  }

  /// All items, unchecked first then checked, each group in insertion order.
  Future<List<ShoppingCacheData>> all() {
    return (select(shoppingCache)
          ..orderBy([
            (s) => OrderingTerm.asc(s.checked),
            (s) => OrderingTerm.asc(s.seq),
          ]))
        .get();
  }
}
