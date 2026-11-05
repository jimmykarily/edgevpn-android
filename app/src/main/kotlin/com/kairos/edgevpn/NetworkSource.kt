package com.kairos.edgevpn

import edgevpn.Edgevpn

/**
 * Where the screens read the network's state from: the JSON snapshot of the Go node. The screen
 * tests replace [snapshot] to show a chosen state without a running node, and call [reset] after.
 */
object NetworkSource {
    private val fromNode: () -> String = { Edgevpn.snapshot() }

    @Volatile
    var snapshot: () -> String = fromNode

    fun reset() {
        snapshot = fromNode
    }
}
