package com.kairos.edgevpn

import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import java.io.File
import org.junit.Assert.assertEquals
import org.junit.Assume.assumeTrue
import org.junit.Test
import org.junit.runner.RunWith

/**
 * The first step of scripts/network-test: stores the test network in the app the way the Join
 * and Settings screens would, so the script can then turn the VPN on from the main screen.
 *
 * The script puts the files in the app's private files directory with run-as: "token" holds the
 * test peer's temporary key and "peers" the peer's libp2p address. They are deleted here once
 * read. Without them, as in make e2e, the test is skipped.
 */
@RunWith(AndroidJUnit4::class)
class NetworkTestSetup {

    private val context get() = InstrumentationRegistry.getInstrumentation().targetContext

    @Test
    fun storeTheTestNetwork() {
        val dir = File(context.filesDir, DIR)
        assumeTrue("only run by scripts/network-test", dir.isDirectory)
        try {
            val token = File(dir, "token").readText().trim()
            val peers = File(dir, "peers").readText().trim()

            AppSettings.startFresh(context)
            TokenStore.save(context, token)
            AppSettings.setAddress(context, ADDRESS)
            AppSettings.setExtraPeers(context, peers)
            AppSettings.setCheckInSeconds(context, CHECK_IN_SECONDS)
            AppSettings.setDetailedLogs(context, true)
            // The setters above write to disk in the background, and the process ends with the
            // test. An empty commit is queued behind those writes and returns once they are done.
            TokenStore.prefs(context).edit().commit()

            assertEquals(token, TokenStore.load(context))
            assertEquals(ADDRESS, AppSettings.address(context))
            assertEquals(peers, AppSettings.extraPeers(context))
            assertEquals(CHECK_IN_SECONDS, AppSettings.checkInSeconds(context))
        } finally {
            dir.deleteRecursively()
        }
    }

    private companion object {
        const val DIR = "network-test"
        const val ADDRESS = "10.1.0.4"

        // The test peer forgets a device whose heartbeat is older than 60 seconds, and the
        // default heartbeat comes every 120.
        const val CHECK_IN_SECONDS = 15
    }
}
