package edgevpn

import (
	"net"
	"testing"
	"time"

	ma "github.com/multiformats/go-multiaddr"
)

func TestClassifyReach(t *testing.T) {
	direct := connInfo{Remote: ma.StringCast("/ip4/192.168.21.73/tcp/4001")}
	relayed := connInfo{Remote: ma.StringCast("/ip4/203.0.113.17/tcp/4001/p2p/12D3KooWF3aRcSwpSy36Z7S4gatUWpBtnyHdtGdYY3WmEjeYxqsJ/p2p-circuit"), Limited: true}
	unlimitedCircuit := connInfo{Remote: relayed.Remote}
	for name, c := range map[string]struct {
		in   []connInfo
		want string
	}{
		"none": {nil, "offline"}, "direct": {[]connInfo{direct}, "direct"},
		"relayed": {[]connInfo{relayed}, "helper"}, "circuit without limit": {[]connInfo{unlimitedCircuit}, "helper"},
		"both, after hole punching": {[]connInfo{relayed, direct}, "direct"},
	} {
		if got, _ := classifyReach(c.in); got != c.want {
			t.Errorf("%s: got %q, want %q", name, got, c.want)
		}
	}
}

func TestClassifyReachBestIsOldestDirect(t *testing.T) {
	now := time.Now()
	newer := connInfo{Remote: ma.StringCast("/ip4/192.168.21.73/tcp/4001"), Opened: now}
	older := connInfo{Remote: ma.StringCast("/ip4/192.168.21.74/tcp/4001"), Opened: now.Add(-time.Minute)}
	relayedOld := connInfo{Remote: ma.StringCast("/ip4/203.0.113.17/tcp/4001/p2p-circuit"), Limited: true, Opened: now.Add(-time.Hour)}

	reach, best := classifyReach([]connInfo{relayedOld, newer, older})
	if reach != "direct" {
		t.Fatalf("reach: got %q, want direct", reach)
	}
	if best == nil || !best.Opened.Equal(older.Opened) {
		t.Fatalf("best: got %+v, want the oldest direct connection", best)
	}
}

func TestClassifyReachBestIsOldestRelayedWithoutDirect(t *testing.T) {
	now := time.Now()
	newer := connInfo{Remote: ma.StringCast("/ip4/203.0.113.17/tcp/4001/p2p-circuit"), Limited: true, Opened: now}
	older := connInfo{Remote: ma.StringCast("/ip4/203.0.113.2/tcp/4001/p2p-circuit"), Limited: true, Opened: now.Add(-time.Minute)}

	reach, best := classifyReach([]connInfo{newer, older})
	if reach != "helper" {
		t.Fatalf("reach: got %q, want helper", reach)
	}
	if best == nil || !best.Opened.Equal(older.Opened) {
		t.Fatalf("best: got %+v, want the oldest relayed connection", best)
	}
}

func TestClassifyReachBestIsCopy(t *testing.T) {
	conns := []connInfo{{Remote: ma.StringCast("/ip4/192.168.21.73/tcp/4001")}}
	_, best := classifyReach(conns)
	if best == nil {
		t.Fatal("best is nil")
	}
	best.Limited = true
	if conns[0].Limited {
		t.Error("best points into the caller's slice")
	}
}

func TestClassifyReachNoBestWhenOffline(t *testing.T) {
	if _, best := classifyReach(nil); best != nil {
		t.Errorf("best: got %+v, want nil", best)
	}
}

func TestIsRelayed(t *testing.T) {
	if !isRelayed(connInfo{Remote: ma.StringCast("/ip4/192.168.21.73/tcp/4001"), Limited: true}) {
		t.Error("limited connection not relayed")
	}
	if !isRelayed(connInfo{Remote: ma.StringCast("/ip4/203.0.113.17/tcp/4001/p2p-circuit")}) {
		t.Error("circuit address not relayed")
	}
	if isRelayed(connInfo{Remote: ma.StringCast("/ip4/192.168.21.73/tcp/4001")}) {
		t.Error("direct connection reported as relayed")
	}
}

func TestSameNetwork(t *testing.T) {
	_, wifi, _ := net.ParseCIDR("192.168.20.0/23")
	_, v6, _ := net.ParseCIDR("2001:db8:5d1e:1::/64")
	local := []*net.IPNet{wifi, v6}
	if !sameNetwork(net.ParseIP("192.168.21.73"), local) {
		t.Error("LAN IPv4 not same network")
	}
	if sameNetwork(net.ParseIP("10.200.7.113"), local) {
		t.Error("other private subnet counted as same network")
	}
	if sameNetwork(net.ParseIP("198.51.100.168"), local) {
		t.Error("public IPv4 counted as same network")
	}
	if sameNetwork(net.ParseIP("2001:db8:5d1e:1::5"), local) {
		t.Error("global IPv6 counted as same network")
	}
}

func TestSameNetworkIPv6LocalAddresses(t *testing.T) {
	_, ula, _ := net.ParseCIDR("fd12:3456::/64")
	_, linkLocal, _ := net.ParseCIDR("fe80::/64")
	local := []*net.IPNet{ula, linkLocal}
	if !sameNetwork(net.ParseIP("fd12:3456::7"), local) {
		t.Error("ULA address in the local ULA prefix not same network")
	}
	if !sameNetwork(net.ParseIP("fe80::1234"), local) {
		t.Error("link-local address in a local link-local prefix not same network")
	}
	if sameNetwork(net.ParseIP("fd99::7"), local) {
		t.Error("ULA address outside the local prefixes counted as same network")
	}
}

func TestSameNetworkRejectsNilAndEmpty(t *testing.T) {
	_, wifi, _ := net.ParseCIDR("192.168.20.0/23")
	if sameNetwork(nil, []*net.IPNet{wifi}) {
		t.Error("nil IP counted as same network")
	}
	if sameNetwork(net.ParseIP("192.168.21.73"), nil) {
		t.Error("no local networks still matched")
	}
}

func TestTransportOf(t *testing.T) {
	for addr, want := range map[string]string{
		"/ip4/192.168.21.73/tcp/4001":               "TCP",
		"/ip4/192.168.21.73/udp/4001/quic-v1":       "QUIC",
		"/ip4/192.168.21.73/udp/4001/quic":          "QUIC",
		"/ip4/203.0.113.17/tcp/4001/p2p-circuit":   "TCP",
		"/ip4/192.168.21.73/udp/4001/webrtc-direct": "other",
	} {
		if got := transportOf(ma.StringCast(addr)); got != want {
			t.Errorf("transportOf(%s): got %q, want %q", addr, got, want)
		}
	}
}
