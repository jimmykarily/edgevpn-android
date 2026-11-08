package com.kairos.edgevpn

import android.content.ClipData
import android.content.ClipboardManager
import android.content.Intent
import android.content.res.ColorStateList
import android.net.ConnectivityManager
import android.net.NetworkCapabilities
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.util.Log
import android.view.LayoutInflater
import android.view.Menu
import android.view.MenuItem
import android.view.View
import android.widget.ImageView
import android.widget.LinearLayout
import android.widget.TextView
import android.widget.Toast
import androidx.activity.result.contract.ActivityResultContracts
import androidx.annotation.ColorRes
import androidx.appcompat.app.AppCompatActivity
import androidx.core.content.ContextCompat
import com.google.android.material.appbar.MaterialToolbar
import com.google.android.material.bottomsheet.BottomSheetDialog
import com.google.android.material.button.MaterialButton
import com.kairos.edgevpn.ui.Ago
import com.kairos.edgevpn.ui.Badge
import com.kairos.edgevpn.ui.DeviceRow
import com.kairos.edgevpn.ui.HelpTerm
import com.kairos.edgevpn.ui.How
import com.kairos.edgevpn.ui.LegendLineDrawable
import com.kairos.edgevpn.ui.MainButton
import com.kairos.edgevpn.ui.NetFacts
import com.kairos.edgevpn.ui.NetworkMapView
import com.kairos.edgevpn.ui.PhoneFacts
import com.kairos.edgevpn.ui.SecondTile
import com.kairos.edgevpn.ui.StatusModel
import com.kairos.edgevpn.ui.Using
import com.kairos.edgevpn.ui.VpnState
import com.kairos.edgevpn.ui.appendHelpIcon
import com.kairos.edgevpn.ui.helpTerm
import com.kairos.edgevpn.ui.formatAgo
import com.kairos.edgevpn.ui.liveValues
import com.kairos.edgevpn.ui.parseStatus
import com.kairos.edgevpn.ui.pickUsing
import com.kairos.edgevpn.ui.showExpanded
import com.kairos.edgevpn.ui.planScreen
import com.kairos.edgevpn.ui.showHelp
import java.util.Locale

class MainActivity : AppCompatActivity() {

    companion object {
        private const val TAG = "EdgeVPN"
        private const val POLL_MS = 2000L
    }

    private lateinit var statusCardBody: View
    private lateinit var statusTitle: TextView
    private lateinit var statusText: TextView
    private lateinit var mainButton: MaterialButton
    private lateinit var seeLogsButton: View
    private lateinit var addressInUseText: TextView
    private lateinit var localNetworkClashText: TextView
    private lateinit var changeAddressButton: View
    private lateinit var pills: View
    private lateinit var addressPill: View
    private lateinit var addressValue: TextView
    private lateinit var usingValue: TextView
    private lateinit var mapSection: View
    private lateinit var map: NetworkMapView
    private lateinit var mapLegend: View
    private lateinit var devicesSection: View
    private lateinit var deviceList: LinearLayout
    private lateinit var findingSection: View
    private lateinit var helpersValue: TextView
    private lateinit var helpersLabel: TextView
    private lateinit var secondTile: View
    private lateinit var secondValue: TextView
    private lateinit var secondLabel: TextView

    private val handler = Handler(Looper.getMainLooper())
    /** The texts the device rows show, to rebuild the rows only when one changes. */
    private var shownRows: List<List<Any?>>? = null
    private var currentButton = MainButton.TURN_ON

    private val poll = object : Runnable {
        override fun run() {
            refresh()
            handler.postDelayed(this, POLL_MS)
        }
    }

    /** A refresh shortly after Turn on or Turn off, so the card follows the tap. */
    private val refreshSoon = Runnable { refresh() }

    /** The open device sheet, dismissed with the activity so its window does not leak. */
    private var deviceSheet: BottomSheetDialog? = null

    /** The open help sheet, dismissed with the activity for the same reason. */
    private var helpSheet: BottomSheetDialog? = null

    /** The latest snapshot, which open help sheets read their live values from. */
    private var lastModel: StatusModel? = null

