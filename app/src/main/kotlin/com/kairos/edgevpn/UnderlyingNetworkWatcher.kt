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
 * Watches the networks that are not a VPN and calls [onChange] with the best one and whether
 * the phone is on a local network. The first state is reported too, and after that only a
 * different best network or a different local flag. A null network means none with internet
 * is left.
 *
 * Two callbacks feed the state:
 * - Every connected Wi-Fi or Ethernet network, with or without internet, gives the local flag
 *   ([onLocalNetwork]). On a Wi-Fi with only a LAN Android ranks validated cellular first,
 *   and mDNS still has to run on that Wi-Fi.
 * - The best network with internet goes to setUnderlyingNetworks. The app's default network
 *   is the VPN itself, so the request asks for NOT_VPN. API 31 and later let Android rank the
 *   networks; before that every matching network is kept and [pickNetwork] chooses.
 *
 * The local callback is registered first, so its networks are usually known when the best
 * network is first reported. Go merges reports that arrive within 500 ms, so a first report
 * with the local flag still unknown is not acted on.
 *
 * Callbacks and [onChange] run on the main thread, and [start] and [stop] must be called
 * there too.
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

    /** The state of the running watch. Callbacks of a watch that is not this one have been
     * stopped and ignore anything Android still delivers to them. */
    private var active: Watch? = null

    fun start() {
        if (active != null) return
        val watch = Watch()
        active = watch
        val local = NetworkRequest.Builder()
            .removeCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET)
            .addCapability(NetworkCapabilities.NET_CAPABILITY_NOT_VPN)
            .addTransportType(NetworkCapabilities.TRANSPORT_WIFI)
            .addTransportType(NetworkCapabilities.TRANSPORT_ETHERNET)
            .build()
        val best = NetworkRequest.Builder()
            .addCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET)
            .addCapability(NetworkCapabilities.NET_CAPABILITY_NOT_VPN)
            .build()
        try {
            connectivity.registerNetworkCallback(local, watch.local, handler)
            if (bestMatching) {
                connectivity.registerBestMatchingNetworkCallback(best, watch.best, handler)
            } else {
                connectivity.registerNetworkCallback(best, watch.best, handler)
            }
        } catch (e: RuntimeException) {
            // Without the callbacks the VPN still works; Go only never learns of a local
            // network, so mDNS stays off.
            Log.e(TAG, "Could not watch the underlying network", e)
            stop()
        }
    }

    fun stop() {
        val watch = active ?: return
        active = null
        for (callback in listOf(watch.local, watch.best)) {
            try {
                connectivity.unregisterNetworkCallback(callback)
            } catch (e: IllegalArgumentException) {
                // Thrown for a callback that was never registered, when start failed halfway.
            }
        }
    }

    private inner class Watch {
        /** The connected Wi-Fi and Ethernet networks. */
        private val localKnown = HashMap<Network, NetworkInfo>()
        /** The networks the best one is chosen from. With the best-matching callback it holds
         * only the latest best network. */
        private val bestKnown = HashMap<Network, NetworkInfo>()
        private val filter = ReportFilter<Network>()

        val local = object : ConnectivityManager.NetworkCallback() {
            override fun onCapabilitiesChanged(network: Network, caps: NetworkCapabilities) {
                if (active !== this@Watch) return
                localKnown[network] = infoOf(network, caps)
                report()
            }

            override fun onLost(network: Network) {
                if (active !== this@Watch) return
                localKnown.remove(network)
                report()
            }
        }

        val best = object : ConnectivityManager.NetworkCallback() {
            override fun onAvailable(network: Network) {
                if (active !== this@Watch) return
                // The best-matching callback reports a new best network here, and its
                // capabilities follow in onCapabilitiesChanged.
                if (bestMatching) bestKnown.clear()
            }

            override fun onCapabilitiesChanged(network: Network, caps: NetworkCapabilities) {
                if (active !== this@Watch) return
                if (bestMatching) bestKnown.clear()
                bestKnown[network] = infoOf(network, caps)
                report()
            }

            override fun onLost(network: Network) {
                if (active !== this@Watch) return
                bestKnown.remove(network)
                report()
            }
        }

        private fun report() {
            val info = pickNetwork(bestKnown.values)
            val network = info?.let { picked -> bestKnown.entries.first { it.value == picked }.key }
            val local = onLocalNetwork(localKnown.values)
            if (!filter.offer(network, local)) return
            Log.i(TAG, "Underlying network: ${networkLabel(info)}, local network: $local")
            onChange(network, local)
        }
    }

    private fun infoOf(network: Network, caps: NetworkCapabilities) = NetworkInfo(
        id = network.networkHandle,
        validated = caps.hasCapability(NetworkCapabilities.NET_CAPABILITY_VALIDATED),
        wifi = caps.hasTransport(NetworkCapabilities.TRANSPORT_WIFI),
        ethernet = caps.hasTransport(NetworkCapabilities.TRANSPORT_ETHERNET),
        cellular = caps.hasTransport(NetworkCapabilities.TRANSPORT_CELLULAR),
    )
}
