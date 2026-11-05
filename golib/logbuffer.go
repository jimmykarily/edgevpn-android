package edgevpn

import (
	"fmt"
	"strings"
	"sync"
	"time"

	logging "github.com/ipfs/go-log/v2"
)

// LogDialError logs a dial failure to the app log (and thus logcat) so we can
// see the underlying error (e.g. connection refused, permission denied).
// Each dial is logged at DEBUG: a node dials hundreds of peers while it
// bootstraps, and these lines would push the startup lines out of the buffer.
func LogDialError(addr string, err error) {
	if err != nil {
		emit("DEBUG", fmt.Sprintf("dial error %s: %v", addr, err))
	}
}

// logLevelOrder defines ordering for filtering (higher = more severe).
var logLevelOrder = map[string]int{
	"DEBUG": 0, "INFO ": 1, "WARN ": 2, "ERROR": 3, "FATAL": 4, "PANIC": 4,
}

var (
	minLogLevel   = "WARN " // default: suppress DEBUG/INFO discovery loop spam
	minLogLevelMu sync.RWMutex
)

// normalizeLevel maps config strings (e.g. "warn") to the keys used in emit.
func normalizeLevel(s string) string {
	switch s {
	case "debug":
		return "DEBUG"
	case "info":
		return "INFO "
	case "warn", "warning":
		return "WARN "
	case "error":
		return "ERROR"
	case "fatal", "panic":
		return "FATAL"
	default:
		return "WARN "
	}
}

// SetMinLogLevel sets the minimum level to emit (debug, info, warn, error, fatal).
// Used by Start() so the config LogLevel is respected.
func SetMinLogLevel(level string) {
	minLogLevelMu.Lock()
	defer minLogLevelMu.Unlock()
	minLogLevel = normalizeLevel(level)
}

func shouldEmit(level string) bool {
	minLogLevelMu.RLock()
	min := minLogLevel
	minLogLevelMu.RUnlock()
	minVal, ok := logLevelOrder[min]
	if !ok {
		minVal = logLevelOrder["WARN "]
	}
	lv := logLevelOrder[level]
	return lv >= minVal
}

// logBufferLines is the number of most recent log lines kept in memory.
const logBufferLines = 2000

var (
	ringMu    sync.Mutex
	ring      [logBufferLines]string
	ringStart int // index of the oldest line
	ringLen   int
)

func ringAppend(line string) {
	ringMu.Lock()
	defer ringMu.Unlock()
	if ringLen < logBufferLines {
		ring[(ringStart+ringLen)%logBufferLines] = line
		ringLen++
		return
	}
	ring[ringStart] = line
	ringStart = (ringStart + 1) % logBufferLines
}

// RecentLogs returns the buffered log lines, oldest first, joined with newlines.
func RecentLogs() string {
	ringMu.Lock()
	defer ringMu.Unlock()
	lines := make([]string, ringLen)
	for i := range lines {
		lines[i] = ring[(ringStart+i)%logBufferLines]
	}
	return strings.Join(lines, "\n")
}

// ClearLogs empties the log buffer.
func ClearLogs() {
	ringMu.Lock()
	defer ringMu.Unlock()
	ring = [logBufferLines]string{}
	ringStart, ringLen = 0, 0
}

var (
	detailedLogs   bool
	detailedLogsMu sync.Mutex
)

// SetDetailedLogs turns debug output on or off for the app and for the
// libraries it uses (libp2p logs through go-log).
func SetDetailedLogs(on bool) {
	detailedLogsMu.Lock()
	detailedLogs = on
	detailedLogsMu.Unlock()
	if on {
		SetMinLogLevel("debug")
		logging.SetAllLoggers(logging.LevelDebug)
	} else {
		SetMinLogLevel("info")
		logging.SetAllLoggers(logging.LevelError)
	}
}

// DetailedLogs reports whether detailed logs are on.
func DetailedLogs() bool {
	detailedLogsMu.Lock()
	defer detailedLogsMu.Unlock()
	return detailedLogs
}

func emit(level, msg string) {
	if !shouldEmit(level) {
		return
	}
	ts := time.Now().Format("15:04:05")
	line := fmt.Sprintf("[%s] %s %s", ts, level, msg)
	ringAppend(line)
	writeLogcat(level, line)
}

func joinArgs(args ...interface{}) string {
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = fmt.Sprintf("%v", a)
	}
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += " "
		}
		result += p
	}
	return result
}

// bufferLogger implements log.StandardLogger from github.com/ipfs/go-log,
// sending each log line through emit.
type bufferLogger struct{}

func (l *bufferLogger) Debug(args ...interface{})            { emit("DEBUG", joinArgs(args...)) }
func (l *bufferLogger) Debugf(f string, args ...interface{}) { emit("DEBUG", fmt.Sprintf(f, args...)) }
func (l *bufferLogger) Info(args ...interface{})             { emit("INFO ", joinArgs(args...)) }
func (l *bufferLogger) Infof(f string, args ...interface{})  { emit("INFO ", fmt.Sprintf(f, args...)) }
func (l *bufferLogger) Warn(args ...interface{})             { emit("WARN ", joinArgs(args...)) }
func (l *bufferLogger) Warnf(f string, args ...interface{})  { emit("WARN ", fmt.Sprintf(f, args...)) }
func (l *bufferLogger) Warning(args ...interface{})          { emit("WARN ", joinArgs(args...)) }
func (l *bufferLogger) Warningf(f string, args ...interface{}) {
	emit("WARN ", fmt.Sprintf(f, args...))
}
func (l *bufferLogger) Error(args ...interface{})            { emit("ERROR", joinArgs(args...)) }
func (l *bufferLogger) Errorf(f string, args ...interface{}) { emit("ERROR", fmt.Sprintf(f, args...)) }
func (l *bufferLogger) Fatal(args ...interface{})            { emit("FATAL", joinArgs(args...)) }
func (l *bufferLogger) Fatalf(f string, args ...interface{}) { emit("FATAL", fmt.Sprintf(f, args...)) }
func (l *bufferLogger) Panic(args ...interface{})            { emit("PANIC", joinArgs(args...)) }
func (l *bufferLogger) Panicf(f string, args ...interface{}) { emit("PANIC", fmt.Sprintf(f, args...)) }
