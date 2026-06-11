import 'dart:io';

import 'package:drift/drift.dart';
import 'package:drift/native.dart';
import 'package:path_provider/path_provider.dart';

import 'dao/chat_messages_dao.dart';
import 'dao/pending_writes_dao.dart';
import 'dao/plan_cache_dao.dart';
import 'dao/products_cache_dao.dart';
import 'dao/recent_summary_dao.dart';
import 'dao/shopping_cache_dao.dart';
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
  // Recency-of-use, mirrored from the backend's products.last_logged_at, so the
  // food picker can order "previously-used foods" most-recently-used first even
  // offline. Null until the food has been logged at least once.
  DateTimeColumn get lastLoggedAt => dateTime().nullable()();
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

/// Chat transcript, held client-side because the backend is stateless. One
/// active conversation at a time; old conversations are retained for scrollback
/// (capped at 20 by the DAO on insert). `role` is user | assistant.
class ChatMessages extends Table {
  TextColumn get id => text()();
  TextColumn get conversationId => text()();
  TextColumn get role => text()();
  TextColumn get content => text()();
  DateTimeColumn get createdAt => dateTime()();

  @override
  Set<Column> get primaryKey => {id};
}

/// Stale-while-revalidate cache of today's planned meals (mirrors the backend
/// /plan rows). Replaced wholesale per date on each fetch.
class PlanCache extends Table {
  TextColumn get id => text()();
  TextColumn get planDate => text()();
  TextColumn get slot => text()();
  TextColumn get productId => text().nullable()();
  TextColumn get productName => text().nullable()();
  RealColumn get quantityG => real().nullable()();
  TextColumn get status => text()();
  DateTimeColumn get refreshedAt => dateTime()();

  @override
  Set<Column> get primaryKey => {id};
}

/// Stale-while-revalidate cache of the shopping list (mirrors backend
/// /shopping/items). Replaced wholesale on each fetch.
class ShoppingCache extends Table {
  TextColumn get id => text()();
  TextColumn get name => text()();
  TextColumn get quantityText => text().nullable()();
  BoolColumn get checked => boolean().withDefault(const Constant(false))();
  IntColumn get seq => integer()();
  DateTimeColumn get refreshedAt => dateTime()();

  @override
  Set<Column> get primaryKey => {id};
}

@DriftDatabase(
  tables: [
    ProductsCache,
    RecentSummary,
    PendingWrites,
    WidgetFailures,
    ChatMessages,
    PlanCache,
    ShoppingCache,
  ],
  daos: [
    ProductsCacheDao,
    RecentSummaryDao,
    PendingWritesDao,
    WidgetFailuresDao,
    ChatMessagesDao,
    PlanCacheDao,
    ShoppingCacheDao,
  ],
)
class AppDatabase extends _$AppDatabase {
  AppDatabase() : super(_openConnection());

  AppDatabase.forTesting(super.executor);

  @override
  int get schemaVersion => 3;

  @override
  MigrationStrategy get migration => MigrationStrategy(
        onCreate: (m) => m.createAll(),
        onUpgrade: (m, from, to) async {
          // v2: products_cache gains last_logged_at for recency-of-use ordering
          // in the food picker. Guarded by a column-existence check because a
          // dev DB can reach v1 with the column already present (out-of-band
          // table-def change), which makes a bare addColumn fail with
          // "duplicate column name". products_cache is a regenerable cache.
          if (from < 2) {
            if (!await _columnExists('products_cache', 'last_logged_at')) {
              await m.addColumn(productsCache, productsCache.lastLoggedAt);
            }
          }
          // v3: chat transcript + plan/shopping caches for the companion-chat
          // surfaces. createTable is idempotent-guarded for the same dev-DB
          // reason as above.
          if (from < 3) {
            if (!await _tableExists('chat_messages')) {
              await m.createTable(chatMessages);
            }
            if (!await _tableExists('plan_cache')) {
              await m.createTable(planCache);
            }
            if (!await _tableExists('shopping_cache')) {
              await m.createTable(shoppingCache);
            }
          }
        },
      );

  Future<bool> _tableExists(String table) async {
    final rows = await customSelect(
      "SELECT name FROM sqlite_master WHERE type='table' AND name=?",
      variables: [Variable.withString(table)],
    ).get();
    return rows.isNotEmpty;
  }

  /// Whether [column] already exists on [table] (via PRAGMA table_info).
  Future<bool> _columnExists(String table, String column) async {
    final rows = await customSelect("PRAGMA table_info('$table')").get();
    return rows.any((r) => r.read<String>('name') == column);
  }

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
