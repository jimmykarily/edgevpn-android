// Package edgevpn provides the Go core for the EdgeVPN Android app.
// gomobile bind exports these functions as Java methods in the edgevpn.Edgevpn class.
package edgevpn

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/libp2p/go-libp2p"
	crypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/mudler/edgevpn/pkg/config"
	"github.com/mudler/edgevpn/pkg/node"
	"github.com/mudler/edgevpn/pkg/services"
	"github.com/mudler/edgevpn/pkg/vpn"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"github.com/wlynxg/anet"

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
)

// GenerateNodeKey returns a new Ed25519 private key in libp2p's marshaled
// form, for Start. The node ID is derived from this key, and ledger entries
// are signed with it, so the caller must store it and pass the same key on
// every Start. A new key makes the phone a new node: its old ledger entries
// stay locked to the old identity until the ownership TTL expires.
func GenerateNodeKey() ([]byte, error) {
	key, _, err := crypto.GenerateKeyPair(crypto.Ed25519, -1)
	if err != nil {
		return nil, fmt.Errorf("generate node key: %w", err)
	}
	raw, err := crypto.MarshalPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("marshal node key: %w", err)
	}
	return raw, nil
}

// networkConfig returns the edgevpn configuration for a token. The edgevpn
// CLI fills most of these fields from its flag defaults (cmd/util.go), and the
// library's zero values differ from those defaults in ways that break
// interoperation with CLI nodes, so every value the CLI sets is set here too.
func networkConfig(token, address string) config.Config {
	logLevel, libp2pLogLevel := "info", "error"
	if DetailedLogs() {
		logLevel, libp2pLogLevel = "debug", "debug"
	}
	return config.Config{
		NetworkToken:   token,
		Address:        address + "/24",
		BootstrapIface: false,
		LowProfile:     true,
		Concurrency:    1,
		PacketMTU:      1500,
		InterfaceMTU:   1500,
		FrameTimeout:   "15s",
		LogLevel:       logLevel,
		Libp2pLogLevel: libp2pLogLevel,
		Discovery: config.Discovery{
			DHT:            true,
			MDNS:           true, // mDNS needed for LAN bootstrap; zeroconf multicast works on Android
			BootstrapPeers: bootstrapPeers(),
		},
		// Match the CLI defaults (cmd/util.go: autorelay=true, holepunch=true).
		// These are essential for NAT traversal when both peers are behind NAT:
		// AutoRelay reserves a slot on a public circuit v2 relay and advertises
		// /p2p-circuit addresses; HolePunch coordinates DCUtR to establish a direct
		// connection once a relayed path exists.
		Connection: config.Connection{
			AutoRelay: true,
			HolePunch: true,
		},
		// CLI defaults both of these to 10s (cmd/util.go). If left at zero, vpn's
		// BackoffTicker(MaxInterval=0) misbehaves and the VPN service's
		// Announce() loop (which publishes this node's IP into the blockchain
		// ledger) either never fires or spins. Without it, the phone appears in
		// /api/nodes but not in /api/machines and never shows in the web UI.
		Ledger: config.Ledger{
			AnnounceInterval: 10 * time.Second,
			SyncInterval:     10 * time.Second,
		},
		// The CLI defaults to "enforce": ledger entries are signed with the
		// node key and unsigned or foreign writes are rejected. The library's
		// zero value means "off", and a node with ownership off writes
		// unsigned entries that every CLI node drops, so the phone never
		// appears in their machine list. All nodes in a network must run the
		// same mode.
		Ownership: config.Ownership{
			Mode: "enforce",
			TTL:  node.DefaultOwnershipTTL,
		},
	}
}

