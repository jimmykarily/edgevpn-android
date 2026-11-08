package edgevpn

import (
	"context"
	"encoding/base64"
	"errors"
	"net"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/mudler/edgevpn/pkg/node"
)

// fakeMDNS replaces runMDNS during a test and records every start.
type fakeMDNS struct {
	mu      sync.Mutex
	ctxs    []context.Context
	tags    []string
	filters []func(net.Interface) bool
	err     error
}

func installFakeMDNS(t *testing.T) *fakeMDNS {
	t.Helper()
	f := &fakeMDNS{}
	orig := runMDNS
	runMDNS = func(ctx context.Context, _ host.Host, tag string, filter func(net.Interface) bool) error {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.ctxs = append(f.ctxs, ctx)
		f.tags = append(f.tags, tag)
		f.filters = append(f.filters, filter)
		return f.err
	}
	t.Cleanup(func() { runMDNS = orig })
	return f
}

func (f *fakeMDNS) starts() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.ctxs)
}

func (f *fakeMDNS) ctx(i int) context.Context {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ctxs[i]
}

func (f *fakeMDNS) filter(i int) func(net.Interface) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.filters[i]
}

func (f *fakeMDNS) running() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	alive := 0
	for _, c := range f.ctxs {
		if c.Err() == nil {
			alive++
		}
	}
	return alive
}

func TestMDNSTagReadsTheTokenField(t *testing.T) {
	token := base64.StdEncoding.EncodeToString([]byte("room: r\nrendezvous: v\nmdns: secret-tag\n"))
	if got := mdnsTag(token); got != "secret-tag" {
		t.Errorf("mdnsTag = %q, want secret-tag", got)
	}
	if got := mdnsTag("not base64 !!"); got != "" {
		t.Errorf("mdnsTag of garbage = %q, want empty", got)
	}
	garbageYAML := base64.StdEncoding.EncodeToString([]byte(":\t: [unclosed"))
	if got := mdnsTag(garbageYAML); got != "" {
		t.Errorf("mdnsTag of bad YAML = %q, want empty", got)
	}
}

func TestMDNSRunnerWaitsForALocalNetwork(t *testing.T) {
	f := installFakeMDNS(t)
	r := newMDNSRunner("tag")
	t.Cleanup(r.stop)

	r.hostReady(context.Background(), nil)
	if f.starts() != 0 {
		t.Fatalf("started %d times before a local network was reported", f.starts())
	}
	r.setLocal(true)
	if f.starts() != 1 {
		t.Fatalf("started %d times, want 1", f.starts())
	}
	r.setLocal(true)
	if f.starts() != 1 {
		t.Errorf("a repeated local report started it again: %d starts", f.starts())
	}
	if f.tags[0] != "tag" {
		t.Errorf("tag = %q, want the runner's tag", f.tags[0])
	}
}

func TestMDNSRunnerStartsWhenLocalWasReportedFirst(t *testing.T) {
	f := installFakeMDNS(t)
	r := newMDNSRunner("tag")
	t.Cleanup(r.stop)

	r.setLocal(true)
	if f.starts() != 0 {
		t.Fatalf("started %d times before the host was ready", f.starts())
	}
	r.hostReady(context.Background(), nil)
	if f.starts() != 1 {
		t.Errorf("started %d times, want 1", f.starts())
	}
}

func TestMDNSRunnerRestartReplacesTheInstance(t *testing.T) {
	f := installFakeMDNS(t)
	r := newMDNSRunner("tag")
	t.Cleanup(r.stop)
	r.setLocal(true)
	r.hostReady(context.Background(), nil)

	r.restart()
	if f.starts() != 2 {
		t.Fatalf("started %d times after a restart, want 2", f.starts())
	}
	if f.ctx(0).Err() == nil {
		t.Error("the first instance still runs after the restart")
	}
	if f.ctx(1).Err() != nil {
		t.Error("the new instance is already stopped")
	}

	for i := 0; i < 20; i++ {
		r.restart()
	}
	if got := f.running(); got != 1 {
		t.Errorf("%d instances run after 20 restarts, want 1", got)
	}
}

