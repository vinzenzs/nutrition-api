import 'dart:async';

import 'package:connectivity_plus/connectivity_plus.dart';
import 'package:flutter/widgets.dart';
import 'package:workmanager/workmanager.dart';

import 'outbox_worker.dart';

const String outboxBackstopTask = 'outbox-backstop-15m';

/// Wires the three replay triggers the design calls out:
///   - app foreground (WidgetsBindingObserver -> AppLifecycleState.resumed)
///   - network state change (connectivity_plus stream)
///   - 15-minute periodic Workmanager backstop
///
/// The Workmanager periodic task is set up at app start. The other two are
/// active for the app's lifetime as long as [start] was called.
class ReplayTriggers with WidgetsBindingObserver {
  final OutboxWorker worker;
  StreamSubscription<List<ConnectivityResult>>? _connSub;
  bool _registered = false;

  ReplayTriggers(this.worker);

  Future<void> start() async {
    if (_registered) return;
    _registered = true;

    WidgetsBinding.instance.addObserver(this);

    _connSub = Connectivity().onConnectivityChanged.listen((results) {
      final online = results.any((r) => r != ConnectivityResult.none);
      if (online) {
        unawaited(worker.drain());
      }
    });

    // Backstop: a periodic 15-minute task. The actual work runs in
    // [workmanagerCallback] (a top-level function below). 15 min is the
    // Android-imposed minimum.
    await Workmanager().registerPeriodicTask(
      outboxBackstopTask,
      outboxBackstopTask,
      frequency: const Duration(minutes: 15),
      existingWorkPolicy: ExistingPeriodicWorkPolicy.keep,
    );

    // Initial drain at startup (covers the "app launched after offline write" case).
    unawaited(worker.drain());
  }

  void stop() {
    WidgetsBinding.instance.removeObserver(this);
    _connSub?.cancel();
    _registered = false;
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed) {
      unawaited(worker.drain());
    }
  }
}
