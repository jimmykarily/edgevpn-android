package com.kairos.edgevpn

import android.content.Context
import android.content.SharedPreferences
import android.util.Log
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKeys
import java.io.File
import java.security.KeyStore

object TokenStore {

    private const val TAG = "EdgeVPN"
    private const val FILE_NAME = "edgevpn_secure_prefs"
    private const val KEY_TOKEN = "vpn_token"

    /** The keystore alias under which MasterKeys keeps the key of the settings file. */
    private const val MASTER_KEY_ALIAS = "_androidx_security_master_key_"

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
     * Opens the file. A file whose master key is missing from the Android keystore is deleted
     * first, because nothing can ever decrypt it: that is a file restored from a backup or moved
     * to another phone, since the key never leaves the phone that made it. The network key and
     * the node key are lost then, and the user joins the network again. Any other failure is
     * left to crash, because it can be a keystore that is briefly unusable, and deleting the file
     * then would throw away settings that the next start could still read.
     */
    private fun open(context: Context): SharedPreferences {
        if (!masterKeyExists() && File(context.dataDir, "shared_prefs/$FILE_NAME.xml").exists()) {
            Log.w(TAG, "The settings were encrypted with a key this phone does not have; deleting them and starting fresh")
            context.deleteSharedPreferences(FILE_NAME)
        }
        return create(context)
    }

    private fun masterKeyExists(): Boolean =
        KeyStore.getInstance("AndroidKeyStore").apply { load(null) }.containsAlias(MASTER_KEY_ALIAS)

    private fun create(context: Context): SharedPreferences {
        val masterKey = MasterKeys.getOrCreate(MasterKeys.AES256_GCM_SPEC)
        check(masterKey == MASTER_KEY_ALIAS) { "unexpected master key alias" }
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
