import 'package:drift/drift.dart';

import '../app_database.dart';
import 'pending_writes_dao.dart';

part 'widget_failures_dao.g.dart';

@DriftAccessor(tables: [WidgetFailures])
class WidgetFailuresDao extends DatabaseAccessor<AppDatabase>
    with _$WidgetFailuresDaoMixin {
  WidgetFailuresDao(super.db);

  // Moves every queued widget failure into pending_writes (as POST /hydration
  // outbox entries) and clears the widget_failures table. The Kotlin worker
  // only knows how to write hydration logs, so the method is opinionated about
  // path and method.
  Future<int> drainInto(PendingWritesDao target) async {
    final rows = await select(widgetFailures).get();
    for (final row in rows) {
      await target.enqueue(
        id: row.id,
        method: 'POST',
        path: '/hydration',
        body: row.body,
        idemKey: row.idemKey,
      );
    }
    await delete(widgetFailures).go();
    return rows.length;
  }
}
