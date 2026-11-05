package com.kairos.edgevpn

import android.util.Base64
import androidx.lifecycle.Lifecycle
import androidx.test.core.app.ActivityScenario
import androidx.test.espresso.Espresso.closeSoftKeyboard
import androidx.test.espresso.Espresso.onView
import androidx.test.espresso.action.ViewActions.click
import androidx.test.espresso.action.ViewActions.replaceText
import androidx.test.espresso.assertion.ViewAssertions.matches
import androidx.test.espresso.matcher.ViewMatchers.isDisplayed
import androidx.test.espresso.matcher.ViewMatchers.withId
import androidx.test.espresso.matcher.ViewMatchers.withText
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import edgevpn.Edgevpn
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith

/** The Join screen with a key that was cut off while copying, checked by the real Go validator. */
@RunWith(AndroidJUnit4::class)
class JoinScreenTest {

    private val context get() = InstrumentationRegistry.getInstrumentation().targetContext

    @Before
    fun startWithNothingStored() = AppSettings.startFresh(context)

    @After
    fun forgetWhatTheTestStored() = AppSettings.startFresh(context)

    @Test
    fun aCutOffKeyIsExplainedAndNotSaved() {
        val key = fakeKey()
        assertEquals("the uncut key is valid, so only the cut makes it fail", "", Edgevpn.validateToken(key))
        val cut = key.dropLast(5)

        ActivityScenario.launch(JoinActivity::class.java).use { scenario ->
            onView(withId(R.id.keyField)).perform(replaceText(cut))
            closeSoftKeyboard()
            onView(withId(R.id.joinButton)).perform(click())

            onView(withText("This doesn't look like a complete key. It may have been cut off while copying."))
                .check(matches(isDisplayed()))
            assertEquals(Lifecycle.State.RESUMED, scenario.state)
            assertNull(TokenStore.load(context))
        }
    }

    /**
     * A network key in the shape edgevpn writes, with made-up OTP keys, room and rendezvous.
     * It joins no real network, and nothing in the test ever connects with it.
     */
    private fun fakeKey(): String {
        val yaml = """
            otp:
              dht:
                interval: 9000
                key: e2etestdhtkeyfornothingreal
                length: 32
              crypto:
                interval: 9000
                key: e2etestcryptokeyfornothingreal
                length: 32
            room: e2e-test-room
            rendezvous: e2e-test-rendezvous
            mdns: e2e-test-mdns
            max_message_size: 20971520
        """.trimIndent() + "\n"
        return Base64.encodeToString(yaml.toByteArray(), Base64.NO_WRAP)
    }
}
