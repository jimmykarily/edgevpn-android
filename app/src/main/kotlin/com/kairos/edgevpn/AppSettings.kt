package com.kairos.edgevpn

import android.content.Context
import android.os.Build
import edgevpn.Edgevpn

object AppSettings {

    private const val KEY_ADDRESS = "address"
    private const val KEY_NAME = "name"
    private const val KEY_DETAILED_LOGS = "detailed_logs"
    private const val KEY_EXTRA_PEERS = "extra_peers"
    private const val KEY_CHECK_IN_SECONDS = "check_in_seconds"
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

    /**
     * Peers to dial directly at start, for Edgevpn.setExtraPeers. Nothing in the app sets this;
     * the network tests do, so the app reaches their test peer without waiting for the DHT.
     */
    fun extraPeers(c: Context): String =
        TokenStore.prefs(c).getString(KEY_EXTRA_PEERS, null) ?: ""

    fun setExtraPeers(c: Context, v: String) {
        TokenStore.prefs(c).edit().putString(KEY_EXTRA_PEERS, v).apply()
    }

    /**
     * How often the device writes its heartbeat, for Edgevpn.setCheckInSeconds; 0 is the default.
     * Nothing in the app sets this; the network tests do, because their test peer forgets a
     * device after 60 seconds without a heartbeat.
     */
    fun checkInSeconds(c: Context): Int =
        TokenStore.prefs(c).getInt(KEY_CHECK_IN_SECONDS, 0)

    fun setCheckInSeconds(c: Context, v: Int) {
        TokenStore.prefs(c).edit().putInt(KEY_CHECK_IN_SECONDS, v).apply()
    }

    // Forgets the token, the node key and every setting, and empties the log buffer.
    // The caller stops the VPN first.
    fun startFresh(c: Context) {
        TokenStore.prefs(c).edit().clear().commit()
        Edgevpn.clearLogs()
    }
}
