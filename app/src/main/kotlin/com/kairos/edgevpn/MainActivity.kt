package com.kairos.edgevpn

import android.content.Intent
import android.os.Bundle
import android.view.View
import android.widget.LinearLayout
import android.widget.TextView
import android.widget.Toast
import androidx.appcompat.app.AppCompatActivity
import com.google.android.material.button.MaterialButton
import edgevpn.Edgevpn

class MainActivity : AppCompatActivity() {

    private lateinit var noTokenGroup: LinearLayout
    private lateinit var connectedGroup: LinearLayout
    private lateinit var connectButton: MaterialButton
    private lateinit var statusText: TextView
    private var connected = false

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
            if (connected) disconnect() else connect()
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
        val token = TokenStore.load(this)
        if (token == null) {
            updateView()
            return
        }

        try {
            Edgevpn.start(token)
            connected = true
            updateView()
        } catch (e: Exception) {
            Toast.makeText(this, "Error: ${e.message}", Toast.LENGTH_LONG).show()
        }
    }

    private fun disconnect() {
        Edgevpn.stop()
        connected = false
        updateView()
    }

    private fun updateView() {
        val hasToken = TokenStore.load(this) != null

        noTokenGroup.visibility = if (hasToken) View.GONE else View.VISIBLE
        connectedGroup.visibility = if (hasToken) View.VISIBLE else View.GONE

        if (hasToken) {
            val status = Edgevpn.status()
            statusText.text = "Status: $status"
            connectButton.text = getString(
                if (connected) R.string.disconnect else R.string.connect
            )
        }
    }
}
