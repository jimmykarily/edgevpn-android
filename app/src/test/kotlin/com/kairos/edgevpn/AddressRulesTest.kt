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
}
