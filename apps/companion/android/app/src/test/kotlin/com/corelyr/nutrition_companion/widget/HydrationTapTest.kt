package com.corelyr.nutrition_companion.widget

import android.content.Context
import android.database.sqlite.SQLiteDatabase
import androidx.test.core.app.ApplicationProvider
import okhttp3.OkHttpClient
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import java.io.File

// Robolectric 4.12 supports up to API 34; the app targets a newer SDK, so pin
// the test runtime explicitly.
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34])
class HydrationTapTest {
    private lateinit var server: MockWebServer
    private val context: Context = ApplicationProvider.getApplicationContext()

    @Before
    fun setUp() {
        server = MockWebServer()
        server.start()
    }

    @After
    fun tearDown() {
        server.shutdown()
    }

    @Test
    fun `successful tap posts the right hydration request`() {
        server.enqueue(MockResponse().setResponseCode(201).setBody("{}"))
        val baseUrl = server.url("/").toString().trimEnd('/')

        val ok = HydrationTap.perform(
            context = context,
            client = OkHttpClient(),
            baseUrl = baseUrl,
            token = "tok-123",
            quantityMl = 250,
            idempotencyKey = "idem-1",
            loggedAtIso = "2026-06-10T10:00:00Z",
        )

        assertTrue(ok)
        val request = server.takeRequest()
        assertEquals("/hydration", request.path)
        assertEquals("POST", request.method)
        assertEquals("Bearer tok-123", request.getHeader("Authorization"))
        assertEquals("idem-1", request.getHeader("Idempotency-Key"))
        assertTrue(request.body.readUtf8().contains("\"quantity_ml\":250"))
    }

    @Test
    fun `offline tap records a widget_failures spillover row`() {
        // A non-2xx (server up but failing) drives the spillover branch.
        server.enqueue(MockResponse().setResponseCode(503))
        val baseUrl = server.url("/").toString().trimEnd('/')

        val dbFile = File.createTempFile("drift", ".sqlite")
        createDriftSchema(dbFile)
        WidgetConfig.update(context, driftDbPath = dbFile.absolutePath)

        val ok = HydrationTap.perform(
            context = context,
            client = OkHttpClient(),
            baseUrl = baseUrl,
            token = "tok",
            quantityMl = 250,
            idempotencyKey = "idem-offline",
        )

        assertFalse(ok)
        SQLiteDatabase.openDatabase(
            dbFile.absolutePath, null, SQLiteDatabase.OPEN_READONLY,
        ).use { db ->
            db.rawQuery("SELECT idem_key FROM widget_failures", null).use { c ->
                assertTrue(c.moveToFirst())
                assertEquals("idem-offline", c.getString(0))
                assertEquals(1, c.count)
            }
        }
    }

    /** Mirrors Drift's `widget_failures` table shape. */
    private fun createDriftSchema(file: File) {
        SQLiteDatabase.openOrCreateDatabase(file, null).use { db ->
            db.execSQL(
                "CREATE TABLE widget_failures (" +
                    "id TEXT NOT NULL PRIMARY KEY, " +
                    "body BLOB NOT NULL, " +
                    "idem_key TEXT NOT NULL, " +
                    "created_at INTEGER NOT NULL)",
            )
        }
    }
}
