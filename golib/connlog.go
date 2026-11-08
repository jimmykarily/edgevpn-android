package edgevpn

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/node"
	"github.com/mudler/edgevpn/pkg/protocol"
	"github.com/mudler/edgevpn/pkg/types"
	ma "github.com/multiformats/go-multiaddr"
)

// connLogInterval is how often connLogService reads the connections. A
// change shows up in the log at most this late.
const connLogInterval = 5 * time.Second

// peerConn is one libp2p connection to a peer, as the connection log shows it.
type peerConn struct {
	Remote  ma.Multiaddr
	Dir     network.Direction
	Limited bool
}

// connLogService is registered as a network service after hostReadyService,
// so it runs once n's host exists. Until ctx ends it logs the connections
// to the other peers of the VPN network whenever they change. The address
// of each connection tells the transport (TCP or QUIC), the IP family and
// whether it goes through a relay. It returns at once.
//
// It reads the connections on a timer instead of following connect events,
// because a peer often joins the ledger only after the connection to it
// is open.
func connLogService(ctx context.Context, _ node.Config, n *node.Node, ledger *blockchain.Ledger) error {
	h := n.Host()
	go func() {
		var l connLogger
		t := time.NewTicker(connLogInterval)
		defer t.Stop()
		for {
			ids := networkPeerIDs(ledger.CurrentData(), h.ID().String())
			l.report(peerConns(h, ids))
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}()
	return nil
}

// networkPeerIDs returns the sorted IDs of the peers in the VPN network
// other than self: those with a machine entry and those with a health
// check. A node without a VPN address, such as one running
// "edgevpn api --enable-healthchecks", has only a health check.
func networkPeerIDs(data map[string]map[string]blockchain.Data, self string) []string {
	seen := map[string]bool{}
	for _, v := range data[protocol.MachinesLedgerKey] {
		var m types.Machine
		if v.Unmarshal(&m) == nil && m.PeerID != "" {
			seen[m.PeerID] = true
		}
	}
	for id := range data[protocol.HealthCheckKey] {
		seen[id] = true
	}
	delete(seen, self)
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// peerConns returns h's open connections to each of ids. Peers without a
// connection and IDs that do not decode are left out.
func peerConns(h host.Host, ids []string) map[string][]peerConn {
	out := map[string][]peerConn{}
	for _, s := range ids {
		id, err := peer.Decode(s)
		if err != nil {
			continue
		}
		for _, c := range h.Network().ConnsToPeer(id) {
			st := c.Stat()
			out[s] = append(out[s], peerConn{Remote: c.RemoteMultiaddr(), Dir: st.Direction, Limited: st.Limited})
		}
	}
	return out
}

// describeConns renders conns as one log line, peers and their
// connections in sorted order so that equal sets give equal lines.
func describeConns(conns map[string][]peerConn) string {
	ids := make([]string, 0, len(conns))
	for id, cs := range conns {
		if len(cs) > 0 {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return "connections to network peers: none"
	}
	sort.Strings(ids)
	var peers []string
	for _, id := range ids {
		var parts []string
		for _, c := range conns[id] {
			dir := "out"
			if c.Dir == network.DirInbound {
				dir = "in"
			}
			part := id + " " + dir + " " + c.Remote.String()
			if c.Limited {
				part += " (relayed)"
			}
			parts = append(parts, part)
		}
		sort.Strings(parts)
		peers = append(peers, strings.Join(parts, ", "))
	}
	return "connections to network peers: " + strings.Join(peers, "; ")
}

// connLogger logs a connection list only when it differs from the last
// one it logged.
type connLogger struct {
	last string
}

func (l *connLogger) report(conns map[string][]peerConn) {
	line := describeConns(conns)
	if line == l.last {
		return
	}
	l.last = line
	emit("INFO ", line)
}
