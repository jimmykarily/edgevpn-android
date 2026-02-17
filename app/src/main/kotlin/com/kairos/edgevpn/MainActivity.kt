package com.kairos.edgevpn

import android.content.Intent
import android.net.VpnService
import android.os.Bundle
import android.util.Log
import android.os.Handler
import android.os.Looper
import android.view.View
import android.widget.LinearLayout
import android.widget.ScrollView
import android.widget.TextView
import android.widget.Toast
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import com.google.android.material.button.MaterialButton
import edgevpn.Edgevpn
import edgevpn.LogReceiver

class MainActivity : AppCompatActivity() {

    private lateinit var noTokenGroup: LinearLayout
    private lateinit var connectedGroup: LinearLayout
    private lateinit var connectButton: MaterialButton
    private lateinit var statusText: TextView
    private lateinit var toggleLogsButton: MaterialButton
    private lateinit var logScrollView: ScrollView
    private lateinit var logText: TextView

    private val handler = Handler(Looper.getMainLooper())
    private var logsVisible = false

    private val logReceiver = object : LogReceiver {
        override fun onLog(line: String?) {
            if (line != null) {
                // Send to logcat so "make logs" shows the same as the in-app log viewer
                Log.i(LOG_TAG, line.trim())
            }
            handler.post {
                if (logsVisible && line != null) {
                    logText.append(line)
                    logText.append("\n")
                    logScrollView.post { logScrollView.fullScroll(View.FOCUS_DOWN) }
                }
            }
        }
    }

    companion object {
        private const val LOG_TAG = "EdgeVPN"
    }

    private val vpnPermissionLauncher = registerForActivityResult(
        ActivityResultContracts.StartActivityForResult()
    ) { result ->
        if (result.resultCode == RESULT_OK) {
            startVpnService()
        } else {
            Toast.makeText(this, "VPN permission denied", Toast.LENGTH_SHORT).show()
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)

        noTokenGroup = findViewById(R.id.noTokenGroup)
        connectedGroup = findViewById(R.id.connectedGroup)
        connectButton = findViewById(R.id.connectButton)
        statusText = findViewById(R.id.statusText)
        toggleLogsButton = findViewById(R.id.toggleLogsButton)
        logScrollView = findViewById(R.id.logScrollView)
        logText = findViewById(R.id.logText)

        findViewById<MaterialButton>(R.id.openSettingsButton).setOnClickListener {
            openSettings()
        }
        findViewById<MaterialButton>(R.id.mainSettingsButton).setOnClickListener {
            openSettings()
        }

        connectButton.setOnClickListener {
            if (EdgeVpnService.isRunning) disconnect() else connect()
        }

        toggleLogsButton.setOnClickListener {
            logsVisible = !logsVisible
            logScrollView.visibility = if (logsVisible) View.VISIBLE else View.GONE
            toggleLogsButton.text = getString(
                if (logsVisible) R.string.hide_logs else R.string.show_logs
            )
        }
    }

    override fun onResume() {
        super.onResume()
        updateView()
        Edgevpn.setLogReceiver(logReceiver)
    }

    override fun onPause() {
        super.onPause()
        Edgevpn.setLogReceiver(null)
    }

    private fun openSettings() {
        startActivity(Intent(this, SettingsActivity::class.java))
    }

    private fun connect() {
        if (TokenStore.load(this) == null) {
            updateView()
            return
        }

        val intent = VpnService.prepare(this)
        if (intent != null) {
            vpnPermissionLauncher.launch(intent)
        } else {
            startVpnService()
        }
    }

    private fun startVpnService() {
        val intent = Intent(this, EdgeVpnService::class.java).apply {
            action = EdgeVpnService.ACTION_CONNECT
        }
        startForegroundService(intent)
        statusText.text = "Status: connected"
        connectButton.text = getString(R.string.disconnect)
    }

    private fun disconnect() {
        val intent = Intent(this, EdgeVpnService::class.java).apply {
            action = EdgeVpnService.ACTION_DISCONNECT
        }
        startService(intent)
        statusText.text = "Status: disconnected"
        connectButton.text = getString(R.string.connect)
    }

    private fun updateView() {
        val hasToken = TokenStore.load(this) != null
        val running = EdgeVpnService.isRunning

        noTokenGroup.visibility = if (hasToken) View.GONE else View.VISIBLE
        connectedGroup.visibility = if (hasToken) View.VISIBLE else View.GONE
        toggleLogsButton.visibility = if (hasToken) View.VISIBLE else View.GONE

        if (!hasToken) {
            logScrollView.visibility = View.GONE
        }

        if (hasToken) {
            statusText.text = if (running) "Status: connected" else "Status: disconnected"
            connectButton.text = getString(
                if (running) R.string.disconnect else R.string.connect
            )
        }
    }
}
