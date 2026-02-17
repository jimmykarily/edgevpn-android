package com.kairos.edgevpn

import android.os.Bundle
import android.view.View
import android.widget.LinearLayout
import android.widget.Toast
import androidx.appcompat.app.AppCompatActivity
import com.google.android.material.button.MaterialButton
import com.google.android.material.textfield.TextInputEditText

class SettingsActivity : AppCompatActivity() {

    private lateinit var tokenInputGroup: LinearLayout
    private lateinit var tokenSavedGroup: LinearLayout
    private lateinit var tokenInput: TextInputEditText
    private lateinit var saveButton: MaterialButton
    private lateinit var deleteButton: MaterialButton

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_settings)

        supportActionBar?.title = getString(R.string.settings_title)
        supportActionBar?.setDisplayHomeAsUpEnabled(true)

        tokenInputGroup = findViewById(R.id.tokenInputGroup)
        tokenSavedGroup = findViewById(R.id.tokenSavedGroup)
        tokenInput = findViewById(R.id.tokenInput)
        saveButton = findViewById(R.id.saveButton)
        deleteButton = findViewById(R.id.deleteButton)

        saveButton.setOnClickListener { saveToken() }
        deleteButton.setOnClickListener { deleteToken() }

        updateView()
    }

    override fun onSupportNavigateUp(): Boolean {
        finish()
        return true
    }

    private fun saveToken() {
        val token = tokenInput.text?.toString()?.trim() ?: ""
        if (token.isEmpty()) {
            Toast.makeText(this, R.string.token_empty_error, Toast.LENGTH_SHORT).show()
            return
        }
        TokenStore.save(this, token)
        tokenInput.text?.clear()
        Toast.makeText(this, R.string.token_saved, Toast.LENGTH_SHORT).show()
        updateView()
    }

    private fun deleteToken() {
        TokenStore.clear(this)
        Toast.makeText(this, R.string.token_deleted, Toast.LENGTH_SHORT).show()
        updateView()
    }

    private fun updateView() {
        val hasToken = TokenStore.load(this) != null
        tokenInputGroup.visibility = if (hasToken) View.GONE else View.VISIBLE
        tokenSavedGroup.visibility = if (hasToken) View.VISIBLE else View.GONE
    }
}
