package com.kairos.edgevpn

import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import android.graphics.Typeface
import android.os.Build
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.text.InputType
import android.util.Log
import android.view.View
import android.view.WindowManager
import android.widget.TextView
import android.widget.Toast
import androidx.appcompat.app.AlertDialog
import androidx.appcompat.app.AppCompatActivity
import androidx.core.app.TaskStackBuilder
import androidx.core.widget.doAfterTextChanged
import com.google.android.material.appbar.MaterialToolbar
import com.google.android.material.bottomsheet.BottomSheetDialog
import com.google.android.material.button.MaterialButton
import com.google.android.material.dialog.MaterialAlertDialogBuilder
import com.google.android.material.materialswitch.MaterialSwitch
import com.google.android.material.textfield.TextInputEditText
import com.google.android.material.textfield.TextInputLayout
import com.kairos.edgevpn.ui.HelpTerm
import com.kairos.edgevpn.ui.PhoneFacts
import com.kairos.edgevpn.ui.appendHelpIcon
import com.kairos.edgevpn.ui.liveValues
import com.kairos.edgevpn.ui.parseStatus
import com.kairos.edgevpn.ui.showHelp
import edgevpn.Edgevpn
import java.util.concurrent.Executors

class SettingsActivity : AppCompatActivity() {

    companion object {
        private const val TAG = "EdgeVPN"

        /** Work that reads or writes the encrypted settings, kept off the main thread. */
        private val io = Executors.newSingleThreadExecutor()
    }

    private val handler = Handler(Looper.getMainLooper())

    private lateinit var netTitle: TextView
    private lateinit var netKey: TextView
    private lateinit var netMore: TextView
    private lateinit var replaceKeyButton: MaterialButton
    private lateinit var leaveButton: View
    private lateinit var nameValue: TextView
    private lateinit var addressValue: TextView
    private lateinit var detailedLogsSwitch: MaterialSwitch
    private lateinit var phoneIdValue: TextView

    /** The full phone ID, once it has been read. */
    private var phoneId: String? = null

    /** The open help sheet, dismissed with the activity so its window does not leak. */
    private var helpSheet: BottomSheetDialog? = null

