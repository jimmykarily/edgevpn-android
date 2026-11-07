package edgevpn

import (
	"errors"
	"net"
	"reflect"
	"testing"

	ma "github.com/multiformats/go-multiaddr"
)

func ipNet(t *testing.T, cidr string) net.Addr {
	t.Helper()
	ip, n, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatal(err)
	}
	n.IP = ip
	return n
}

func fixedAddrs(addrs ...net.Addr) func() ([]net.Addr, error) {
	return func() ([]net.Addr, error) { return addrs, nil }
}

func addrStrings(addrs []ma.Multiaddr) []string {
	out := []string{}
	for _, a := range addrs {
		out = append(out, a.String())
	}
	return out
}

var wildcardTCP = []ma.Multiaddr{ma.StringCast("/ip4/0.0.0.0/tcp/4001")}

func TestAddrsFactoryLeavesOutTheConfiguredVPNSubnet(t *testing.T) {
	factory := newAddrsFactory(vpnSubnet("10.5.0.4"), fixedAddrs(
		ipNet(t, "192.168.1.35/23"),
		ipNet(t, "10.5.0.4/24"),
	))
	got := addrStrings(factory(wildcardTCP))
	want := []string{"/ip4/192.168.1.35/tcp/4001"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestAddrsFactoryKeepsALANAddressOutsideTheVPNSubnet(t *testing.T) {
	// A home network on 10.1.0.0/24 with the VPN on another subnet.
	factory := newAddrsFactory(vpnSubnet("10.5.0.4"), fixedAddrs(
		ipNet(t, "10.1.0.35/24"),
		ipNet(t, "10.5.0.4/24"),
	))
	got := addrStrings(factory(wildcardTCP))
	want := []string{"/ip4/10.1.0.35/tcp/4001"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestAddrsFactoryLeavesOutTheDefaultVPNSubnet(t *testing.T) {
	factory := newAddrsFactory(vpnSubnet("10.1.0.4"), fixedAddrs(
		ipNet(t, "192.168.1.35/23"),
		ipNet(t, "10.1.0.4/24"),
	))
	got := addrStrings(factory(wildcardTCP))
	want := []string{"/ip4/192.168.1.35/tcp/4001"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestAddrsFactoryKeepsListenAddrsWhenInterfacesCannotBeRead(t *testing.T) {
	factory := newAddrsFactory(vpnSubnet("10.1.0.4"), func() ([]net.Addr, error) {
		return nil, errors.New("netlink blocked")
	})
	got := addrStrings(factory(wildcardTCP))
	want := []string{"/ip4/0.0.0.0/tcp/4001"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestVPNSubnet(t *testing.T) {
	for _, tc := range []struct {
		address string
		want    string
	}{
		{"10.1.0.4", "10.1.0.0/24"},
		{"192.168.50.7", "192.168.50.0/24"},
	} {
		got := vpnSubnet(tc.address)
		if got == nil || got.String() != tc.want {
			t.Errorf("vpnSubnet(%q) = %v, want %s", tc.address, got, tc.want)
		}
	}
	for _, bad := range []string{"", "not-an-ip", "fd00::4"} {
		if got := vpnSubnet(bad); got != nil {
			t.Errorf("vpnSubnet(%q) = %v, want nil", bad, got)
		}
	}
}
