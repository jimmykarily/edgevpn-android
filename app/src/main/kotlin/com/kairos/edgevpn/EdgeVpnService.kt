package com.kairos.edgevpn

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Intent
import android.net.VpnService
import android.os.ParcelFileDescriptor
import androidx.core.app.NotificationCompat

class EdgeVpnService : VpnService() {

    companion object {
        const val ACTION_CONNECT = "com.kairos.edgevpn.CONNECT"
        const val ACTION_DISCONNECT = "com.kairos.edgevpn.DISCONNECT"

        private const val CHANNEL_ID = "edgevpn_status"
        private const val NOTIFICATION_ID = 1

        var isRunning = false
            private set
    }

    private var tunFd: ParcelFileDescriptor? = null

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
        if (tunFd != null) return

        startForeground(NOTIFICATION_ID, buildNotification())

        tunFd = Builder()
            .addAddress("10.0.0.2", 32)
            .addRoute("10.0.0.0", 24)
            .setMtu(1500)
            .setSession("EdgeVPN")
            .establish()

        if (tunFd == null) {
            stopSelf()
            return
        }

        isRunning = true

        // TODO: pass tunFd.detachFd() to Go library
    }

    private fun disconnect() {
        isRunning = false
        tunFd?.close()
        tunFd = null
        stopForeground(STOP_FOREGROUND_REMOVE)
        stopSelf()
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
