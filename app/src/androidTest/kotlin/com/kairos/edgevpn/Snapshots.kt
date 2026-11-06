package com.kairos.edgevpn

import org.json.JSONArray
import org.json.JSONObject

/**
 * Network snapshots in the shape golib/snapshot.go writes, for showing a screen in a chosen state
 * without a running node. Times are relative to [now], in Unix seconds, so "2 h ago" stays true.
 */
object Snapshots {

    fun now(): Long = System.currentTimeMillis() / 1000

    fun off(): String = snapshot(state = "off").toString()

    fun looking(now: Long = now(), helpers: Int = 3): String =
        snapshot(state = "looking", startedAt = now - 40, helpers = helpers).toString()

    /**
     * Connected to two of three devices. They are listed in the order Go writes them (by name),
     * which differs from the order the screen shows them in (by badge).
     */
    fun connectedWithThreeDevices(now: Long = now()): String = snapshot(
        state = "connected",
        startedAt = now - 600,
        helpers = 4,
        devices = listOf(
            device(
                id = "12D3KooWAtticPc", name = "attic-pc", address = "10.1.0.4",
                reach = "offline", lastSeen = now - 7200,
            ),
            device(
                id = "12D3KooWDesktop", name = "desktop", address = "10.1.0.1",
                reach = "direct", remote = "192.168.21.20:4001",
                remoteAddr = "/ip4/192.168.21.20/tcp/4001", transport = "tcp",
                openedAt = now - 300, sameNetwork = true, lastSeen = now - 30,
            ),
            device(
                id = "12D3KooWLaptop", name = "laptop", address = "10.1.0.3",
                reach = "helper", remote = "203.0.113.7:4001",
                remoteAddr = RELAY_ADDR, transport = "tcp",
                limited = true, openedAt = now - 90, lastSeen = now - 60,
            ),
        ),
        lastHeard = JSONObject().put("at", now - 30).put("name", "desktop"),
    ).toString()

    /** Looking for devices while another device claims this phone's address 10.1.0.2. */
    fun lookingWithAddressInUse(now: Long = now()): String =
        snapshot(state = "looking", startedAt = now - 40, helpers = 3).put("addressInUse", true).toString()

    fun problem(error: String, now: Long = now()): String =
        snapshot(state = "problem", error = error, failedAt = now - 20).toString()

    const val RELAY_ADDR = "/ip4/203.0.113.7/tcp/4001/p2p/12D3KooWRelay/p2p-circuit"

    private fun snapshot(
        state: String,
        startedAt: Long = 0,
        error: String = "",
        failedAt: Long = 0,
        helpers: Int = 0,
        devices: List<JSONObject> = emptyList(),
        lastHeard: JSONObject? = null,
    ): JSONObject = JSONObject()
        .put("state", state)
        .put("startedAt", startedAt)
        .put("error", error)
        .put("failedAt", failedAt)
        .put(
            "self",
            JSONObject().put("id", "12D3KooWThisPhone").put("address", "10.1.0.2").put("name", "Test phone"),
        )
        .put("devices", JSONArray(devices))
        .put("helpers", helpers)
        .apply { if (lastHeard != null) put("lastHeard", lastHeard) }

    private fun device(
        id: String,
        name: String,
        address: String,
        reach: String,
        remote: String = "",
        remoteAddr: String = "",
        transport: String = "",
        limited: Boolean = false,
        openedAt: Long = 0,
        sameNetwork: Boolean = false,
        lastSeen: Long = 0,
    ): JSONObject = JSONObject()
        .put("id", id)
        .put("name", name)
        .put("address", address)
        .put("reach", reach)
        .put("remote", remote)
        .put("remoteAddr", remoteAddr)
        .put("transport", transport)
        .put("limited", limited)
        .put("openedAt", openedAt)
        .put("sameNetwork", sameNetwork)
        .apply { if (lastSeen > 0) put("lastSeen", lastSeen) }
}
