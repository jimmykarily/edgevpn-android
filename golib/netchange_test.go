package edgevpn

import (
	"net"
	"reflect"
	"testing"

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
