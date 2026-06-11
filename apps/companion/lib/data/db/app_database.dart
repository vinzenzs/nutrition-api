import 'dart:io';

import 'package:drift/drift.dart';
import 'package:drift/native.dart';
import 'package:path_provider/path_provider.dart';

import 'dao/pending_writes_dao.dart';
import 'dao/products_cache_dao.dart';
import 'dao/recent_summary_dao.dart';
import 'dao/widget_failures_dao.dart';

part 'app_database.g.dart';

class ProductsCache extends Table {
  TextColumn get id => text()();
  TextColumn get name => text()();
  TextColumn get brand => text().nullable()();
  TextColumn get source => text()();
  TextColumn get nutrimentsPer100gJson => text()();
  RealColumn get servingSizeG => real().nullable()();
  RealColumn get lastLoggedQuantityG => real().nullable()();
  DateTimeColumn get refreshedAt => dateTime()();

  @override
  Set<Column> get primaryKey => {id};
}

class RecentSummary extends Table {
  TextColumn get date => text()();
  TextColumn get tz => text()();
  TextColumn get totalsJson => text()();
  TextColumn get entriesJson => text()();
  DateTimeColumn get refreshedAt => dateTime()();

  @override
  Set<Column> get primaryKey => {date, tz};
}

class PendingWrites extends Table {
  TextColumn get id => text()();
  TextColumn get method => text()();
  TextColumn get path => text()();
  BlobColumn get body => blob()();
  TextColumn get idemKey => text()();
  DateTimeColumn get createdAt => dateTime()();
  TextColumn get status => text().withDefault(const Constant('pending'))();
  DateTimeColumn get lastAttemptAt => dateTime().nullable()();
  IntColumn get attemptCount => integer().withDefault(const Constant(0))();
  TextColumn get lastError => text().nullable()();

  @override
  Set<Column> get primaryKey => {id};
}

class WidgetFailures extends Table {
  TextColumn get id => text()();
  BlobColumn get body => blob()();
  TextColumn get idemKey => text()();
  DateTimeColumn get createdAt => dateTime()();

  @override
  Set<Column> get primaryKey => {id};
}

@DriftDatabase(
  tables: [ProductsCache, RecentSummary, PendingWrites, WidgetFailures],
  daos: [
    ProductsCacheDao,
    RecentSummaryDao,
    PendingWritesDao,
    WidgetFailuresDao,
  ],
)
class AppDatabase extends _$AppDatabase {
  AppDatabase() : super(_openConnection());

  AppDatabase.forTesting(super.executor);

  @override
  int get schemaVersion => 1;

  @override
  MigrationStrategy get migration => MigrationStrategy(
        onCreate: (m) => m.createAll(),
        onUpgrade: (m, from, to) async {
          // v2 hook reserved — append branches as the schema evolves.
        },
      );

  static QueryExecutor _openConnection() {
    return LazyDatabase(() async {
      final file = File(await resolveDbPath());
      return NativeDatabase.createInBackground(file);
    });
  }

  /// Absolute path to the SQLite file. Shared with the Kotlin widget worker
  /// (via the widget bridge) so its `widget_failures` spillover targets the
  /// same database the app drains.
  static Future<String> resolveDbPath() async {
    final dir = await getApplicationDocumentsDirectory();
    return '${dir.path}/companion.sqlite';
  }
}
