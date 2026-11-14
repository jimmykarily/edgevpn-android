package com.kairos.edgevpn

import androidx.test.core.app.ActivityScenario
import androidx.test.espresso.Espresso.onView
import androidx.test.espresso.action.ViewActions.click
import androidx.test.espresso.assertion.ViewAssertions.matches
import androidx.test.espresso.matcher.RootMatchers.isDialog
import androidx.test.espresso.matcher.ViewMatchers.isDisplayed
import androidx.test.espresso.matcher.ViewMatchers.withContentDescription
import androidx.test.espresso.matcher.ViewMatchers.withId
import androidx.test.espresso.matcher.ViewMatchers.withText
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import org.hamcrest.Matchers.allOf
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import java.io.File

@RunWith(AndroidJUnit4::class)
class LogsScreenTest {

    private val context get() = InstrumentationRegistry.getInstrumentation().targetContext

    /** Also empties the Go log buffer, which other tests in this process may have written to. */
    @Before
    fun startWithAnEmptyBuffer() = AppSettings.startFresh(context)

    @Test
    fun anEmptyBufferSaysNothingHasBeenLogged() {
        ActivityScenario.launch(LogsActivity::class.java).use {
            onView(withId(R.id.logsText)).check(matches(allOf(isDisplayed(), withText("Nothing has been logged yet."))))
        }
    }

    @Test
    fun clearingTheLogFileEmptiesItAndDeletesItsBackup() {
        val file = File(context.noBackupFilesDir, "edgevpn.log")
        val backup = File(context.noBackupFilesDir, "edgevpn.log.1")
        ActivityScenario.launch(LogsActivity::class.java).use {
            file.appendText("2026-11-14T05:05:15.891Z DEBUG an old line\n")
            backup.writeText("2026-11-14T04:05:15.891Z DEBUG an older line\n")

            onView(withContentDescription("More options")).perform(click())
            onView(withText("Clear log file")).perform(click())
            onView(withText("Clear")).inRoot(isDialog()).perform(pressButton())

            assertEquals("", file.readText())
            assertFalse(backup.exists())
            onView(withId(R.id.logsText)).check(matches(withText("Nothing has been logged yet.")))
        }
    }
}
