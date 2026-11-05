package com.kairos.edgevpn

import android.content.Context
import android.util.Log
import edgevpn.Edgevpn
import java.util.Base64

/**
 * Stores the libp2p node key, from which the phone's node ID is derived and
 * with which its ledger entries are signed. It must survive app restarts:
 * a new key makes the phone a new node to its peers, and its old ledger
 * entries stay locked to the old identity until the ownership TTL expires.
 */
object NodeKeyStore {

    private const val TAG = "NodeKeyStore"
    private const val KEY_NODE_KEY = "node_key"

    fun loadOrCreate(context: Context): ByteArray {
        val prefs = TokenStore.prefs(context)
        return loadOrCreateNodeKey(
            stored = prefs.getString(KEY_NODE_KEY, null),
            save = { prefs.edit().putString(KEY_NODE_KEY, it).commit() },
            generate = {
                Log.i(TAG, "No stored node key; generating a new one")
                Edgevpn.generateNodeKey()
            },
        )
    }
}

/**
 * Returns the key encoded in [stored], or generates one with [generate] and
 * passes its encoding to [save] when nothing usable is stored.
 */
internal fun loadOrCreateNodeKey(
    stored: String?,
    save: (String) -> Unit,
    generate: () -> ByteArray,
): ByteArray {
    if (!stored.isNullOrEmpty()) {
        try {
            return Base64.getDecoder().decode(stored)
        } catch (e: IllegalArgumentException) {
            // Fall through and replace it: an unreadable key is as good as none.
        }
    }
    val key = generate()
    save(Base64.getEncoder().encodeToString(key))
    return key
}
