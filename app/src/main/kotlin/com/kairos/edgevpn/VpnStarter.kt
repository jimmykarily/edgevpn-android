package com.kairos.edgevpn

import android.content.Context
import android.content.Intent
import android.net.VpnService

/**
 * How the main screen turns the VPN on. The screen tests replace [prepare] and [start] to see
 * whether the screen turned the VPN on, without the permission dialog or a running node, and
 * call [reset] after.
 */
internal object VpnStarter {
    private val askAndroid: (Context) -> Intent? = { VpnService.prepare(it) }
    private val startService: (Context) -> Unit = {
        it.startForegroundService(
            Intent(it, EdgeVpnService::class.java).setAction(EdgeVpnService.ACTION_CONNECT)
        )
    }

    /** The intent that asks the user for the VPN permission, or null when it is granted. */
    @Volatile
    internal var prepare: (Context) -> Intent? = askAndroid

    @Volatile
    internal var start: (Context) -> Unit = startService

    internal fun reset() {
        prepare = askAndroid
        start = startService
    }
}
