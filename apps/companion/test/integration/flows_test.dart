import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:drift/native.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:kazper/data/auth/token_store.dart';
import 'package:kazper/data/db/app_database.dart';
import 'package:kazper/data/sync/outbox_worker.dart';
import 'package:kazper/state/app_providers.dart';
import 'package:kazper/state/pairing_provider.dart';
import 'package:kazper/state/recent_provider.dart';
import 'package:kazper/state/scan_provider.dart';

import '../support/fake_repository.dart';

/// In-memory token store — no Keystore, no method channel.
class FakeTokenStore implements TokenStore {
  String? _token;
  String? _baseUrl;
  @override
  Future<String?> getToken() async => _token;
  @override
  Future<String?> getBaseUrl() async => _baseUrl;
  @override
  Future<void> pair({required String baseUrl, required String token}) async {
    _baseUrl = baseUrl;
    _token = token;
  }

  @override
  Future<void> clear() async {
    _token = null;
    _baseUrl = null;
  }
}

/// A sender that can be toggled offline/online and records each request path.
class _ToggleSender implements OutboxSender {
  bool online = false;
  final List<String> sent = [];

  @override
  Future<Response<dynamic>> send({
    required String method,
    required String path,
    required List<int> body,
    required String idempotencyKey,
  }) async {
    if (!online) {
      throw DioException.connectionError(
        requestOptions: RequestOptions(path: path),
        reason: 'offline',
      );
    }
    sent.add(path);
    return Response(
      requestOptions: RequestOptions(path: path),
      statusCode: 201,
      data: const <String, dynamic>{},
    );
  }
}

void main() {
  test('pair → log meal via barcode → meal appears in Recent', () async {
    final repo = FakeRepository()
      ..fresh = summaryFixture()
      ..cachedProductValue = productFixture(serving: 30);
    final tokenStore = FakeTokenStore();
    final c = ProviderContainer(overrides: [
      repositoryProvider.overrideWithValue(repo),
      tokenStoreProvider.overrideWithValue(tokenStore),
    ]);
    addTearDown(c.dispose);

    // Initially unpaired.
    expect(await c.read(pairingProvider.future), isFalse);

    // Pair via the QR payload.
    await c.read(pairingProvider.notifier).pair(
          baseUrl: 'http://10.0.0.5:8080',
          token: 'secret',
        );
    expect(c.read(pairingProvider).value, isTrue);
    expect(await tokenStore.getToken(), 'secret');

    // Scan a cached barcode and log it.
    final scan = c.read(scanProvider.notifier);
    await scan.onBarcode('12345');
    expect(c.read(scanProvider).phase, ScanPhase.product);
    await scan.log();
    expect(repo.meals, hasLength(1));

    // The logged meal shows up in Recent.
    final recent = await c.read(recentProvider.future);
    expect(recent.whereType<RecentMeal>(), isNotEmpty);
    expect(
      recent.whereType<RecentMeal>().map((r) => r.meal.effectiveName),
      contains('Test bar'),
    );
  });

  test('offline log → reconnect → queue drains → request reaches the API',
      () async {
    final db = AppDatabase.forTesting(NativeDatabase.memory());
    addTearDown(db.close);
    final sender = _ToggleSender();
    final worker = OutboxWorker(
      pending: db.pendingWritesDao,
      widgetFailures: db.widgetFailuresDao,
      sender: sender,
    );

    // Offline write: enqueue a hydration log, drain fails to send.
    await db.pendingWritesDao.enqueue(
      id: 'row-1',
      method: 'POST',
      path: '/hydration',
      body: Uint8List.fromList([0x7b, 0x7d]),
      idemKey: 'idem-1',
    );
    await worker.drain();

    var row = await (db.select(db.pendingWrites)
          ..where((r) => r.id.equals('row-1')))
        .getSingle();
    expect(row.status, 'pending', reason: 'offline keeps the row queued');
    expect(sender.sent, isEmpty);

    // Reconnect and drain again.
    sender.online = true;
    await worker.drain();

    row = await (db.select(db.pendingWrites)
          ..where((r) => r.id.equals('row-1')))
        .getSingle();
    expect(row.status, 'done');
    expect(sender.sent, ['/hydration']);
  });
}
