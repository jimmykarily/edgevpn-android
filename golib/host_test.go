package edgevpn

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/mudler/edgevpn/pkg/node"
	manet "github.com/multiformats/go-multiaddr/net"
)

// newLoopbackHost returns a libp2p host listening on a loopback TCP port and
// that port's address.
func newLoopbackHost(t *testing.T) (host.Host, string) {
	t.Helper()
	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { h.Close() })
	for _, a := range h.Network().ListenAddresses() {
		if addr, err := manet.ToNetAddr(a); err == nil {
			return h, addr.String()
		}
	}
	t.Fatalf("no TCP listen address in %v", h.Network().ListenAddresses())
	return nil, ""
}

func listening(addr string) bool {
	c, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		return false
	}
	c.Close()
	return true
}

// fakeHosts makes hostOf return h for n during the test.
func fakeHosts(t *testing.T, hosts map[*node.Node]host.Host) {
	t.Helper()
	orig := hostOf
	hostOf = func(n *node.Node) host.Host { return hosts[n] }
	t.Cleanup(func() { hostOf = orig })
}

func TestCloseHostStopsListeningAndIsIdempotent(t *testing.T) {
	h, addr := newLoopbackHost(t)
	if !listening(addr) {
		t.Fatalf("the host does not listen on %s before closing", addr)
	}
	closeHost(h)
	if listening(addr) {
		t.Errorf("the host still listens on %s after closing", addr)
	}
	closeHost(h)
	closeHost(nil)
}

func TestStopWithoutANodeDoesNotPanic(t *testing.T) {
	Stop()
	Stop()
}

func TestStopClosesTheHostOfTheCurrentSession(t *testing.T) {
	t.Cleanup(Stop)
	n := newTestNode(t)
	h, addr := newLoopbackHost(t)
	fakeHosts(t, map[*node.Node]host.Host{n: h})
	mu.Lock()
	currentNode, hostReady, cancelFn = n, true, func() {}
	mu.Unlock()

	Stop()

	if listening(addr) {
		t.Error("the host still listens after Stop")
	}
}

func TestStopLeavesAHostThatIsNotReadyAlone(t *testing.T) {
	t.Cleanup(Stop)
	n := newTestNode(t)
	read := false
	orig := hostOf
	hostOf = func(*node.Node) host.Host { read = true; return nil }
	t.Cleanup(func() { hostOf = orig })
	mu.Lock()
	currentNode, hostReady, cancelFn = n, false, func() {}
	mu.Unlock()

	Stop()

	if read {
		t.Error("Stop read the host before it was marked ready")
	}
}

func TestHostReadyServiceClosesTheHostOfAStoppedSession(t *testing.T) {
	t.Cleanup(Stop)
	stale := newTestNode(t)
	h, addr := newLoopbackHost(t)
	fakeHosts(t, map[*node.Node]host.Host{stale: h})

	err := hostReadyService(context.Background(), node.Config{}, stale, nil)

	if err == nil {
		t.Error("the service of a stopped session returned no error, so later services would start")
	}
	if listening(addr) {
		t.Error("the host of a stopped session still listens")
	}
}

func TestHandleNodeExitClosesTheHost(t *testing.T) {
	t.Cleanup(func() { Stop(); ClearLogs() })
	n := newTestNode(t)
	h, addr := newLoopbackHost(t)
	fakeHosts(t, map[*node.Node]host.Host{n: h})
	tun, _ := newTunPipe(t)
	mu.Lock()
	currentNode, tunFile, cancelFn = n, tun, func() {}
	mu.Unlock()

	handleNodeExit(n, func() {}, context.DeadlineExceeded)

	if listening(addr) {
		t.Error("the host still listens after the node exited")
	}
}
