package com.kairos.edgevpn

import org.junit.After
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class ConnectRequestTest {

    @After
    fun forgetTheRequest() {
        ConnectRequest.take()
    }

    @Test
    fun `nothing is asked until the Join screen asks`() {
        assertFalse(ConnectRequest.take())
    }

    @Test
    fun `a request is taken once`() {
        ConnectRequest.ask()

        assertTrue(ConnectRequest.take())
        assertFalse(ConnectRequest.take())
    }
}