    /** The open dialog, dismissed with the activity for the same reason. */
    private var dialog: AlertDialog? = null

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_settings)
        val toolbar = findViewById<MaterialToolbar>(R.id.toolbar)
        toolbar.setNavigationOnClickListener { finish() }

        netTitle = findViewById(R.id.netTitle)
        netKey = findViewById(R.id.netKey)
        netMore = findViewById(R.id.netMore)
        replaceKeyButton = findViewById(R.id.replaceKeyButton)
        leaveButton = findViewById(R.id.leaveButton)
        nameValue = findViewById(R.id.nameValue)
        addressValue = findViewById(R.id.addressValue)
        detailedLogsSwitch = findViewById(R.id.detailedLogsSwitch)
        phoneIdValue = findViewById(R.id.phoneIdValue)

        helpIcon(R.id.nameTitle, HelpTerm.NAME)
        helpIcon(R.id.addressTitle, HelpTerm.ADDRESS)
        helpIcon(R.id.detailedLogsTitle, HelpTerm.DETAILED_LOGS)
        helpIcon(R.id.phoneIdTitle, HelpTerm.PHONE_ID)
        helpIcon(R.id.startFreshTitle, HelpTerm.START_FRESH)

        replaceKeyButton.setOnClickListener { openJoin(clearBackStack = false) }
        leaveButton.setOnClickListener { confirmLeave() }
        findViewById<View>(R.id.nameRow).setOnClickListener { editName() }
        findViewById<View>(R.id.addressRow).setOnClickListener { editAddress() }
        findViewById<View>(R.id.logsRow).setOnClickListener {
            startActivity(Intent(this, LogsActivity::class.java))
        }
        findViewById<View>(R.id.detailedLogsRow).setOnClickListener { detailedLogsSwitch.toggle() }
        findViewById<View>(R.id.phoneIdRow).setOnClickListener { copyPhoneId() }
        findViewById<View>(R.id.startFreshRow).setOnClickListener { confirmStartFresh() }

        detailedLogsSwitch.isChecked = AppSettings.detailedLogs(this)
        detailedLogsSwitch.setOnCheckedChangeListener { _, on ->
            AppSettings.setDetailedLogs(this, on)
            Edgevpn.setDetailedLogs(on)
        }

        val libraryVersion = Edgevpn.libraryVersion().removePrefix("v")
        findViewById<TextView>(R.id.versionValue).text =
            getString(R.string.setting_version_sub, BuildConfig.VERSION_NAME, libraryVersion)

        renderNetwork()
        renderPhone()
        loadPhoneId()
    }

    override fun onDestroy() {
        helpSheet?.dismiss()
        helpSheet = null
        dialog?.dismiss()
        dialog = null
        super.onDestroy()
    }

    private fun renderNetwork() {
        val joined = TokenStore.load(this) != null
        netTitle.setText(if (joined) R.string.net_joined_title else R.string.net_none_title)
        netKey.setText(if (joined) R.string.net_joined_key else R.string.net_none_key)
        netKey.appendHelpIcon(HelpTerm.NETWORK_KEY) { openHelp(it) }
        netMore.setText(if (joined) R.string.net_joined_private else R.string.net_none_where)
        replaceKeyButton.setText(if (joined) R.string.net_replace_key else R.string.net_join)
        leaveButton.visibility = if (joined) View.VISIBLE else View.GONE
    }

    private fun renderPhone() {
        nameValue.text = AppSettings.name(this)
        addressValue.text = AppSettings.address(this)
    }

    /** Reads the phone ID off the main thread: the first read may create and store the identity key. */
    private fun loadPhoneId() {
        val app = applicationContext
        io.execute {
            val id = try {
                Edgevpn.peerIDFromKey(NodeKeyStore.loadOrCreate(app))
            } catch (e: Exception) {
                Log.w(TAG, "Could not read the phone ID", e)
                ""
            }
            runOnUiThread {
                if (isDestroyed) return@runOnUiThread
                phoneId = id.ifEmpty { null }
                phoneIdValue.text = getString(R.string.setting_phone_id_sub, shortId(id))
            }
        }
    }

    private fun copyPhoneId() {
        val id = phoneId ?: return
        val clipboard = getSystemService(ClipboardManager::class.java)
        clipboard.setPrimaryClip(ClipData.newPlainText(getString(R.string.setting_phone_id), id))
        Toast.makeText(this, R.string.phone_id_copied, Toast.LENGTH_SHORT).show()
    }

    private fun editName() {
        showFieldDialog(
            title = R.string.name_title,
            hint = getString(R.string.name_hint),
            text = AppSettings.name(this),
            helper = getString(R.string.name_empty_note, Build.MODEL),
            inputType = InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_FLAG_CAP_WORDS,
        ) { entered ->
            AppSettings.setName(this, entered.trim())
            val name = AppSettings.name(this)
            if (EdgeVpnService.isRunning) Edgevpn.setName(name)
            renderPhone()
            null
        }
    }

    private fun editAddress() {
        val current = AppSettings.address(this)
        showFieldDialog(
            title = R.string.address_title,
            hint = null,
            text = current,
            helper = null,
            inputType = InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_VARIATION_URI,
        ) { entered ->
            when (addressChange(current, entered, EdgeVpnService.isRunning)) {
                AddressChange.INVALID -> return@showFieldDialog getString(R.string.address_invalid)
                AddressChange.UNCHANGED -> Unit
                AddressChange.SAVE -> AppSettings.setAddress(this, entered.trim())
                AddressChange.SAVE_AND_ASK_RECONNECT -> {
                    AppSettings.setAddress(this, entered.trim())
                    handler.post { askReconnect() }
                }
            }
            renderPhone()
            null
        }
    }

    private fun askReconnect() {
        show(
            MaterialAlertDialogBuilder(this)
                .setMessage(R.string.reconnect_text)
                .setNegativeButton(R.string.reconnect_later, null)
                .setPositiveButton(R.string.reconnect_confirm) { _, _ -> reconnect() }
        )
    }

    private fun reconnect() = VpnControl.reconnect(applicationContext)

    private fun stopVpn(c: Context) = VpnControl.stop(c)

    private fun confirmLeave() {
        show(
            MaterialAlertDialogBuilder(this)
                .setTitle(R.string.leave_title)
                .setMessage(R.string.leave_text)
                .setNegativeButton(R.string.dialog_cancel, null)
                .setPositiveButton(R.string.leave_confirm) { _, _ ->
                    if (EdgeVpnService.isRunning) stopVpn(this)
                    TokenStore.clear(this)
                    finish()
                }
        )
    }

    private fun confirmStartFresh() {
        show(
            MaterialAlertDialogBuilder(this)
                .setTitle(R.string.start_fresh_title)
                .setMessage(R.string.start_fresh_text)
                .setNegativeButton(R.string.dialog_cancel, null)
                .setPositiveButton(R.string.start_fresh_confirm) { _, _ -> startFresh() }
        )
    }

    /**
     * Stops the VPN and deletes everything the app stores. Go is stopped here as well as through
     * the service, so no session writes to the log buffer after it has been emptied.
     */
    private fun startFresh() {
        val app = applicationContext
        if (EdgeVpnService.isRunning) stopVpn(app)
        io.execute {
            Edgevpn.stop()
            AppSettings.startFresh(app)
            runOnUiThread {
                if (!isDestroyed) openJoin(clearBackStack = true)
            }
        }
    }

    /**
     * Opens the Join screen. With [clearBackStack] the screens behind this one are closed first,
     * and the main screen with Join on top opens fresh.
     */
    private fun openJoin(clearBackStack: Boolean) {
        if (!clearBackStack) {
            startActivity(Intent(this, JoinActivity::class.java))
            return
        }
        TaskStackBuilder.create(this)
            .addNextIntent(Intent(this, MainActivity::class.java))
            .addNextIntent(Intent(this, JoinActivity::class.java))
            .startActivities()
    }

    /**
     * Shows a dialog with one text field. [onSave] gets the text and returns an error to show
     * under the field, which keeps the dialog open, or null to close it.
     */
    private fun showFieldDialog(
        title: Int,
        hint: String?,
        text: String,
        helper: String?,
        inputType: Int,
        onSave: (String) -> String?,
    ) {
        val view = layoutInflater.inflate(R.layout.dialog_field, null)
        val layout = view.findViewById<TextInputLayout>(R.id.fieldLayout)
        val field = view.findViewById<TextInputEditText>(R.id.field)
        layout.hint = hint
        layout.helperText = helper
        field.inputType = inputType
        if (inputType and InputType.TYPE_TEXT_FLAG_MULTI_LINE != 0) {
            field.minLines = 3
            field.maxLines = 6
            field.typeface = Typeface.MONOSPACE
        }
        field.setText(text)
        field.setSelection(field.length())
        field.doAfterTextChanged { layout.error = null }

        val d = show(
            MaterialAlertDialogBuilder(this)
                .setTitle(title)
                .setView(view)
                .setNegativeButton(R.string.dialog_cancel, null)
                .setPositiveButton(R.string.dialog_save, null),
            keyboard = true,
        )
        // Set here rather than on the builder, so an error keeps the dialog open.
        d.getButton(AlertDialog.BUTTON_POSITIVE).setOnClickListener {
            val error = onSave(field.text?.toString().orEmpty())
            if (error == null) {
                d.dismiss()
            } else {
                layout.error = error
            }
        }
        field.requestFocus()
    }

    private fun show(builder: MaterialAlertDialogBuilder, keyboard: Boolean = false): AlertDialog {
        dialog?.dismiss()
        val d = builder.create()
        d.setOnDismissListener { if (dialog === d) dialog = null }
        dialog = d
        if (keyboard) d.window?.setSoftInputMode(WindowManager.LayoutParams.SOFT_INPUT_STATE_VISIBLE)
        d.show()
        return d
    }

    private fun helpIcon(id: Int, term: HelpTerm) {
        findViewById<TextView>(id).appendHelpIcon(term) { openHelp(it) }
    }

    /** Opens the help sheet for [term], with live values from the newest snapshot. */
    private fun openHelp(term: HelpTerm) {
        helpSheet?.dismiss()
        helpSheet = showHelp(this, term) {
            val model = try {
                parseStatus(NetworkSource.snapshot(), System.currentTimeMillis() / 1000)
            } catch (e: Exception) {
                Log.w(TAG, "Could not read the network snapshot", e)
                return@showHelp emptyList()
            }
            liveValues(term, model, null, PhoneFacts(detailedLogs = AppSettings.detailedLogs(this)))
        }
    }
}
