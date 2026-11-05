package com.kairos.edgevpn.ui

/** One term the app explains with a (?) help sheet, one per row of the glossary. */
enum class HelpTerm {
    CONNECTED, LOOKING, OFF, PROBLEM, THIS_PHONE, USING, YOUR_DEVICES, DIRECT, VIA_HELPER, OFFLINE,
    SAME_NETWORK, FINDING, HELPERS, LAST_HEARD, NETWORK_KEY, NAME, ADDRESS, DETAILED_LOGS, PHONE_ID, START_FRESH;

    /** What the live values of this term describe, which names their heading in the sheet. */
    val liveScope: LiveScope
        get() = when (this) {
            DIRECT, VIA_HELPER, OFFLINE, SAME_NETWORK -> LiveScope.CONNECTION
            YOUR_DEVICES, LAST_HEARD -> LiveScope.NETWORK
            CONNECTED, LOOKING, OFF, PROBLEM, THIS_PHONE, USING, FINDING, HELPERS, NETWORK_KEY, NAME,
            ADDRESS, DETAILED_LOGS, PHONE_ID, START_FRESH -> LiveScope.PHONE
        }

    /**
     * For a badge term about [device], the term of the badge the device has now, so a sheet
     * opened on "Direct" follows the device when it moves to a helper. Other terms stay as they are.
     */
    fun followBadge(device: DeviceRow?): HelpTerm =
        if (device != null && (this == DIRECT || this == VIA_HELPER || this == OFFLINE)) device.badge.helpTerm else this
}

/** What a help sheet's live values describe: one device's connection, this phone, or the whole network. */
enum class LiveScope { CONNECTION, PHONE, NETWORK }

val Badge.helpTerm: HelpTerm
    get() = when (this) {
        Badge.DIRECT -> HelpTerm.DIRECT
        Badge.VIA_HELPER -> HelpTerm.VIA_HELPER
        Badge.OFFLINE -> HelpTerm.OFFLINE
    }

/** Facts about this phone that the status snapshot does not carry. */
data class PhoneFacts(val using: String? = null, val detailedLogs: Boolean? = null)

/** A length of time in plain units: "48 s", "5 min", "2 h", "3 days". */
fun durationText(seconds: Long): String = when (val ago = formatAgo(seconds)) {
    Ago.JustNow -> "${maxOf(0L, seconds)} s"
    is Ago.Seconds -> "${ago.n} s"
    is Ago.Minutes -> "${ago.n} min"
    is Ago.Hours -> "${ago.n} h"
    is Ago.Days -> if (ago.n == 1L) "1 day" else "${ago.n} days"
}

private fun agoText(seconds: Long) = "${durationText(seconds)} ago"

/** The /24 route the VPN sends through the tunnel for [address], or null if it is not IPv4. */
private fun routeOf(address: String): String? {
    val parts = address.split(".")
    if (parts.size != 4) return null
    return "${parts[0]}.${parts[1]}.${parts[2]}.0/24"
}

/**
 * The live values a help sheet shows for [term], as label and value pairs. Values that are
 * unknown are left out. [device] is the device the sheet is about, or null for a general sheet.
 */
fun liveValues(
    term: HelpTerm,
    model: StatusModel,
    device: DeviceRow?,
    phone: PhoneFacts = PhoneFacts(),
): List<Pair<String, String>> {
    val out = mutableListOf<Pair<String, String>>()
    fun add(label: String, value: String?) {
        if (!value.isNullOrEmpty()) out += label to value
    }
    when (term) {
        HelpTerm.CONNECTED, HelpTerm.LOOKING -> add("Running for", model.runningSeconds?.let(::durationText))
        HelpTerm.PROBLEM -> {
            add("Error", model.error)
            add("Error details", model.detail)
            add("When", model.failedSecondsAgo?.let(::agoText))
        }
        HelpTerm.THIS_PHONE -> {
            add("Address", model.selfAddress)
            add("Phone ID", model.selfId)
        }
        HelpTerm.USING -> add("Using", phone.using)
        HelpTerm.YOUR_DEVICES -> add("Devices", model.total.toString())
        HelpTerm.DIRECT -> if (device != null) {
            add("Remote address", device.remoteAddr)
            add("Transport", device.transport?.uppercase())
            add("Open for", device.openSeconds?.let(::durationText))
        }
        HelpTerm.VIA_HELPER -> if (device != null) {
            add("Relay address", device.remoteAddr)
            add("Limited", if (device.limited) "yes" else "no")
            add("Open for", device.openSeconds?.let(::durationText))
        }
        HelpTerm.OFFLINE -> (device?.how as? How.LastSeen)?.let { add("Last health check", agoText(it.secondsAgo)) }
        HelpTerm.SAME_NETWORK -> add("Remote address", device?.remote)
        HelpTerm.HELPERS -> add("Helpers", model.helpers.toString())
        HelpTerm.LAST_HEARD -> {
            add("Last check-in", model.lastHeardSecondsAgo?.let(::agoText))
            add("From", model.lastHeardName)
        }
        HelpTerm.NAME -> add("Name", model.selfName)
        HelpTerm.ADDRESS -> {
            add("Address", model.selfAddress)
            add("Route", routeOf(model.selfAddress))
        }
        HelpTerm.DETAILED_LOGS -> add("Detailed logs", phone.detailedLogs?.let { if (it) "On" else "Off" })
        HelpTerm.PHONE_ID -> add("Phone ID", model.selfId)
        HelpTerm.OFF, HelpTerm.FINDING, HelpTerm.NETWORK_KEY, HelpTerm.START_FRESH -> Unit
    }
    return out
}