    private val vpnPermissionLauncher = registerForActivityResult(
        ActivityResultContracts.StartActivityForResult()
    ) { result ->
        if (result.resultCode == RESULT_OK) {
            startVpnService()
        } else {
            Toast.makeText(this, R.string.vpn_permission_denied, Toast.LENGTH_SHORT).show()
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)
        setSupportActionBar(findViewById<MaterialToolbar>(R.id.toolbar))

        statusCardBody = findViewById(R.id.statusCardBody)
        statusTitle = findViewById(R.id.statusTitle)
        statusText = findViewById(R.id.statusText)
        mainButton = findViewById(R.id.mainButton)
        seeLogsButton = findViewById(R.id.seeLogsButton)
        addressInUseText = findViewById(R.id.addressInUseText)
        localNetworkClashText = findViewById(R.id.localNetworkClashText)
        changeAddressButton = findViewById(R.id.changeAddressButton)
        pills = findViewById(R.id.pills)
        addressPill = findViewById(R.id.addressPill)
        addressValue = findViewById(R.id.addressValue)
        usingValue = findViewById(R.id.usingValue)
        mapSection = findViewById(R.id.mapSection)
        map = findViewById(R.id.map)
        mapLegend = findViewById(R.id.mapLegend)
        map.onDeviceClick = { showDeviceSheet(it) }
        legendLine(R.id.legendDirect, Badge.DIRECT)
        legendLine(R.id.legendViaHelper, Badge.VIA_HELPER)
        legendLine(R.id.legendOffline, Badge.OFFLINE)
        helpIcon(R.id.pillThisPhoneLabel, HelpTerm.THIS_PHONE)
        helpIcon(R.id.pillUsingLabel, HelpTerm.USING)
        sectionHeader(R.id.sectionDevices, HelpTerm.YOUR_DEVICES)
        sectionHeader(R.id.sectionFinding, HelpTerm.FINDING)
        devicesSection = findViewById(R.id.devicesSection)
        deviceList = findViewById(R.id.deviceList)
        findingSection = findViewById(R.id.findingSection)
        helpersValue = findViewById(R.id.helpersValue)
        helpersLabel = findViewById(R.id.helpersLabel)
        secondTile = findViewById(R.id.secondTile)
        secondValue = findViewById(R.id.secondValue)
        secondLabel = findViewById(R.id.secondLabel)

        mainButton.setOnClickListener {
            when (currentButton) {
                MainButton.TURN_OFF -> disconnect()
                MainButton.TURN_ON, MainButton.TRY_AGAIN -> connect()
            }
        }
        seeLogsButton.setOnClickListener { openLogs() }
        changeAddressButton.setOnClickListener { openSettings() }
        connectIfAsked()
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        connectIfAsked()
    }

    /** Joining a network sends the user here to start the VPN with the new key. */
    private fun connectIfAsked() {
        if (ConnectRequest.take() && !EdgeVpnService.isRunning) connect()
    }

    override fun onStart() {
        super.onStart()
        handler.post(poll)
    }

    override fun onStop() {
        handler.removeCallbacks(poll)
        handler.removeCallbacks(refreshSoon)
        super.onStop()
    }

    override fun onDestroy() {
        deviceSheet?.dismiss()
        deviceSheet = null
        helpSheet?.dismiss()
        helpSheet = null
        super.onDestroy()
    }

    override fun onCreateOptionsMenu(menu: Menu): Boolean {
        menuInflater.inflate(R.menu.main, menu)
        return true
    }

    override fun onOptionsItemSelected(item: MenuItem): Boolean = when (item.itemId) {
        R.id.menu_settings -> {
            openSettings()
            true
        }
        R.id.menu_logs -> {
            openLogs()
            true
        }
        else -> super.onOptionsItemSelected(item)
    }

    private fun openLogs() {
        startActivity(Intent(this, LogsActivity::class.java))
    }

    private fun openSettings() {
        startActivity(Intent(this, SettingsActivity::class.java))
    }

    private fun connect() {
        if (TokenStore.load(this) == null) {
            startActivity(Intent(this, JoinActivity::class.java))
            return
        }
        val intent = VpnStarter.prepare(this)
        if (intent != null) {
            vpnPermissionLauncher.launch(intent)
        } else {
            startVpnService()
        }
    }

    private fun startVpnService() {
        VpnStarter.start(this)
        handler.postDelayed(refreshSoon, 300)
    }

