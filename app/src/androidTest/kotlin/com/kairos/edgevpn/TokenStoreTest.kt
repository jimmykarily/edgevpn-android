package com.kairos.edgevpn

import androidx.security.crypto.MasterKeys
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import java.security.KeyStore
import java.util.concurrent.Callable
import java.util.concurrent.Executors
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertSame
import org.junit.Test
import org.junit.runner.RunWith

/** The encrypted settings file, opened once per process and shared by every reader. */
@RunWith(AndroidJUnit4::class)
class TokenStoreTest {

    private val context get() = InstrumentationRegistry.getInstrumentation().targetContext

    @After
    fun forgetWhatTheTestStored() = AppSettings.startFresh(context)

    @Test
    fun everyReadSharesOneInstanceAcrossThreads() {
        val pool = Executors.newFixedThreadPool(8)
        try {
            val all = pool.invokeAll(List(16) { Callable { TokenStore.prefs(context) } }).map { it.get() }
            all.forEach { assertSame(all[0], it) }
            assertSame(all[0], TokenStore.prefs(context.applicationContext))
        } finally {
            pool.shutdown()
        }
    }

    @Test
    fun aSavedValueIsReadBack() {
        TokenStore.save(context, "not-a-real-key")
        assertEquals("not-a-real-key", TokenStore.load(context))
    }

    /**
     * A settings file restored from a backup or moved to a new phone meets a Keystore without the
     * key that encrypted it. Deleting that key from the Keystore makes the same situation here.
     */
    @Test
    fun aFileThatCannotBeDecryptedIsReplacedByAnEmptyOne() {
        TokenStore.save(context, "not-a-real-key")
        TokenStore.prefs(context).edit().commit()
        val masterKey = MasterKeys.getOrCreate(MasterKeys.AES256_GCM_SPEC)
        KeyStore.getInstance("AndroidKeyStore").apply { load(null) }.deleteEntry(masterKey)
        TokenStore.openAgain()

        assertNull(TokenStore.load(context))
        TokenStore.save(context, "another-made-up-key")
        assertEquals("another-made-up-key", TokenStore.load(context))
    }
}
