package edgevpn

import (
	"context"
	"net"

	libp2p "github.com/libp2p/go-libp2p"
	libp2pnetwork "github.com/libp2p/go-libp2p/core/network"
	libp2pquic "github.com/libp2p/go-libp2p/p2p/transport/quic"
	"github.com/libp2p/go-libp2p/p2p/transport/quicreuse"
	quicgo "github.com/quic-go/quic-go"
	"go.uber.org/fx"
)

// protectedListenUDP creates a UDP PacketConn whose fd is passed through
// VpnService.protect() before bind(). Used as the listenUDP override in the
// QUIC connection manager so that every UDP socket the QUIC transport creates
// is excluded from the VPN tunnel (otherwise it would loop back into the TUN).
func protectedListenUDP(network string, laddr *net.UDPAddr) (net.PacketConn, error) {
	addr := ""
	if laddr != nil {
		addr = laddr.String()
	}
	return ProtectedListenConfig().ListenPacket(context.Background(), network, addr)
}

// newProtectedQUICConnManager is an fx-injectable constructor for a
// quicreuse.ConnManager with protected UDP sockets. It mirrors the default
// ConnManager setup inside go-libp2p's config but overrides the UDP listener
// with protectedListenUDP. Dials and hole punches reuse the listening
// socket, so they leave from the advertised port: a hole punch opens the
// phone's firewall only for the port its packets come from.
func newProtectedQUICConnManager(
	key quicgo.StatelessResetKey,
	tokenKey quicgo.TokenGeneratorKey,
	rcmgr libp2pnetwork.ResourceManager,
	lifecycle fx.Lifecycle,
) (*quicreuse.ConnManager, error) {
	cm, err := quicreuse.NewConnManager(key, tokenKey,
		quicreuse.OverrideListenUDP(protectedListenUDP),
		quicreuse.ConnContext(func(ctx context.Context, clientInfo *quicgo.ClientInfo) (context.Context, error) {
			addr, addrErr := quicreuse.ToQuicMultiaddr(clientInfo.RemoteAddr, quicgo.Version1)
			if addrErr != nil {
				addr = nil
			}
			scope, err := rcmgr.OpenConnection(libp2pnetwork.DirInbound, false, addr)
			if err != nil {
				return ctx, err
			}
			ctx = libp2pnetwork.WithConnManagementScope(ctx, scope)
			context.AfterFunc(ctx, func() { scope.Done() })
			return ctx, nil
		}),
		quicreuse.VerifySourceAddress(func(addr net.Addr) bool {
			return rcmgr.VerifySourceAddress(addr)
		}),
	)
	if err != nil {
		return nil, err
	}
	lifecycle.Append(fx.StopHook(cm.Close))
	return cm, nil
}

// quicTransportOptions returns the libp2p options that add QUIC transport
// with Android-protected UDP sockets alongside the TCP transport.
func quicTransportOptions() []libp2p.Option {
	return []libp2p.Option{
		libp2p.Transport(libp2pquic.NewTransport),
		libp2p.QUICReuse(newProtectedQUICConnManager),
	}
}
