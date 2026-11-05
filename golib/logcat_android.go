//go:build android && cgo

package edgevpn

/*
#cgo LDFLAGS: -llog
#include <stdlib.h>
#include <android/log.h>
*/
import "C"

import "unsafe"

var logcatTag = C.CString("EdgeVPN")

// writeLogcat writes a log line to logcat under the tag EdgeVPN.
func writeLogcat(level, line string) {
	prio := C.int(C.ANDROID_LOG_INFO)
	switch level {
	case "DEBUG":
		prio = C.ANDROID_LOG_DEBUG
	case "WARN ":
		prio = C.ANDROID_LOG_WARN
	case "ERROR":
		prio = C.ANDROID_LOG_ERROR
	case "FATAL", "PANIC":
		prio = C.ANDROID_LOG_FATAL
	}
	text := C.CString(line)
	defer C.free(unsafe.Pointer(text))
	C.__android_log_write(prio, logcatTag, text)
}
