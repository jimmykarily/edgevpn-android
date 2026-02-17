package edgevpn

import (
	"fmt"
	"sync"
	"time"
)

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