    private fun disconnect() {
        startService(
            Intent(this, EdgeVpnService::class.java).setAction(EdgeVpnService.ACTION_DISCONNECT)
        )
        handler.postDelayed(refreshSoon, 300)
    }

    private fun refresh() {
        val model = try {
            parseStatus(NetworkSource.snapshot(), System.currentTimeMillis() / 1000)
        } catch (e: Exception) {
            Log.w(TAG, "Could not read the network snapshot", e)
            return
        }
        lastModel = model
        render(model)
    }

    private fun render(m: StatusModel) {
        val plan = planScreen(m)
        renderCard(m, plan.onlyDevice)

        currentButton = plan.button
        mainButton.setText(
            when (plan.button) {
                MainButton.TURN_ON -> R.string.button_turn_on
                MainButton.TURN_OFF -> R.string.button_turn_off
                MainButton.TRY_AGAIN -> R.string.button_try_again
            }
        )
        seeLogsButton.visibility = visibleIf(plan.showSeeLogs)
        addressInUseText.visibility = visibleIf(plan.showAddressInUse)
        changeAddressButton.visibility = visibleIf(plan.showAddressInUse)
        if (plan.showAddressInUse) {
            addressInUseText.text = getString(R.string.address_in_use_text, m.selfAddress)
            addressInUseText.appendHelpIcon(HelpTerm.ADDRESS_IN_USE) { openHelp(it) }
        }
        localNetworkClashText.visibility = visibleIf(plan.showLocalNetworkClash)
        if (plan.showLocalNetworkClash) {
            localNetworkClashText.text = getString(R.string.local_network_clash_text, vpnRangeLabel(m.selfAddress))
            localNetworkClashText.appendHelpIcon(HelpTerm.LOCAL_NETWORK_CLASH) { openHelp(it) }
        }

        pills.visibility = visibleIf(plan.showPills)
        addressPill.visibility = visibleIf(plan.showAddressPill)
        addressValue.text = m.selfAddress
        if (plan.showPills) usingValue.text = usingText()

        mapSection.visibility = visibleIf(plan.showMap)
        mapLegend.visibility = visibleIf(plan.showMapLegend)
        map.setLooking(plan.mapLooking)
        map.setDevices(m.devices)

        devicesSection.visibility = visibleIf(plan.showDevices)
        if (plan.showDevices) {
            val rows = m.devices.map { listOf(it.id, deviceName(it), it.address, it.badge, howText(it.how)) }
            if (rows != shownRows) {
                shownRows = rows
                renderDevices(m.devices)
            }
        }

        findingSection.visibility = visibleIf(plan.showTiles)
        helpersValue.text = m.helpers.toString()
        helpersLabel.text = resources.getQuantityString(R.plurals.tile_helpers, m.helpers)
        helpersLabel.appendHelpIcon(HelpTerm.HELPERS) { openHelp(it) }
        secondTile.visibility = visibleIf(plan.secondTile != SecondTile.NONE)
        when (plan.secondTile) {
            SecondTile.LAST_HEARD -> {
                secondValue.text = agoText(m.lastHeardSecondsAgo ?: 0)
                secondLabel.setText(R.string.tile_last_heard)
                secondLabel.appendHelpIcon(HelpTerm.LAST_HEARD) { openHelp(it) }
            }
            SecondTile.FOUND_SO_FAR -> {
                secondValue.text = m.total.toString()
                secondLabel.text = resources.getQuantityString(R.plurals.tile_found, m.total)
                secondLabel.appendHelpIcon(HelpTerm.YOUR_DEVICES) { openHelp(it) }
            }
            SecondTile.NONE -> Unit
        }
    }

