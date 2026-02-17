package com.kairos.edgevpn

import android.content.Intent
import android.net.VpnService
import android.os.Bundle
import android.view.View
import android.widget.LinearLayout
import android.widget.TextView
import android.widget.Toast
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import com.google.android.material.button.MaterialButton

class MainActivity : AppCompatActivity() {

    private lateinit var noTokenGroup: LinearLayout
    private lateinit var connectedGroup: LinearLayout
    private lateinit var connectButton: MaterialButton
    private lateinit var statusText: TextView

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

        findViewById<MaterialButton>(R.id.openSettingsButton).setOnClickListener {
            openSettings()
        }
        findViewById<MaterialButton>(R.id.mainSettingsButton).setOnClickListener {
            openSettings()
        }

        connectButton.setOnClickListener {
            if (EdgeVpnService.isRunning) disconnect() else connect()
        }
    }

    override fun onResume() {
        super.onResume()
        updateView()
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

        if (hasToken) {
            statusText.text = if (running) "Status: connected" else "Status: disconnected"
            connectButton.text = getString(
                if (running) R.string.disconnect else R.string.connect
            )
        }
    }
}
