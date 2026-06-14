package com.corelyr.kazper.widget

import android.content.ContentValues
import android.content.Context
import android.database.sqlite.SQLiteDatabase
import java.util.UUID

/**
 * Writes a failed hydration tap into Drift's `widget_failures` table so the
 * Flutter app drains it into the outbox on next foreground. This is the
 * A-lite pattern's offline branch: the widget never reaches Flutter directly,
 * it just leaves a row in the shared SQLite file.
 *
 * Drift stores `DateTime` columns as integer seconds since the Unix epoch and
 * `BlobColumn` as a SQLite BLOB, so we mirror those encodings exactly.
 */
object WidgetSpillover {
    fun record(context: Context, body: ByteArray, idempotencyKey: String): Boolean {
        val path = WidgetConfig.driftDbPath(context) ?: return false
        return try {
            SQLiteDatabase.openDatabase(
                path,
                null,
                SQLiteDatabase.OPEN_READWRITE,
            ).use { db ->
                val values = ContentValues().apply {
                    put("id", UUID.randomUUID().toString())
                    put("body", body)
                    put("idem_key", idempotencyKey)
                    put("created_at", System.currentTimeMillis() / 1000)
                }
                db.insert("widget_failures", null, values)
            }
            true
        } catch (e: Exception) {
            false
        }
    }
}
