package edgevpn

import (
	"fmt"
	"sync"
	"time"
)

// LogDialError logs a dial failure to the app log (and thus logcat) so we can
// see the underlying error (e.g. connection refused, permission denied).
// Uses WARN level so it is not dropped when min log level is "warn".
func LogDialError(addr string, err error) {
	if err != nil {
		emit("WARN ", fmt.Sprintf("dial error %s: %v", addr, err))
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
	minVal := logLevelOrder[min]
	if minVal == 0 {
		minVal = logLevelOrder["WARN "]
	}
	lv := logLevelOrder[level]
	return lv >= minVal
}

// LogReceiver is implemented on the Android side to receive log lines in
// real time. gomobile generates a Java interface that Kotlin can implement.
type LogReceiver interface {
	OnLog(line string)
}

var (
	receiverMu sync.Mutex
	receiver   LogReceiver
)

// SetLogReceiver registers (or unregisters with nil) a callback that receives
// every log line as it is emitted.
func SetLogReceiver(r LogReceiver) {
	receiverMu.Lock()
	defer receiverMu.Unlock()
	receiver = r
}

func emit(level, msg string) {
	if !shouldEmit(level) {
		return
	}
	ts := time.Now().Format("15:04:05")
	line := fmt.Sprintf("[%s] %s %s", ts, level, msg)

	receiverMu.Lock()
	r := receiver
	receiverMu.Unlock()

	if r != nil {
		r.OnLog(line)
	}
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
// pushing each log line to the registered LogReceiver callback.
type bufferLogger struct{}

func (l *bufferLogger) Debug(args ...interface{})              { emit("DEBUG", joinArgs(args...)) }
func (l *bufferLogger) Debugf(f string, args ...interface{})   { emit("DEBUG", fmt.Sprintf(f, args...)) }
func (l *bufferLogger) Info(args ...interface{})               { emit("INFO ", joinArgs(args...)) }
func (l *bufferLogger) Infof(f string, args ...interface{})    { emit("INFO ", fmt.Sprintf(f, args...)) }
func (l *bufferLogger) Warn(args ...interface{})               { emit("WARN ", joinArgs(args...)) }
func (l *bufferLogger) Warnf(f string, args ...interface{})    { emit("WARN ", fmt.Sprintf(f, args...)) }
func (l *bufferLogger) Warning(args ...interface{})            { emit("WARN ", joinArgs(args...)) }
func (l *bufferLogger) Warningf(f string, args ...interface{}) { emit("WARN ", fmt.Sprintf(f, args...)) }
func (l *bufferLogger) Error(args ...interface{})              { emit("ERROR", joinArgs(args...)) }
func (l *bufferLogger) Errorf(f string, args ...interface{})   { emit("ERROR", fmt.Sprintf(f, args...)) }
func (l *bufferLogger) Fatal(args ...interface{})              { emit("FATAL", joinArgs(args...)) }
func (l *bufferLogger) Fatalf(f string, args ...interface{})   { emit("FATAL", fmt.Sprintf(f, args...)) }
func (l *bufferLogger) Panic(args ...interface{})              { emit("PANIC", joinArgs(args...)) }
func (l *bufferLogger) Panicf(f string, args ...interface{})   { emit("PANIC", fmt.Sprintf(f, args...)) }
