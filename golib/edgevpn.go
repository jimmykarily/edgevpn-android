// Package edgevpn provides the Go core for the EdgeVPN Android app.
// gomobile bind exports these functions as Java methods in the edgevpn.Edgevpn class.
package edgevpn

import (
	"context"
	"fmt"
	"net"
	"os"
	"runtime"
	"sync"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/mudler/edgevpn/pkg/config"
	"github.com/mudler/edgevpn/pkg/node"
	"github.com/mudler/edgevpn/pkg/vpn"
	ma "github.com/multiformats/go-multiaddr"

	// Required by gomobile's generated binding code at build time.
	_ "golang.org/x/mobile/bind"
)

// loggingDialer wraps a net.Dialer and logs the underlying error when a dial fails.
type loggingDialer struct{ d *net.Dialer }

func (l *loggingDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	conn, err := l.d.DialContext(ctx, network, address)
	if err != nil {
		LogDialError(network+" "+address, err)
	}
	return conn, err
}

var (
	mu       sync.Mutex
	cancelFn context.CancelFunc
	tunFile  *os.File
	vpnState = "disconnected"
)

// Start initialises the VPN with a TUN file descriptor and a base64-encoded
// EdgeVPN token. The fd must come from Android's VpnService.Builder.establish().
// This function returns immediately; the VPN runs in a background goroutine.
func Start(fd int, token string) error {
	mu.Lock()
	defer mu.Unlock()

	if cancelFn != nil {
		return fmt.Errorf("already running")
	}

	tunFile = os.NewFile(uintptr(fd), "tun")
	if tunFile == nil {
		return fmt.Errorf("invalid file descriptor: %d", fd)
	}

	address := "10.1.0.2/24"

	llger := &bufferLogger{}

	cfg := config.Config{
		NetworkToken:   token,
		Address:        address,
		BootstrapIface: false,
		LowProfile:     true,
		Concurrency:    1,
		PacketMTU:      1500,
		InterfaceMTU:   1500,
		FrameTimeout:   "15s",
		LogLevel:       "warn", // avoid DEBUG/INFO discovery loop spam in logs
		Libp2pLogLevel: "error",
		Discovery: config.Discovery{
			DHT:  true,
			MDNS: runtime.GOOS != "android", // mDNS uses net.InterfaceAddrs() -> netlink permission denied on Android
		},
		Ledger: config.Ledger{},
	}

	SetMinLogLevel(cfg.LogLevel)
	nodeOpts, vpnOpts, err := cfg.ToOpts(llger)
	if err != nil {
		tunFile.Close()
		return fmt.Errorf("config error: %w", err)
	}

	// Inject a protected TCP transport so every socket fd is passed through
	// VpnService.protect() before connect/bind. This prevents the VPN's own
	// control-plane sockets from being routed back into the TUN interface.
	//
	// NoTransports clears the default set (which includes QUIC/WebRTC that
	// we can't protect yet), then we add back TCP with our protected dialer.
	// On Android we use a custom dial that creates the socket, protects it, then
	// connects—guaranteeing protect() runs before connect (avoids "operation not permitted").
	protectedDialerForAddr := func(_ ma.Multiaddr) (tcp.ContextDialer, error) {
		if runtime.GOOS == "android" {
			return newAndroidProtectedDialer(), nil
		}
		return &loggingDialer{d: ProtectedDialer()}, nil
	}
	nodeOpts = append(nodeOpts,
		node.WithLibp2pAdditionalOptions(
			libp2p.NoTransports,
			libp2p.Transport(tcp.NewTCPTransport,
				tcp.WithDialerForAddr(protectedDialerForAddr),
				tcp.DisableReuseport(),
			),
		),
	)

	vpnOpts = append(vpnOpts, vpn.WithInterface(tunFile))

	vpnNodeOpts, err := vpn.Register(vpnOpts...)
	if err != nil {
		tunFile.Close()
		return fmt.Errorf("vpn register error: %w", err)
	}
	nodeOpts = append(nodeOpts, vpnNodeOpts...)

	n, err := node.New(nodeOpts...)
	if err != nil {
		tunFile.Close()
		return fmt.Errorf("node create error: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancelFn = cancel
	vpnState = "connecting"

	go func() {
		err := n.Start(ctx)
		mu.Lock()
		defer mu.Unlock()
		cancelFn = nil
		if err != nil && ctx.Err() == nil {
			vpnState = "error: " + err.Error()
		} else {
			vpnState = "disconnected"
		}
	}()

	vpnState = "connected"
	return nil
}

// Stop shuts down the VPN and closes the TUN file descriptor.
// Closing the fd tells Android the VPN interface is gone so it tears down routing.
func Stop() {
	mu.Lock()
	defer mu.Unlock()
	if cancelFn != nil {
		cancelFn()
		cancelFn = nil
	}
	if tunFile != nil {
		tunFile.Close()
		tunFile = nil
	}
	vpnState = "disconnected"
}

// Status returns the current VPN status as a string.
func Status() string {
	mu.Lock()
	defer mu.Unlock()
	return vpnState
}