func TestMDNSRunnerRestartsFromManyGoroutines(t *testing.T) {
	f := installFakeMDNS(t)
	r := newMDNSRunner("tag")
	t.Cleanup(r.stop)
	r.hostReady(context.Background(), nil)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r.setLocal(i%2 == 0)
			r.restart()
		}(i)
	}
	wg.Wait()
	r.setLocal(true)
	r.restart()
	if got := f.running(); got != 1 {
		t.Errorf("%d instances run, want 1", got)
	}
}

func TestMDNSRunnerStopsOnMobileData(t *testing.T) {
	f := installFakeMDNS(t)
	r := newMDNSRunner("tag")
	t.Cleanup(r.stop)
	r.setLocal(true)
	r.hostReady(context.Background(), nil)

	r.setLocal(false)
	if f.ctx(0).Err() == nil {
		t.Error("mDNS still runs after a report of mobile data")
	}
	r.restart()
	if f.starts() != 1 {
		t.Errorf("a restart on mobile data started mDNS: %d starts", f.starts())
	}
}

func TestMDNSRunnerStopEndsEverything(t *testing.T) {
	f := installFakeMDNS(t)
	r := newMDNSRunner("tag")
	r.setLocal(true)
	r.hostReady(context.Background(), nil)

	r.stop()
	if f.ctx(0).Err() == nil {
		t.Error("mDNS still runs after stop")
	}
	r.setLocal(true)
	r.restart()
	r.hostReady(context.Background(), nil)
	if f.starts() != 1 {
		t.Errorf("started %d times, want no start after stop", f.starts())
	}
	r.stop()
}

func TestMDNSRunnerStopsWithTheNodeContext(t *testing.T) {
	f := installFakeMDNS(t)
	r := newMDNSRunner("tag")
	t.Cleanup(r.stop)
	ctx, cancel := context.WithCancel(context.Background())
	r.setLocal(true)
	r.hostReady(ctx, nil)

	cancel()
	if f.ctx(0).Err() == nil {
		t.Error("mDNS still runs after the node context ended")
	}
}

func TestMDNSRunnerFailureIsLoggedWithoutTheTag(t *testing.T) {
	f := installFakeMDNS(t)
	f.err = errors.New("cannot register secret-tag on wlan0")
	SetMinLogLevel("debug")
	t.Cleanup(func() { SetMinLogLevel("warn") })
	ClearLogs()
	r := newMDNSRunner("secret-tag")
	t.Cleanup(r.stop)

	r.setLocal(true)
	r.hostReady(context.Background(), nil)

	logs := RecentLogs()
	if !strings.Contains(logs, "WARN") || !strings.Contains(logs, "mdns") {
		t.Errorf("the failure was not logged as a warning: %q", logs)
	}
	if strings.Contains(logs, "secret-tag") {
		t.Errorf("the log carries the mDNS tag: %q", logs)
	}
	if f.ctx(0).Err() == nil {
		t.Error("the context of a failed start was left running")
	}
}

func TestMDNSServiceIgnoresAStaleSession(t *testing.T) {
	t.Cleanup(Stop)
	f := installFakeMDNS(t)
	current, stale := newTestNode(t), newTestNode(t)
	r := newMDNSRunner("tag")
	r.setLocal(true)
	mu.Lock()
	currentNode, cancelFn, currentMDNS = current, func() {}, r
	mu.Unlock()

	if err := mdnsService(context.Background(), node.Config{}, stale, nil); err != nil {
		t.Fatalf("mdnsService: %v", err)
	}
	if f.starts() != 0 {
		t.Fatalf("a stale session started mDNS")
	}
	if err := mdnsService(context.Background(), node.Config{}, current, nil); err != nil {
		t.Fatalf("mdnsService: %v", err)
	}
	if f.starts() != 1 {
		t.Errorf("started %d times for the current session, want 1", f.starts())
	}
}

func TestStopEndsTheMDNSRunner(t *testing.T) {
	f := installFakeMDNS(t)
	r := newMDNSRunner("tag")
	r.setLocal(true)
	r.hostReady(context.Background(), nil)
	mu.Lock()
	currentNode, cancelFn, currentMDNS = newTestNode(t), func() {}, r
	mu.Unlock()

	Stop()

	mu.Lock()
	left := currentMDNS
	mu.Unlock()
	if left != nil {
		t.Error("Stop left the mDNS runner in place for the next session")
	}
	if f.ctx(0).Err() == nil {
		t.Error("mDNS still runs after Stop")
	}
	r.restart()
	if f.starts() != 1 {
		t.Error("the runner of a stopped session started mDNS again")
	}
}

