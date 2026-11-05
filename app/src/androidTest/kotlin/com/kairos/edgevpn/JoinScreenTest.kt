package com.kairos.edgevpn

import android.util.Base64
import android.view.View
import android.view.WindowManager
import android.view.inputmethod.EditorInfo
import android.widget.EditText
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
import java.security.SecureRandom
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
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
        val key = temporaryKey()
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

    @Test
    fun theKeyStaysOutOfRecentsAutofillAndKeyboardLearning() {
        ActivityScenario.launch(JoinActivity::class.java).use { scenario ->
            scenario.onActivity { activity ->
                val flags = activity.window.attributes.flags
                assertTrue("Recents must not show the key", flags and WindowManager.LayoutParams.FLAG_SECURE != 0)
                val field = activity.findViewById<EditText>(R.id.keyField)
                assertEquals(View.IMPORTANT_FOR_AUTOFILL_NO_EXCLUDE_DESCENDANTS, field.importantForAutofill)
                assertTrue(
                    "the keyboard must not learn the key",
                    field.imeOptions and EditorInfo.IME_FLAG_NO_PERSONALIZED_LEARNING != 0,
                )
            }
        }
    }

    /**
     * A temporary network key in the shape edgevpn writes, made fresh for each run: random OTP keys
     * (43 base64url characters, as edgevpn makes them), a random room, rendezvous and mDNS name.
     * Nothing in the test ever connects with it, and no key is stored in the repository.
     */
    private fun temporaryKey(): String {
        val yaml = """
            otp:
              dht:
                interval: 9000
                key: ${random()}
                length: 43
              crypto:
                interval: 9000
                key: ${random()}
                length: 43
            room: ${random()}
            rendezvous: ${random()}
            mdns: ${random()}
            max_message_size: 20971520
        """.trimIndent() + "\n"
        return Base64.encodeToString(yaml.toByteArray(), Base64.NO_WRAP)
    }

    private fun random(): String {
        val bytes = ByteArray(32).also { SecureRandom().nextBytes(it) }
        return Base64.encodeToString(bytes, Base64.URL_SAFE or Base64.NO_PADDING or Base64.NO_WRAP)
    }
}
