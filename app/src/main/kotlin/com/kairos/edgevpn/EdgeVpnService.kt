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
import android.system.OsConstants
import android.util.Log
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
     * It runs on the calling thread: protect() is a binder call to the system, and a hop
     * to the main thread would make every dial wait for the main looper. protect never
     * failed off the main thread in the network test runs on an API 35 emulator; vendor
     * phone builds are untested, so a "VpnService.protect(...) returned false" log line
     * is the sign to look for. A Java exception thrown into a gomobile callback aborts
     * the process, so any failure is reported as false instead. */
    private val socketProtector = object : SocketProtector {
        override fun protect(fd: Long): Boolean {
            val ok = try {
                this@EdgeVpnService.protect(fd.toInt())
            } catch (t: Throwable) {
                Log.w(TAG, "VpnService.protect($fd) threw $t")
                return false
            }
            if (!ok) Log.w(TAG, "VpnService.protect($fd) returned false")
            return ok
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
                startForeground(NOTIFICATION_ID, buildNotification(text))
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
            // The VPN carries only IPv4. Without this Android gives the VPN an
            // unreachable ::/0 route and no app can use IPv6. With Android's
            // "Always-on VPN, block connections without VPN" IPv6 may now bypass
            // the VPN; that is acceptable because the VPN carries only
            // 10.1.0.0/24 (untested).
            .allowFamily(OsConstants.AF_INET6)
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
        Edgevpn.setSocketProtector(null)
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
