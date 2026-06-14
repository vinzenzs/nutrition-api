package com.corelyr.kazper.widget

import android.content.Context

/**
 * Non-secret widget configuration mirrored from Flutter (glass size, hydration
 * goal) plus the resolved path to Drift's SQLite file so the spillover writer
 * can target the same `widget_failures` table the Flutter app drains. The
 * bearer token is NOT here — it lives in EncryptedSharedPreferences via
 * [com.corelyr.kazper.TokenBridge].
 */
object WidgetConfig {
    private const val PREFS = "nutrition_companion_widget"
    private const val KEY_GLASS = "glass_size_ml"
    private const val KEY_GOAL = "hydration_goal_ml"
    private const val KEY_DRIFT_PATH = "drift_db_path"

    private fun prefs(context: Context) =
        context.applicationContext.getSharedPreferences(PREFS, Context.MODE_PRIVATE)

    fun glassSizeMl(context: Context): Int = prefs(context).getInt(KEY_GLASS, 250)

    fun hydrationGoalMl(context: Context): Int =
        prefs(context).getInt(KEY_GOAL, 2500)

    fun driftDbPath(context: Context): String? =
        prefs(context).getString(KEY_DRIFT_PATH, null)

    fun update(
        context: Context,
        glassSizeMl: Int? = null,
        hydrationGoalMl: Int? = null,
        driftDbPath: String? = null,
    ) {
        prefs(context).edit().apply {
            glassSizeMl?.let { putInt(KEY_GLASS, it) }
            hydrationGoalMl?.let { putInt(KEY_GOAL, it) }
            driftDbPath?.let { putString(KEY_DRIFT_PATH, it) }
            apply()
        }
    }
}
