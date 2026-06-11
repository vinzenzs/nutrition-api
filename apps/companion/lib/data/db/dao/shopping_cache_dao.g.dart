// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'shopping_cache_dao.dart';

// ignore_for_file: type=lint
mixin _$ShoppingCacheDaoMixin on DatabaseAccessor<AppDatabase> {
  $ShoppingCacheTable get shoppingCache => attachedDatabase.shoppingCache;
  ShoppingCacheDaoManager get managers => ShoppingCacheDaoManager(this);
}

class ShoppingCacheDaoManager {
  final _$ShoppingCacheDaoMixin _db;
  ShoppingCacheDaoManager(this._db);
  $$ShoppingCacheTableTableManager get shoppingCache =>
      $$ShoppingCacheTableTableManager(_db.attachedDatabase, _db.shoppingCache);
}
