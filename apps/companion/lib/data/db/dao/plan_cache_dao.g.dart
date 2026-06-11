// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'plan_cache_dao.dart';

// ignore_for_file: type=lint
mixin _$PlanCacheDaoMixin on DatabaseAccessor<AppDatabase> {
  $PlanCacheTable get planCache => attachedDatabase.planCache;
  PlanCacheDaoManager get managers => PlanCacheDaoManager(this);
}

class PlanCacheDaoManager {
  final _$PlanCacheDaoMixin _db;
  PlanCacheDaoManager(this._db);
  $$PlanCacheTableTableManager get planCache =>
      $$PlanCacheTableTableManager(_db.attachedDatabase, _db.planCache);
}
