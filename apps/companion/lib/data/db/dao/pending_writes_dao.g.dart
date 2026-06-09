// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'pending_writes_dao.dart';

// ignore_for_file: type=lint
mixin _$PendingWritesDaoMixin on DatabaseAccessor<AppDatabase> {
  $PendingWritesTable get pendingWrites => attachedDatabase.pendingWrites;
  PendingWritesDaoManager get managers => PendingWritesDaoManager(this);
}

class PendingWritesDaoManager {
  final _$PendingWritesDaoMixin _db;
  PendingWritesDaoManager(this._db);
  $$PendingWritesTableTableManager get pendingWrites =>
      $$PendingWritesTableTableManager(_db.attachedDatabase, _db.pendingWrites);
}
