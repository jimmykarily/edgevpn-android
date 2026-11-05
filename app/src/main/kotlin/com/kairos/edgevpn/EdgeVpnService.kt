package com.kairos.edgevpn

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.net.VpnService
import android.net.wifi.WifiManager
import android.os.Handler
import android.os.Looper
import android.os.ParcelFileDescriptor
import android.util.Log
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import androidx.core.app.NotificationCompat
import edgevpn.Edgevpn
import edgevpn.SocketProtector
import edgevpn.StatusListener

class EdgeVpnService : VpnService() {

    companion object {
        const val ACTION_CONNECT = "com.kairos.edgevpn.CONNECT"
        const val ACTION_DISCONNECT = "com.kairos.edgevpn.DISCONNECT"

        private const val TAG = "EdgeVpnService"
        private const val CHANNEL_ID = "edgevpn_status"
        private const val NOTIFICATION_ID = 1
        private const val TICK_MS = 5000L
        private const val TEXT_CONNECTED = "Connected"
        private const val TEXT_LOOKING = "Looking for your devices"

        var isRunning = false
            private set
    }

    private var tunPfd: ParcelFileDescriptor? = null
    private var multicastLock: WifiManager.MulticastLock? = null

    /** Wraps VpnService.protect(int) so Go can call it via the SocketProtector interface.
     * Must run on the main thread or protect() can fail with "operation not permitted". */
    private val socketProtector = object : SocketProtector {
        override fun protect(fd: Long): Boolean {
            if (Looper.getMainLooper().isCurrentThread) {
                val ok = this@EdgeVpnService.protect(fd.toInt())
                if (!ok) Log.w(TAG, "VpnService.protect($fd) returned false")
                return ok
            }
            val latch = CountDownLatch(1)
            val result = BooleanArray(1)
            val mainHandler = Handler(Looper.getMainLooper())
            mainHandler.postAtFrontOfQueue {
                result[0] = this@EdgeVpnService.protect(fd.toInt())
                if (!result[0]) Log.w(TAG, "VpnService.protect($fd) returned false")
                latch.countDown()
            }
            if (!latch.await(5, TimeUnit.SECONDS)) {
                Log.e(TAG, "VpnService.protect($fd) timed out (5s) waiting for main thread")
                return false
            }
            return result[0]
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

    private val mainHandler = Handler(Looper.getMainLooper())

    /** True once a start failure has been reported to Go. The remembered failure
     * must survive the service being destroyed, so onDestroy skips Edgevpn.stop(). */
    private var failed = false

    /** Counts start attempts so a late callback from an earlier one is ignored. */
    private var session = 0

    private var shownText: String? = null

    private val statusTicker = object : Runnable {
        override fun run() {
            val text = when (Edgevpn.status()) {
                "connected" -> TEXT_CONNECTED
                "looking" -> TEXT_LOOKING
                else -> return
            }
            if (text != shownText) {
                shownText = text
                getSystemService(NotificationManager::class.java)
                    .notify(NOTIFICATION_ID, buildNotification(text))
            }
            mainHandler.postDelayed(this, TICK_MS)
        }
    }

    private fun newStatusListener(mySession: Int) = object : StatusListener {
        override fun onStopped(reason: String?) {
            mainHandler.post {
                if (mySession != session) return@post
                Log.w(TAG, "Node stopped: $reason")
                failed = true
                teardown()
            }
        }
    }

    private fun connect() {
        if (tunPfd != null) return
        failed = false
        session++
        shownText = TEXT_LOOKING
        // Called first: a service started with startForegroundService that stops
        // before this call crashes the app.
        startForeground(NOTIFICATION_ID, buildNotification(TEXT_LOOKING))

        val token = TokenStore.load(this)
        if (token == null) {
            Log.e(TAG, "No token configured")
            stopSelf()
            return
        }

        val address = AppSettings.address(this)
        if (!isValidAddress(address)) {
            Log.e(TAG, "Invalid address in settings: $address")
            failStart("The address in your network is not valid.")
            return
        }

        // Loaded before the VPN interface exists, so a failure here has no
        // TUN descriptor to leak.
        val nodeKey = try {
            NodeKeyStore.loadOrCreate(this)
        } catch (e: Exception) {
            Log.e(TAG, "Failed to load or create the node key", e)
            failStart("The phone's identity key could not be loaded.")
            return
        }

        Edgevpn.setDetailedLogs(AppSettings.detailedLogs(this))
        Edgevpn.setCheckInSeconds(AppSettings.checkInSeconds(this).toLong())
        try {
            Edgevpn.setExtraPeers(AppSettings.extraPeers(this))
        } catch (e: Exception) {
            Log.e(TAG, "Invalid extra peers stored by a test: ${AppSettings.extraPeers(this)}", e)
            failStart("The VPN could not start. The logs have the details.")
            return
        }
        Edgevpn.setStatusListener(newStatusListener(session))

        val pfd = Builder()
            .addAddress(address, 24)
            .addRoute(routeFor(address), 24)
            .setMtu(1500)
            .setSession("EdgeVPN")
            .establish()

        if (pfd == null) {
            Log.e(TAG, "VPN establish() returned null")
            failStart("Android did not allow the VPN to start.")
            return
        }

        // Acquire MulticastLock so the WiFi chip delivers mDNS multicast packets
        // to the app. Without this, Android drops incoming multicast in hardware.
        val wifiManager = applicationContext.getSystemService(Context.WIFI_SERVICE) as WifiManager
        multicastLock = wifiManager.createMulticastLock("edgevpn-mdns").also {
            it.setReferenceCounted(true)
            it.acquire()
            Log.i(TAG, "MulticastLock acquired")
        }

        // Register the socket protector BEFORE starting Go networking.
        // Every socket fd created by libp2p will be passed through
        // VpnService.protect() so it bypasses the TUN interface.
        Edgevpn.setSocketProtector(socketProtector)

        val fd = pfd.detachFd()
        tunPfd = pfd

        try {
            Edgevpn.start(fd.toLong(), token, nodeKey, address, AppSettings.name(this))
            isRunning = true
            mainHandler.postDelayed(statusTicker, TICK_MS)
            Log.i(TAG, "VPN started")
        } catch (e: Exception) {
            Log.e(TAG, "Failed to start VPN", e)
            failStart("The VPN could not start with this network key. The logs have the details.")
        }
    }

    /** Tells Go why the start failed, then tears everything down. */
    private fun failStart(reason: String) {
        failed = true
        Edgevpn.reportStartFailure(reason)
        teardown()
    }

    private fun disconnect() {
        Edgevpn.stop()
        teardown()
        Log.i(TAG, "VPN stopped")
    }

    /** Releases everything the service holds, without touching Go's remembered failure. */
    private fun teardown() {
        isRunning = false
        tunPfd = null
        Edgevpn.setStatusListener(null)
        mainHandler.removeCallbacks(statusTicker)
        multicastLock?.let {
            if (it.isHeld) it.release()
            multicastLock = null
            Log.i(TAG, "MulticastLock released")
        }
        stopForeground(STOP_FOREGROUND_REMOVE)
        stopSelf()
    }

    override fun onRevoke() {
        disconnect()
    }

    override fun onDestroy() {
        if (failed) teardown() else disconnect()
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

    private fun buildNotification(text: String): Notification {
        val openApp = PendingIntent.getActivity(
            this, 0,
            Intent(this, MainActivity::class.java),
            PendingIntent.FLAG_IMMUTABLE
        )
        return NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle("EdgeVPN")
            .setContentText(text)
            .setSmallIcon(android.R.drawable.ic_lock_lock)
            .setContentIntent(openApp)
            .setOngoing(true)
            .build()
    }
}
