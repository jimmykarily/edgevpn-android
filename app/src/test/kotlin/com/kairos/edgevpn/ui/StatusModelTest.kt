package com.kairos.edgevpn.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class StatusModelTest {
    private fun device(
        id: String,
        name: String,
        reach: String,
        extra: String = "",
    ) = """{"id":"$id","name":"$name","address":"10.1.0.$id","reach":"$reach","remote":"1.2.3.4","remoteAddr":"1.2.3.4:4001","transport":"tcp","limited":false,"openedAt":900,"sameNetwork":false$extra}"""

    private fun snapshot(devices: String, state: String = "connected") =
        """{"state":"$state","startedAt":400,"error":"","failedAt":0,"self":{"id":"me","address":"10.1.0.2","name":"Phone"},"devices":[$devices],"helpers":3,"lastHeard":{"at":950,"name":"laptop"}}"""

    @Test
    fun sortsDirectThenHelperThenOfflineThenByNameWithUnnamedLast() {
        val json = snapshot(
            listOf(
                device("1", "zeta", "offline"),
                device("2", "beta", "helper"),
                device("3", "Bravo", "direct"),
                device("4", "", "direct"),
                device("5", "alpha", "direct"),
                device("6", "Alpha", "helper"),
            ).joinToString(","),
        )
        val ids = parseStatus(json, 1000).devices.map { it.id }
        assertEquals(listOf("5", "3", "4", "6", "2", "1"), ids)
    }

    @Test
    fun localhostAndEmptyNamesAreNull() {
        val m = parseStatus(snapshot(device("1", "localhost", "direct") + "," + device("2", "", "direct")), 1000)
        assertNull(m.devices[0].name)
        assertNull(m.devices[1].name)
    }

    @Test
    fun countsReachableAndTotal() {
        val m = parseStatus(
            snapshot(
                listOf(device("1", "a", "direct"), device("2", "b", "helper"), device("3", "c", "offline")).joinToString(","),
            ),
            1000,
        )
        assertEquals(2, m.reachable)
        assertEquals(3, m.total)
    }

    @Test
    fun howPerDevice() {
        val m = parseStatus(
            snapshot(
                listOf(
                    device("1", "a", "direct").replace("\"sameNetwork\":false", "\"sameNetwork\":true"),
                    device("2", "b", "direct"),
                    device("3", "c", "helper"),
                    device("4", "d", "offline", ",\"lastSeen\":940"),
                    device("5", "e", "offline"),
                ).joinToString(","),
            ),
            1000,
        )
        val by = m.devices.associateBy { it.id }
        assertEquals(How.SameWifi, by["1"]!!.how)
        assertEquals(How.OverInternet, by["2"]!!.how)
        assertEquals(How.ThroughHelper, by["3"]!!.how)
        assertEquals(How.LastSeen(60), by["4"]!!.how)
        assertEquals(How.NotReachable, by["5"]!!.how)
        assertEquals(Badge.VIA_HELPER, by["3"]!!.badge)
        assertEquals(Badge.OFFLINE, by["5"]!!.badge)
    }

    @Test
    fun parsesFullSnapshot() {
        val m = parseStatus(snapshot(device("1", "laptop", "direct")), 1000)
        assertEquals(VpnState.CONNECTED, m.state)
        assertEquals("10.1.0.2", m.selfAddress)
        assertEquals("me", m.selfId)
        assertEquals(600L, m.runningSeconds)
        assertNull(m.error)
        assertEquals(3, m.helpers)
        assertEquals(50L, m.lastHeardSecondsAgo)
        assertEquals("laptop", m.lastHeardName)
        val d = m.devices.single()
        assertEquals("laptop", d.name)
        assertEquals("1.2.3.4", d.remote)
        assertEquals("1.2.3.4:4001", d.remoteAddr)
        assertEquals("tcp", d.transport)
        assertEquals(100L, d.openSeconds)
    }

    @Test
    fun offSnapshotWithoutDevicesKeyIsEmpty() {
        val m = parseStatus("""{"state":"off","startedAt":0,"error":"","failedAt":0,"self":{"id":"","address":"","name":""},"helpers":0}""", 1000)
        assertEquals(VpnState.OFF, m.state)
        assertEquals(emptyList<DeviceRow>(), m.devices)
        assertNull(m.runningSeconds)
        assertNull(m.lastHeardSecondsAgo)
        assertNull(m.lastHeardName)
    }

    @Test
    fun problemCarriesErrorAndUnknownValuesFallBack() {
        val m = parseStatus(
            """{"state":"weird","error":"boom","devices":[{"id":"1","name":"x","address":"a","reach":"???"}]}""",
            10,
        )
        assertEquals(VpnState.OFF, m.state)
        assertEquals("boom", m.error)
        assertNull(m.detail)
        assertEquals("raw boom", parseStatus("""{"state":"problem","error":"e","detail":"raw boom"}""", 1).detail)
        assertEquals(Badge.OFFLINE, m.devices[0].badge)
        assertEquals(VpnState.PROBLEM, parseStatus("""{"state":"problem","error":"e"}""", 1).state)
        assertEquals(VpnState.LOOKING, parseStatus("""{"state":"looking"}""", 1).state)
    }

    @Test
    fun readsOwnNameAndFailureTime() {
        val m = parseStatus(snapshot("").replace("\"failedAt\":0", "\"failedAt\":880"), 1000)
        assertEquals("Phone", m.selfName)
        assertEquals(120L, m.failedSecondsAgo)
        assertNull(parseStatus(snapshot(""), 1000).failedSecondsAgo)
    }

    @Test
    fun negativeOpenSecondsClampToZero() {
        val m = parseStatus(snapshot(device("1", "a", "direct")), 500)
        assertEquals(0L, m.devices[0].openSeconds)
    }

    @Test
    fun formatAgoBuckets() {
        assertEquals(Ago.JustNow, formatAgo(-30))
        assertEquals(Ago.JustNow, formatAgo(9))
        assertEquals(Ago.Seconds(45), formatAgo(45))
        assertEquals(Ago.Minutes(5), formatAgo(300))
        assertEquals(Ago.Hours(2), formatAgo(7200))
        assertEquals(Ago.Days(2), formatAgo(2 * 86400L))
    }

    @Test
    fun readsWhetherAnotherDeviceClaimsThisPhonesAddress() {
        val clash = snapshot("", state = "looking").replace("\"helpers\":3", "\"helpers\":3,\"addressInUse\":true")
        assertTrue(parseStatus(clash, 1000).addressInUse)
        assertFalse(parseStatus(snapshot(""), 1000).addressInUse)
    }
}
