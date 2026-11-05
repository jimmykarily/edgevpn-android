package edgevpn

import (
	"testing"
	"time"
)

func TestComputeState(t *testing.T) {
	cases := []struct {
		running   bool
		failure   string
		reachable int
		want      string
	}{
		{false, "", 0, "off"},
		{false, "bad key", 0, "problem"},
		{true, "", 0, "looking"},
		{true, "", 2, "connected"},
		{true, "boom", 2, "problem"},
	}
	for _, c := range cases {
		if got := computeState(c.running, c.failure, c.reachable); got != c.want {
			t.Errorf("computeState(%v,%q,%d) = %q, want %q", c.running, c.failure, c.reachable, got, c.want)
		}
	}
}

func TestReportStartFailureThenStop(t *testing.T) {
	t.Cleanup(Stop)
	ReportStartFailure("VPN interface could not be created")
	if Status() != "problem" {
		t.Fatalf("Status() = %q, want problem", Status())
	}
	Stop()
	if Status() != "off" {
		t.Fatalf("Status() = %q after Stop, want off", Status())
	}
}

func TestAliveIntervalsMatchCLIDefaults(t *testing.T) {
	// edgevpn cmd/util.go: aliveness-healthcheck-interval 120, -scrub-interval 600, -max-interval 900
	if aliveAnnounce != 120*time.Second || aliveScrub != 600*time.Second || aliveMax != 900*time.Second {
		t.Fatal("check-in intervals differ from the edgevpn CLI defaults")
	}
}
