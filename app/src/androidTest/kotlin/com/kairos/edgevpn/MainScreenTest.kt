package com.kairos.edgevpn

import androidx.core.content.ContextCompat
import androidx.test.core.app.ActivityScenario
import androidx.test.espresso.Espresso.onView
import androidx.test.espresso.Espresso.pressBack
import androidx.test.espresso.action.ViewActions.click
import androidx.test.espresso.assertion.ViewAssertions.matches
import androidx.test.espresso.matcher.ViewMatchers.hasDescendant
import androidx.test.espresso.matcher.ViewMatchers.isDisplayed
import androidx.test.espresso.matcher.ViewMatchers.withId
import androidx.test.espresso.matcher.ViewMatchers.withParent
import androidx.test.espresso.matcher.ViewMatchers.withText
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import org.hamcrest.Matchers.allOf
import org.hamcrest.Matchers.containsString
import org.hamcrest.Matchers.not
import org.junit.After
import org.junit.Assert.assertSame
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith

/** The main screen in each state, shown from a chosen network snapshot instead of a running node. */
@RunWith(AndroidJUnit4::class)
class MainScreenTest {

    private var snapshot = Snapshots.off()

    private val context get() = InstrumentationRegistry.getInstrumentation().targetContext

    /** No stored key or setting from another test or an earlier install changes what the screen shows. */
    @Before
    fun showChosenSnapshot() {
        AppSettings.startFresh(context)
        NetworkSource.snapshot = { snapshot }
    }

    @After
    fun readTheNodeAgain() {
        NetworkSource.reset()
    }

    @Test
    fun offShowsTurnOnAndNoDevices() {
        snapshot = Snapshots.off()
        ActivityScenario.launch(MainActivity::class.java).use {
            onView(withId(R.id.statusTitle)).check(matches(withTextBeforeHelpIcon("VPN is off")))
            onView(withId(R.id.mainButton)).check(matches(allOf(isDisplayed(), withText("Turn on"))))
            onView(withId(R.id.devicesSection)).check(matches(not(isDisplayed())))
            onView(withId(R.id.findingSection)).check(matches(not(isDisplayed())))
        }
    }

    @Test
    fun lookingShowsTheOrangeCardAndTheFindingTiles() {
        snapshot = Snapshots.looking(helpers = 3)
        ActivityScenario.launch(MainActivity::class.java).use { scenario ->
            onView(withId(R.id.statusTitle)).check(matches(withTextBeforeHelpIcon("Looking for your devices")))
            scenario.onActivity { activity ->
                val card = activity.findViewById<android.view.View>(R.id.statusCardBody)
                val looking = ContextCompat.getDrawable(activity, R.drawable.bg_card_looking)!!
                assertSame(looking.constantState, card.background.constantState)
            }
            onView(withId(R.id.mainButton)).check(matches(withText("Turn off")))
            onView(withId(R.id.helpersValue)).perform(nestedScrollTo()).check(matches(allOf(isDisplayed(), withText("3"))))
            onView(withId(R.id.helpersLabel)).check(matches(withTextBeforeHelpIcon("public helpers available")))
            onView(withId(R.id.secondValue)).check(matches(allOf(isDisplayed(), withText("0"))))
            onView(withId(R.id.secondLabel)).check(matches(withTextBeforeHelpIcon("devices found so far")))
        }
    }

    @Test
    fun connectedListsTheDevicesByHowTheyAreReached() {
        snapshot = Snapshots.connectedWithThreeDevices()
        ActivityScenario.launch(MainActivity::class.java).use {
            onView(withId(R.id.statusTitle)).check(matches(withTextBeforeHelpIcon("Connected")))
            onView(withId(R.id.statusText)).check(matches(withText("You can reach 2 of your 3 devices.")))
            onView(withId(R.id.deviceList)).perform(nestedScrollTo()).check(
                deviceRowsInOrder(
                    DeviceRowText("desktop", "Direct", "10.1.0.1 · same Wi-Fi as you"),
                    DeviceRowText("laptop", "Via helper", "10.1.0.3 · through a helper computer"),
                    DeviceRowText("attic-pc", "Offline", "10.1.0.4 · last seen 2 h ago"),
                )
            )

            onView(withText("desktop")).perform(nestedScrollTo(), click())
            onView(withId(R.id.sheetName)).check(matches(allOf(isDisplayed(), withText("desktop"))))
            onView(withId(R.id.sheetAddress)).check(matches(allOf(isDisplayed(), withText("10.1.0.1"))))
        }
    }

