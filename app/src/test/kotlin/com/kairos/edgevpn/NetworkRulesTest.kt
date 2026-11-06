package com.kairos.edgevpn

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class NetworkRulesTest {

    private fun wifi(id: Long, validated: Boolean = true) =
        NetworkInfo(id, validated, wifi = true, ethernet = false, cellular = false)

    private fun ethernet(id: Long, validated: Boolean = true) =
        NetworkInfo(id, validated, wifi = false, ethernet = true, cellular = false)

    private fun cellular(id: Long, validated: Boolean = true) =
        NetworkInfo(id, validated, wifi = false, ethernet = false, cellular = true)

    @Test
    fun validatedCellularBeatsUnvalidatedWifi() {
        assertEquals(cellular(2), pickNetwork(listOf(wifi(1, validated = false), cellular(2))))
    }

    @Test
    fun validatedWifiBeatsValidatedCellular() {
        assertEquals(wifi(2), pickNetwork(listOf(cellular(1), wifi(2))))
    }

    @Test
    fun ethernetBeatsCellular() {
        assertEquals(ethernet(2), pickNetwork(listOf(cellular(1), ethernet(2))))
    }

    @Test
    fun wifiBeatsEthernet() {
        assertEquals(wifi(2), pickNetwork(listOf(ethernet(1), wifi(2))))
    }

    @Test
    fun emptyGivesNull() {
        assertNull(pickNetwork(emptyList()))
    }

    @Test
    fun equalNetworksPickTheLowerId() {
        assertEquals(wifi(3), pickNetwork(listOf(wifi(7), wifi(3), wifi(5))))
    }

    @Test
    fun localNetworkIsWifiOrEthernet() {
        assertTrue(isLocalNetwork(wifi(1)))
        assertTrue(isLocalNetwork(ethernet(1)))
        assertFalse(isLocalNetwork(cellular(1)))
        assertFalse(isLocalNetwork(null))
    }

    @Test
    fun reportFilterPassesTheFirstChoiceOnce() {
        val filter = ReportFilter<String>()
        assertTrue(filter.offer("wlan", true))
        assertFalse(filter.offer("wlan", true))
    }

    @Test
    fun reportFilterPassesAFirstChoiceOfNoNetwork() {
        val filter = ReportFilter<String>()
        assertTrue(filter.offer(null, false))
        assertFalse(filter.offer(null, false))
    }

    @Test
    fun reportFilterPassesOnlyRealChanges() {
        val filter = ReportFilter<String>()
        assertTrue(filter.offer("wlan", true))
        assertTrue(filter.offer("wlan", false))
        assertFalse(filter.offer("wlan", false))
        assertTrue(filter.offer("lte", false))
        assertTrue(filter.offer(null, false))
        assertFalse(filter.offer(null, false))
        assertTrue(filter.offer("wlan", true))
    }

    @Test
    fun labelsNameTheTransport() {
        assertEquals("wifi", networkLabel(wifi(1)))
        assertEquals("ethernet", networkLabel(ethernet(1)))
        assertEquals("cellular", networkLabel(cellular(1)))
        assertEquals("none", networkLabel(null))
    }
}
