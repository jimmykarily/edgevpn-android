package edgevpn

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// useLogFile points the log at a file in a fresh directory and stops writing
// to it when the test ends.
func useLogFile(t *testing.T) string {
	t.Helper()
	resetLogs(t)
	path := filepath.Join(t.TempDir(), "edgevpn.log")
	if err := SetLogFile(path); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = SetLogFile("") })
	return path
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestLogLinesReachTheLogFileWithTheirDate(t *testing.T) {
	path := useLogFile(t)
	emit("INFO ", "connected to brain")

	got := readFile(t, path)
	// The date, milliseconds and time zone let these lines be matched
	// against the logs of the other nodes.
	want := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}(Z|[+-]\d{2}:\d{2}) INFO  connected to brain\n$`)
	if !want.MatchString(got) {
		t.Fatalf("log file = %q", got)
	}
}

func TestTheLogFileHasDebugLinesOnlyWithDetailedLogs(t *testing.T) {
	path := useLogFile(t)
	emit("DEBUG", "hidden")
	SetDetailedLogs(true)
	emit("DEBUG", "shown")

	got := readFile(t, path)
	if strings.Contains(got, "hidden") || !strings.Contains(got, "shown") {
		t.Fatalf("log file = %q", got)
	}
}

func TestTheLogFileGetsTheRedactedLine(t *testing.T) {
	path := useLogFile(t)
	(&bufferLogger{}).Infof("mdns(android): starting, service=%s", "secretservice123")

	got := readFile(t, path)
	if strings.Contains(got, "secretservice123") || !strings.Contains(got, "service=<hidden>") {
		t.Fatalf("log file = %q", got)
	}
}

func TestOnlyTheAppCanReadTheLogFile(t *testing.T) {
	path := useLogFile(t)
	emit("INFO ", "something")

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("log file mode = %o, want 600", mode)
	}
}

// The app process restarts often (Android stops it, the user updates the
// app), and the lines from before a restart are the ones that explain it.
func TestReopeningTheLogFileKeepsWhatItHad(t *testing.T) {
	path := useLogFile(t)
	emit("INFO ", "before the restart")
	if err := SetLogFile(""); err != nil {
		t.Fatal(err)
	}
	emit("INFO ", "while no file is set")
	if err := SetLogFile(path); err != nil {
		t.Fatal(err)
	}
	emit("INFO ", "after the restart")

	got := readFile(t, path)
	if !strings.Contains(got, "before the restart") || !strings.Contains(got, "after the restart") || strings.Contains(got, "while no file is set") {
		t.Fatalf("log file = %q", got)
	}
}

func TestSettingTheSameLogFileAgainKeepsWriting(t *testing.T) {
	path := useLogFile(t)
	if err := SetLogFile(path); err != nil {
		t.Fatal(err)
	}
	emit("INFO ", "one line")

	if got := readFile(t, path); strings.Count(got, "one line") != 1 {
		t.Fatalf("log file = %q", got)
	}
}

func TestTheLogFileMovesToABackupWhenFull(t *testing.T) {
	old := logFileMaxBytes
	logFileMaxBytes = 300
	t.Cleanup(func() { logFileMaxBytes = old })
	path := useLogFile(t)

	for i := 0; i < 20; i++ {
		emit("INFO ", strings.Repeat("x", 40))
	}

	for _, p := range []string{path, path + ".1"} {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if info.Size() > logFileMaxBytes {
			t.Fatalf("%s has %d bytes, over the %d limit", p, info.Size(), logFileMaxBytes)
		}
	}
}

func TestExportLogsWritesTheBackupThenTheCurrentFile(t *testing.T) {
	old := logFileMaxBytes
	logFileMaxBytes = 100
	t.Cleanup(func() { logFileMaxBytes = old })
	path := useLogFile(t)
	emit("INFO ", "first line, which ends up in the backup")
	emit("INFO ", "second line")

	out := filepath.Join(t.TempDir(), "shared.log")
	if err := ExportLogs(out); err != nil {
		t.Fatal(err)
	}
	got := readFile(t, out)
	first, second := strings.Index(got, "first line"), strings.Index(got, "second line")
	if first < 0 || second < first {
		t.Fatalf("exported logs = %q", got)
	}
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("the test did not fill the log file: %v", err)
	}
}

func TestExportLogsWithoutAFileFails(t *testing.T) {
	resetLogs(t)
	if err := ExportLogs(filepath.Join(t.TempDir(), "shared.log")); err == nil {
		t.Fatal("ExportLogs succeeded without a log file")
	}
}
