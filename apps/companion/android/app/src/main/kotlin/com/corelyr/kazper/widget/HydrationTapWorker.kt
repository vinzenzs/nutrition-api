package com.corelyr.kazper.widget

import android.content.Context
import androidx.work.CoroutineWorker
import androidx.work.WorkerParameters
import com.corelyr.kazper.TokenBridge
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import java.io.IOException
import java.time.Instant
import java.time.LocalDate
import java.util.UUID

/**
 * Pure-ish tap logic, split out so it can be unit-tested with MockWebServer.
 * Posts a single hydration log and, on any non-2xx / network failure, records
 * the request into Drift's `widget_failures` table for the Flutter outbox.
 */
object HydrationTap {
    private val JSON = "application/json".toMediaType()

    fun bodyJson(quantityMl: Int, loggedAtIso: String): String =
        """{"quantity_ml":$quantityMl,"logged_at":"$loggedAtIso"}"""

    /** Returns true on a 2xx ack, false (and records a spillover row) otherwise. */
    fun perform(
        context: Context,
        client: OkHttpClient,
        baseUrl: String,
        token: String,
        quantityMl: Int,
        idempotencyKey: String = UUID.randomUUID().toString(),
        loggedAtIso: String = Instant.now().toString(),
    ): Boolean {
        val body = bodyJson(quantityMl, loggedAtIso)
        val request = Request.Builder()
            .url(baseUrl.trimEnd('/') + "/hydration")
            .addHeader("Authorization", "Bearer $token")
            .addHeader("Idempotency-Key", idempotencyKey)
            .post(body.toRequestBody(JSON))
            .build()
        return try {
            client.newCall(request).execute().use { resp ->
                if (resp.isSuccessful) {
                    true
                } else {
                    WidgetSpillover.record(context, body.toByteArray(), idempotencyKey)
                    false
                }
            }
        } catch (e: IOException) {
            WidgetSpillover.record(context, body.toByteArray(), idempotencyKey)
            false
        }
    }
}

/**
 * One-shot WorkManager job enqueued by the widget on tap. Reads the bearer
 * token from EncryptedSharedPreferences (no Flutter wakeup), POSTs the glass,
 * optimistically bumps the local snapshot, and refreshes the widget UI. On
 * failure the request is already queued in `widget_failures`.
 */
class HydrationTapWorker(
    appContext: Context,
    params: WorkerParameters,
) : CoroutineWorker(appContext, params) {

    private val client = OkHttpClient()

    override suspend fun doWork(): Result = withContext(Dispatchers.IO) {
        val token = TokenBridge.getToken(applicationContext)
        val baseUrl = TokenBridge.getBaseUrl(applicationContext)
        if (token.isNullOrEmpty() || baseUrl.isNullOrEmpty()) {
            return@withContext Result.failure()
        }

        val glass = WidgetConfig.glassSizeMl(applicationContext)
        HydrationTap.perform(applicationContext, client, baseUrl, token, glass)

        // Optimistically reflect the increment locally whether or not the POST
        // succeeded — offline taps still show progress; a later foreground
        // sync reconciles against the server total.
        bumpSnapshot(glass.toDouble())
        HydrationWidget.requestUpdate(applicationContext)

        // The outbox owns retries for the offline case, so we never ask
        // WorkManager to retry — always report success.
        Result.success()
    }

    private fun bumpSnapshot(addMl: Double) {
        val dao = SnapshotDb.get(applicationContext).snapshotDao()
        val today = LocalDate.now().toString()
        val current = dao.forDate(today)
        val goal = WidgetConfig.hydrationGoalMl(applicationContext).toDouble()
        dao.upsert(
            HydrationSnapshot(
                date = today,
                totalMl = (current?.totalMl ?: 0.0) + addMl,
                dailyGoalMl = current?.dailyGoalMl ?: goal,
                updatedAt = System.currentTimeMillis(),
            ),
        )
    }
}
