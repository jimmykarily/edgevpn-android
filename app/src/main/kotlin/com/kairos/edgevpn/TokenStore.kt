package com.kairos.edgevpn

import android.content.Context
import android.content.SharedPreferences
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKeys

object TokenStore {

    private const val FILE_NAME = "edgevpn_secure_prefs"
    private const val KEY_TOKEN = "vpn_token"

    @Volatile
    private var cached: SharedPreferences? = null

    /**
     * The encrypted settings file, which also holds the node key (see NodeKeyStore). Opening it
     * reads the master key from the Android keystore, which is slow, so it is opened once per
     * process, with the application context, and shared by every caller and thread.
     */
    internal fun prefs(context: Context): SharedPreferences {
        cached?.let { return it }
        return synchronized(this) {
            cached ?: open(context.applicationContext).also { cached = it }
        }
    }

    private fun open(context: Context): SharedPreferences {
        val masterKey = MasterKeys.getOrCreate(MasterKeys.AES256_GCM_SPEC)
        return EncryptedSharedPreferences.create(
            FILE_NAME,
            masterKey,
            context,
            EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
            EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM
        )
    }

    fun save(context: Context, token: String) {
        prefs(context).edit().putString(KEY_TOKEN, token).apply()
    }

    fun load(context: Context): String? {
        return prefs(context).getString(KEY_TOKEN, null)
    }

    fun clear(context: Context) {
        prefs(context).edit().remove(KEY_TOKEN).apply()
    }
}
