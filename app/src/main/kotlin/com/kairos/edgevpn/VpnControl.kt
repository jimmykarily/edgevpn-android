package com.kairos.edgevpn

import android.content.Context
import android.os.Handler
import android.os.Looper

/** Starting and stopping the VPN service from any screen. */
object VpnControl {
    /** How often a reconnect checks whether the VPN has stopped before starting it again. */
    private const val RECONNECT_POLL_MS = 200L

    /** The pause between the VPN stopping and starting again, so the old service is gone first. */
    private const val RECONNECT_PAUSE_MS = 500L

    fun stop(c: Context) {
        c.startService(
            android.content.Intent(c, EdgeVpnService::class.java).setAction(EdgeVpnService.ACTION_DISCONNECT)
        )
    }

    /**
     * Stops the VPN, waits until the service has let go of it, and starts it again, so it comes
     * up with the settings and key as they are now. Pass the application context, so leaving the
     * screen does not cancel it.
     */
    fun reconnect(app: Context) {
        stop(app)
        val main = Handler(Looper.getMainLooper())
        val startWhenStopped = object : Runnable {
            override fun run() {
                if (EdgeVpnService.isRunning) {
                    main.postDelayed(this, RECONNECT_POLL_MS)
                    return
                }
                main.postDelayed({
                    app.startForegroundService(
                        android.content.Intent(app, EdgeVpnService::class.java)
                            .setAction(EdgeVpnService.ACTION_CONNECT)
                    )
                }, RECONNECT_PAUSE_MS)
            }
        }
        main.postDelayed(startWhenStopped, RECONNECT_POLL_MS)
    }
}