    private fun renderCard(m: StatusModel, onlyDevice: Boolean) {
        val (background, icon) = when (m.state) {
            VpnState.OFF -> R.drawable.bg_card_off to 0
            VpnState.LOOKING -> R.drawable.bg_card_looking to R.drawable.ic_sync
            VpnState.CONNECTED -> R.drawable.bg_card_connected to R.drawable.ic_check
            VpnState.PROBLEM -> R.drawable.bg_card_problem to R.drawable.ic_error
        }
        statusCardBody.setBackgroundResource(background)
        statusTitle.setCompoundDrawablesRelativeWithIntrinsicBounds(icon, 0, 0, 0)
        statusTitle.setText(
            when (m.state) {
                VpnState.OFF -> R.string.state_off_title
                VpnState.LOOKING -> R.string.state_looking_title
                VpnState.CONNECTED -> R.string.state_connected_title
                VpnState.PROBLEM -> R.string.state_problem_title
            }
        )
        statusTitle.appendHelpIcon(
            when (m.state) {
                VpnState.OFF -> HelpTerm.OFF
                VpnState.LOOKING -> HelpTerm.LOOKING
                VpnState.CONNECTED -> HelpTerm.CONNECTED
                VpnState.PROBLEM -> HelpTerm.PROBLEM
            }
        ) { openHelp(it) }
        statusText.text = when (m.state) {
            VpnState.OFF -> getString(R.string.state_off_text)
            VpnState.LOOKING -> m.runningSeconds?.let {
                getString(R.string.state_looking_text_started, agoText(it))
            } ?: getString(R.string.state_looking_text)
            VpnState.CONNECTED -> if (onlyDevice) {
                getString(R.string.state_connected_text_one)
            } else {
                resources.getQuantityString(R.plurals.state_connected_text, m.total, m.reachable, m.total)
            }
            VpnState.PROBLEM -> m.error ?: getString(R.string.state_problem_text)
        }
    }

    private fun renderDevices(devices: List<DeviceRow>) {
        deviceList.removeAllViews()
        val inflater = LayoutInflater.from(this)
        devices.forEachIndexed { i, d ->
            if (i > 0) deviceList.addView(divider())
            val row = inflater.inflate(R.layout.item_device, deviceList, false)
            val badge = row.findViewById<TextView>(R.id.deviceBadge)
            bindBadge(row.findViewById(R.id.deviceIcon), badge, d.badge)
            badge.appendHelpIcon(d.badge.helpTerm) { openHelp(it, d) }
            row.findViewById<TextView>(R.id.deviceName).text = deviceName(d)
            row.findViewById<TextView>(R.id.deviceLine).text =
                getString(R.string.device_line, d.address, howText(d.how))
            row.setOnClickListener { showDeviceSheet(d) }
            deviceList.addView(row)
        }
    }

    private fun legendLine(id: Int, badge: Badge) {
        val line = findViewById<TextView>(id)
        line.setCompoundDrawablesRelativeWithIntrinsicBounds(
            LegendLineDrawable(badge, resources.displayMetrics.density), null, null, null,
        )
        line.appendHelpIcon(badge.helpTerm) { openHelp(it) }
    }

    private fun helpIcon(id: Int, term: HelpTerm) {
        findViewById<TextView>(id).appendHelpIcon(term) { openHelp(it) }
    }

    /** A section header in capitals, set in code so the (?) icon span survives. */
    private fun sectionHeader(id: Int, term: HelpTerm) {
        val header = findViewById<TextView>(id)
        header.text = header.text.toString().uppercase(Locale.getDefault())
        header.appendHelpIcon(term) { openHelp(it) }
    }

    /**
     * Opens the help sheet for [term]. With a [device] the sheet is about that device and shows
     * the live values of its connection, read from the newest snapshot by the device's ID.
     */
    private fun openHelp(term: HelpTerm, device: DeviceRow? = null) {
        helpSheet?.dismiss()
        fun current() = device?.let { d -> lastModel?.devices?.find { it.id == d.id } }
        helpSheet = showHelp(this, term, device?.let { deviceName(it) }, termNow = { term.followBadge(current()) }) {
            val m = lastModel ?: return@showHelp emptyList()
            val now = current()
            liveValues(term.followBadge(now), m, now, PhoneFacts(usingText(), AppSettings.detailedLogs(this)))
        }
    }

    private fun divider(): View = View(this).apply {
        layoutParams = LinearLayout.LayoutParams(LinearLayout.LayoutParams.MATCH_PARENT, dp(1)).apply {
            marginStart = dp(66)
        }
        setBackgroundColor(ContextCompat.getColor(this@MainActivity, R.color.divider))
    }

