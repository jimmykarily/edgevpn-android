package com.kairos.edgevpn

import android.content.Context
import android.net.ConnectivityManager
import android.net.Network
import android.net.NetworkCapabilities
import android.net.NetworkRequest
import android.os.Build
import android.os.Handler
import android.os.Looper
import android.util.Log

/**
 * Watches the best network that is not a VPN and calls [onChange] with it and whether it is a
 * local network (Wi-Fi or Ethernet). The first choice is reported too, and after that only a
 * different network or a different local flag. A null network means none is left.
 *
 * The app's default network is the VPN itself, so the watcher asks for NOT_VPN networks.
 * API 31 and later let Android rank them; before that every matching network is kept and
 * [pickNetwork] chooses. Callbacks and [onChange] run on the main thread, and [start] and
 * [stop] must be called there too.
 */
class UnderlyingNetworkWatcher(
    context: Context,
    private val onChange: (Network?, Boolean) -> Unit,
) {
    companion object {
        private const val TAG = "EdgeVpnService"
    }

    private val connectivity = context.getSystemService(ConnectivityManager::class.java)
    private val handler = Handler(Looper.getMainLooper())
    private val bestMatching = Build.VERSION.SDK_INT >= Build.VERSION_CODES.S

    /** The registered callback. A callback that is not this one has been stopped and
     * ignores anything Android still delivers to it. */
    private var active: Callback? = null

    fun start() {
        if (active != null) return
        val request = NetworkRequest.Builder()
            .addCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET)
            .addCapability(NetworkCapabilities.NET_CAPABILITY_NOT_VPN)
            .build()
        val callback = Callback()
        active = callback
        try {
            if (bestMatching) {
                connectivity.registerBestMatchingNetworkCallback(request, callback, handler)
            } else {
                connectivity.registerNetworkCallback(request, callback, handler)
            }
        } catch (e: RuntimeException) {
            // Without the callback the VPN still works; Go only never learns of a local
            // network, so mDNS stays off.
            active = null
            Log.e(TAG, "Could not watch the underlying network", e)
        }
    }

    fun stop() {
        val callback = active ?: return
        active = null
        try {
            connectivity.unregisterNetworkCallback(callback)
        } catch (e: IllegalArgumentException) {
            Log.w(TAG, "Network callback was not registered", e)
        }
    }

    private inner class Callback : ConnectivityManager.NetworkCallback() {
        /** The networks the choice is made from. With the best-matching callback it holds
         * only the latest best network. */
        private val known = HashMap<Network, NetworkInfo>()
        private val filter = ReportFilter<Network>()

        override fun onAvailable(network: Network) {
            if (active !== this) return
            // The best-matching callback reports a new best network here, and its
            // capabilities follow in onCapabilitiesChanged.
            if (bestMatching) known.clear()
        }

        override fun onCapabilitiesChanged(network: Network, caps: NetworkCapabilities) {
            if (active !== this) return
            if (bestMatching) known.clear()
            known[network] = NetworkInfo(
                id = network.networkHandle,
                validated = caps.hasCapability(NetworkCapabilities.NET_CAPABILITY_VALIDATED),
                wifi = caps.hasTransport(NetworkCapabilities.TRANSPORT_WIFI),
                ethernet = caps.hasTransport(NetworkCapabilities.TRANSPORT_ETHERNET),
                cellular = caps.hasTransport(NetworkCapabilities.TRANSPORT_CELLULAR),
            )
            report()
        }

        override fun onLost(network: Network) {
            if (active !== this) return
            known.remove(network)
            report()
        }

        private fun report() {
            val info = pickNetwork(known.values)
            val network = info?.let { picked -> known.entries.first { it.value == picked }.key }
            val local = isLocalNetwork(info)
            if (!filter.offer(network, local)) return
            Log.i(TAG, "Underlying network: ${networkLabel(info)}")
            onChange(network, local)
        }
    }
}
