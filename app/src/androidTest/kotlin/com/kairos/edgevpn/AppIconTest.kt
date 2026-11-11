package com.kairos.edgevpn

import android.graphics.drawable.AdaptiveIconDrawable
import android.os.Build
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith

/** The launcher shows the EdgeVPN logo, not Android's default icon. */
@RunWith(AndroidJUnit4::class)
class AppIconTest {

    private val context get() = InstrumentationRegistry.getInstrumentation().targetContext

    @Test
    fun theAppHasItsOwnAdaptiveIcon() {
        val info = context.applicationInfo
        assertEquals(R.mipmap.ic_launcher, info.icon)
        val icon = context.packageManager.getApplicationIcon(info)
        assertTrue("launcher icon is ${icon.javaClass.name}", icon is AdaptiveIconDrawable)
        // Themed icons on Android 13 and later draw the monochrome layer.
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            assertNotNull((icon as AdaptiveIconDrawable).monochrome)
        }
    }
}
