package com.corelyr.nutrition_companion

import com.corelyr.nutrition_companion.widget.HydrationSnapshot
import com.corelyr.nutrition_companion.widget.HydrationWidget
import com.corelyr.nutrition_companion.widget.SnapshotDb
import com.corelyr.nutrition_companion.widget.WidgetConfig
import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodChannel

class MainActivity : FlutterActivity() {
    private val tokenBridgeChannel = "com.corelyr.nutrition_companion/token_bridge"
    private val widgetBridgeChannel = "com.corelyr.nutrition_companion/widget_bridge"

    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)
        MethodChannel(flutterEngine.dartExecutor.binaryMessenger, tokenBridgeChannel)
            .setMethodCallHandler { call, result ->
                when (call.method) {
                    "mirror" -> {
                        val baseUrl = call.argument<String>("base_url")
                        val token = call.argument<String>("token")
                        if (baseUrl == null || token == null) {
                            result.error("ARG", "base_url and token required", null)
                        } else {
                            TokenBridge.mirror(applicationContext, baseUrl, token)
                            result.success(null)
                        }
                    }
                    "clear" -> {
                        TokenBridge.clear(applicationContext)
                        result.success(null)
                    }
                    else -> result.notImplemented()
                }
            }

        MethodChannel(flutterEngine.dartExecutor.binaryMessenger, widgetBridgeChannel)
            .setMethodCallHandler { call, result ->
                when (call.method) {
                    "setConfig" -> {
                        WidgetConfig.update(
                            applicationContext,
                            glassSizeMl = call.argument<Int>("glass_size_ml"),
                            hydrationGoalMl = call.argument<Int>("hydration_goal_ml"),
                            driftDbPath = call.argument<String>("drift_db_path"),
                        )
                        result.success(null)
                    }
                    "updateSnapshot" -> {
                        val date = call.argument<String>("date")
                        val totalMl = call.argument<Double>("total_ml")
                        val goalMl = call.argument<Double>("goal_ml")
                        if (date == null || totalMl == null || goalMl == null) {
                            result.error("ARG", "date, total_ml, goal_ml required", null)
                        } else {
                            SnapshotDb.get(applicationContext).snapshotDao().upsert(
                                HydrationSnapshot(
                                    date = date,
                                    totalMl = totalMl,
                                    dailyGoalMl = goalMl,
                                    updatedAt = System.currentTimeMillis(),
                                ),
                            )
                            HydrationWidget.requestUpdate(applicationContext)
                            result.success(null)
                        }
                    }
                    else -> result.notImplemented()
                }
            }
    }
}
