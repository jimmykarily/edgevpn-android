package edgevpn

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/net/swarm"
	relayclient "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/client"
	relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	"github.com/libp2p/go-libp2p/p2p/protocol/holepunch"
	libp2pquic "github.com/libp2p/go-libp2p/p2p/transport/quic"
	"github.com/libp2p/go-libp2p/p2p/transport/quicreuse"
	"github.com/marcopolo/simnet"
	"github.com/mudler/edgevpn/pkg/node"
	ma "github.com/multiformats/go-multiaddr"
)

// The phone switches from Wi-Fi to mobile data while connected to a desktop
// behind a router that blocks incoming connections. The phone's new mobile
// address accepts incoming connections. The phone drops its connections on
// the Wi-Fi address and reaches the desktop again through a relay. The desktop
// still holds its connection on the Wi-Fi address, because nothing tells it
// that address is gone. edgevpn's stale direct connection watcher, which
// node.New starts, must replace that connection with a direct one on the
// mobile address.

type simSourceIP struct{ ip atomic.Pointer[net.IP] }

func (s *simSourceIP) PreferredSourceIPForDestination(*net.UDPAddr) (net.IP, error) {
	return *s.ip.Load(), nil
}

// simQUIC makes the host's QUIC sockets live on the simulated network.
// public marks every address the host listens on as reachable from outside.
func simQUIC(router *simnet.SimpleFirewallRouter, public func(*net.UDPAddr) bool) libp2p.Option {
	src := &simSourceIP{}
	return libp2p.QUICReuse(
		quicreuse.NewConnManager,
		quicreuse.OverrideSourceIPSelector(func() (quicreuse.SourceIPSelector, error) { return src, nil }),
		quicreuse.OverrideListenUDP(func(_ string, addr *net.UDPAddr) (net.PacketConn, error) {
			src.ip.Store(&addr.IP)
			if public(addr) {
				router.SetAddrPubliclyReachable(addr)
			}
			c := simnet.NewSimConn(addr)
			c.SetUpPacketReceiver(router)
			router.AddNode(addr, c)
			return c, nil
		}))
}

type recordingTracer struct {
	events chan *holepunch.Event
}

func (r recordingTracer) Trace(e *holepunch.Event) {
	select {
	case r.events <- e:
	default:
	}
}

