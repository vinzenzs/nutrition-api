// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'products_cache_dao.dart';

// ignore_for_file: type=lint
mixin _$ProductsCacheDaoMixin on DatabaseAccessor<AppDatabase> {
  $ProductsCacheTable get productsCache => attachedDatabase.productsCache;
  ProductsCacheDaoManager get managers => ProductsCacheDaoManager(this);
}

class ProductsCacheDaoManager {
  final _$ProductsCacheDaoMixin _db;
  ProductsCacheDaoManager(this._db);
  $$ProductsCacheTableTableManager get productsCache =>
      $$ProductsCacheTableTableManager(_db.attachedDatabase, _db.productsCache);
}
