package edgevpn

import (
	"net"
	"time"

	ma "github.com/multiformats/go-multiaddr"
)

// connInfo is the part of a libp2p connection that the reach rules need.
type connInfo struct {
	Remote  ma.Multiaddr
	Limited bool
	Opened  time.Time
}

// classifyReach reports how a device is reached: "direct" when at least one
// connection goes peer to peer, "helper" when traffic only flows through a
// relay, and "offline" when there is no connection. best is a copy of the
// oldest direct connection, or of the oldest relayed one when no direct
// connection exists, and nil when there are no connections.
func classifyReach(conns []connInfo) (reach string, best *connInfo) {
	var oldestDirect, oldestRelayed *connInfo
	for i := range conns {
		c := &conns[i]
		if isRelayed(*c) {
			if oldestRelayed == nil || c.Opened.Before(oldestRelayed.Opened) {
				oldestRelayed = c
			}
			continue
		}
		if oldestDirect == nil || c.Opened.Before(oldestDirect.Opened) {
			oldestDirect = c
		}
	}
	switch {
	case oldestDirect != nil:
		c := *oldestDirect
		return "direct", &c
	case oldestRelayed != nil:
		c := *oldestRelayed
		return "helper", &c
	default:
		return "offline", nil
	}
}

// isRelayed reports whether traffic on the connection goes through a relay,
// either because libp2p marks the connection as limited or because its
// address contains a /p2p-circuit hop.
func isRelayed(c connInfo) bool {
	if c.Limited {
		return true
	}
	if c.Remote == nil {
		return false
	}
	for _, p := range c.Remote.Protocols() {
		if p.Code == ma.P_CIRCUIT {
			return true
		}
	}
	return false
}

// sameNetwork reports whether remote is a private address (IPv4 private
// ranges, or IPv6 unique local fc00::/7, which net.IP.IsPrivate covers) or an
// IPv6 link-local address, and lies inside one of the phone's own networks.
// Public addresses never count, even when they fall in a local prefix.
func sameNetwork(remote net.IP, local []*net.IPNet) bool {
	if remote == nil {
		return false
	}
	if !remote.IsPrivate() && !remote.IsLinkLocalUnicast() {
		return false
	}
	for _, n := range local {
		if n != nil && n.Contains(remote) {
			return true
		}
	}
	return false
}

// transportOf names the transport of a connection address: "TCP" for a TCP
// address, "QUIC" for a QUIC address, and "other" for anything else.
func transportOf(m ma.Multiaddr) string {
	if m == nil {
		return "other"
	}
	for _, p := range m.Protocols() {
		switch p.Code {
		case ma.P_TCP:
			return "TCP"
		case ma.P_QUIC, ma.P_QUIC_V1:
			return "QUIC"
		}
	}
	return "other"
}
