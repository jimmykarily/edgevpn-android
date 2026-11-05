package edgevpn

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/mudler/edgevpn/pkg/node"
)

// hostOf returns the libp2p host of n. Tests replace it, because the host
// can only be set by edgevpn's node.Start.
var hostOf = func(n *node.Node) host.Host { return n.Host() }

// closeHost shuts down a libp2p host: its listeners, connections, DHT and
// stream handlers. edgevpn never closes the host it creates, and the app
// process outlives a VPN session, so without this every session would leave
// a host running with the phone's identity. It is safe to call more than once
// and with a nil host.
func closeHost(h host.Host) {
	if h == nil {
		return
	}
	if err := h.Close(); err != nil {
		emit("WARN ", fmt.Sprintf("failed to close the libp2p host: %v", err))
	}
}

// closeNodeHost closes the host of n. The caller must know that reading the
// host is safe: either it runs on the goroutine that ran n.Start, or n's
// host was marked ready. Closing can take a while, so the caller must not
// hold mu.
func closeNodeHost(n *node.Node) {
	if n == nil {
		return
	}
	closeHost(hostOf(n))
}