func TestNodeExitEndsTheMDNSRunner(t *testing.T) {
	f := installFakeMDNS(t)
	n := newTestNode(t)
	r := newMDNSRunner("tag")
	r.setLocal(true)
	r.hostReady(context.Background(), nil)
	mu.Lock()
	currentNode, cancelFn, currentMDNS = n, func() {}, r
	mu.Unlock()

	handleNodeExit(n, func() {}, errors.New("boom"))

	mu.Lock()
	left := currentMDNS
	mu.Unlock()
	if left != nil {
		t.Error("the node exit left the mDNS runner in place")
	}
	if f.ctx(0).Err() == nil {
		t.Error("mDNS still runs after the node exited")
	}
}

func TestParseInterfaceNames(t *testing.T) {
	got := parseInterfaceNames(" wlan0, eth1,,wlan0 ")
	want := map[string]bool{"wlan0": true, "eth1": true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseInterfaceNames = %v, want %v", got, want)
	}
	if got := parseInterfaceNames(""); len(got) != 0 {
		t.Errorf("parseInterfaceNames(\"\") = %v, want empty", got)
	}
}

func TestMDNSRunnerUsesEveryInterfaceWhenNoneIsKnown(t *testing.T) {
	f := installFakeMDNS(t)
	r := newMDNSRunner("tag")
	t.Cleanup(r.stop)
	r.setLocal(true)
	r.hostReady(context.Background(), nil)

	if f.filter(0) != nil {
		t.Error("mDNS got an interface filter before any local interface was reported")
	}
}

func TestMDNSRunnerKeepsOnlyTheLocalInterfaces(t *testing.T) {
	f := installFakeMDNS(t)
	r := newMDNSRunner("tag")
	t.Cleanup(r.stop)
	r.setInterfaces("wlan0")
	r.setLocal(true)
	r.hostReady(context.Background(), nil)

	filter := f.filter(0)
	if filter == nil {
		t.Fatal("mDNS got no interface filter")
	}
	if !filter(net.Interface{Name: "wlan0"}) {
		t.Error("the filter rejects wlan0, a local interface")
	}
	if filter(net.Interface{Name: "eth0"}) {
		t.Error("the filter accepts eth0, which is not a local interface")
	}
}

func TestMDNSRunnerRestartsWhenTheLocalInterfacesChange(t *testing.T) {
	f := installFakeMDNS(t)
	r := newMDNSRunner("tag")
	t.Cleanup(r.stop)
	r.setLocal(true)
	r.hostReady(context.Background(), nil)

	r.setInterfaces("wlan0")
	if f.starts() != 2 || f.running() != 1 {
		t.Fatalf("starts = %d, running = %d after new interfaces, want 2 and 1", f.starts(), f.running())
	}
	if f.filter(1) == nil || !f.filter(1)(net.Interface{Name: "wlan0"}) {
		t.Error("the new instance does not run on wlan0")
	}
	r.setInterfaces("wlan0")
	if f.starts() != 2 {
		t.Errorf("the same interfaces restarted mDNS: %d starts", f.starts())
	}
}

func TestMDNSRunnerDoesNotStartOnInterfacesAlone(t *testing.T) {
	f := installFakeMDNS(t)
	r := newMDNSRunner("tag")
	t.Cleanup(r.stop)
	r.hostReady(context.Background(), nil)

	r.setInterfaces("wlan0")
	if f.starts() != 0 {
		t.Errorf("started %d times without a local network", f.starts())
	}
}

func TestSetLocalInterfacesReachesTheSessionRunner(t *testing.T) {
	t.Cleanup(Stop)
	f := installFakeMDNS(t)
	r := newMDNSRunner("tag")
	r.setLocal(true)
	r.hostReady(context.Background(), nil)
	mu.Lock()
	currentNode, cancelFn, currentMDNS = newTestNode(t), func() {}, r
	mu.Unlock()

	SetLocalInterfaces("wlan0")
	if f.starts() != 2 {
		t.Fatalf("started %d times, want a restart for the new interfaces", f.starts())
	}
	if f.filter(1)(net.Interface{Name: "eth0"}) {
		t.Error("mDNS still runs on eth0")
	}
}

func TestSetLocalInterfacesWithoutASessionDoesNothing(t *testing.T) {
	Stop()
	SetLocalInterfaces("wlan0")
}
