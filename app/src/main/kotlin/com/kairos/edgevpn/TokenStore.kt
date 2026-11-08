package com.kairos.edgevpn

import android.content.Context
import android.content.SharedPreferences
import android.util.Log
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKeys
import java.io.IOException
import java.security.GeneralSecurityException

object TokenStore {

    private const val TAG = "EdgeVPN"
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

    /** Makes the next call open the file again, as a new process would. For tests. */
    internal fun openAgain() {
        synchronized(this) { cached = null }
    }

    /**
     * Opens the file, or deletes it and starts with an empty one when it cannot be decrypted.
     * That happens to a file restored from a backup or moved to another phone: its key lives in
     * the Android keystore of the phone that wrote it and never leaves that phone. The network
     * key and the node key are lost then, and the user joins the network again.
     */
    private fun open(context: Context): SharedPreferences = try {
        create(context)
    } catch (e: GeneralSecurityException) {
        startOver(context, e)
    } catch (e: IOException) {
        startOver(context, e)
    }

    private fun startOver(context: Context, e: Exception): SharedPreferences {
        // Only the exception's class, so nothing read from the file can reach the log.
        Log.w(TAG, "Could not decrypt the settings (${e.javaClass.simpleName}); deleting them and starting fresh")
        context.deleteSharedPreferences(FILE_NAME)
        return create(context)
    }

    private fun create(context: Context): SharedPreferences {
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
