package com.kairos.edgevpn

import android.content.Context
import android.util.Log
import edgevpn.Edgevpn
import java.io.File

/**
 * The file the Go library writes its log lines to, so the lines outlive the
 * in-memory buffer and app restarts. It lives in the no-backup folder, which
 * Android leaves out of backups and of the copy to a new phone.
 */
object LogFile {
    private const val TAG = "EdgeVPN"
    private const val NAME = "edgevpn.log"

    /** The folder in the cache that the share provider serves, see res/xml/log_share_paths.xml. */
    private const val SHARE_DIR = "shared"
    private const val SHARE_NAME = "edgevpn-log.txt"

    /** Starts writing log lines to the file. Calling it again changes nothing. */
    fun attach(c: Context) {
        try {
            Edgevpn.setLogFile(File(c.noBackupFilesDir, NAME).path)
        } catch (e: Exception) {
            Log.w(TAG, "Cannot write the log file", e)
        }
    }

    /** Copies the log, oldest line first, into one file the share provider can hand out. */
    fun export(c: Context): File {
        attach(c)
        val dir = File(c.cacheDir, SHARE_DIR).apply { mkdirs() }
        val out = File(dir, SHARE_NAME)
        Edgevpn.exportLogs(out.path)
        return out
    }
}