    @Test
    fun problemShowsTheReasonWithTryAgainAndSeeLogs() {
        snapshot = Snapshots.problem("Could not create the VPN interface.")
        ActivityScenario.launch(MainActivity::class.java).use {
            onView(withId(R.id.statusTitle)).check(matches(withTextBeforeHelpIcon("The VPN could not run")))
            onView(withId(R.id.statusText)).check(matches(withText("Could not create the VPN interface.")))
            onView(withId(R.id.mainButton)).check(matches(allOf(isDisplayed(), withText("Try again"))))
            onView(withId(R.id.seeLogsButton)).check(matches(allOf(isDisplayed(), withText("See logs"))))
        }
    }

    @Test
    fun badgeHelpShowsTechnicalDetailsAndTheDevicesLiveValues() {
        snapshot = Snapshots.connectedWithThreeDevices()
        ActivityScenario.launch(MainActivity::class.java).use {
            onView(allOf(withId(R.id.deviceBadge), withParent(hasDescendant(withText("laptop")))))
                .perform(nestedScrollTo(), clickHelpIcon())

            onView(withId(R.id.helpTitle)).check(matches(allOf(isDisplayed(), withText("Via helper"))))
            onView(withId(R.id.helpDevice)).check(matches(withText("laptop")))
            onView(withText("Technical details")).perform(nestedScrollTo()).check(matches(isDisplayed()))
            onView(withId(R.id.helpLive)).perform(nestedScrollTo()).check(
                matches(
                    allOf(
                        isDisplayed(),
                        withText(containsString("Relay address: ${Snapshots.RELAY_ADDR}")),
                        withText(containsString("Limited: yes")),
                    )
                )
            )
        }
    }

    @Test
    fun anotherDeviceAtThisAddressShowsTheWarningWithHelpAndChangeAddress() {
        snapshot = Snapshots.lookingWithAddressInUse()
        ActivityScenario.launch(MainActivity::class.java).use {
            onView(withId(R.id.statusTitle)).check(matches(withTextBeforeHelpIcon("Looking for your devices")))
            onView(withId(R.id.addressInUseText)).check(
                matches(
                    allOf(
                        isDisplayed(),
                        withTextBeforeHelpIcon("Another device already uses 10.1.0.2. Pick another address in Settings."),
                    )
                )
            )

            onView(withId(R.id.addressInUseText)).perform(clickHelpIcon())
            onView(withId(R.id.helpTitle)).check(matches(allOf(isDisplayed(), withText("Address already in use"))))
            onView(withId(R.id.helpLive)).perform(nestedScrollTo())
                .check(matches(withText(containsString("Address: 10.1.0.2"))))
            pressBack()

            onView(withId(R.id.changeAddressButton)).check(matches(allOf(isDisplayed(), withText("Change address"))))
                .perform(click())
            onView(withId(R.id.detailedLogsSwitch)).perform(nestedScrollTo()).check(matches(isDisplayed()))
            pressBack()
        }
    }

    @Test
    fun aWifiInTheVpnRangeShowsTheWarningWithHelp() {
        snapshot = Snapshots.connectedOnAClashingWifi()
        ActivityScenario.launch(MainActivity::class.java).use {
            onView(withId(R.id.localNetworkClashText)).check(
                matches(
                    allOf(
                        isDisplayed(),
                        withTextBeforeHelpIcon(
                            "This Wi-Fi uses the same addresses as your VPN (10.1.0.x). " +
                                "While the VPN is on, some devices on this Wi-Fi may not answer."
                        ),
                    )
                )
            )
            onView(withId(R.id.changeAddressButton)).check(matches(not(isDisplayed())))

            onView(withId(R.id.localNetworkClashText)).perform(clickHelpIcon())
            onView(withId(R.id.helpTitle)).check(matches(allOf(isDisplayed(), withText("Wi-Fi uses the VPN's addresses"))))
            onView(withId(R.id.helpLive)).perform(nestedScrollTo())
                .check(matches(withText(containsString("VPN route: 10.1.0.0/24"))))
            pressBack()
        }
    }

    @Test
    fun noWarningWhenNoOtherDeviceHoldsTheAddress() {
        snapshot = Snapshots.connectedWithThreeDevices()
        ActivityScenario.launch(MainActivity::class.java).use {
            onView(withId(R.id.addressInUseText)).check(matches(not(isDisplayed())))
            onView(withId(R.id.changeAddressButton)).check(matches(not(isDisplayed())))
            onView(withId(R.id.localNetworkClashText)).check(matches(not(isDisplayed())))
        }
    }
}
