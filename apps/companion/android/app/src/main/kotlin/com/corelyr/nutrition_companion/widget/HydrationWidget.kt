package com.corelyr.nutrition_companion.widget

import android.app.PendingIntent
import android.appwidget.AppWidgetManager
import android.appwidget.AppWidgetProvider
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.widget.RemoteViews
import androidx.work.OneTimeWorkRequestBuilder
import androidx.work.WorkManager
import com.corelyr.nutrition_companion.R
import java.time.LocalDate
import kotlin.math.roundToInt

/**
 * Home-screen hydration widget (A-lite pattern). Renders today's progress from
 * the Room snapshot and, on tap, enqueues [HydrationTapWorker] — no Flutter
 * wakeup on the happy path.
 */
class HydrationWidget : AppWidgetProvider() {

    override fun onUpdate(
        context: Context,
        appWidgetManager: AppWidgetManager,
        appWidgetIds: IntArray,
    ) {
        for (id in appWidgetIds) {
            render(context, appWidgetManager, id)
        }
    }

    override fun onReceive(context: Context, intent: Intent) {
        super.onReceive(context, intent)
        if (intent.action == ACTION_TAP) {
            WorkManager.getInstance(context).enqueue(
                OneTimeWorkRequestBuilder<HydrationTapWorker>().build(),
            )
            requestUpdate(context)
        }
    }

    companion object {
        private const val ACTION_TAP =
            "com.corelyr.nutrition_companion.widget.ACTION_TAP"

        /** Re-renders every instance of the widget from the current snapshot. */
        fun requestUpdate(context: Context) {
            val manager = AppWidgetManager.getInstance(context)
            val ids = manager.getAppWidgetIds(
                ComponentName(context, HydrationWidget::class.java),
            )
            for (id in ids) render(context, manager, id)
        }

        private fun render(
            context: Context,
            manager: AppWidgetManager,
            appWidgetId: Int,
        ) {
            val today = LocalDate.now().toString()
            val snapshot = SnapshotDb.get(context).snapshotDao().forDate(today)
            val total = snapshot?.totalMl ?: 0.0
            val goal = snapshot?.dailyGoalMl
                ?: WidgetConfig.hydrationGoalMl(context).toDouble()
            val pct = if (goal <= 0) 0 else ((total / goal) * 100).roundToInt().coerceIn(0, 100)

            val views = RemoteViews(context.packageName, R.layout.hydration_widget).apply {
                setTextViewText(R.id.widget_total, "${total.roundToInt()} / ${goal.roundToInt()} ml")
                setTextViewText(R.id.widget_pct, "$pct%")
                setProgressBar(R.id.widget_progress, 100, pct, false)
                setOnClickPendingIntent(R.id.widget_root, tapIntent(context))
            }
            manager.updateAppWidget(appWidgetId, views)
        }

        private fun tapIntent(context: Context): PendingIntent {
            val intent = Intent(context, HydrationWidget::class.java).apply {
                action = ACTION_TAP
            }
            return PendingIntent.getBroadcast(
                context,
                0,
                intent,
                PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
            )
        }
    }
}
