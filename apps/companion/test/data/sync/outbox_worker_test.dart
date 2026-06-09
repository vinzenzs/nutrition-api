import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:drift/native.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:nutrition_companion/data/db/app_database.dart';
import 'package:nutrition_companion/data/db/dao/pending_writes_dao.dart';
import 'package:nutrition_companion/data/db/dao/widget_failures_dao.dart';
import 'package:nutrition_companion/data/sync/outbox_worker.dart';

/// In-memory sender — returns a canned response per `path` and records how
/// many times each row id was sent.
class _StubSender implements OutboxSender {
  final Map<String, Response<dynamic> Function(int callIndex)> responders;
  final List<String> sentIds = [];

  _StubSender(this.responders);

  @override
  Future<Response<dynamic>> send({
    required String method,
    required String path,
    required List<int> body,
    required String idempotencyKey,
  }) async {
    sentIds.add(idempotencyKey);
    final responder = responders[path];
    if (responder == null) {
      throw StateError('no responder configured for path $path');
    }
    final callIndex = sentIds.where((k) => k == idempotencyKey).length - 1;
    return responder(callIndex);
  }
}

Response<dynamic> _resp(int status, [Map<String, dynamic>? body]) =>
    Response(
      requestOptions: RequestOptions(path: ''),
      statusCode: status,
      data: body ?? <String, dynamic>{},
    );

void main() {
  late AppDatabase db;
  late PendingWritesDao pending;
  late WidgetFailuresDao widgetFailures;

  setUp(() {
    db = AppDatabase.forTesting(NativeDatabase.memory());
    pending = db.pendingWritesDao;
    widgetFailures = db.widgetFailuresDao;
  });

  tearDown(() => db.close());

  Future<void> enqueueRow({
    required String id,
    required String path,
    String method = 'POST',
  }) {
    return pending.enqueue(
      id: id,
      method: method,
      path: path,
      body: Uint8List.fromList([0x7b, 0x7d]),
      idemKey: id,
    );
  }

  test('2xx response marks the row done', () async {
    await enqueueRow(id: 'row-1', path: '/meals');
    final sender = _StubSender({'/meals': (_) => _resp(201)});
    final worker = OutboxWorker(
      pending: pending,
      widgetFailures: widgetFailures,
      sender: sender,
    );

    final outcome = await worker.drain();
    expect(outcome, DrainOutcome.drained);

    final after = await (db.select(db.pendingWrites)
          ..where((r) => r.id.equals('row-1')))
        .getSingle();
    expect(after.status, 'done');
  });

  test('4xx (non-409) response marks failed_permanent', () async {
    await enqueueRow(id: 'row-bad', path: '/meals');
    final sender = _StubSender({
      '/meals': (_) => _resp(400, {'error': 'meal_type_invalid'})
    });
    final worker = OutboxWorker(
      pending: pending,
      widgetFailures: widgetFailures,
      sender: sender,
    );

    await worker.drain();

    final after = await (db.select(db.pendingWrites)
          ..where((r) => r.id.equals('row-bad')))
        .getSingle();
    expect(after.status, 'failed_permanent');
    expect(after.lastError, contains('400'));
  });

  test('5xx leaves the row pending and increments attempt_count', () async {
    await enqueueRow(id: 'row-flaky', path: '/meals');
    final sender = _StubSender({
      '/meals': (_) => _resp(503, {'error': 'unavailable'})
    });
    final worker = OutboxWorker(
      pending: pending,
      widgetFailures: widgetFailures,
      sender: sender,
    );

    await worker.drain();
    await worker.drain();

    final after = await (db.select(db.pendingWrites)
          ..where((r) => r.id.equals('row-flaky')))
        .getSingle();
    expect(after.status, 'pending');
    expect(after.attemptCount, 2);
  });

  test('cached replay: a transient failure followed by 2xx resolves to done',
      () async {
    await enqueueRow(id: 'row-replay', path: '/meals');
    final sender = _StubSender({
      '/meals': (call) => call == 0 ? _resp(503) : _resp(201),
    });
    final worker = OutboxWorker(
      pending: pending,
      widgetFailures: widgetFailures,
      sender: sender,
    );

    await worker.drain();
    await worker.drain();

    final after = await (db.select(db.pendingWrites)
          ..where((r) => r.id.equals('row-replay')))
        .getSingle();
    expect(after.status, 'done');
    // The idempotency key is the same across both attempts, so the server
    // would safely deduplicate. We assert the client kept it stable.
    expect(sender.sentIds, ['row-replay', 'row-replay']);
  });

  test('concurrent drain calls are coalesced via the in-flight guard',
      () async {
    await enqueueRow(id: 'row-coalesce', path: '/meals');
    var calls = 0;
    final sender = _StubSender({'/meals': (_) {
      calls++;
      return _resp(201);
    }});
    final worker = OutboxWorker(
      pending: pending,
      widgetFailures: widgetFailures,
      sender: sender,
    );

    final f1 = worker.drain();
    final f2 = worker.drain();
    final r1 = await f1;
    final r2 = await f2;

    expect({r1, r2}, contains(DrainOutcome.alreadyRunning));
    expect(calls, 1);
  });

  test('widget_failures are drained into pending_writes first', () async {
    final body = Uint8List.fromList([0x7b, 0x7d]);
    await db.into(db.widgetFailures).insert(WidgetFailuresCompanion.insert(
          id: 'wf-1',
          body: body,
          idemKey: 'wf-1',
          createdAt: DateTime.now(),
        ));

    final sender = _StubSender({'/hydration': (_) => _resp(201)});
    final worker = OutboxWorker(
      pending: pending,
      widgetFailures: widgetFailures,
      sender: sender,
    );

    await worker.drain();

    final remaining = await db.select(db.widgetFailures).get();
    expect(remaining, isEmpty);

    final after = await (db.select(db.pendingWrites)
          ..where((r) => r.id.equals('wf-1')))
        .getSingle();
    expect(after.status, 'done');
    expect(after.path, '/hydration');
  });
}
