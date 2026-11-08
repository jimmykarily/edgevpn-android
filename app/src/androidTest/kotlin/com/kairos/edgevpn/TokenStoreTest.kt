package com.kairos.edgevpn

import android.content.Context
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
import org.junit.Assert.fail
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

    /**
     * A failure while the Keystore still has the master key, such as a Keystore that is briefly
     * unusable, must not cost the user their network key. A damaged keyset in the file makes such
     * a failure here: opening fails, and the file must still be there afterwards.
     */
    @Test
    fun aFailureWithTheMasterKeyPresentKeepsTheFile() {
        TokenStore.save(context, "not-a-real-key")
        TokenStore.prefs(context).edit().commit()
        val raw = context.getSharedPreferences("edgevpn_secure_prefs", Context.MODE_PRIVATE)
        val keyset = raw.all.keys.single { it.startsWith("__androidx_security_crypto_encrypted_prefs_key") }
        val entries = raw.all.size
        raw.edit().putString(keyset, "damaged").commit()
        TokenStore.openAgain()

        try {
            try {
                TokenStore.prefs(context)
                fail("opening a file with a damaged keyset must fail")
            } catch (expected: Exception) {
            }
            assertEquals("the file must keep every entry", entries, raw.all.size)
            assertEquals("damaged", raw.getString(keyset, null))
        } finally {
            // The damaged file cannot be opened, so the cleanup after the test needs a new one.
            context.deleteSharedPreferences("edgevpn_secure_prefs")
            TokenStore.openAgain()
        }
    }
}