    private fun showDeviceSheet(d: DeviceRow) {
        val sheet = BottomSheetDialog(this)
        val view = layoutInflater.inflate(R.layout.sheet_device, null)
        bindBadge(view.findViewById(R.id.sheetIcon), view.findViewById(R.id.sheetBadge), d.badge)
        view.findViewById<TextView>(R.id.sheetName).text = deviceName(d)
        view.findViewById<TextView>(R.id.sheetAddress).text = d.address
        val how = view.findViewById<TextView>(R.id.sheetHow)
        how.text = howText(d.how).replaceFirstChar { it.uppercase() }
        if (d.how == How.SameWifi || d.how == How.OverInternet) {
            how.appendHelpIcon(HelpTerm.SAME_NETWORK) { openHelp(it, d) }
        }
        view.findViewById<View>(R.id.sheetRemoteGroup).visibility = visibleIf(d.remote != null)
        view.findViewById<TextView>(R.id.sheetRemote).text = d.remote
        view.findViewById<View>(R.id.sheetTransportGroup).visibility = visibleIf(d.transport != null)
        view.findViewById<TextView>(R.id.sheetTransport).text = d.transport
        view.findViewById<TextView>(R.id.sheetId).text = d.id
        view.findViewById<View>(R.id.sheetCopy).setOnClickListener {
            val clipboard = getSystemService(ClipboardManager::class.java)
            clipboard.setPrimaryClip(ClipData.newPlainText(getString(R.string.sheet_id), d.id))
            Toast.makeText(this, R.string.sheet_copied, Toast.LENGTH_SHORT).show()
        }
        sheet.setContentView(view)
        sheet.setOnDismissListener { if (deviceSheet === sheet) deviceSheet = null }
        deviceSheet = sheet
        sheet.showExpanded()
    }

    private fun bindBadge(icon: ImageView, badge: TextView, b: Badge) {
        val (bg, fg, label) = when (b) {
            Badge.DIRECT -> Triple(R.color.direct_bg, R.color.direct_fg, R.string.badge_direct)
            Badge.VIA_HELPER -> Triple(R.color.helper_bg, R.color.helper_fg, R.string.badge_via_helper)
            Badge.OFFLINE -> Triple(R.color.offline_bg, R.color.offline_fg, R.string.badge_offline)
        }
        icon.backgroundTintList = colorList(bg)
        icon.imageTintList = colorList(fg)
        badge.backgroundTintList = colorList(bg)
        badge.setTextColor(ContextCompat.getColor(this, fg))
        badge.setText(label)
    }

    private fun deviceName(d: DeviceRow): String = d.name ?: getString(R.string.unnamed_device)

    private fun howText(how: How): String = when (how) {
        How.SameWifi -> getString(R.string.how_same_wifi)
        How.OverInternet -> getString(R.string.how_over_internet)
        How.ThroughHelper -> getString(R.string.how_through_helper)
        is How.LastSeen -> getString(R.string.how_last_seen, agoText(how.secondsAgo))
        How.NotReachable -> getString(R.string.how_not_reachable)
    }

    private fun agoText(seconds: Long): String = when (val ago = formatAgo(seconds)) {
        Ago.JustNow -> getString(R.string.ago_just_now)
        is Ago.Seconds -> getString(R.string.ago_seconds, ago.n)
        is Ago.Minutes -> getString(R.string.ago_minutes, ago.n)
        is Ago.Hours -> getString(R.string.ago_hours, ago.n)
        is Ago.Days -> resources.getQuantityString(R.plurals.ago_days, ago.n.toInt(), ago.n)
    }

    @Suppress("DEPRECATION")
    private fun usingText(): String {
        val cm = getSystemService(ConnectivityManager::class.java)
        val facts = cm.allNetworks.mapNotNull { cm.getNetworkCapabilities(it) }.map {
            NetFacts(
                internet = it.hasCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET),
                vpn = it.hasTransport(NetworkCapabilities.TRANSPORT_VPN),
                wifi = it.hasTransport(NetworkCapabilities.TRANSPORT_WIFI),
                cellular = it.hasTransport(NetworkCapabilities.TRANSPORT_CELLULAR),
            )
        }
        return getString(
            when (pickUsing(facts)) {
                Using.WIFI -> R.string.using_wifi
                Using.MOBILE_DATA -> R.string.using_mobile_data
                Using.OTHER -> R.string.using_other
                null -> R.string.using_none
            }
        )
    }

    private fun colorList(@ColorRes id: Int) = ColorStateList.valueOf(ContextCompat.getColor(this, id))

    private fun visibleIf(show: Boolean) = if (show) View.VISIBLE else View.GONE

    private fun dp(v: Int) = (v * resources.displayMetrics.density).toInt()
}
