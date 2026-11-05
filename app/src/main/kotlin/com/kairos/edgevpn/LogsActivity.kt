package com.kairos.edgevpn

import android.content.ClipData
import android.content.ClipDescription
import android.content.ClipboardManager
import android.os.Build
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.os.PersistableBundle
import android.view.Menu
import android.view.MenuItem
import android.widget.ScrollView
import android.widget.TextView
import android.widget.Toast
import androidx.appcompat.app.AppCompatActivity
import com.google.android.material.appbar.MaterialToolbar
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
        note.visibility = if (Edgevpn.detailedLogs()) android.view.View.VISIBLE else android.view.View.GONE
        val logs = Edgevpn.recentLogs()
        if (logs == shown) return
        val first = shown == null
        val stick = first || shouldStickToBottom(
            scroll.scrollY, scroll.height, text.height + scroll.paddingTop + scroll.paddingBottom,
            (SLACK_DP * resources.displayMetrics.density).toInt()
        )
        shown = logs
        text.text = logs.ifEmpty { getString(R.string.logs_empty) }
        if (stick) scroll.post { scroll.fullScroll(ScrollView.FOCUS_DOWN) }
    }

    private fun onMenuItem(item: MenuItem): Boolean {
        if (item.itemId != R.id.menu_copy_all) return false
        copyAll()
        return true
    }

    private fun copyAll() {
        val clip = ClipData.newPlainText(getString(R.string.logs_title), Edgevpn.recentLogs())
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            clip.description.extras = PersistableBundle().apply {
                putBoolean(ClipDescription.EXTRA_IS_SENSITIVE, true)
            }
        }
        getSystemService(ClipboardManager::class.java).setPrimaryClip(clip)
        Toast.makeText(this, R.string.logs_copied, Toast.LENGTH_SHORT).show()
    }

    private companion object {
        const val POLL_MS = 1000L
        const val SLACK_DP = 8
    }
}
