package com.kairos.edgevpn

/**
 * True when the viewport shows the end of the content, give or take [slackPx],
 * which is when new log lines should keep the view pinned to the bottom.
 * Content shorter than the viewport counts as being at the bottom.
 */
fun shouldStickToBottom(scrollY: Int, viewportHeight: Int, contentHeight: Int, slackPx: Int): Boolean =
    scrollY + viewportHeight >= contentHeight - slackPx

/**
 * True when the shown text should be replaced: the logs differ from what is
 * shown (or nothing has been shown yet) and the user is not selecting text,
 * since replacing the text would drop the selection.
 */
fun shouldRefreshLogs(shown: String?, latest: String, hasSelection: Boolean): Boolean =
    !hasSelection && latest != shown
