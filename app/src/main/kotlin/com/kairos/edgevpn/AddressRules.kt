package com.kairos.edgevpn

private val ADDRESS_PART = Regex("0|[1-9][0-9]{0,2}")

fun isValidAddress(s: String): Boolean {
    val parts = s.split(".")
    return parts.size == 4 && parts.all { ADDRESS_PART.matches(it) && it.toInt() <= 255 }
}

// The /24 route that covers the address: same first three numbers, last one 0.
fun routeFor(address: String): String = address.substringBeforeLast('.') + ".0"

/** An IPv4 or IPv6 address of a network interface with its prefix length, as Android reports it. */
data class LinkPrefix(val address: String, val prefixLength: Int)

private fun ipv4ToInt(s: String): Int? {
    if (!isValidAddress(s)) return null
    return s.split(".").fold(0) { acc, part -> (acc shl 8) or part.toInt() }
}

/**
 * True when one of [prefixes] overlaps the /24 the VPN routes into the tunnel for [vpnAddress]:
 * the two networks share their first min(24, prefix length) bits. IPv6 and malformed entries
 * never clash.
 */
fun clashesWithVpn(vpnAddress: String, prefixes: Collection<LinkPrefix>): Boolean {
    val vpn = ipv4ToInt(vpnAddress) ?: return false
    return prefixes.any { p ->
        val addr = ipv4ToInt(p.address)
        if (addr == null || p.prefixLength !in 0..32) return@any false
        val bits = minOf(24, p.prefixLength)
        val mask = if (bits == 0) 0 else -1 shl (32 - bits)
        (addr and mask) == (vpn and mask)
    }
}

/** The VPN's range for people to read: "10.1.0.x" for 10.1.0.2. */
fun vpnRangeLabel(vpnAddress: String): String = vpnAddress.substringBeforeLast('.') + ".x"
