package com.kairos.edgevpn.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class ScreenPlanTest {
    private fun row(id: String, badge: Badge) = DeviceRow(
        id = id, name = "d$id", address = "10.1.0.$id", badge = badge, how = How.NotReachable,
        remote = null, remoteAddr = null, transport = null, limited = false, openSeconds = null,
    )

    private fun model(
        state: VpnState,
        devices: List<DeviceRow> = emptyList(),
        lastHeard: Long? = null,
        address: String = "10.1.0.2",
    ) = StatusModel(
        state = state, reachable = devices.count { it.badge != Badge.OFFLINE }, total = devices.size,
        selfAddress = address, selfId = "me", runningSeconds = 40, error = null, devices = devices,
        helpers = 3, lastHeardSecondsAgo = lastHeard, lastHeardName = null,
    )

    @Test
    fun offShowsOnlyTheCardWithTurnOn() {
        val p = planScreen(model(VpnState.OFF, listOf(row("1", Badge.OFFLINE)), lastHeard = 5))
        assertEquals(MainButton.TURN_ON, p.button)
        assertFalse(p.showPills)
        assertFalse(p.showMap)
        assertFalse(p.showDevices)
        assertFalse(p.showTiles)
        assertFalse(p.showSeeLogs)
    }

    @Test
    fun connectedShowsEverythingWithLastHeardTile() {
        val p = planScreen(model(VpnState.CONNECTED, listOf(row("1", Badge.DIRECT)), lastHeard = 4))
        assertEquals(MainButton.TURN_OFF, p.button)
        assertTrue(p.showPills)
        assertTrue(p.showMap)
        assertTrue(p.showDevices)
        assertTrue(p.showTiles)
        assertEquals(SecondTile.LAST_HEARD, p.secondTile)
        assertFalse(p.showSeeLogs)
    }

    @Test
    fun connectedWithOneDeviceSaysYourDevice() {
        assertTrue(planScreen(model(VpnState.CONNECTED, listOf(row("1", Badge.DIRECT)))).onlyDevice)
    }

    @Test
    fun connectedWithSeveralDevicesCountsThem() {
        val devices = listOf(row("1", Badge.DIRECT), row("2", Badge.OFFLINE))
        assertFalse(planScreen(model(VpnState.CONNECTED, devices)).onlyDevice)
    }

    @Test
    fun connectedWithoutAnyCheckInLeavesTheLastHeardTileOut() {
        val p = planScreen(model(VpnState.CONNECTED, listOf(row("1", Badge.DIRECT)), lastHeard = null))
        assertEquals(SecondTile.NONE, p.secondTile)
    }

    @Test
    fun lookingShowsMapAndFoundSoFarTileButNoEmptyList() {
        val p = planScreen(model(VpnState.LOOKING, lastHeard = 4))
        assertEquals(MainButton.TURN_OFF, p.button)
        assertTrue(p.showPills)
        assertTrue(p.showMap)
        assertFalse(p.showDevices)
        assertTrue(p.showTiles)
        assertEquals(SecondTile.FOUND_SO_FAR, p.secondTile)
    }

    @Test
    fun lookingListsDevicesItAlreadyKnowsAbout() {
        val p = planScreen(model(VpnState.LOOKING, listOf(row("1", Badge.OFFLINE))))
        assertTrue(p.showDevices)
    }

    @Test
    fun problemShowsTryAgainAndSeeLogsWithoutMapOrLists() {
        val p = planScreen(model(VpnState.PROBLEM, listOf(row("1", Badge.OFFLINE))))
        assertEquals(MainButton.TRY_AGAIN, p.button)
        assertTrue(p.showSeeLogs)
        assertTrue(p.showPills)
        assertFalse(p.showMap)
        assertFalse(p.showDevices)
        assertFalse(p.showTiles)
    }

    @Test
    fun theAddressPillIsLeftOutWhenTheAddressIsUnknown() {
        assertTrue(planScreen(model(VpnState.CONNECTED)).showAddressPill)
        assertFalse(planScreen(model(VpnState.PROBLEM, address = "")).showAddressPill)
    }

    private fun net(internet: Boolean = true, vpn: Boolean = false, wifi: Boolean = false, cellular: Boolean = false) =
        NetFacts(internet = internet, vpn = vpn, wifi = wifi, cellular = cellular)

    @Test
    fun usingIgnoresTheVpnAndPrefersWifi() {
        val nets = listOf(net(vpn = true), net(cellular = true), net(wifi = true))
        assertEquals(Using.WIFI, pickUsing(nets))
    }

    @Test
    fun usingIsMobileDataWhenOnlyCellularHasInternet() {
        val nets = listOf(net(vpn = true), net(internet = false, wifi = true), net(cellular = true))
        assertEquals(Using.MOBILE_DATA, pickUsing(nets))
    }

    @Test
    fun usingIsOtherForAnyOtherNetworkWithInternet() {
        assertEquals(Using.OTHER, pickUsing(listOf(net())))
    }

    @Test
    fun usingIsNullWithoutAnyNetworkWithInternet() {
        assertNull(pickUsing(listOf(net(vpn = true), net(internet = false, wifi = true))))
    }
}