// Start initialises the VPN with a TUN file descriptor, a base64-encoded
// EdgeVPN token and the node key from GenerateNodeKey. The fd must come from
// Android's VpnService.Builder.establish(); Start takes ownership of it and
// closes it on failure. The address is this device's VPN address without a
// prefix length, such as "10.1.0.2", and the name is how this device is
// shown to the user. This function returns immediately; the VPN runs in a
// background goroutine.
func Start(fd int, token string, nodeKey []byte, address string, name string) error {
	mu.Lock()
	defer mu.Unlock()

	if cancelFn != nil {
		// The caller handed over ownership of fd, so a rejected start closes it.
		syscall.Close(fd)
		return fmt.Errorf("already running")
	}

	lastFailure = ""
	failedAt = time.Time{}
	selfAddress = address
	selfName = name

	tunFile = os.NewFile(uintptr(fd), "tun")
	if tunFile == nil {
		fail(reasonStart, fmt.Sprintf("invalid file descriptor: %d", fd))
		return fmt.Errorf("invalid file descriptor: %d", fd)
	}

	// gomobile passes a Java byte[] as a slice over Java memory that is only
	// valid until this call returns, and the node reads the key later, from
	// the goroutine below. Without a copy it reads freed memory and fails
	// with "proto: cannot parse invalid wire-format data".
	nodeKey = append([]byte(nil), nodeKey...)
	if _, err := crypto.UnmarshalPrivateKey(nodeKey); err != nil {
		closeTunOnError("invalid node key")
		fail(reasonKey, fmt.Sprintf("invalid node key: %v", err))
		return fmt.Errorf("invalid node key: %w", err)
	}

	llger := &bufferLogger{}
	cfg := networkConfig(normalizeToken(token), address)

	SetMinLogLevel(cfg.LogLevel)
	nodeOpts, vpnOpts, err := cfg.ToOpts(llger)
	if err != nil {
		closeTunOnError("config error")
		fail(reasonKey, fmt.Sprintf("config error: %v", err))
		return fmt.Errorf("config error: %w", err)
	}
	nodeOpts = append(nodeOpts, node.WithPrivKey(nodeKey))
	// The Alive options must come before the VPN service options: the VPN
	// service blocks in its packet loop, so services after it never start.
	nodeOpts = append(nodeOpts, services.Alive(checkInInterval(), aliveScrub, aliveMax)...)
	nodeOpts = append(nodeOpts, node.WithNetworkService(hostReadyService))

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
			// libp2p.NoTransports sets cfg.Transports to a non-nil empty slice,
			// which prevents the DefaultListenAddrs fallback from firing. We must
			// provide explicit listen addresses so the node actually binds a port.
			// Port 0 lets the OS pick a free port; we use AddrsFactory below to
			// advertise the real LAN IP instead of 0.0.0.0.
			libp2p.ListenAddrStrings(
				"/ip4/0.0.0.0/tcp/0",
				"/ip6/::/tcp/0",
			),
			// Disable NATPortMap which is in edgevpn's defaultLibp2pOptions.
			// On Android, the netroute call inside NAT gateway discovery is
			// blocked by SELinux. Mobile devices sit behind carrier/router NAT
			// anyway and can't map ports, so this is a no-op in practice.
			libp2p.NATManager(nil),
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
			libp2p.AddrsFactory(androidAddrsFactory),
		),
	)

	// QUIC temporarily disabled: PSK (Private Shared Key) wraps bytes with XSalsa20,
	// which breaks QUIC's TLS handshake. mDNS fires at ~220ms and h.Connect() attempts
	// QUIC first; the handshake hangs indefinitely, blocking node startup entirely.
	// TCP-only until QUIC+PSK incompatibility is resolved.
	// nodeOpts = append(nodeOpts,
	// 	node.WithLibp2pAdditionalOptions(quicTransportOptions()...),
	// )

	vpnOpts = append(vpnOpts, vpn.WithInterface(tunFile))

	vpnNodeOpts, err := vpn.Register(vpnOpts...)
	if err != nil {
		closeTunOnError("vpn register error")
		fail(reasonStart, fmt.Sprintf("vpn register error: %v", err))
		return fmt.Errorf("vpn register error: %w", err)
	}
	nodeOpts = append(nodeOpts, vpnNodeOpts...)

	n, err := node.New(nodeOpts...)
	if err != nil {
		closeTunOnError("node create error")
		fail(reasonStart, fmt.Sprintf("node create error: %v", err))
		return fmt.Errorf("node create error: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancelFn = cancel
	currentNode = n
	hostReady = false
	startedAt = time.Now()

	go func() {
		emit("INFO ", "node.Start: entering (blocks in VPN service readPackets loop)")
		err := n.Start(ctx)
		if err == nil || ctx.Err() != nil {
			emit("INFO ", "node.Start returned cleanly (context cancelled)")
			return
		}
		handleNodeExit(n, cancel, err)
	}()

	return nil
}

// handleNodeExit deals with node n stopping on its own with err. It always
// cancels n's context and closes n's host so the node's background work ends.
// It runs on the goroutine that ran n.Start, so reading n's host is safe even
// when the host was never marked ready. Only when n is still the current
// session does it release the tunnel, record the failure and tell the
// listener; a stale exit must not disturb a newer session.
func handleNodeExit(n *node.Node, cancel context.CancelFunc, err error) {
	cancel()
	closeNodeHost(n)
	mu.Lock()
	current := currentNode == n
	if current {
		if tunFile != nil {
			closeTunOnError("node failure")
		}
		currentNode = nil
		cancelFn = nil
		hostReady = false
		fail(reasonStopped, fmt.Sprintf("node.Start returned error: %v", err))
	}
	mu.Unlock()
	if current {
		notifyStopped(err.Error())
	}
}

// closeTunOnError closes the TUN file when Start fails after taking it over,
// and forgets it so a later Stop does not close the descriptor a second time.
// The caller must hold mu.
func closeTunOnError(reason string) {
	if err := tunFile.Close(); err != nil {
		emit("WARN ", fmt.Sprintf("failed to close tun file on %s: %v", reason, err))
	}
	tunFile = nil
}

// Stop shuts down the VPN, closes the libp2p host and closes the TUN file
// descriptor. Closing the fd tells Android the VPN interface is gone so it
// tears down routing. The host is closed only when it was marked ready; a
// session stopped earlier closes its own host in hostReadyService.
func Stop() {
	mu.Lock()
	var hostToClose *node.Node
	if hostReady {
		hostToClose = currentNode
	}
	if cancelFn != nil {
		cancelFn()
		cancelFn = nil
	}
	lastFailure = ""
	failedAt = time.Time{}
	currentNode = nil
	startedAt = time.Time{}
	hostReady = false
	selfAddress = ""
	selfName = ""
	if tunFile != nil {
		if err := tunFile.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			emit("WARN ", fmt.Sprintf("failed to close tun file in Stop(): %v", err))
		}
		tunFile = nil
	}
	mu.Unlock()
	closeNodeHost(hostToClose)
}

