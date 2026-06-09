import 'package:drift/drift.dart';

import '../app_database.dart';

part 'pending_writes_dao.g.dart';

@DriftAccessor(tables: [PendingWrites])
class PendingWritesDao extends DatabaseAccessor<AppDatabase>
    with _$PendingWritesDaoMixin {
  PendingWritesDao(super.db);

  Future<void> enqueue({
    required String id,
    required String method,
    required String path,
    required Uint8List body,
    required String idemKey,
  }) {
    return into(pendingWrites).insert(
      PendingWritesCompanion.insert(
        id: id,
        method: method,
        path: path,
        body: body,
        idemKey: idemKey,
        createdAt: DateTime.now(),
      ),
    );
  }

  Future<List<PendingWrite>> pendingInArrivalOrder() {
    return (select(pendingWrites)
          ..where((row) => row.status.equals('pending') |
              row.status.equals('in_flight'))
          ..orderBy([(row) => OrderingTerm.asc(row.createdAt)]))
        .get();
  }

  Future<int> markInFlight(String id) {
    return (update(pendingWrites)..where((row) => row.id.equals(id))).write(
      PendingWritesCompanion(
        status: const Value('in_flight'),
        lastAttemptAt: Value(DateTime.now()),
        attemptCount: const Value.absent(),
      ),
    );
  }

  Future<int> markDone(String id) {
    return (update(pendingWrites)..where((row) => row.id.equals(id))).write(
      const PendingWritesCompanion(status: Value('done')),
    );
  }

  Future<int> markFailedPermanent(String id, String error) {
    return (update(pendingWrites)..where((row) => row.id.equals(id))).write(
      PendingWritesCompanion(
        status: const Value('failed_permanent'),
        lastError: Value(error),
      ),
    );
  }

  // Used by the worker to schedule the next retry — bumps attempt_count and
  // sets lastError without changing status (stays 'pending').
  Future<int> recordTransientFailure(String id, String error) {
    return customUpdate(
      'UPDATE pending_writes SET status = ?, attempt_count = attempt_count + 1, '
      'last_attempt_at = ?, last_error = ? WHERE id = ?',
      variables: [
        Variable.withString('pending'),
        Variable.withDateTime(DateTime.now()),
        Variable.withString(error),
        Variable.withString(id),
      ],
      updates: {pendingWrites},
    );
  }
}
