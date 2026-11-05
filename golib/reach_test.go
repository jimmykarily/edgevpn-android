package edgevpn

import (
	"net"
	"testing"
	"time"

	ma "github.com/multiformats/go-multiaddr"
)

func TestClassifyReach(t *testing.T) {
	direct := connInfo{Remote: ma.StringCast("/ip4/192.168.1.73/tcp/4001")}
	relayed := connInfo{Remote: ma.StringCast("/ip4/65.21.90.217/tcp/4001/p2p/12D3KooW9ySGMJtJ9Qa4LvN1ErUEM2qxv1ZANXW7Mr9XjR8bd5BK/p2p-circuit"), Limited: true}
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
	newer := connInfo{Remote: ma.StringCast("/ip4/192.168.1.73/tcp/4001"), Opened: now}
	older := connInfo{Remote: ma.StringCast("/ip4/192.168.1.74/tcp/4001"), Opened: now.Add(-time.Minute)}
	relayedOld := connInfo{Remote: ma.StringCast("/ip4/65.21.90.217/tcp/4001/p2p-circuit"), Limited: true, Opened: now.Add(-time.Hour)}

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
	newer := connInfo{Remote: ma.StringCast("/ip4/65.21.90.217/tcp/4001/p2p-circuit"), Limited: true, Opened: now}
	older := connInfo{Remote: ma.StringCast("/ip4/95.216.1.2/tcp/4001/p2p-circuit"), Limited: true, Opened: now.Add(-time.Minute)}

	reach, best := classifyReach([]connInfo{newer, older})
	if reach != "helper" {
		t.Fatalf("reach: got %q, want helper", reach)
	}
	if best == nil || !best.Opened.Equal(older.Opened) {
		t.Fatalf("best: got %+v, want the oldest relayed connection", best)
	}
}

func TestClassifyReachBestIsCopy(t *testing.T) {
	conns := []connInfo{{Remote: ma.StringCast("/ip4/192.168.1.73/tcp/4001")}}
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
	if !isRelayed(connInfo{Remote: ma.StringCast("/ip4/192.168.1.73/tcp/4001"), Limited: true}) {
		t.Error("limited connection not relayed")
	}
	if !isRelayed(connInfo{Remote: ma.StringCast("/ip4/65.21.90.217/tcp/4001/p2p-circuit")}) {
		t.Error("circuit address not relayed")
	}
	if isRelayed(connInfo{Remote: ma.StringCast("/ip4/192.168.1.73/tcp/4001")}) {
		t.Error("direct connection reported as relayed")
	}
}

var vpnAddress = net.ParseIP("10.1.0.2")

func TestSameNetwork(t *testing.T) {
	_, wifi, _ := net.ParseCIDR("192.168.0.0/23")
	_, v6, _ := net.ParseCIDR("2a02:1388:2a25:5d1e::/64")
	local := []*net.IPNet{wifi, v6}
	if !sameNetwork(net.ParseIP("192.168.1.73"), local, vpnAddress) {
		t.Error("LAN IPv4 not same network")
	}
	if sameNetwork(net.ParseIP("10.106.78.113"), local, vpnAddress) {
		t.Error("other private subnet counted as same network")
	}
	if sameNetwork(net.ParseIP("84.247.173.168"), local, vpnAddress) {
		t.Error("public IPv4 counted as same network")
	}
	if !sameNetwork(net.ParseIP("2a02:1388:2a25:5d1e::5"), local, vpnAddress) {
		t.Error("global IPv6 in the phone's prefix not same network")
	}
	if sameNetwork(net.ParseIP("2a02:dead::5"), local, vpnAddress) {
		t.Error("global IPv6 outside the phone's prefixes counted as same network")
	}
}

func TestSameNetworkIPv6LocalAddresses(t *testing.T) {
	_, ula, _ := net.ParseCIDR("fd12:3456::/64")
	_, linkLocal, _ := net.ParseCIDR("fe80::/64")
	local := []*net.IPNet{ula, linkLocal}
	if !sameNetwork(net.ParseIP("fd12:3456::7"), local, vpnAddress) {
		t.Error("ULA address in the local ULA prefix not same network")
	}
	if !sameNetwork(net.ParseIP("fe80::1234"), local, vpnAddress) {
		t.Error("link-local address in a local link-local prefix not same network")
	}
	if sameNetwork(net.ParseIP("fd99::7"), local, vpnAddress) {
		t.Error("ULA address outside the local prefixes counted as same network")
	}
}

func TestSameNetworkIgnoresHostOnlyPrefixes(t *testing.T) {
	v6Host := &net.IPNet{IP: net.ParseIP("2a02:1388:2a25:5d1e::5"), Mask: net.CIDRMask(128, 128)}
	v4Host := &net.IPNet{IP: net.ParseIP("192.168.1.73").To4(), Mask: net.CIDRMask(32, 32)}
	local := []*net.IPNet{v6Host, v4Host}
	if sameNetwork(net.ParseIP("2a02:1388:2a25:5d1e::5"), local, vpnAddress) {
		t.Error("a /128 local address counted as a network")
	}
	if sameNetwork(net.ParseIP("192.168.1.73"), local, vpnAddress) {
		t.Error("a /32 local address counted as a network")
	}
}

func TestSameNetworkIgnoresTheVPNSubnet(t *testing.T) {
	_, tun, _ := net.ParseCIDR("10.1.0.0/24")
	if sameNetwork(net.ParseIP("10.1.0.1"), []*net.IPNet{tun}, vpnAddress) {
		t.Error("an address in the VPN's own subnet counted as same network")
	}
}

func TestSameNetworkRejectsNilAndEmpty(t *testing.T) {
	_, wifi, _ := net.ParseCIDR("192.168.0.0/23")
	if sameNetwork(nil, []*net.IPNet{wifi}, vpnAddress) {
		t.Error("nil IP counted as same network")
	}
	if sameNetwork(net.ParseIP("192.168.1.73"), nil, vpnAddress) {
		t.Error("no local networks still matched")
	}
}

func TestTransportOf(t *testing.T) {
	for addr, want := range map[string]string{
		"/ip4/192.168.1.73/tcp/4001":               "TCP",
		"/ip4/192.168.1.73/udp/4001/quic-v1":       "QUIC",
		"/ip4/192.168.1.73/udp/4001/quic":          "QUIC",
		"/ip4/65.21.90.217/tcp/4001/p2p-circuit":   "TCP",
		"/ip4/192.168.1.73/udp/4001/webrtc-direct": "other",
	} {
		if got := transportOf(ma.StringCast(addr)); got != want {
			t.Errorf("transportOf(%s): got %q, want %q", addr, got, want)
		}
	}
}
