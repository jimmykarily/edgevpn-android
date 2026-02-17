package com.kairos.edgevpn

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Intent
import android.net.VpnService
import android.os.ParcelFileDescriptor
import android.util.Log
import androidx.core.app.NotificationCompat
import edgevpn.Edgevpn
import edgevpn.SocketProtector

class EdgeVpnService : VpnService() {

    companion object {
        const val ACTION_CONNECT = "com.kairos.edgevpn.CONNECT"
        const val ACTION_DISCONNECT = "com.kairos.edgevpn.DISCONNECT"

        private const val TAG = "EdgeVpnService"
        private const val CHANNEL_ID = "edgevpn_status"
        private const val NOTIFICATION_ID = 1

        var isRunning = false
            private set
    }

    private var tunPfd: ParcelFileDescriptor? = null

    /** Wraps VpnService.protect(int) so Go can call it via the SocketProtector interface. */
    private val socketProtector = object : SocketProtector {
        override fun protect(fd: Long): Boolean {
            return this@EdgeVpnService.protect(fd.toInt())
        }
    }

    override fun onCreate() {
        super.onCreate()
        createNotificationChannel()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_DISCONNECT -> {
                disconnect()
                return START_NOT_STICKY
            }
        }
        connect()
        return START_STICKY
    }

    private fun connect() {
        if (tunPfd != null) return

        val token = TokenStore.load(this)
        if (token == null) {
            Log.e(TAG, "No token configured")
            stopSelf()
            return
        }

        startForeground(NOTIFICATION_ID, buildNotification())

        val pfd = Builder()
            .addAddress("10.1.0.2", 24)
            .addRoute("10.1.0.0", 24)
            .setMtu(1500)
            .setSession("EdgeVPN")
            .establish()

        if (pfd == null) {
            Log.e(TAG, "VPN establish() returned null")
            stopSelf()
            return
        }

        // Register the socket protector BEFORE starting Go networking.
        // Every socket fd created by libp2p will be passed through
        // VpnService.protect() so it bypasses the TUN interface.
        Edgevpn.setSocketProtector(socketProtector)

        val fd = pfd.detachFd()
        tunPfd = pfd

        try {
            Edgevpn.start(fd.toLong(), token)
            isRunning = true
            Log.i(TAG, "VPN started")
        } catch (e: Exception) {
            Log.e(TAG, "Failed to start VPN", e)
            isRunning = false
            stopForeground(STOP_FOREGROUND_REMOVE)
            stopSelf()
        }
    }

    private fun disconnect() {
        Edgevpn.stop()
        isRunning = false
        tunPfd = null
        stopForeground(STOP_FOREGROUND_REMOVE)
        stopSelf()
        Log.i(TAG, "VPN stopped")
    }

    override fun onRevoke() {
        disconnect()
    }

    override fun onDestroy() {
        disconnect()
        super.onDestroy()
    }

    private fun createNotificationChannel() {
        val channel = NotificationChannel(
            CHANNEL_ID,
            "VPN Status",
            NotificationManager.IMPORTANCE_LOW
        ).apply {
            description = "Shows when EdgeVPN is active"
        }
        getSystemService(NotificationManager::class.java).createNotificationChannel(channel)
    }

    private fun buildNotification(): Notification {
        val openApp = PendingIntent.getActivity(
            this, 0,
            Intent(this, MainActivity::class.java),
            PendingIntent.FLAG_IMMUTABLE
        )
        return NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle("EdgeVPN")
            .setContentText("Connected")
            .setSmallIcon(android.R.drawable.ic_lock_lock)
            .setContentIntent(openApp)
            .setOngoing(true)
            .build()
    }
}
