package com.kairos.edgevpn

private val ADDRESS_PART = Regex("0|[1-9][0-9]{0,2}")

fun isValidAddress(s: String): Boolean {
    val parts = s.split(".")
    return parts.size == 4 && parts.all { ADDRESS_PART.matches(it) && it.toInt() <= 255 }
}

// The /24 route that covers the address: same first three numbers, last one 0.
fun routeFor(address: String): String = address.substringBeforeLast('.') + ".0"
