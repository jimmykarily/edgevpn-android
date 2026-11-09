package edgevpn

import (
	"runtime"

	libp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/p2p/protocol/holepunch"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/wlynxg/anet"
)

// hostOptions returns the libp2p options the app adds to edgevpn's own:
// the transports, the listen addresses, the muxer and the address factory.
// address is this device's VPN address, whose /24 the address factory
// leaves out.
//
// NoTransports clears libp2p's default set, then the app adds back TCP and
// QUIC with protected sockets: outgoing TCP sockets and QUIC's UDP sockets
// are passed through VpnService.protect() before connect or bind. That keeps
// the VPN's own control-plane sockets from being routed back into the TUN
// interface. On Android the TCP dialer creates the socket, protects it, then
// connects, which guarantees protect() runs before connect (otherwise
// connect fails with "operation not permitted"). QUIC runs over UDP, where
// hole punching through NAT works far more often than over TCP.
//
// Two kinds of socket are not protected. The TCP listening sockets on
// 0.0.0.0 and :: come from go-libp2p's TCP transport, which has a hook for
// the dialer but none for the listener, and connections accepted on them are
// not protected either. The mDNS multicast sockets are opened by the
// zeroconf library inside edgevpn. This is harmless because the VPN routes
// only its own /24 into the tunnel, so traffic from these sockets to any
// other address leaves through Wi-Fi or mobile data as usual. The exception
// is a local network that overlaps the VPN's /24: replies to a peer on it
// would go into the tunnel. The app warns the user about that overlap
// (SetLocalNetworkClash).
func hostOptions(address string) []libp2p.Option {
	protectedDialerForAddr := func(_ ma.Multiaddr) (tcp.ContextDialer, error) {
		if runtime.GOOS == "android" {
			return newAndroidProtectedDialer(), nil
		}
		return &loggingDialer{d: ProtectedDialer()}, nil
	}
	opts := []libp2p.Option{
		libp2p.NoTransports,
		libp2p.Transport(tcp.NewTCPTransport,
			tcp.WithDialerForAddr(protectedDialerForAddr),
			tcp.DisableReuseport(),
		),
		// libp2p.NoTransports sets cfg.Transports to a non-nil empty slice,
		// which prevents the DefaultListenAddrs fallback from firing. We must
		// provide explicit listen addresses so the node actually binds a port.
		// Port 0 lets the OS pick a free port; we use AddrsFactory below to
		// advertise the real LAN IP instead of 0.0.0.0.
		libp2p.ListenAddrStrings(
			"/ip4/0.0.0.0/tcp/0",
			"/ip6/::/tcp/0",
			"/ip4/0.0.0.0/udp/0/quic-v1",
			"/ip6/::/udp/0/quic-v1",
		),
		// Disable NATPortMap which is in edgevpn's defaultLibp2pOptions.
		// On Android, the netroute call inside NAT gateway discovery is
		// blocked by SELinux. Mobile devices sit behind carrier/router NAT
		// anyway and can't map ports, so this is a no-op in practice.
		libp2p.NATManager(nil),
		muxerOption(),
		// On Android, Go's net.InterfaceAddrs() fails because it uses
		// syscall.NetlinkRIB which calls bind() on a netlink_route_socket
		// -- blocked by SELinux for untrusted apps. libp2p's address
		// manager falls back to loopback-only, so the node never
		// advertises its LAN IP and can't be reached by peers on the
		// same network.
		//
		// Fix: use AddrsFactory to resolve listen addresses (0.0.0.0)
		// into real interface IPs via anet.InterfaceAddrs(), which uses
		// sendto() instead of bind() on the netlink socket and works on
		// Android.
		libp2p.AddrsFactory(newAddrsFactory(vpnSubnet(address), anet.InterfaceAddrs)),
		// edgevpn turns hole punching on as well (networkConfig sets
		// HolePunch); this option comes later and adds the tracer.
		libp2p.EnableHolePunching(holepunch.WithTracer(holePunchLogger{})),
	}
	return append(opts, quicTransportOptions()...)
}
