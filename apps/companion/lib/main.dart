import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:workmanager/workmanager.dart';

import 'data/auth/token_store.dart';
import 'data/db/app_database.dart';
import 'data/net/api_client.dart';
import 'data/prefs.dart';
import 'data/sync/outbox_worker.dart';
import 'state/app_providers.dart';
import 'state/pairing_provider.dart';
import 'ui/home_shell.dart';
import 'ui/pair/pair_page.dart';

/// WorkManager backstop entry point. Runs in a background isolate, so it wires
/// its own DB/api rather than reaching into the app's provider graph, drains
/// the outbox once, and tears down.
@pragma('vm:entry-point')
void callbackDispatcher() {
  Workmanager().executeTask((task, inputData) async {
    final db = AppDatabase();
    final tokenStore = SecureTokenStore();
    final worker = OutboxWorker(
      pending: db.pendingWritesDao,
      widgetFailures: db.widgetFailuresDao,
      sender: ApiClientSender(ApiClient(tokenStore: tokenStore)),
    );
    try {
      await worker.drain();
    } finally {
      await db.close();
    }
    return true;
  });
}

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  final prefs = await Prefs.open();
  await Workmanager().initialize(callbackDispatcher);

  runApp(
    ProviderScope(
      overrides: [prefsProvider.overrideWithValue(prefs)],
      child: const CompanionApp(),
    ),
  );
}

class CompanionApp extends ConsumerWidget {
  const CompanionApp({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final paired = ref.watch(pairingProvider);
    return MaterialApp(
      title: 'Nutrition Companion',
      theme: ThemeData(
        colorScheme: ColorScheme.fromSeed(seedColor: Colors.teal),
        useMaterial3: true,
      ),
      home: paired.when(
        loading: () =>
            const Scaffold(body: Center(child: CircularProgressIndicator())),
        error: (e, _) => Scaffold(body: Center(child: Text('$e'))),
        data: (isPaired) => isPaired ? const HomeShell() : const PairPage(),
      ),
    );
  }
}
