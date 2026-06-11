import 'package:connectivity_plus/connectivity_plus.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart';

import '../data/auth/token_store.dart';
import '../data/db/app_database.dart';
import '../data/net/api_client.dart';
import '../data/net/chat_client.dart';
import '../data/prefs.dart';
import '../data/repository.dart';
import '../data/sync/outbox_worker.dart';
import '../data/widget_bridge.dart';

/// Overridden in `main()` with the opened [Prefs]. Kept synchronous at the
/// read site so screens don't have to await it.
final prefsProvider = Provider<Prefs>((ref) {
  throw UnimplementedError('prefsProvider must be overridden in main()');
});

final appDatabaseProvider = Provider<AppDatabase>((ref) {
  final db = AppDatabase();
  ref.onDispose(db.close);
  return db;
});

final tokenStoreProvider = Provider<TokenStore>((ref) => SecureTokenStore());

final widgetBridgeProvider = Provider<WidgetBridge>((ref) => WidgetBridge());

final apiClientProvider = Provider<ApiClient>(
  (ref) => ApiClient(tokenStore: ref.watch(tokenStoreProvider)),
);

/// SSE chat client (separate from the dio-based [apiClient] — chat streams and
/// can't be outboxed). Overridden in tests.
final chatClientProvider = Provider<ChatClient>(
  (ref) => ChatClient(tokenStore: ref.watch(tokenStoreProvider)),
);

final outboxWorkerProvider = Provider<OutboxWorker>((ref) {
  final db = ref.watch(appDatabaseProvider);
  return OutboxWorker(
    pending: db.pendingWritesDao,
    widgetFailures: db.widgetFailuresDao,
    sender: ApiClientSender(ref.watch(apiClientProvider)),
  );
});

/// The data surface every feature provider reads. Overridden in tests with a
/// fake (no Drift, no Dio).
final repositoryProvider = Provider<Repository>((ref) {
  return ApiRepository(
    db: ref.watch(appDatabaseProvider),
    api: ref.watch(apiClientProvider),
    outbox: ref.watch(outboxWorkerProvider),
  );
});

/// Online/offline signal (reused for the chat composer gating — chat is the
/// app's lone online-only surface). Starts optimistically online.
final onlineProvider = StreamProvider<bool>((ref) async* {
  yield true;
  await for (final results in Connectivity().onConnectivityChanged) {
    yield results.any((r) => r != ConnectivityResult.none);
  }
});

/// Today's date in `yyyy-MM-dd`, device-local. The backend defaults the
/// timezone when the app omits it.
String todayDate() => DateFormat('yyyy-MM-dd').format(DateTime.now());

/// Meal type derived from the time of day, per the scan-flow spec:
/// breakfast 04:00–10:59, lunch 11:00–14:59, dinner 17:00–22:59, snack else.
String mealTypeForNow([DateTime? at]) {
  final h = (at ?? DateTime.now()).hour;
  if (h >= 4 && h < 11) return 'breakfast';
  if (h >= 11 && h < 15) return 'lunch';
  if (h >= 17 && h < 23) return 'dinner';
  return 'snack';
}
