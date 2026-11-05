package com.kairos.edgevpn

import android.view.View
import android.view.ViewGroup
import android.widget.TextView
import androidx.core.widget.NestedScrollView
import androidx.test.espresso.UiController
import androidx.test.espresso.ViewAction
import androidx.test.espresso.ViewAssertion
import androidx.test.espresso.action.GeneralClickAction
import androidx.test.espresso.action.Press
import androidx.test.espresso.action.ScrollToAction
import androidx.test.espresso.action.Tap
import androidx.test.espresso.matcher.ViewMatchers.isAssignableFrom
import androidx.test.espresso.matcher.ViewMatchers.isDescendantOfA
import androidx.test.espresso.matcher.BoundedMatcher
import androidx.test.espresso.matcher.ViewMatchers.withEffectiveVisibility
import androidx.test.espresso.matcher.ViewMatchers.Visibility
import org.hamcrest.Description
import org.hamcrest.Matcher
import org.hamcrest.Matchers.allOf
import org.junit.Assert.assertEquals

/**
 * Scrolls a view inside a NestedScrollView into sight, which Espresso's own scrollTo() refuses
 * because it only accepts the platform ScrollView.
 */
fun nestedScrollTo(): ViewAction = object : ViewAction {
    override fun getConstraints(): Matcher<View> = allOf(
        isDescendantOfA(isAssignableFrom(NestedScrollView::class.java)),
        withEffectiveVisibility(Visibility.VISIBLE),
    )

    override fun getDescription() = "scroll to the view inside its NestedScrollView"

    override fun perform(uiController: UiController, view: View) = ScrollToAction().perform(uiController, view)
}

/**
 * Taps the (?) icon at the end of a TextView's text, where a person taps it. The icon is the
 * last character of the text, so the tap lands between its start and the end of its line.
 */
fun clickHelpIcon(): ViewAction = GeneralClickAction(
    Tap.SINGLE,
    { view ->
        val t = view as TextView
        val layout = t.layout
        val at = t.text.length - 1
        val line = layout.getLineForOffset(at)
        val x = (layout.getPrimaryHorizontal(at) + layout.getLineRight(line)) / 2f + t.totalPaddingLeft
        val y = (layout.getLineTop(line) + layout.getLineBottom(line)) / 2f + t.totalPaddingTop
        val xy = IntArray(2).also { t.getLocationOnScreen(it) }
        floatArrayOf(xy[0] + x, xy[1] + y)
    },
    Press.FINGER,
    0,
    0,
)

/** The words of one device row: its name, its badge and the line under the name. */
data class DeviceRowText(val name: String, val badge: String, val line: String)

/** Asserts the rows of a device list, top to bottom. */
fun deviceRowsInOrder(vararg rows: DeviceRowText) = ViewAssertion { view, noView ->
    if (noView != null) throw noView
    val list = view as ViewGroup
    val shown = (0 until list.childCount).map { list.getChildAt(it) }
        .filter { it.findViewById<TextView>(R.id.deviceName) != null }
        .map { row ->
            fun text(id: Int) = row.findViewById<TextView>(id).text.toString().removeSuffix(HELP_ICON)
            DeviceRowText(text(R.id.deviceName), text(R.id.deviceBadge), text(R.id.deviceLine))
        }
    assertEquals(rows.toList(), shown)
}

/** Matches a TextView whose text, without the (?) icon appended to it, is [expected]. */
fun withTextBeforeHelpIcon(expected: String): Matcher<View> = object : BoundedMatcher<View, TextView>(TextView::class.java) {
    override fun describeTo(description: Description) {
        description.appendText("with text before its help icon: ").appendValue(expected)
    }

    override fun matchesSafely(item: TextView) = item.text.toString().removeSuffix(HELP_ICON) == expected
}

/** What appendHelpIcon adds after the words: a non-breaking space and the icon's placeholder. */
private const val HELP_ICON = "\u00A0\uFFFC"
