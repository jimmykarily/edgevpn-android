package com.kairos.edgevpn.ui

import org.json.JSONObject

enum class VpnState { OFF, LOOKING, CONNECTED, PROBLEM }

enum class Badge { DIRECT, VIA_HELPER, OFFLINE }

sealed interface How {
    object SameWifi : How
    object OverInternet : How
    object ThroughHelper : How
    data class LastSeen(val secondsAgo: Long) : How
    object NotReachable : How
}

/** A null [name] means the device announced nothing useful; show "Unnamed device". */
data class DeviceRow(
    val id: String,
    val name: String?,
    val address: String,
    val badge: Badge,
    val how: How,
    val remote: String?,
    val remoteAddr: String?,
    val transport: String?,
    val limited: Boolean,
    val openSeconds: Long?,
)

data class StatusModel(
    val state: VpnState,
    val reachable: Int,
    val total: Int,
    val selfAddress: String,
    val selfId: String,
    val runningSeconds: Long?,
    val error: String?,
    val devices: List<DeviceRow>,
    val helpers: Int,
    val lastHeardSecondsAgo: Long?,
    val lastHeardName: String?,
    val selfName: String = "",
    val failedSecondsAgo: Long? = null,
    /** The raw error behind [error], for the Problem help sheet only. */
    val detail: String? = null,
    /** True while another device claims this phone's address, which keeps this phone out of the device list. */
    val addressInUse: Boolean = false,
    /** True while a Wi-Fi or Ethernet network the phone is on uses addresses in the VPN's range. */
    val localNetworkClash: Boolean = false,
)

sealed interface Ago {
    object JustNow : Ago
    data class Seconds(val n: Long) : Ago
    data class Minutes(val n: Long) : Ago
    data class Hours(val n: Long) : Ago
    data class Days(val n: Long) : Ago
}

fun formatAgo(seconds: Long): Ago = when {
    seconds < 10 -> Ago.JustNow
    seconds < 60 -> Ago.Seconds(seconds)
    seconds < 3600 -> Ago.Minutes(seconds / 60)
    seconds < 86400 -> Ago.Hours(seconds / 3600)
    else -> Ago.Days(seconds / 86400)
}

private fun parseState(s: String) = when (s) {
    "looking" -> VpnState.LOOKING
    "connected" -> VpnState.CONNECTED
    "problem" -> VpnState.PROBLEM
    else -> VpnState.OFF
}

private fun nonEmpty(s: String): String? = s.ifEmpty { null }

private fun parseDevice(o: JSONObject, now: Long): DeviceRow {
    val badge = when (o.optString("reach")) {
        "direct" -> Badge.DIRECT
        "helper" -> Badge.VIA_HELPER
        else -> Badge.OFFLINE
    }
    val lastSeen = o.optLong("lastSeen", 0)
    val how = when (badge) {
        Badge.DIRECT -> if (o.optBoolean("sameNetwork")) How.SameWifi else How.OverInternet
        Badge.VIA_HELPER -> How.ThroughHelper
        Badge.OFFLINE -> if (lastSeen > 0) How.LastSeen(now - lastSeen) else How.NotReachable
    }
    val name = o.optString("name")
    val openedAt = o.optLong("openedAt", 0)
    return DeviceRow(
        id = o.optString("id"),
        name = if (name.isEmpty() || name == "localhost") null else name,
        address = o.optString("address"),
        badge = badge,
        how = how,
        remote = nonEmpty(o.optString("remote")),
        remoteAddr = nonEmpty(o.optString("remoteAddr")),
        transport = nonEmpty(o.optString("transport")),
        limited = o.optBoolean("limited"),
        openSeconds = if (openedAt > 0) maxOf(0L, now - openedAt) else null,
    )
}

fun parseStatus(json: String, nowSeconds: Long): StatusModel {
    val root = JSONObject(json)
    val self = root.optJSONObject("self")
    val arr = root.optJSONArray("devices")
    val parsed = (0 until (arr?.length() ?: 0)).map { parseDevice(arr!!.getJSONObject(it), nowSeconds) }
    val devices = parsed.sortedWith(
        compareBy<DeviceRow> { it.badge.ordinal }
            .thenBy { it.name == null }
            .thenBy { it.name?.lowercase() ?: "" },
    )
    val startedAt = root.optLong("startedAt", 0)
    val heard = root.optJSONObject("lastHeard")
    return StatusModel(
        state = parseState(root.optString("state")),
        reachable = devices.count { it.badge != Badge.OFFLINE },
        total = devices.size,
        selfAddress = self?.optString("address") ?: "",
        selfId = self?.optString("id") ?: "",
        runningSeconds = if (startedAt > 0) maxOf(0L, nowSeconds - startedAt) else null,
        error = nonEmpty(root.optString("error")),
        devices = devices,
        helpers = root.optInt("helpers", 0),
        lastHeardSecondsAgo = heard?.let { nowSeconds - it.optLong("at") },
        lastHeardName = heard?.let { nonEmpty(it.optString("name")) },
        selfName = self?.optString("name") ?: "",
        failedSecondsAgo = root.optLong("failedAt", 0).takeIf { it > 0 }?.let { maxOf(0L, nowSeconds - it) },
        detail = nonEmpty(root.optString("detail")),
        addressInUse = root.optBoolean("addressInUse"),
        localNetworkClash = root.optBoolean("localNetworkClash"),
    )
}
