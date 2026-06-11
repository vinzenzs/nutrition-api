import 'package:drift/drift.dart';

import '../app_database.dart';

part 'plan_cache_dao.g.dart';

@DriftAccessor(tables: [PlanCache])
class PlanCacheDao extends DatabaseAccessor<AppDatabase>
    with _$PlanCacheDaoMixin {
  PlanCacheDao(super.db);

  /// Replace the cached plan for [date] wholesale (stale-while-revalidate).
  Future<void> replaceForDate(String date, List<PlanCacheCompanion> items) {
    return transaction(() async {
      await (delete(planCache)..where((p) => p.planDate.equals(date))).go();
      for (final c in items) {
        await into(planCache).insert(c);
      }
    });
  }

  Future<List<PlanCacheData>> forDate(String date) {
    return (select(planCache)
          ..where((p) => p.planDate.equals(date))
          ..orderBy([(p) => OrderingTerm.asc(p.slot)]))
        .get();
  }
}
