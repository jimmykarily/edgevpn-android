package com.kairos.edgevpn

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class AddressRulesTest {

    @Test
    fun acceptsFourDecimalNumbers() {
        assertTrue(isValidAddress("10.1.0.2"))
        assertTrue(isValidAddress("192.168.5.1"))
    }

    @Test
    fun rejectsMalformedAddresses() {
        for (s in listOf("", "10.1.0", "10.1.0.256", "10.1.0.02", " 10.1.0.2", "a.b.c.d", "10.1.0.2/24")) {
            assertFalse("'$s' should be invalid", isValidAddress(s))
        }
    }

    @Test
    fun routeKeepsFirstThreeNumbers() {
        assertEquals("10.1.0.0", routeFor("10.1.0.2"))
        assertEquals("192.168.5.0", routeFor("192.168.5.77"))
    }

    @Test
    fun aLocalNetworkInsideTheVpnRangeClashes() {
        assertTrue(clashesWithVpn("10.1.0.2", listOf(LinkPrefix("10.1.0.35", 24))))
        // A smaller network inside the VPN's /24.
        assertTrue(clashesWithVpn("10.1.0.2", listOf(LinkPrefix("10.1.0.130", 25))))
        // A bigger network that contains the VPN's /24.
        assertTrue(clashesWithVpn("192.168.21.4", listOf(LinkPrefix("192.168.20.20", 23))))
    }

    @Test
    fun aLocalNetworkOutsideTheVpnRangeDoesNotClash() {
        assertFalse(clashesWithVpn("10.1.0.2", listOf(LinkPrefix("192.168.21.35", 23))))
        assertFalse(clashesWithVpn("10.1.0.2", listOf(LinkPrefix("10.1.1.35", 24))))
        assertFalse(clashesWithVpn("192.168.2.4", listOf(LinkPrefix("192.168.20.20", 23))))
        assertFalse(clashesWithVpn("10.1.0.2", emptyList()))
    }

    @Test
    fun anyClashingNetworkCounts() {
        val prefixes = listOf(LinkPrefix("192.168.21.35", 23), LinkPrefix("10.1.0.9", 24))
        assertTrue(clashesWithVpn("10.1.0.2", prefixes))
    }

    @Test
    fun ipv6AndMalformedEntriesNeverClash() {
        assertFalse(clashesWithVpn("10.1.0.2", listOf(LinkPrefix("fe80::1", 64), LinkPrefix("10.1.0", 24))))
        assertFalse(clashesWithVpn("not an address", listOf(LinkPrefix("10.1.0.35", 24))))
        assertFalse(clashesWithVpn("10.1.0.2", listOf(LinkPrefix("10.1.0.35", 33))))
    }

    @Test
    fun theRangeIsShownWithAnXForTheLastNumber() {
        assertEquals("10.1.0.x", vpnRangeLabel("10.1.0.2"))
    }
}
