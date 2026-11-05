package com.kairos.edgevpn

import androidx.test.core.app.ActivityScenario
import androidx.test.espresso.Espresso.onView
import androidx.test.espresso.assertion.ViewAssertions.matches
import androidx.test.espresso.matcher.ViewMatchers.isDisplayed
import androidx.test.espresso.matcher.ViewMatchers.withId
import androidx.test.espresso.matcher.ViewMatchers.withText
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import org.hamcrest.Matchers.allOf
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith

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
}
