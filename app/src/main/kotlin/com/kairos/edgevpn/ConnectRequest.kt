package com.kairos.edgevpn

import java.util.concurrent.atomic.AtomicBoolean

/**
 * The Join screen's request that the main screen turn the VPN on, taken once. It is held in this
 * process's memory because the main screen is exported: any app on the phone can open it with an
 * intent and any extra, so an intent cannot carry the request.
 */
object ConnectRequest {
    private val pending = AtomicBoolean(false)

    fun ask() = pending.set(true)

    /** True once after [ask], then false until the next [ask]. */
    fun take(): Boolean = pending.getAndSet(false)
}
