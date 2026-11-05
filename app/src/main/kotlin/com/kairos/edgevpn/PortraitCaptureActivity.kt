package com.kairos.edgevpn

import com.journeyapps.barcodescanner.CaptureActivity

/** The QR scanner screen; the manifest fixes it to portrait, where the library's own is landscape. */
class PortraitCaptureActivity : CaptureActivity()
