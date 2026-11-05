package com.kairos.edgevpn.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class MapLayoutTest {
    @Test
    fun noDevicesGiveNoSlots() {
        assertTrue(mapSlots(0).isEmpty())
    }

    @Test
    fun threeDevicesStartAtTheTopAndGoClockwise() {
        val slots = mapSlots(3)
        assertEquals(listOf(-90f, 30f, 150f), slots.map { it.angleDegrees })
        assertEquals(listOf(0, 1, 2), slots.map { it.deviceIndex })
        slots.forEach { assertNull(it.moreCount) }
    }

    @Test
    fun eightDevicesAllGetASlot() {
        val slots = mapSlots(8)
        assertEquals((0 until 8).toList(), slots.map { it.deviceIndex })
        assertEquals(-90f, slots[0].angleDegrees)
        assertEquals(-45f, slots[1].angleDegrees)
        slots.forEach { assertNull(it.moreCount) }
    }

    @Test
    fun moreThanEightDevicesShowSevenAndAMoreBubble() {
        val slots = mapSlots(11)
        assertEquals(8, slots.size)
        assertEquals((0 until 7).toList(), slots.take(7).map { it.deviceIndex })
        val last = slots.last()
        assertNull(last.deviceIndex)
        assertEquals(4, last.moreCount)
        assertEquals(225f, last.angleDegrees)
    }

    @Test
    fun aSmallerMaxIsRespected() {
        val slots = mapSlots(5, max = 4)
        assertEquals(listOf(0, 1, 2, null), slots.map { it.deviceIndex })
        assertEquals(2, slots.last().moreCount)
    }

    @Test
    fun shortNamesStayWhole() {
        assertEquals("desktop", mapLabel("desktop"))
        assertEquals("abcdefghij", mapLabel("abcdefghij"))
    }

    @Test
    fun longNamesAreCutToTenCharactersWithAnEllipsis() {
        assertEquals("home-serv…", mapLabel("home-server-2"))
        assertEquals(10, mapLabel("a-very-long-hostname").length)
    }
}
