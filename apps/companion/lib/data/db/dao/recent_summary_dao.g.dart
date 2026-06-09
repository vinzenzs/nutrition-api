// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'recent_summary_dao.dart';

// ignore_for_file: type=lint
mixin _$RecentSummaryDaoMixin on DatabaseAccessor<AppDatabase> {
  $RecentSummaryTable get recentSummary => attachedDatabase.recentSummary;
  RecentSummaryDaoManager get managers => RecentSummaryDaoManager(this);
}

class RecentSummaryDaoManager {
  final _$RecentSummaryDaoMixin _db;
  RecentSummaryDaoManager(this._db);
  $$RecentSummaryTableTableManager get recentSummary =>
      $$RecentSummaryTableTableManager(_db.attachedDatabase, _db.recentSummary);
}
