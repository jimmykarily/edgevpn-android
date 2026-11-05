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

func TestLogDialErrorIsOnlyKeptWithDetailedLogs(t *testing.T) {
	resetLogs(t)
	LogDialError("tcp4 10.0.0.1:4001", fmt.Errorf("connection refused"))
	if got := RecentLogs(); got != "" {
		t.Fatalf("detailed logs off: logs = %q, want empty", got)
	}

	SetDetailedLogs(true)
	LogDialError("tcp4 10.0.0.1:4001", fmt.Errorf("connection refused"))
	if got := RecentLogs(); !strings.Contains(got, "dial error tcp4 10.0.0.1:4001: connection refused") {
		t.Fatalf("detailed logs on: logs = %q, want the dial error", got)
	}
}

// edgevpn logs the mDNS service name when it starts mDNS, and that name is
// a secret part of the network key.
func TestTheMDNSServiceNameIsHiddenInTheLogs(t *testing.T) {
	resetLogs(t)
	l := &bufferLogger{}
	l.Infof("mdns(android): starting, service=%s", "secretservice123")
	l.Info("some other line, service=keep")

	got := RecentLogs()
	if strings.Contains(got, "secretservice123") {
		t.Errorf("logs hold the mDNS service name:\n%s", got)
	}
	if !strings.Contains(got, "mdns(android): starting, service=<hidden>") {
		t.Errorf("logs lost the mDNS start line:\n%s", got)
	}
	if !strings.Contains(got, "some other line, service=keep") {
		t.Errorf("a line that is not about mDNS changed:\n%s", got)
	}
}

func TestRedactLine(t *testing.T) {
	for in, want := range map[string]string{
		"mdns(android): starting, service=abc":     "mdns(android): starting, service=<hidden>",
		"mdns(android): starting, service=abc def": "mdns(android): starting, service=<hidden> def",
		"mdns(android): starting, service=":        "mdns(android): starting, service=<hidden>",
		"mdns: peer found":                         "mdns: peer found",
		"unrelated service=abc":                    "unrelated service=abc",
	} {
		if got := redactLine(in); got != want {
			t.Errorf("redactLine(%q) = %q, want %q", in, got, want)
		}
	}
}
