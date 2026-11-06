package com.kairos.edgevpn.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class HelpTermTest {
    private val fullId = "12D3KooWQ1x9mN4oPqRsTuVwXyZaBcDeFgHiJkLmNoPqRsTuVwXy"

    private val laptopRow = DeviceRow(
        id = "12D3KooWLaptop", name = "laptop", address = "10.1.0.3", badge = Badge.VIA_HELPER,
        how = How.ThroughHelper, remote = "203.0.113.17:4001",
        remoteAddr = "/ip4/203.0.113.17/tcp/4001/p2p/12D3KooWRelay/p2p-circuit",
        transport = "tcp", limited = true, openSeconds = 48,
    )

    private val desktopRow = DeviceRow(
        id = "12D3KooWDesktop", name = "desktop", address = "10.1.0.1", badge = Badge.DIRECT,
        how = How.SameWifi, remote = "192.168.21.20:4001", remoteAddr = "/ip4/192.168.21.20/udp/4001/quic-v1",
        transport = "quic", limited = false, openSeconds = 600,
    )

    private val serverRow = DeviceRow(
        id = "12D3KooWServer", name = "home-server", address = "10.1.0.4", badge = Badge.OFFLINE,
        how = How.LastSeen(7200), remote = null, remoteAddr = null, transport = null, limited = false,
        openSeconds = null,
    )

    private val model = StatusModel(
        state = VpnState.CONNECTED, reachable = 2, total = 3, selfAddress = "10.1.0.2", selfId = fullId,
        runningSeconds = 125, error = null, devices = listOf(desktopRow, laptopRow, serverRow), helpers = 143,
        lastHeardSecondsAgo = 4, lastHeardName = "desktop", selfName = "pixel", failedSecondsAgo = null,
    )

    @Test
    fun oneTermPerGlossaryRow() {
        assertEquals(21, HelpTerm.values().size)
    }

    @Test
    fun viaHelperShowsRelayLimitAndAge() {
        val live = liveValues(HelpTerm.VIA_HELPER, model, laptopRow)
        assertTrue(live.contains("Relay address" to laptopRow.remoteAddr))
        assertTrue(live.contains("Limited" to "yes"))
        assertTrue(live.contains("Open for" to "48 s"))
    }

    @Test
    fun directShowsMultiaddrTransportAndAge() {
        val live = liveValues(HelpTerm.DIRECT, model, desktopRow)
        assertEquals(
            listOf(
                "Remote address" to "/ip4/192.168.21.20/udp/4001/quic-v1",
                "Transport" to "QUIC",
                "Open for" to "10 min",
            ),
            live,
        )
    }

    @Test
    fun badgeTermsWithoutADeviceHaveNoLiveValues() {
        assertTrue(liveValues(HelpTerm.DIRECT, model, null).isEmpty())
        assertTrue(liveValues(HelpTerm.VIA_HELPER, model, null).isEmpty())
        assertTrue(liveValues(HelpTerm.OFFLINE, model, null).isEmpty())
    }

    @Test
    fun offlineShowsLastHealthCheckWhenKnown() {
        assertEquals(listOf("Last health check" to "2 h ago"), liveValues(HelpTerm.OFFLINE, model, serverRow))
        val never = serverRow.copy(how = How.NotReachable)
        assertTrue(liveValues(HelpTerm.OFFLINE, model, never).isEmpty())
    }

    @Test
    fun unknownValuesAreLeftOut() {
        val unknown = laptopRow.copy(remoteAddr = null, openSeconds = null)
        val live = liveValues(HelpTerm.VIA_HELPER, model, unknown)
        assertEquals(listOf("Limited" to "yes"), live)
    }

    @Test
    fun termsWithoutLiveValuesAreEmpty() {
        for (t in listOf(HelpTerm.OFF, HelpTerm.FINDING, HelpTerm.NETWORK_KEY, HelpTerm.START_FRESH)) {
            assertTrue(t.name, liveValues(t, model, null).isEmpty())
        }
    }

    @Test
    fun phoneIdShowsTheFullId() {
        assertTrue(liveValues(HelpTerm.PHONE_ID, model, null).contains("Phone ID" to fullId))
    }

    @Test
    fun phoneTerms() {
        assertEquals(
            listOf("Address" to "10.1.0.2", "Phone ID" to fullId),
            liveValues(HelpTerm.THIS_PHONE, model, null),
        )
        assertEquals(
            listOf("Address" to "10.1.0.2", "Route" to "10.1.0.0/24"),
            liveValues(HelpTerm.ADDRESS, model, null),
        )
        assertEquals(listOf("Name" to "pixel"), liveValues(HelpTerm.NAME, model, null))
        assertEquals(listOf("Devices" to "3"), liveValues(HelpTerm.YOUR_DEVICES, model, null))
        assertEquals(listOf("Running for" to "2 min"), liveValues(HelpTerm.CONNECTED, model, null))
        assertEquals(listOf("Running for" to "2 min"), liveValues(HelpTerm.LOOKING, model, null))
        assertEquals(listOf("Helpers" to "143"), liveValues(HelpTerm.HELPERS, model, null))
        assertEquals(
            listOf("Last check-in" to "4 s ago", "From" to "desktop"),
            liveValues(HelpTerm.LAST_HEARD, model, null),
        )
    }

    @Test
    fun addressInUseShowsTheAddress() {
        val clash = model.copy(state = VpnState.LOOKING, addressInUse = true)
        assertEquals(listOf("Address" to "10.1.0.2"), liveValues(HelpTerm.ADDRESS_IN_USE, clash, null))
    }

    @Test
    fun sameNetworkShowsTheRemoteAddress() {
        assertEquals(
            listOf("Remote address" to "192.168.21.20:4001"),
            liveValues(HelpTerm.SAME_NETWORK, model, desktopRow),
        )
    }

    @Test
    fun problemShowsErrorAndWhen() {
        val failed = model.copy(state = VpnState.PROBLEM, error = "could not open the tunnel", failedSecondsAgo = 90)
        assertEquals(
            listOf("Error" to "could not open the tunnel", "When" to "1 min ago"),
            liveValues(HelpTerm.PROBLEM, failed, null),
        )
    }

    @Test
    fun problemShowsTheErrorDetailsWhenThereAreAny() {
        val failed = model.copy(
            state = VpnState.PROBLEM,
            error = "The VPN stopped unexpectedly.",
            detail = "node.Start returned error: boom",
            failedSecondsAgo = 5,
        )
        assertEquals(
            listOf(
                "Error" to "The VPN stopped unexpectedly.",
                "Error details" to "node.Start returned error: boom",
                "When" to "5 s ago",
            ),
            liveValues(HelpTerm.PROBLEM, failed, null),
        )
    }

    @Test
    fun phoneFactsFeedUsingAndDetailedLogs() {
        val facts = PhoneFacts(using = "Wi-Fi", detailedLogs = true)
        assertEquals(listOf("Using" to "Wi-Fi"), liveValues(HelpTerm.USING, model, null, facts))
        assertEquals(listOf("Detailed logs" to "On"), liveValues(HelpTerm.DETAILED_LOGS, model, null, facts))
        assertTrue(liveValues(HelpTerm.USING, model, null).isEmpty())
    }

    @Test
    fun durationsUsePlainUnits() {
        assertEquals("3 s", durationText(3))
        assertEquals("48 s", durationText(48))
        assertEquals("5 min", durationText(330))
        assertEquals("2 h", durationText(7300))
        assertEquals("1 day", durationText(86400))
        assertEquals("3 days", durationText(3 * 86400))
    }

    @Test
    fun everyTermHasALiveScope() {
        val expected = mapOf(
            HelpTerm.CONNECTED to LiveScope.PHONE,
            HelpTerm.LOOKING to LiveScope.PHONE,
            HelpTerm.OFF to LiveScope.PHONE,
            HelpTerm.PROBLEM to LiveScope.PHONE,
            HelpTerm.THIS_PHONE to LiveScope.PHONE,
            HelpTerm.USING to LiveScope.PHONE,
            HelpTerm.YOUR_DEVICES to LiveScope.NETWORK,
            HelpTerm.DIRECT to LiveScope.CONNECTION,
            HelpTerm.VIA_HELPER to LiveScope.CONNECTION,
            HelpTerm.OFFLINE to LiveScope.CONNECTION,
            HelpTerm.SAME_NETWORK to LiveScope.CONNECTION,
            HelpTerm.FINDING to LiveScope.PHONE,
            HelpTerm.HELPERS to LiveScope.PHONE,
            HelpTerm.LAST_HEARD to LiveScope.NETWORK,
            HelpTerm.NETWORK_KEY to LiveScope.PHONE,
            HelpTerm.NAME to LiveScope.PHONE,
            HelpTerm.ADDRESS to LiveScope.PHONE,
            HelpTerm.DETAILED_LOGS to LiveScope.PHONE,
            HelpTerm.PHONE_ID to LiveScope.PHONE,
            HelpTerm.START_FRESH to LiveScope.PHONE,
            HelpTerm.ADDRESS_IN_USE to LiveScope.PHONE,
        )
        assertEquals(HelpTerm.values().toSet(), expected.keys)
        for (t in HelpTerm.values()) assertEquals(t.name, expected[t], t.liveScope)
    }

    @Test
    fun badgeTermsFollowTheDevicesCurrentBadge() {
        assertEquals(HelpTerm.VIA_HELPER, HelpTerm.DIRECT.followBadge(laptopRow))
        assertEquals(HelpTerm.OFFLINE, HelpTerm.VIA_HELPER.followBadge(serverRow))
        assertEquals(HelpTerm.DIRECT, HelpTerm.DIRECT.followBadge(null))
        assertEquals(HelpTerm.SAME_NETWORK, HelpTerm.SAME_NETWORK.followBadge(laptopRow))
        assertEquals(HelpTerm.DIRECT, Badge.DIRECT.helpTerm)
    }
}
