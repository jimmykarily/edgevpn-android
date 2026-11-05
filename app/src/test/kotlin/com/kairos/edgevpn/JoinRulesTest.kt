package com.kairos.edgevpn

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class JoinRulesTest {

    @Test
    fun cancelledScanChangesNothing() {
        assertNull(scannedKey(null))
        assertNull(scannedKey("  \n"))
    }

    @Test
    fun scannedTextFillsTheField() {
        assertEquals("abc def", scannedKey("  abc def\n"))
    }

    @Test
    fun keyLosesAllWhitespace() {
        assertEquals("abcdef", cleanKey(" ab\ncd\t ef "))
    }

    @Test
    fun problemFromTheValidatorIsShownAndNothingIsSaved() {
        assertEquals(
            JoinOutcome.ShowProblem("cut off"),
            joinOutcome("abc", "cut off", vpnRunning = true),
        )
    }

    @Test
    fun validKeyIsSavedWithoutWhitespaceAndStartsTheVpnWhenItIsOff() {
        assertEquals(
            JoinOutcome.Save("abcd", askReconnect = false),
            joinOutcome("ab\ncd", "", vpnRunning = false),
        )
    }

    @Test
    fun validKeyWhileRunningAsksToReconnect() {
        assertEquals(
            JoinOutcome.Save("abcd", askReconnect = true),
            joinOutcome("abcd", "", vpnRunning = true),
        )
    }
}
