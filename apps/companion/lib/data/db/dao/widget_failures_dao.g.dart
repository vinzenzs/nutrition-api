// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'widget_failures_dao.dart';

// ignore_for_file: type=lint
mixin _$WidgetFailuresDaoMixin on DatabaseAccessor<AppDatabase> {
  $WidgetFailuresTable get widgetFailures => attachedDatabase.widgetFailures;
  WidgetFailuresDaoManager get managers => WidgetFailuresDaoManager(this);
}

class WidgetFailuresDaoManager {
  final _$WidgetFailuresDaoMixin _db;
  WidgetFailuresDaoManager(this._db);
  $$WidgetFailuresTableTableManager get widgetFailures =>
      $$WidgetFailuresTableTableManager(
        _db.attachedDatabase,
        _db.widgetFailures,
      );
}
