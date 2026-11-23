package com.kairos.edgevpn

/** [id] with its middle cut out: the first 8 characters, "…", and the last 4. Short IDs stay whole. */
fun shortId(id: String): String = if (id.length <= 13) id else id.take(8) + "…" + id.takeLast(4)

/** What saving an address typed in the settings does. */
enum class AddressChange { INVALID, UNCHANGED, SAVE, SAVE_AND_ASK_RECONNECT }

/**
 * What to do with the address [entered] in place of [current]. A running VPN keeps the address it
 * started with, so a new address while it runs needs a reconnect, which the user is asked about.
 */
fun addressChange(current: String, entered: String, vpnRunning: Boolean): AddressChange {
    val address = entered.trim()
    return when {
        !isValidAddress(address) -> AddressChange.INVALID
        address == current -> AddressChange.UNCHANGED
        vpnRunning -> AddressChange.SAVE_AND_ASK_RECONNECT
        else -> AddressChange.SAVE
    }
}

/**
 * Whether switching a setting that the VPN reads only when it starts, such as "Relay for other
 * devices" or "Save mobile data", to [chosen] asks the user to reconnect. A running VPN keeps the
 * value it started with, [runningWith], until it reconnects.
 */
fun startSettingChangeAsksReconnect(vpnRunning: Boolean, runningWith: Boolean, chosen: Boolean): Boolean =
    vpnRunning && runningWith != chosen
