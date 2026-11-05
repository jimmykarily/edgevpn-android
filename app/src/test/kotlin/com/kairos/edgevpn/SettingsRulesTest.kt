package com.kairos.edgevpn

import org.junit.Assert.assertEquals
import org.junit.Test

class SettingsRulesTest {

    @Test
    fun shortIdKeepsFirstEightAndLastFour() {
        assertEquals("12D3KooW…xUS5", shortId("12D3KooWB2mBabcdefghijklmnopqrstuvwxyzxUS5"))
    }

    @Test
    fun shortIdLeavesShortIdsWhole() {
        assertEquals("", shortId(""))
        assertEquals("12D3KooWabcde", shortId("12D3KooWabcde"))
    }

    @Test
    fun addressThatIsNotFourNumbersIsInvalid() {
        assertEquals(AddressChange.INVALID, addressChange("10.1.0.2", "10.1.0", vpnRunning = true))
        assertEquals(AddressChange.INVALID, addressChange("10.1.0.2", "", vpnRunning = false))
    }

    @Test
    fun sameAddressChangesNothing() {
        assertEquals(AddressChange.UNCHANGED, addressChange("10.1.0.2", "10.1.0.2", vpnRunning = true))
    }

    @Test
    fun surroundingSpacesAreIgnored() {
        assertEquals(AddressChange.UNCHANGED, addressChange("10.1.0.2", " 10.1.0.2 ", vpnRunning = true))
        assertEquals(AddressChange.SAVE, addressChange("10.1.0.2", " 10.1.0.3\n", vpnRunning = false))
    }

    @Test
    fun newAddressAsksToReconnectOnlyWhileTheVpnRuns() {
        assertEquals(AddressChange.SAVE, addressChange("10.1.0.2", "10.1.0.3", vpnRunning = false))
        assertEquals(AddressChange.SAVE_AND_ASK_RECONNECT, addressChange("10.1.0.2", "10.1.0.3", vpnRunning = true))
    }
}
