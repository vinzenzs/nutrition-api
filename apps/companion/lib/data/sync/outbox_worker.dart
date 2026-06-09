import 'dart:async';

import 'package:dio/dio.dart';

import '../db/app_database.dart';
import '../db/dao/pending_writes_dao.dart';
import '../db/dao/widget_failures_dao.dart';
import '../net/api_client.dart';

enum DrainOutcome { drained, alreadyRunning, empty }

/// What the worker needs from a network layer — just enough to mock in tests.
abstract class OutboxSender {
  Future<Response<dynamic>> send({
    required String method,
    required String path,
    required List<int> body,
    required String idempotencyKey,
  });
}

/// Bridges [ApiClient] to the [OutboxSender] interface the worker takes.
class ApiClientSender implements OutboxSender {
  final ApiClient client;
  ApiClientSender(this.client);

  @override
  Future<Response<dynamic>> send({
    required String method,
    required String path,
    required List<int> body,
    required String idempotencyKey,
  }) =>
      client.send(
        method: method,
        path: path,
        body: body,
        idempotencyKey: idempotencyKey,
      );
}

/// Drains [PendingWritesDao] in arrival order, classifying each response:
///   - 2xx          → markDone
///   - 4xx (non-409)→ markFailedPermanent (a 409 means idempotency replay
///                    saw a body mismatch — this is a client-side bug, we
///                    still mark permanent so we don't loop)
///   - 5xx / network→ recordTransientFailure (stays pending; attempt_count++)
///
/// One-flight guard: a single Dart isolate is enough — there's no concurrent
/// HTTP client. `_inFlight` prevents foreground+connectivity events from
/// double-draining.
class OutboxWorker {
  final PendingWritesDao pending;
  final WidgetFailuresDao widgetFailures;
  final OutboxSender sender;

  bool _inFlight = false;

  OutboxWorker({
    required this.pending,
    required this.widgetFailures,
    required this.sender,
  });

  /// Moves widget failures into the outbox, then drains it. Returns the
  /// outcome so callers can log.
  Future<DrainOutcome> drain() async {
    if (_inFlight) return DrainOutcome.alreadyRunning;
    _inFlight = true;
    try {
      await widgetFailures.drainInto(pending);
      final rows = await pending.pendingInArrivalOrder();
      if (rows.isEmpty) return DrainOutcome.empty;
      for (final row in rows) {
        await _processOne(row);
      }
      return DrainOutcome.drained;
    } finally {
      _inFlight = false;
    }
  }

  Future<void> _processOne(PendingWrite row) async {
    await pending.markInFlight(row.id);
    try {
      final resp = await sender.send(
        method: row.method,
        path: row.path,
        body: row.body,
        idempotencyKey: row.idemKey,
      );
      _classify(row, resp);
    } on DioException catch (e) {
      // Connection / timeout / unknown — keep pending for next drain.
      await pending.recordTransientFailure(row.id, e.message ?? e.type.name);
    } catch (e) {
      await pending.recordTransientFailure(row.id, e.toString());
    }
  }

  Future<void> _classify(PendingWrite row, Response<dynamic> resp) async {
    final status = resp.statusCode ?? 0;
    if (status >= 200 && status < 300) {
      await pending.markDone(row.id);
    } else if (status >= 400 && status < 500) {
      // 4xx is permanent — the request is malformed or violates a server
      // invariant (e.g. idempotency_key_conflict). Don't loop.
      await pending.markFailedPermanent(
          row.id, 'http $status: ${_describe(resp)}');
    } else if (status >= 500 || status == 0) {
      await pending.recordTransientFailure(
          row.id, 'http $status: ${_describe(resp)}');
    } else {
      // 3xx is unexpected for a JSON write endpoint; treat as permanent.
      await pending.markFailedPermanent(
          row.id, 'unexpected status $status');
    }
  }

  String _describe(Response<dynamic> resp) {
    final data = resp.data;
    if (data is Map<String, dynamic>) return data['error']?.toString() ?? '';
    if (data is String) return data;
    return '';
  }
}