// Status returns "off", "looking", "connected" or "problem".
func Status() string {
	return collectSnapshot().State
}

// androidAddrsFactory replaces unspecified listen addresses (0.0.0.0, ::)
// with real interface IPs obtained via anet.InterfaceAddrs(). On Android,
// the standard Go net.InterfaceAddrs() fails because SELinux blocks
// bind() on netlink_route_socket. The anet library works around this by
// using sendto() directly, avoiding the bind call.
//
// For non-wildcard listen addresses the original address is kept as-is.
func androidAddrsFactory(listenAddrs []ma.Multiaddr) []ma.Multiaddr {
	// Get real interface addresses via anet (works on Android).
	ifAddrs, err := anet.InterfaceAddrs()
	if err != nil {
		emit("WARN ", fmt.Sprintf("anet.InterfaceAddrs failed: %v; using listen addrs as-is", err))
		return listenAddrs
	}

	// Convert net.Addr to multiaddrs, skip loopback, link-local IPv6, and our
	// own VPN subnet (10.1.0.0/24) — advertising 10.1.0.2 would make peers
	// try routing through the not-yet-established VPN tunnel.
	vpnNet := &net.IPNet{IP: net.IP{10, 1, 0, 0}, Mask: net.CIDRMask(24, 32)}
	var resolved []ma.Multiaddr
	for _, a := range ifAddrs {
		ipNet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		ip := ipNet.IP
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() || vpnNet.Contains(ip) {
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
	// We must handle loopback as well as wildcards because on Android,
	// go-libp2p's interfaceAddrsCache falls back to loopback-only when
	// manet.InterfaceMultiaddrs() fails (netlinkrib is blocked by SELinux).
	// go-libp2p then resolves wildcard listen addrs (0.0.0.0) to 127.0.0.1
	// before calling AddrsFactory, so by the time we get here the wildcards
	// are already gone and we see 127.0.0.1:PORT instead of 0.0.0.0:PORT.
	var out []ma.Multiaddr
	for _, la := range listenAddrs {
		ip, err := manet.ToIP(la)
		if err != nil {
			// Not an IP-based address (e.g. /p2p-circuit) — keep as-is.
			out = append(out, la)
			continue
		}
		if !ip.IsUnspecified() && !ip.IsLoopback() {
			// Already a specific, routable address — keep as-is.
			out = append(out, la)
			continue
		}

		// Wildcard or loopback → expand to real interface IPs with the same transport/port.
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
