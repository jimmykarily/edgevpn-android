package edgevpn

import (
	"fmt"
	"net"

	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
)

// vpnSubnet returns the /24 network of this device's VPN address, the
// subnet edgevpn routes into the tunnel (networkConfig appends "/24"). It
// returns nil for an address that is not IPv4.
func vpnSubnet(address string) *net.IPNet {
	ip := net.ParseIP(address).To4()
	if ip == nil {
		return nil
	}
	mask := net.CIDRMask(24, 32)
	return &net.IPNet{IP: ip.Mask(mask), Mask: mask}
}

// newAddrsFactory returns the libp2p AddrsFactory for Android. It replaces
// unspecified listen addresses (0.0.0.0, ::) with the device's interface IPs
// from interfaceAddrs. On Android the standard Go net.InterfaceAddrs() fails
// because SELinux blocks bind() on netlink_route_socket; the anet library,
// which the caller passes in, avoids that bind call.
//
// Loopback, link-local IPv6 and addresses in vpnNet are left out. Peers
// cannot reach this device at its VPN address before the tunnel to it
// exists, and a LAN address inside vpnNet is routed into the tunnel too.
// A nil vpnNet leaves out no subnet.
//
// For non-wildcard listen addresses the original address is kept as is.
func newAddrsFactory(vpnNet *net.IPNet, interfaceAddrs func() ([]net.Addr, error)) func([]ma.Multiaddr) []ma.Multiaddr {
	return func(listenAddrs []ma.Multiaddr) []ma.Multiaddr {
		ifAddrs, err := interfaceAddrs()
		if err != nil {
			emit("WARN ", fmt.Sprintf("anet.InterfaceAddrs failed: %v; using listen addrs as-is", err))
			return listenAddrs
		}

		var resolved []ma.Multiaddr
		for _, a := range ifAddrs {
			ipNet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipNet.IP
			if ip.IsLoopback() || ip.IsLinkLocalUnicast() || (vpnNet != nil && vpnNet.Contains(ip)) {
				continue
			}
			maddr, err := manet.FromIP(ip)
			if err != nil {
				continue
			}
			resolved = append(resolved, maddr)
		}

		if len(resolved) == 0 {
			emit("WARN ", "androidAddrsFactory: no usable interface addresses found")
			return listenAddrs
		}

		// For each listen address, if it's on a wildcard or loopback IP, expand it
		// to all resolved IPs with the same transport/port. Otherwise keep it.
		//
		// Loopback is handled as well as wildcards because on Android,
		// go-libp2p's interfaceAddrsCache falls back to loopback-only when
		// manet.InterfaceMultiaddrs() fails (netlinkrib is blocked by SELinux).
		// go-libp2p then resolves wildcard listen addrs (0.0.0.0) to 127.0.0.1
		// before calling AddrsFactory, so the wildcards arrive here as
		// 127.0.0.1:PORT instead of 0.0.0.0:PORT.
		var out []ma.Multiaddr
		for _, la := range listenAddrs {
			ip, err := manet.ToIP(la)
			if err != nil {
				// Not an IP-based address (e.g. /p2p-circuit): keep as is.
				out = append(out, la)
				continue
			}
			if !ip.IsUnspecified() && !ip.IsLoopback() {
				// Already a specific, routable address: keep as is.
				out = append(out, la)
				continue
			}

			// Wildcard or loopback: expand to the interface IPs with the same transport/port.
			_, transport := ma.SplitFirst(la)
			for _, rAddr := range resolved {
				expanded := rAddr
				if transport != nil {
					expanded = rAddr.Encapsulate(transport)
				}
				out = append(out, expanded)
			}
		}
		return out
	}
}
