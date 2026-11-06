package edgevpn

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/net/swarm"
	"github.com/mudler/edgevpn/pkg/node"
	ma "github.com/multiformats/go-multiaddr"
)

func TestStaleConns(t *testing.T) {
	mk := func(s string) ma.Multiaddr {
		m, err := ma.NewMultiaddr(s)
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	v4 := net.ParseIP("192.168.1.5")
	v6 := net.ParseIP("2001:db8::1")
	tests := []struct {
		name  string
		local []net.IP
		conns []ma.Multiaddr
		want  []int
	}{
		{"ipv4 gone", []net.IP{v6}, []ma.Multiaddr{mk("/ip4/192.168.1.5/tcp/4001")}, []int{0}},
		{"ipv4 kept", []net.IP{v4}, []ma.Multiaddr{mk("/ip4/192.168.1.5/tcp/4001")}, nil},
		{"ipv6 gone", []net.IP{v4}, []ma.Multiaddr{mk("/ip6/2001:db8::1/tcp/4001")}, []int{0}},
		{"ipv6 kept", []net.IP{v6}, []ma.Multiaddr{mk("/ip6/2001:db8::1/tcp/4001")}, nil},
		{"no ip component", nil, []ma.Multiaddr{mk("/p2p-circuit")}, nil},
		{"empty local returns all ip conns", nil, []ma.Multiaddr{
			mk("/ip4/10.0.0.1/tcp/1"), mk("/p2p-circuit"), mk("/ip6/2001:db8::2/udp/1/quic-v1"),
		}, []int{0, 2}},
		{"mixed", []net.IP{v4}, []ma.Multiaddr{
			mk("/ip4/192.168.1.5/tcp/1"), mk("/ip4/10.0.0.1/tcp/1"),
		}, []int{1}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := staleConns(tc.local, tc.conns)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// recorder collects the values a debouncer's work was called with.
type recorder struct {
	mu     sync.Mutex
	values []bool
}

func (r *recorder) add(v bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.values = append(r.values, v)
}

func (r *recorder) get() []bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]bool(nil), r.values...)
}

func TestDebounceMergesABurst(t *testing.T) {
	rec := &recorder{}
	d := newNetChangeDebouncer(50*time.Millisecond, rec.add)
	t.Cleanup(d.stop)
	for _, v := range []bool{false, true, false, false, true} {
		d.trigger(v)
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(200 * time.Millisecond)
	if got := rec.get(); !reflect.DeepEqual(got, []bool{true}) {
		t.Fatalf("work ran with %v, want [true]", got)
	}
}

func TestDebounceRunsOnceMoreAfterACallDuringARun(t *testing.T) {
	rec := &recorder{}
	started := make(chan struct{}, 10)
	release := make(chan struct{})
	first := true
	d := newNetChangeDebouncer(10*time.Millisecond, func(v bool) {
		rec.add(v)
		started <- struct{}{}
		if first {
			first = false
			<-release
		}
	})
	t.Cleanup(d.stop)
	d.trigger(true)
	<-started
	d.trigger(true)
	d.trigger(false)
	time.Sleep(50 * time.Millisecond)
	close(release)
	time.Sleep(150 * time.Millisecond)
	if got := rec.get(); !reflect.DeepEqual(got, []bool{true, false}) {
		t.Fatalf("work ran with %v, want [true false]", got)
	}
}

func TestDebounceStopDropsThePendingRun(t *testing.T) {
	rec := &recorder{}
	d := newNetChangeDebouncer(30*time.Millisecond, rec.add)
	d.trigger(true)
	d.stop()
	time.Sleep(100 * time.Millisecond)
	if got := rec.get(); len(got) != 0 {
		t.Fatalf("work ran with %v after stop", got)
	}
	d.trigger(false)
	time.Sleep(100 * time.Millisecond)
	if got := rec.get(); !reflect.DeepEqual(got, []bool{false}) {
		t.Fatalf("after stop a new call ran the work with %v, want [false]", got)
	}
}

func testPeerID(t *testing.T) peer.ID {
	t.Helper()
	_, pub, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		t.Fatal(err)
	}
	id, err := peer.IDFromPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestRedialTargets(t *testing.T) {
	self, a, b, c := testPeerID(t), testPeerID(t), testPeerID(t), testPeerID(t)
	bootstrap := []string{
		"/ip4/1.2.3.4/tcp/4001/p2p/" + a.String(),
		"/ip4/1.2.3.5/tcp/4001/p2p/" + a.String(),
		"/ip4/1.2.3.6/tcp/4001/p2p/" + self.String(),
		"not an address",
		"/ip4/1.2.3.7/tcp/4001",
		"/ip4/1.2.3.8/tcp/4001/p2p/" + b.String(),
	}
	machines := []string{b.String(), c.String(), self.String(), "garbage", "", c.String()}

	got := redialTargets(self, bootstrap, machines)

	mk := func(s string) ma.Multiaddr {
		m, err := ma.NewMultiaddr(s)
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	want := []peer.AddrInfo{
		{ID: a, Addrs: []ma.Multiaddr{mk("/ip4/1.2.3.4/tcp/4001"), mk("/ip4/1.2.3.5/tcp/4001")}},
		{ID: b, Addrs: []ma.Multiaddr{mk("/ip4/1.2.3.8/tcp/4001")}},
		{ID: c},
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i].ID != want[i].ID || len(got[i].Addrs) != len(want[i].Addrs) {
			t.Fatalf("target %d: got %v, want %v", i, got[i], want[i])
		}
		for j := range want[i].Addrs {
			if !got[i].Addrs[j].Equal(want[i].Addrs[j]) {
				t.Fatalf("target %d addr %d: got %v, want %v", i, j, got[i].Addrs[j], want[i].Addrs[j])
			}
		}
	}
}

func TestRedialWithoutANetworkFinishesQuickly(t *testing.T) {
	h, _ := newLoopbackHost(t)
	closed, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := closed.Addr().(*net.TCPAddr).Port
	closed.Close()
	addr, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", port))
	if err != nil {
		t.Fatal(err)
	}
	targets := []peer.AddrInfo{{ID: testPeerID(t), Addrs: []ma.Multiaddr{addr}}, {ID: testPeerID(t)}}

	begin := time.Now()
	dialled, ok := redial(context.Background(), h, targets)
	if took := time.Since(begin); took > 3*time.Second {
		t.Errorf("redial took %v with no reachable peer", took)
	}
	if dialled != 2 || ok != 0 {
		t.Errorf("dialled %d, connected %d; want 2 and 0", dialled, ok)
	}
}

func TestRedialPassesTheBackoffAndSkipsConnectedPeers(t *testing.T) {
	h, _ := newLoopbackHost(t)
	connected, _ := newLoopbackHost(t)
	backedOff, _ := newLoopbackHost(t)
	if err := h.Connect(context.Background(), peer.AddrInfo{ID: connected.ID(), Addrs: connected.Addrs()}); err != nil {
		t.Fatal(err)
	}
	sw, isSwarm := h.Network().(*swarm.Swarm)
	if !isSwarm {
		t.Fatalf("host network is %T, want *swarm.Swarm", h.Network())
	}
	for _, a := range backedOff.Addrs() {
		sw.Backoff().AddBackoff(backedOff.ID(), a)
	}
	target := peer.AddrInfo{ID: backedOff.ID(), Addrs: backedOff.Addrs()}
	if err := h.Connect(context.Background(), target); err == nil {
		t.Fatal("a plain dial passed the backoff, so this test proves nothing")
	}

	dialled, ok := redial(context.Background(), h, []peer.AddrInfo{{ID: connected.ID()}, target})

	if dialled != 1 || ok != 1 {
		t.Errorf("dialled %d, connected %d; want 1 and 1", dialled, ok)
	}
	if h.Network().Connectedness(backedOff.ID()) != network.Connected {
		t.Error("the backed-off peer is not connected after redial")
	}
}

// stepsCall is one call of netChangeSteps seen by a test.
type stepsCall struct {
	ctx     context.Context
	n       *node.Node
	onLocal bool
}

// installNetChange replaces the debouncer with a fast one running the real
// session check, and netChangeSteps with a recorder.
func installNetChange(t *testing.T) func() []stepsCall {
	t.Helper()
	var callsMu sync.Mutex
	var calls []stepsCall
	origSteps, origDebouncer := netChangeSteps, netChange
	netChangeSteps = func(ctx context.Context, n *node.Node, _ host.Host, _ *mdnsRunner, onLocal bool) {
		callsMu.Lock()
		defer callsMu.Unlock()
		calls = append(calls, stepsCall{ctx, n, onLocal})
	}
	netChange = newNetChangeDebouncer(20*time.Millisecond, runNetChange)
	t.Cleanup(func() {
		netChange.stop()
		netChangeSteps, netChange = origSteps, origDebouncer
	})
	return func() []stepsCall {
		callsMu.Lock()
		defer callsMu.Unlock()
		return append([]stepsCall(nil), calls...)
	}
}

func setSession(n *node.Node, ready bool, r *mdnsRunner, ctx context.Context) {
	mu.Lock()
	defer mu.Unlock()
	currentNode, hostReady, cancelFn, currentMDNS, sessionCtx = n, ready, func() {}, r, ctx
}

func TestNetworkChangedWithoutASessionDoesNothing(t *testing.T) {
	calls := installNetChange(t)
	var works int
	var worksMu sync.Mutex
	netChange = newNetChangeDebouncer(20*time.Millisecond, func(bool) {
		worksMu.Lock()
		works++
		worksMu.Unlock()
	})
	Stop()
	NetworkChanged(true)
	time.Sleep(80 * time.Millisecond)

	setSession(newTestNode(t), true, nil, context.Background())
	Stop()
	NetworkChanged(false)
	time.Sleep(80 * time.Millisecond)

	worksMu.Lock()
	defer worksMu.Unlock()
	if works != 0 || len(calls()) != 0 {
		t.Errorf("work ran %d times and the steps %d times without a session", works, len(calls()))
	}
}

func TestNetworkChangedSkipsAStaleSession(t *testing.T) {
	t.Cleanup(Stop)
	calls := installNetChange(t)
	a, b := newTestNode(t), newTestNode(t)
	setSession(a, true, nil, context.Background())
	NetworkChanged(true)
	mu.Lock()
	currentNode = b
	mu.Unlock()
	time.Sleep(80 * time.Millisecond)
	if got := calls(); len(got) != 0 {
		t.Fatalf("work for session A ran the steps while B was current: %v", got)
	}

	NetworkChanged(false)
	time.Sleep(80 * time.Millisecond)
	got := calls()
	if len(got) != 1 || got[0].n != b || got[0].onLocal {
		t.Fatalf("steps for the current session: got %v, want one call for B with false", got)
	}
}

func TestNetworkChangedBeforeHostReadyOnlyRecordsTheValue(t *testing.T) {
	t.Cleanup(Stop)
	f := installFakeMDNS(t)
	calls := installNetChange(t)
	r := newMDNSRunner("tag")
	setSession(newTestNode(t), false, r, context.Background())

	NetworkChanged(true)
	time.Sleep(80 * time.Millisecond)
	if got := calls(); len(got) != 0 {
		t.Fatalf("the steps ran before the host was ready: %v", got)
	}
	r.hostReady(context.Background(), nil)
	if f.starts() != 1 {
		t.Errorf("mDNS started %d times once the host was ready, want 1", f.starts())
	}
}

func TestStopDropsAPendingNetworkChange(t *testing.T) {
	calls := installNetChange(t)
	setSession(newTestNode(t), true, nil, context.Background())
	NetworkChanged(true)
	Stop()
	setSession(newTestNode(t), true, nil, context.Background())
	t.Cleanup(Stop)
	time.Sleep(80 * time.Millisecond)
	if got := calls(); len(got) != 0 {
		t.Fatalf("a change reported before Stop ran the steps: %v", got)
	}
}

func TestStopCancelsTheContextOfARunningNetworkChange(t *testing.T) {
	calls := installNetChange(t)
	ctx, cancel := context.WithCancel(context.Background())
	setSession(newTestNode(t), true, nil, ctx)
	mu.Lock()
	cancelFn = cancel
	mu.Unlock()
	NetworkChanged(true)
	time.Sleep(80 * time.Millisecond)
	got := calls()
	if len(got) != 1 {
		t.Fatalf("steps ran %d times, want 1", len(got))
	}
	Stop()
	if got[0].ctx.Err() == nil {
		t.Error("Stop left the context of the network change running")
	}
}

func TestNetworkChangeStepsWithoutANetworkLogOneLine(t *testing.T) {
	h, _ := newLoopbackHost(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	SetMinLogLevel("info")
	t.Cleanup(func() { SetMinLogLevel("warn"); ClearLogs() })
	ClearLogs()

	begin := time.Now()
	networkChangeSteps(ctx, newTestNode(t), h, nil, true)
	if took := time.Since(begin); took > 3*time.Second {
		t.Errorf("the run took %v", took)
	}

	var lines []string
	for _, l := range strings.Split(RecentLogs(), "\n") {
		if strings.Contains(l, "INFO") && strings.Contains(l, "network changed") {
			lines = append(lines, l)
		}
	}
	if len(lines) != 1 || !strings.Contains(lines[0], "local network: true") {
		t.Fatalf("got INFO lines %q, want one saying local network: true", lines)
	}
}
