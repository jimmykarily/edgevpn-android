package edgevpn

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// logFileMaxBytes is the size at which the log file moves to its backup
// (path + ".1") and a new file starts, so the two together hold at most twice
// this much. At debug level 5 MB holds a few hours of a busy node.
var logFileMaxBytes int64 = 5 << 20

// logFileTimeFormat dates every line to the millisecond with the time zone,
// so the phone's lines can be matched against the logs of the other nodes.
const logFileTimeFormat = "2006-01-02T15:04:05.000Z07:00"

var (
	logFileMu   sync.Mutex
	logFilePath string
	logFile     *os.File
	logFileSize int64
)

// SetLogFile makes every log line that is kept also go to the file at path,
// appending to what it already holds. An empty path stops writing to a file.
// Setting the path that is already in use changes nothing.
func SetLogFile(path string) error {
	logFileMu.Lock()
	defer logFileMu.Unlock()
	if path == logFilePath && (path == "" || logFile != nil) {
		return nil
	}
	closeLogFileLocked()
	if path == "" {
		return nil
	}
	if err := openLogFileLocked(path); err != nil {
		return err
	}
	logFilePath = path
	return nil
}

func openLogFileLocked(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	logFile, logFileSize = f, info.Size()
	return nil
}

func closeLogFileLocked() {
	if logFile != nil {
		logFile.Close()
	}
	logFile, logFilePath, logFileSize = nil, "", 0
}

// writeLogFile appends one line to the log file, if one is set. When the line
// would take the file over its limit, the file first moves to its backup. A
// file that cannot be written is dropped and the line goes only to the other
// log outputs.
func writeLogFile(now time.Time, level, msg string) {
	logFileMu.Lock()
	defer logFileMu.Unlock()
	if logFile == nil {
		return
	}
	line := fmt.Sprintf("%s %s %s\n", now.Format(logFileTimeFormat), level, msg)
	if logFileSize > 0 && logFileSize+int64(len(line)) > logFileMaxBytes {
		if err := rotateLogFileLocked(); err != nil {
			dropLogFileLocked(err)
			return
		}
	}
	n, err := logFile.WriteString(line)
	logFileSize += int64(n)
	if err != nil {
		dropLogFileLocked(err)
	}
}

func rotateLogFileLocked() error {
	path := logFilePath
	logFile.Close()
	logFile = nil
	if err := os.Rename(path, path+".1"); err != nil {
		return err
	}
	return openLogFileLocked(path)
}

func dropLogFileLocked(err error) {
	path := logFilePath
	closeLogFileLocked()
	writeLogcat("WARN ", fmt.Sprintf("stopped writing the log file %s: %v", path, err))
}

// ExportLogs writes the log backup and then the current log file, oldest line
// first, into one file at dst, for the user to share.
func ExportLogs(dst string) error {
	logFileMu.Lock()
	defer logFileMu.Unlock()
	if logFile == nil {
		return errors.New("no log file is set")
	}
	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	for _, p := range []string{logFilePath + ".1", logFilePath} {
		if err := appendFile(out, p); err != nil && !errors.Is(err, os.ErrNotExist) {
			out.Close()
			os.Remove(tmp)
			return err
		}
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}

func appendFile(out io.Writer, path string) error {
	in, err := os.Open(path)
	if err != nil {
		return err
	}
	defer in.Close()
	_, err = io.Copy(out, in)
	return err
}
