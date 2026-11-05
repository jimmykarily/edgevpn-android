package com.kairos.edgevpn

/**
 * True when the viewport shows the end of the content, give or take [slackPx],
 * which is when new log lines should keep the view pinned to the bottom.
 * Content shorter than the viewport counts as being at the bottom.
 */
fun shouldStickToBottom(scrollY: Int, viewportHeight: Int, contentHeight: Int, slackPx: Int): Boolean =
    scrollY + viewportHeight >= contentHeight - slackPx
