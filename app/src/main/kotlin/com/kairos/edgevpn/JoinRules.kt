package com.kairos.edgevpn

/** The text a QR scan puts in the key field, or null when the scan was cancelled or empty. */
fun scannedKey(contents: String?): String? = contents?.trim()?.takeIf { it.isNotEmpty() }

/** [entered] with every whitespace character removed, which is how the key is validated and saved. */
fun cleanKey(entered: String): String = entered.filterNot { it.isWhitespace() }

/** What tapping "Join network" does. */
sealed class JoinOutcome {
    /** The key is not usable: [message] is shown under the field and nothing is saved. */
    data class ShowProblem(val message: String) : JoinOutcome()

    /** Save [key]. With [askReconnect] the VPN is running and keeps the old key until it restarts. */
    data class Save(val key: String, val askReconnect: Boolean) : JoinOutcome()
}

/** Decides what to do with the [entered] key, given the validator's [problem] (empty when valid). */
fun joinOutcome(entered: String, problem: String, vpnRunning: Boolean): JoinOutcome =
    if (problem.isNotEmpty()) JoinOutcome.ShowProblem(problem)
    else JoinOutcome.Save(cleanKey(entered), askReconnect = vpnRunning)
