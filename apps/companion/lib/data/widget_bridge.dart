import 'package:flutter/services.dart';

/// Flutter → Kotlin bridge for the hydration widget. Pushes config (glass size,
/// goal, the Drift DB path for the offline spillover) and snapshot updates into
/// the native side so the widget can render and the tap worker can run without
/// waking Flutter. All calls tolerate a missing channel (e.g. unit tests).
class WidgetBridge {
  static const _channel =
      MethodChannel('com.corelyr.nutrition_companion/widget_bridge');

  final MethodChannel _ch;
  WidgetBridge({MethodChannel? channel}) : _ch = channel ?? _channel;

  Future<void> setConfig({
    required int glassSizeMl,
    required int hydrationGoalMl,
    required String driftDbPath,
  }) =>
      _invoke('setConfig', {
        'glass_size_ml': glassSizeMl,
        'hydration_goal_ml': hydrationGoalMl,
        'drift_db_path': driftDbPath,
      });

  /// Mirrors the freshest totals into the widget's Room snapshot and refreshes
  /// the widget UI. Called after every hydration log.
  Future<void> updateSnapshot({
    required String date,
    required double totalMl,
    required double goalMl,
  }) =>
      _invoke('updateSnapshot', {
        'date': date,
        'total_ml': totalMl,
        'goal_ml': goalMl,
      });

  Future<void> _invoke(String method, Map<String, dynamic> args) async {
    try {
      await _ch.invokeMethod<void>(method, args);
    } on PlatformException {
      // Channel error — non-fatal; the in-app experience is unaffected.
    } on MissingPluginException {
      // Bridge not wired in this build (tests / non-Android).
    }
  }
}
