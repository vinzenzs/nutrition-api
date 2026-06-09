import 'dart:convert';

import 'package:drift/drift.dart';

import '../app_database.dart';

part 'recent_summary_dao.g.dart';

class CachedDailySummary {
  final String date;
  final String tz;
  final Map<String, dynamic> totals;
  final List<Map<String, dynamic>> entries;
  final DateTime refreshedAt;

  CachedDailySummary({
    required this.date,
    required this.tz,
    required this.totals,
    required this.entries,
    required this.refreshedAt,
  });
}

@DriftAccessor(tables: [RecentSummary])
class RecentSummaryDao extends DatabaseAccessor<AppDatabase>
    with _$RecentSummaryDaoMixin {
  RecentSummaryDao(super.db);

  Future<void> upsertForDate({
    required String date,
    required String tz,
    required Map<String, dynamic> totals,
    required List<Map<String, dynamic>> entries,
  }) {
    return into(recentSummary).insertOnConflictUpdate(
      RecentSummaryCompanion.insert(
        date: date,
        tz: tz,
        totalsJson: jsonEncode(totals),
        entriesJson: jsonEncode(entries),
        refreshedAt: DateTime.now(),
      ),
    );
  }

  Future<CachedDailySummary?> getForDate({
    required String date,
    required String tz,
  }) async {
    final row = await (select(recentSummary)
          ..where((r) => r.date.equals(date) & r.tz.equals(tz)))
        .getSingleOrNull();
    if (row == null) return null;
    return CachedDailySummary(
      date: row.date,
      tz: row.tz,
      totals: jsonDecode(row.totalsJson) as Map<String, dynamic>,
      entries: (jsonDecode(row.entriesJson) as List)
          .cast<Map<String, dynamic>>(),
      refreshedAt: row.refreshedAt,
    );
  }
}
