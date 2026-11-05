package com.kairos.edgevpn

import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class LogsRulesTest {

    @Test
    fun sticksWhenViewportEndsAtTheContentEnd() {
        assertTrue(shouldStickToBottom(scrollY = 900, viewportHeight = 100, contentHeight = 1000, slackPx = 8))
    }

    @Test
    fun sticksWithinTheSlack() {
        assertTrue(shouldStickToBottom(scrollY = 895, viewportHeight = 100, contentHeight = 1000, slackPx = 8))
    }

    @Test
    fun doesNotStickWhenScrolledUp() {
        assertFalse(shouldStickToBottom(scrollY = 500, viewportHeight = 100, contentHeight = 1000, slackPx = 8))
    }

    @Test
    fun sticksWhenAllContentFitsInTheViewport() {
        assertTrue(shouldStickToBottom(scrollY = 0, viewportHeight = 800, contentHeight = 300, slackPx = 8))
    }

    @Test
    fun sticksBeforeAnythingIsLaidOut() {
        assertTrue(shouldStickToBottom(scrollY = 0, viewportHeight = 0, contentHeight = 0, slackPx = 8))
    }
}

class ShouldRefreshLogsTest {

    @Test
    fun refreshesWhenLogsChangedAndNothingIsSelected() {
        assertTrue(shouldRefreshLogs(shown = "a", latest = "a\nb", hasSelection = false))
    }

    @Test
    fun refreshesTheFirstTime() {
        assertTrue(shouldRefreshLogs(shown = null, latest = "", hasSelection = false))
    }

    @Test
    fun skipsWhenLogsAreUnchanged() {
        assertFalse(shouldRefreshLogs(shown = "a", latest = "a", hasSelection = false))
    }

    @Test
    fun skipsWhileTheUserHasASelection() {
        assertFalse(shouldRefreshLogs(shown = "a", latest = "a\nb", hasSelection = true))
    }
}
