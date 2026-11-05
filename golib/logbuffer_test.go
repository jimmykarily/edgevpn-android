package edgevpn

import (
	"fmt"
	"strings"
	"testing"
)

func resetLogs(t *testing.T) {
	t.Helper()
	ClearLogs()
	SetDetailedLogs(false)
	t.Cleanup(func() {
		ClearLogs()
		SetDetailedLogs(false)
	})
}

func TestRingBufferKeepsTheNewest2000Lines(t *testing.T) {
	resetLogs(t)
	for i := 0; i < 2100; i++ {
		emit("INFO ", fmt.Sprintf("line %d", i))
	}
	lines := strings.Split(RecentLogs(), "\n")
	if len(lines) != logBufferLines {
		t.Fatalf("got %d lines, want %d", len(lines), logBufferLines)
	}
	if !strings.HasSuffix(lines[0], "line 100") || !strings.HasSuffix(lines[1999], "line 2099") {
		t.Fatalf("first/last = %q / %q", lines[0], lines[1999])
	}
}

func TestClearLogsEmptiesTheBuffer(t *testing.T) {
	resetLogs(t)
	emit("INFO ", "something")
	ClearLogs()
	if got := RecentLogs(); got != "" {
		t.Fatalf("logs = %q, want empty", got)
	}
}

func TestDetailedLogsControlsDebugLines(t *testing.T) {
	resetLogs(t)
	emit("DEBUG", "hidden")
	SetDetailedLogs(true)
	if !DetailedLogs() {
		t.Fatal("DetailedLogs() = false after SetDetailedLogs(true)")
	}
	emit("DEBUG", "shown")
	SetDetailedLogs(false)
	got := RecentLogs()
	if strings.Contains(got, "hidden") || !strings.Contains(got, "shown") {
		t.Fatalf("logs = %q", got)
	}
}

func TestNetworkConfigLogLevelsFollowDetailedLogs(t *testing.T) {
	resetLogs(t)
	if c := networkConfig("t", "10.1.0.2"); c.LogLevel != "info" || c.Libp2pLogLevel != "error" {
		t.Fatal(c.LogLevel, c.Libp2pLogLevel)
	}
	SetDetailedLogs(true)
	if c := networkConfig("t", "10.1.0.2"); c.LogLevel != "debug" || c.Libp2pLogLevel != "debug" {
		t.Fatal(c.LogLevel, c.Libp2pLogLevel)
	}
}
