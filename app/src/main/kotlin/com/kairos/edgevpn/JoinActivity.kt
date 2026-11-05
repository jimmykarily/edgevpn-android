package com.kairos.edgevpn

import android.Manifest
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Bundle
import android.util.Log
import android.widget.TextView
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AlertDialog
import androidx.appcompat.app.AppCompatActivity
import androidx.core.content.ContextCompat
import androidx.core.widget.doAfterTextChanged
import com.google.android.material.appbar.MaterialToolbar
import com.google.android.material.dialog.MaterialAlertDialogBuilder
import com.google.android.material.textfield.TextInputEditText
import com.google.android.material.textfield.TextInputLayout
import com.journeyapps.barcodescanner.ScanContract
import com.journeyapps.barcodescanner.ScanOptions
import com.kairos.edgevpn.ui.HelpTerm
import com.kairos.edgevpn.ui.PhoneFacts
import com.kairos.edgevpn.ui.appendHelpIcon
import com.kairos.edgevpn.ui.liveValues
import com.kairos.edgevpn.ui.parseStatus
import com.kairos.edgevpn.ui.showHelp
import edgevpn.Edgevpn

/** Where the user enters the network key, by pasting it or scanning its QR code. */
class JoinActivity : AppCompatActivity() {

    private lateinit var keyLayout: TextInputLayout
    private lateinit var keyField: TextInputEditText

    private var helpSheet: com.google.android.material.bottomsheet.BottomSheetDialog? = null
    private var dialog: AlertDialog? = null

    private val scanner = registerForActivityResult(ScanContract()) { result ->
        val text = scannedKey(result.contents) ?: return@registerForActivityResult
        keyField.setText(text)
        keyField.setSelection(keyField.length())
        validate()
    }

    private val cameraPermission = registerForActivityResult(ActivityResultContracts.RequestPermission()) { granted ->
        if (granted) scan() else keyLayout.error = getString(R.string.join_camera_denied)
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_join)
        findViewById<MaterialToolbar>(R.id.toolbar).setNavigationOnClickListener { finish() }

        keyLayout = findViewById(R.id.keyLayout)
        keyField = findViewById(R.id.keyField)
        keyField.doAfterTextChanged { keyLayout.error = null }

        findViewById<TextView>(R.id.joinHeading).appendHelpIcon(HelpTerm.NETWORK_KEY) { openHelp(it) }
        findViewById<android.view.View>(R.id.joinButton).setOnClickListener { validate() }
        findViewById<android.view.View>(R.id.scanButton).setOnClickListener { scanWithPermission() }
    }

    override fun onDestroy() {
        helpSheet?.dismiss()
        helpSheet = null
        dialog?.dismiss()
        dialog = null
        super.onDestroy()
    }

    /** Asks for the camera first, because the scanner shows a confusing error when it is denied. */
    private fun scanWithPermission() {
        if (ContextCompat.checkSelfPermission(this, Manifest.permission.CAMERA) == PackageManager.PERMISSION_GRANTED) {
            scan()
        } else {
            cameraPermission.launch(Manifest.permission.CAMERA)
        }
    }

    private fun scan() {
        scanner.launch(
            ScanOptions()
                .setCaptureActivity(PortraitCaptureActivity::class.java)
                .setDesiredBarcodeFormats(ScanOptions.QR_CODE)
                .setBeepEnabled(false)
                .setOrientationLocked(true)
                .setPrompt(getString(R.string.join_scan_prompt))
        )
    }

    /** Checks the key in the field: shows the problem under it, or saves it and leaves. */
    private fun validate() {
        val entered = keyField.text?.toString().orEmpty()
        val outcome = joinOutcome(entered, Edgevpn.validateToken(entered), EdgeVpnService.isRunning)
        when (outcome) {
            is JoinOutcome.ShowProblem -> keyLayout.error = outcome.message
            is JoinOutcome.Save -> {
                TokenStore.save(this, outcome.key)
                if (outcome.askReconnect) askReconnect() else openMain(connect = true)
            }
        }
    }

    private fun askReconnect() {
        dialog?.dismiss()
        dialog = MaterialAlertDialogBuilder(this)
            .setMessage(R.string.reconnect_key_text)
            .setCancelable(false)
            .setNegativeButton(R.string.reconnect_later) { _, _ -> openMain(connect = false) }
            .setPositiveButton(R.string.reconnect_confirm) { _, _ ->
                VpnControl.reconnect(applicationContext)
                openMain(connect = false)
            }
            .show()
    }

    /** Goes back to the main screen, closing the screens above it, and starts the VPN if asked. */
    private fun openMain(connect: Boolean) {
        startActivity(
            Intent(this, MainActivity::class.java)
                .addFlags(Intent.FLAG_ACTIVITY_CLEAR_TOP or Intent.FLAG_ACTIVITY_SINGLE_TOP)
                .putExtra(MainActivity.EXTRA_CONNECT, connect)
        )
        finish()
    }

    private fun openHelp(term: HelpTerm) {
        helpSheet?.dismiss()
        helpSheet = showHelp(this, term) {
            val model = try {
                parseStatus(Edgevpn.snapshot(), System.currentTimeMillis() / 1000)
            } catch (e: Exception) {
                Log.w("EdgeVPN", "Could not read the network snapshot", e)
                return@showHelp emptyList()
            }
            liveValues(term, model, null, PhoneFacts(detailedLogs = AppSettings.detailedLogs(this)))
        }
    }
}
