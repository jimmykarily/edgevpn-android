package com.kairos.edgevpn.ui

/**
 * One place on the map's circle. It holds either the device at [deviceIndex]
 * or, for the last place when there are too many devices, a bubble for the
 * [moreCount] devices left out.
 */
data class MapSlot(val angleDegrees: Float, val deviceIndex: Int?, val moreCount: Int?)

/**
 * Spreads [deviceCount] devices evenly on a circle, the first at the top
 * (-90 degrees) and the rest clockwise. Beyond [max] devices, the first
 * `max - 1` get a place and the last place says how many are left.
 */
fun mapSlots(deviceCount: Int, max: Int = 8): List<MapSlot> {
    if (deviceCount <= 0) return emptyList()
    val count = minOf(deviceCount, max)
    val overflow = deviceCount > max
    return (0 until count).map { i ->
        val angle = -90f + i * 360f / count
        if (overflow && i == count - 1) {
            MapSlot(angle, null, deviceCount - (max - 1))
        } else {
            MapSlot(angle, i, null)
        }
    }
}

/** A device name short enough to sit under its icon: at most [max] characters. */
fun mapLabel(name: String, max: Int = 10): String =
    if (name.length <= max) name else name.take(max - 1) + "…"
