package com.kairos.edgevpn

import androidx.test.core.app.ActivityScenario
import androidx.test.espresso.Espresso.closeSoftKeyboard
import androidx.test.espresso.Espresso.onView
import androidx.test.espresso.action.ViewActions.click
import androidx.test.espresso.action.ViewActions.replaceText
import androidx.test.espresso.assertion.ViewAssertions.matches
import androidx.test.espresso.matcher.ViewMatchers.isChecked
import androidx.test.espresso.matcher.ViewMatchers.isDisplayed
import androidx.test.espresso.matcher.ViewMatchers.isNotChecked
import androidx.test.espresso.matcher.ViewMatchers.withId
import androidx.test.espresso.matcher.ViewMatchers.withText
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import edgevpn.Edgevpn
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class SettingsScreenTest {

    private val context get() = InstrumentationRegistry.getInstrumentation().targetContext

    @Before
    fun startWithDefaults() = AppSettings.startFresh(context)

    @After
    fun putTheDefaultsBack() {
        AppSettings.startFresh(context)
        Edgevpn.setDetailedLogs(false)
    }

    @Test
    fun anInvalidAddressIsExplainedAndNotSaved() {
        ActivityScenario.launch(SettingsActivity::class.java).use {
            onView(withId(R.id.addressRow)).perform(nestedScrollTo(), click())
            onView(withId(R.id.field)).perform(replaceText("10.1.0.256"))
            closeSoftKeyboard()
            onView(withText("Save")).perform(click())

            onView(withText("Use four numbers from 0 to 255, like 10.1.0.2.")).check(matches(isDisplayed()))
            assertEquals("10.1.0.2", AppSettings.address(context))
        }
    }

    @Test
    fun detailedLogsStaysOnWhenTheScreenOpensAgain() {
        ActivityScenario.launch(SettingsActivity::class.java).use {
            onView(withId(R.id.detailedLogsSwitch)).perform(nestedScrollTo())
                .check(matches(isNotChecked()))
                .perform(click())
                .check(matches(isChecked()))
        }
        assertTrue(AppSettings.detailedLogs(context))
        assertTrue(Edgevpn.detailedLogs())

        ActivityScenario.launch(SettingsActivity::class.java).use {
            onView(withId(R.id.detailedLogsSwitch)).perform(nestedScrollTo()).check(matches(isChecked()))
        }
    }
}
