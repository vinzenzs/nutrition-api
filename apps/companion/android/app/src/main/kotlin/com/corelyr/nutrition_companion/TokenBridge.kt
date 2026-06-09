package com.corelyr.nutrition_companion

import android.content.Context
import android.content.SharedPreferences
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey

/**
 * Tiny adapter around EncryptedSharedPreferences so both the Flutter side
 * (via MethodChannel) and the Kotlin widget worker can read/write the
 * same Android Keystore–backed token blob.
 *
 * The Flutter app's `flutter_secure_storage` plugin already uses the same
 * MasterKey alias under the hood, but it's stored in a different file
 * namespace — so we mirror writes here on pair(). Reads inside Kotlin
 * (e.g. the widget worker) must go through this class, not directly
 * through flutter_secure_storage.
 */
object TokenBridge {
    private const val PREFS_FILE = "nutrition_companion_token_bridge"
    private const val KEY_TOKEN = "mobile_api_token"
    private const val KEY_BASE_URL = "base_url"

    private fun prefs(context: Context): SharedPreferences {
        val masterKey = MasterKey.Builder(context.applicationContext)
            .setKeyScheme(MasterKey.KeyScheme.AES256_GCM)
            .build()
        return EncryptedSharedPreferences.create(
            context.applicationContext,
            PREFS_FILE,
            masterKey,
            EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
            EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM,
        )
    }

    fun getToken(context: Context): String? =
        prefs(context).getString(KEY_TOKEN, null)

    fun getBaseUrl(context: Context): String? =
        prefs(context).getString(KEY_BASE_URL, null)

    fun mirror(context: Context, baseUrl: String, token: String) {
        prefs(context).edit()
            .putString(KEY_BASE_URL, baseUrl)
            .putString(KEY_TOKEN, token)
            .apply()
    }

    fun clear(context: Context) {
        prefs(context).edit().clear().apply()
    }
}
