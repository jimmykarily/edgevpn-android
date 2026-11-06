package com.kairos.edgevpn.ui

enum class MainButton { TURN_ON, TURN_OFF, TRY_AGAIN }

/** What the second "Finding your devices" tile shows, next to the public helpers. */
enum class SecondTile { NONE, LAST_HEARD, FOUND_SO_FAR }

/** Which parts of the main screen are visible for one snapshot. */
data class ScreenPlan(
    val button: MainButton,
    val showPills: Boolean,
    val showAddressPill: Boolean,
    val showMap: Boolean,
    /** The words under the map that explain its line styles; only useful when devices are drawn. */
    val showMapLegend: Boolean,
    /** The map draws only the phone with a pulsing ring while looking for devices. */
    val mapLooking: Boolean,
    val showDevices: Boolean,
    val showTiles: Boolean,
    val secondTile: SecondTile,
    val showSeeLogs: Boolean,
    /** Connected with exactly one device, which reads "You can reach your device." */
    val onlyDevice: Boolean,
    /** The warning on the status card that another device uses this phone's address. */
    val showAddressInUse: Boolean,
)

fun planScreen(m: StatusModel): ScreenPlan {
    val running = m.state == VpnState.LOOKING || m.state == VpnState.CONNECTED
    val secondTile = when {
        m.state == VpnState.LOOKING -> SecondTile.FOUND_SO_FAR
        m.state == VpnState.CONNECTED && m.lastHeardSecondsAgo != null -> SecondTile.LAST_HEARD
        else -> SecondTile.NONE
    }
    return ScreenPlan(
        button = when (m.state) {
            VpnState.OFF -> MainButton.TURN_ON
            VpnState.PROBLEM -> MainButton.TRY_AGAIN
            else -> MainButton.TURN_OFF
        },
        showPills = m.state != VpnState.OFF,
        showAddressPill = m.state != VpnState.OFF && m.selfAddress.isNotEmpty(),
        showMap = running,
        showMapLegend = m.state == VpnState.CONNECTED,
        mapLooking = m.state == VpnState.LOOKING,
        showDevices = running && m.devices.isNotEmpty(),
        showTiles = running,
        secondTile = secondTile,
        showSeeLogs = m.state == VpnState.PROBLEM,
        onlyDevice = m.state == VpnState.CONNECTED && m.total == 1,
        showAddressInUse = running && m.addressInUse && m.selfAddress.isNotEmpty(),
    )
}

/** The kind of network the phone uses to reach the internet, for the "Using" pill. */
enum class Using { WIFI, MOBILE_DATA, OTHER }

/** The parts of an Android network's capabilities that decide [Using]. */
data class NetFacts(val internet: Boolean, val vpn: Boolean, val wifi: Boolean, val cellular: Boolean)

/**
 * Picks the network under the VPN. While the VPN runs it is the active network,
 * so the VPN itself is skipped and Wi-Fi wins over mobile data. Null when no
 * network reaches the internet.
 */
fun pickUsing(networks: List<NetFacts>): Using? {
    val candidates = networks.filter { it.internet && !it.vpn }
    return when {
        candidates.any { it.wifi } -> Using.WIFI
        candidates.any { it.cellular } -> Using.MOBILE_DATA
        candidates.isNotEmpty() -> Using.OTHER
        else -> null
    }
}
