package com.kairos.edgevpn

import java.util.Base64
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class NodeKeyStoreTest {

    private val generatedKey = byteArrayOf(1, 2, 3, 4)

    @Test
    fun `generates and saves a key when none is stored`() {
        var saved: String? = null

        val key = loadOrCreateNodeKey(stored = null, save = { saved = it }, generate = { generatedKey })

        assertArrayEquals(generatedKey, key)
        assertEquals(Base64.getEncoder().encodeToString(generatedKey), saved)
    }

    @Test
    fun `returns the stored key without generating a new one`() {
        val storedKey = byteArrayOf(9, 8, 7)
        var saved: String? = null

        val key = loadOrCreateNodeKey(
            stored = Base64.getEncoder().encodeToString(storedKey),
            save = { saved = it },
            generate = { error("must not generate when a key is stored") },
        )

        assertArrayEquals(storedKey, key)
        assertNull(saved)
    }

    @Test
    fun `replaces a stored value that is not valid base64`() {
        var saved: String? = null

        val key = loadOrCreateNodeKey(stored = "not base64!", save = { saved = it }, generate = { generatedKey })

        assertArrayEquals(generatedKey, key)
        assertEquals(Base64.getEncoder().encodeToString(generatedKey), saved)
    }

    @Test
    fun `replaces an empty stored value`() {
        var saved: String? = null

        val key = loadOrCreateNodeKey(stored = "", save = { saved = it }, generate = { generatedKey })

        assertArrayEquals(generatedKey, key)
        assertEquals(Base64.getEncoder().encodeToString(generatedKey), saved)
    }
}
