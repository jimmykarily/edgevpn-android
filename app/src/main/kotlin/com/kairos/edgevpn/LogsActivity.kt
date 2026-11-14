package com.kairos.edgevpn

import android.content.ClipData
import android.content.ClipDescription
import android.content.ClipboardManager
import android.content.Intent
import android.os.Build
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.os.PersistableBundle
import android.util.Log
import android.view.Menu
import android.view.MenuItem
import android.view.View
import android.widget.ScrollView
import android.widget.TextView
import android.widget.Toast
import androidx.appcompat.app.AppCompatActivity
import androidx.core.content.FileProvider
import com.google.android.material.appbar.MaterialToolbar
import com.google.android.material.dialog.MaterialAlertDialogBuilder
import edgevpn.Edgevpn

/** Shows the Go library's recent log lines, refreshed every second while visible. */
class LogsActivity : AppCompatActivity() {

    private lateinit var scroll: ScrollView
    private lateinit var text: TextView
    private lateinit var note: TextView

    private val handler = Handler(Looper.getMainLooper())
    private var shown: String? = null

    private val poll = object : Runnable {
        override fun run() {
            refresh()
            handler.postDelayed(this, POLL_MS)
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_logs)
        val toolbar = findViewById<MaterialToolbar>(R.id.toolbar)
        toolbar.setNavigationOnClickListener { finish() }
        toolbar.inflateMenu(R.menu.logs)
        toolbar.setOnMenuItemClickListener { item -> onMenuItem(item) }
        scroll = findViewById(R.id.logsScroll)
        text = findViewById(R.id.logsText)
        note = findViewById(R.id.logsNote)
    }

    override fun onStart() {
        super.onStart()
        poll.run()
    }

    override fun onStop() {
        handler.removeCallbacks(poll)
        super.onStop()
    }

    private fun refresh() {
        note.visibility = if (Edgevpn.detailedLogs()) View.VISIBLE else View.GONE
        val logs = Edgevpn.recentLogs()
        if (!shouldRefreshLogs(shown, logs, text.hasSelection())) return
        val first = shown == null
        val stick = first || shouldStickToBottom(
            scroll.scrollY, scroll.height, text.height + scroll.paddingTop + scroll.paddingBottom,
            (SLACK_DP * resources.displayMetrics.density).toInt()
        )
        shown = logs
        text.text = logs.ifEmpty { getString(R.string.logs_empty) }
        if (stick) scroll.post { scroll.fullScroll(ScrollView.FOCUS_DOWN) }
    }

    private fun onMenuItem(item: MenuItem): Boolean = when (item.itemId) {
        R.id.menu_copy_all -> { copyAll(); true }
        R.id.menu_share_file -> { shareFile(); true }
        R.id.menu_clear_file -> { askToClearFile(); true }
        else -> false
    }

    /** Sends the log file, which holds far more than the screen, to an app the user picks. */
    private fun shareFile() {
        val uri = try {
            FileProvider.getUriForFile(this, "$packageName.logs", LogFile.export(this))
        } catch (e: Exception) {
            Log.w(TAG, "Cannot export the log file", e)
            Toast.makeText(this, R.string.logs_share_failed, Toast.LENGTH_SHORT).show()
            return
        }
        val send = Intent(Intent.ACTION_SEND).apply {
            type = "text/plain"
            putExtra(Intent.EXTRA_STREAM, uri)
            addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
        }
        startActivity(Intent.createChooser(send, getString(R.string.logs_share_file)))
    }

    private fun askToClearFile() {
        MaterialAlertDialogBuilder(this)
            .setTitle(R.string.logs_clear_file)
            .setMessage(R.string.logs_clear_file_question)
            .setNegativeButton(android.R.string.cancel, null)
            .setPositiveButton(R.string.logs_clear) { _, _ -> clearFile() }
            .show()
    }

    private fun clearFile() {
        try {
            LogFile.clear(this)
        } catch (e: Exception) {
            Log.w(TAG, "Cannot clear the log file", e)
            Toast.makeText(this, R.string.logs_clear_failed, Toast.LENGTH_SHORT).show()
            return
        }
        Toast.makeText(this, R.string.logs_cleared, Toast.LENGTH_SHORT).show()
        refresh()
    }

    private fun copyAll() {
        val logs = Edgevpn.recentLogs()
        if (logs.isEmpty()) {
            Toast.makeText(this, R.string.logs_nothing_to_copy, Toast.LENGTH_SHORT).show()
            return
        }
        val clip = ClipData.newPlainText(getString(R.string.logs_title), logs)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            clip.description.extras = PersistableBundle().apply {
                putBoolean(ClipDescription.EXTRA_IS_SENSITIVE, true)
            }
        }
        getSystemService(ClipboardManager::class.java).setPrimaryClip(clip)
        Toast.makeText(this, R.string.logs_copied, Toast.LENGTH_SHORT).show()
    }

    private companion object {
        const val TAG = "EdgeVPN"
        const val POLL_MS = 1000L
        const val SLACK_DP = 8
    }
}
