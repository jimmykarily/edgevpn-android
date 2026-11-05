package com.kairos.edgevpn

import android.content.Context
import android.os.Build
import edgevpn.Edgevpn

object AppSettings {

    private const val KEY_ADDRESS = "address"
    private const val KEY_NAME = "name"
    private const val KEY_DETAILED_LOGS = "detailed_logs"
    private const val DEFAULT_ADDRESS = "10.1.0.2"

    fun address(c: Context): String =
        TokenStore.prefs(c).getString(KEY_ADDRESS, null) ?: DEFAULT_ADDRESS

    fun setAddress(c: Context, v: String) {
        TokenStore.prefs(c).edit().putString(KEY_ADDRESS, v).apply()
    }

    fun name(c: Context): String {
        val stored = TokenStore.prefs(c).getString(KEY_NAME, null)
        return if (stored.isNullOrBlank()) Build.MODEL else stored
    }

    fun setName(c: Context, v: String) {
        TokenStore.prefs(c).edit().putString(KEY_NAME, v).apply()
    }

    fun detailedLogs(c: Context): Boolean =
        TokenStore.prefs(c).getBoolean(KEY_DETAILED_LOGS, false)

    fun setDetailedLogs(c: Context, v: Boolean) {
        TokenStore.prefs(c).edit().putBoolean(KEY_DETAILED_LOGS, v).apply()
    }

    // Forgets the token, the node key and every setting, and empties the log buffer.
    // The caller stops the VPN first.
    fun startFresh(c: Context) {
        TokenStore.prefs(c).edit().clear().commit()
        Edgevpn.clearLogs()
    }
}