func newSimHost(t *testing.T, opts ...libp2p.Option) host.Host {
	t.Helper()
	h, err := libp2p.New(append([]libp2p.Option{
		libp2p.ResourceManager(&network.NullResourceManager{}),
		libp2p.NoTransports,
		libp2p.Transport(libp2pquic.NewTransport),
	}, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { h.Close() })
	return h
}

func hasDirectConn(h host.Host, p peer.ID) bool {
	for _, c := range h.Network().ConnsToPeer(p) {
		if !c.Stat().Limited {
			return true
		}
	}
	return false
}

func TestSimHandoverDesktopRedialsPhone(t *testing.T) {
	t.Run("Wi-Fi connection younger than the watcher's grace", func(t *testing.T) { simHandover(t, 0) })
	t.Run("Wi-Fi connection older than the watcher's grace", func(t *testing.T) { simHandover(t, time.Second) })
}

// grace, when set, replaces the watcher's default grace, and the Wi-Fi connection is
// kept up for longer than it before the switch.
func simHandover(t *testing.T, grace time.Duration) {
	if testing.Short() {
		t.Skip("takes about a minute")
	}
	router := &simnet.SimpleFirewallRouter{}
	everyAddr := func(*net.UDPAddr) bool { return true }
	noAddr := func(*net.UDPAddr) bool { return false }
	mobile := &net.UDPAddr{IP: net.ParseIP("3.3.0.1"), Port: 9000}
	phonePublic := func(a *net.UDPAddr) bool { return a.IP.Equal(mobile.IP) || a.IP.Equal(net.ParseIP("2.2.0.2")) }

	relayHost := newSimHost(t,
		simQUIC(router, everyAddr),
		libp2p.ListenAddrStrings("/ip4/1.2.0.1/udp/8000/quic-v1"),
		libp2p.DisableRelay(),
		libp2p.ForceReachabilityPublic(),
	)
	if _, err := relayv2.New(relayHost); err != nil {
		t.Fatal(err)
	}
	relayInfo := peer.AddrInfo{ID: relayHost.ID(), Addrs: relayHost.Addrs()}

	tracer := recordingTracer{events: make(chan *holepunch.Event, 100)}
	desktop := newSimHost(t,
		simQUIC(router, noAddr),
		libp2p.ListenAddrStrings("/ip4/2.2.0.1/udp/8000/quic-v1"),
		libp2p.ForceReachabilityPrivate(),
		libp2p.EnableHolePunching(holepunch.WithTracer(tracer)),
	)
	var opts []node.StaleConnsOption
	if grace > 0 {
		opts = append(opts, node.WithStaleConnsGrace(grace))
	}
	w := node.WatchStaleDirectConns(desktop, opts...)
	t.Cleanup(func() { w.Close() })
	phone := newSimHost(t,
		simQUIC(router, phonePublic),
		libp2p.ListenAddrStrings("/ip4/2.2.0.2/udp/8001/quic-v1"),
		libp2p.ForceReachabilityPrivate(),
		libp2p.EnableHolePunching(),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := desktop.Connect(ctx, relayInfo); err != nil {
		t.Fatal(err)
	}
	if _, err := relayclient.Reserve(ctx, desktop, relayInfo); err != nil {
		t.Fatal(err)
	}
	if err := phone.Connect(ctx, relayInfo); err != nil {
		t.Fatal(err)
	}

	// On the same Wi-Fi the desktop dials the phone directly.
	if err := desktop.Connect(ctx, peer.AddrInfo{ID: phone.ID(), Addrs: phone.Addrs()}); err != nil {
		t.Fatal(err)
	}
	if !hasDirectConn(desktop, phone.ID()) {
		t.Fatal("no direct connection on the same Wi-Fi")
	}

	time.Sleep(grace + grace/2)

	// Wi-Fi goes away: packets from and to the Wi-Fi address are lost.
	wifi := &net.UDPAddr{IP: net.ParseIP("2.2.0.2"), Port: 8001}
	router.RemoveNode(wifi)
	if err := phone.Network().Listen(ma.StringCast("/ip4/3.3.0.1/udp/9000/quic-v1")); err != nil {
		t.Fatal(err)
	}
	phone.Network().(*swarm.Swarm).ListenClose(ma.StringCast("/ip4/2.2.0.2/udp/8001/quic-v1"))
	// The phone closes its connections on the gone address, as the app's
	// NetworkChanged does, and redials the desktop.
	for _, c := range phone.Network().Conns() {
		if c.LocalMultiaddr().Equal(ma.StringCast("/ip4/2.2.0.2/udp/8001/quic-v1")) {
			c.Close()
		}
	}
	if err := phone.Connect(ctx, relayInfo); err != nil {
		t.Fatalf("phone reconnect to relay: %v", err)
	}
	switched := time.Now()
	circuit := ma.StringCast("/p2p/" + relayHost.ID().String() + "/p2p-circuit")
	var desktopAddrs []ma.Multiaddr
	for _, a := range relayHost.Addrs() {
		desktopAddrs = append(desktopAddrs, a.Encapsulate(circuit))
	}
	desktopAddrs = append(desktopAddrs, desktop.Addrs()...)
	phone.Peerstore().AddAddrs(desktop.ID(), desktopAddrs, time.Hour)
	if err := phone.Connect(network.WithDialPeerTimeout(ctx, 10*time.Second), peer.AddrInfo{ID: desktop.ID()}); err != nil {
		t.Fatalf("phone redial: %v", err)
	}
	t.Logf("phone redial done after %s, phone conns: %v", time.Since(switched), simConns(phone, desktop.ID()))

	deadline := time.After(70 * time.Second)
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case e := <-tracer.events:
			t.Logf("+%s desktop hole punch event: %s", time.Since(switched).Round(time.Millisecond), describeHolePunch(e))
		case <-tick.C:
			if directOnNewAddr(desktop, phone.ID()) {
				t.Logf("desktop has a direct connection on the mobile address after %s", time.Since(switched))
				return
			}
		case <-deadline:
			t.Fatalf("no direct connection on the mobile address %s after the switch; desktop conns: %v",
				time.Since(switched).Round(time.Second), simConns(desktop, phone.ID()))
		}
	}
}

func directOnNewAddr(h host.Host, p peer.ID) bool {
	for _, c := range h.Network().ConnsToPeer(p) {
		if !c.Stat().Limited && c.RemoteMultiaddr().Equal(ma.StringCast("/ip4/3.3.0.1/udp/9000/quic-v1")) {
			return true
		}
	}
	return false
}

func simConns(h host.Host, p peer.ID) []string {
	var out []string
	for _, c := range h.Network().ConnsToPeer(p) {
		s := c.RemoteMultiaddr().String()
		if c.Stat().Limited {
			s += " (relayed)"
		}
		out = append(out, s)
	}
	return out
}
