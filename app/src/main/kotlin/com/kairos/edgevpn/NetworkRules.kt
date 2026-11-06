package com.kairos.edgevpn

/** What the VPN needs to know about one non-VPN network. [id] is the network handle. */
data class NetworkInfo(
    val id: Long,
    val validated: Boolean,
    val wifi: Boolean,
    val ethernet: Boolean,
    val cellular: Boolean,
)

private fun rank(n: NetworkInfo): Int = when {
    n.wifi -> 0
    n.ethernet -> 1
    n.cellular -> 2
    else -> 3
}

/** Picks the network the VPN rides on when Android does not rank them itself (API 26 to 30):
 * validated first, then Wi-Fi, then Ethernet, then cellular, then the lowest id so the
 * choice stays the same between calls. */
fun pickNetwork(networks: Collection<NetworkInfo>): NetworkInfo? =
    networks.minWithOrNull(
        compareBy<NetworkInfo>({ !it.validated }, { rank(it) }, { it.id })
    )

/** True on a network that can have other devices on the same link, where mDNS makes sense. */
fun isLocalNetwork(n: NetworkInfo?): Boolean = n != null && (n.wifi || n.ethernet)

/** True while any non-VPN Wi-Fi or Ethernet network is connected, with or without internet
 * access. A Wi-Fi with only a LAN is still where mDNS finds peers, even when Android sends
 * the traffic over validated cellular. */
fun onLocalNetwork(connected: Collection<NetworkInfo>): Boolean = connected.any { isLocalNetwork(it) }

fun networkLabel(n: NetworkInfo?): String = when {
    n == null -> "none"
    n.wifi -> "wifi"
    n.ethernet -> "ethernet"
    n.cellular -> "cellular"
    else -> "other"
}

/** Lets a choice through when it is the first one or differs from the last one let through,
 * in the network or in the local flag. Android often repeats a callback without a change. */
class ReportFilter<T> {
    private var reported = false
    private var lastNetwork: T? = null
    private var lastLocal = false

    fun offer(network: T?, local: Boolean): Boolean {
        if (reported && network == lastNetwork && local == lastLocal) return false
        reported = true
        lastNetwork = network
        lastLocal = local
        return true
    }
}
