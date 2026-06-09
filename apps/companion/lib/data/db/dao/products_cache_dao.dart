import 'dart:convert';

import 'package:drift/drift.dart';

import '../app_database.dart';

part 'products_cache_dao.g.dart';

@DriftAccessor(tables: [ProductsCache])
class ProductsCacheDao extends DatabaseAccessor<AppDatabase>
    with _$ProductsCacheDaoMixin {
  ProductsCacheDao(super.db);

  Future<void> upsertFromApi(Map<String, dynamic> product) {
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
}
