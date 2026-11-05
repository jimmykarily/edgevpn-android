package edgevpn

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mudler/edgevpn/pkg/node"
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

type recordingListener struct{ reasons []string }

func (r *recordingListener) OnStopped(reason string) { r.reasons = append(r.reasons, reason) }

func newTunPipe(t *testing.T) (*os.File, *os.File) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close(); w.Close() })
	return r, w
}

func TestHandleNodeExitOfTheCurrentSession(t *testing.T) {
	l := &recordingListener{}
	SetStatusListener(l)
	t.Cleanup(func() { SetStatusListener(nil); Stop(); ClearLogs() })

	n := newTestNode(t)
	tun, _ := newTunPipe(t)
	cancelled := 0
	mu.Lock()
	currentNode, tunFile = n, tun
	cancelFn = func() { cancelled++ }
	mu.Unlock()

	handleNodeExit(n, func() { cancelled++ }, errors.New("boom"))

	if got := Status(); got != "problem" {
		t.Errorf("Status() = %q, want problem", got)
	}
	if cancelled == 0 {
		t.Error("the node context was not cancelled")
	}
	if _, err := tun.Stat(); err == nil {
		t.Error("the tunnel file is still open")
	}
	if len(l.reasons) != 1 || l.reasons[0] != "boom" {
		t.Errorf("listener calls = %v, want one call with boom", l.reasons)
	}
	if got := rememberedFailure(); got != reasonStopped {
		t.Errorf("remembered failure = %q, want %q", got, reasonStopped)
	}
	if !strings.Contains(RecentLogs(), "ERROR node.Start returned error: boom") {
		t.Errorf("logs do not hold the raw error:\n%s", RecentLogs())
	}
}

func rememberedFailure() string {
	mu.Lock()
	defer mu.Unlock()
	return lastFailure
}

func TestHandleNodeExitOfAStaleSessionLeavesTheNewOneAlone(t *testing.T) {
	l := &recordingListener{}
	SetStatusListener(l)
	t.Cleanup(func() { SetStatusListener(nil); Stop() })

	newer := newTestNode(t)
	tun, _ := newTunPipe(t)
	mu.Lock()
	currentNode, tunFile = newer, tun
	cancelFn = func() {}
	mu.Unlock()

	handleNodeExit(newTestNode(t), func() {}, errors.New("boom"))

	if got := Status(); got != "looking" {
		t.Errorf("Status() = %q, want looking (newer session untouched)", got)
	}
	if _, err := tun.Stat(); err != nil {
		t.Errorf("the newer session's tunnel file was closed: %v", err)
	}
	if len(l.reasons) != 0 {
		t.Errorf("listener calls = %v, want none", l.reasons)
	}
}

// newTestNode returns a node that is never started.
func newTestNode(t *testing.T) *node.Node {
	t.Helper()
	n, err := node.New()
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestMarkHostReadyOnlyForTheCurrentSession(t *testing.T) {
	t.Cleanup(Stop)
	current := newTestNode(t)
	mu.Lock()
	currentNode = current
	mu.Unlock()

	if markHostReady(newTestNode(t)) {
		t.Error("a stale session was marked ready")
	}
	mu.Lock()
	ready := hostReady
	mu.Unlock()
	if ready {
		t.Error("hostReady set by a stale session")
	}

	if !markHostReady(current) {
		t.Fatal("the current session was not marked ready")
	}
	mu.Lock()
	ready = hostReady
	mu.Unlock()
	if !ready {
		t.Error("hostReady not set")
	}

	Stop()
	mu.Lock()
	ready = hostReady
	mu.Unlock()
	if ready {
		t.Error("Stop left hostReady set")
	}
}

func TestHandleNodeExitClearsHostReady(t *testing.T) {
	t.Cleanup(Stop)
	n := newTestNode(t)
	mu.Lock()
	currentNode, cancelFn, hostReady = n, func() {}, true
	mu.Unlock()
	handleNodeExit(n, func() {}, errors.New("boom"))
	mu.Lock()
	defer mu.Unlock()
	if hostReady {
		t.Error("hostReady survives the node exit")
	}
}
