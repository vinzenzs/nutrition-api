package com.corelyr.nutrition_companion

import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodChannel

class MainActivity : FlutterActivity() {
    private val tokenBridgeChannel = "com.corelyr.nutrition_companion/token_bridge"

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
    }
}
