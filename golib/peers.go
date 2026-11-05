package edgevpn

import (
	"fmt"
	"strings"
	"sync"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/peer"
)

var (
	extraPeersMu sync.Mutex
	extraPeers   []string
)

// SetExtraPeers gives the next Start peers to dial directly, as libp2p
// addresses with a peer ID separated by white space, such as
// "/ip4/10.0.2.2/tcp/40001/p2p/12D3Koo...". An empty string removes them.
// The network tests use it to reach their test peer in seconds instead of
// waiting for the public DHT. The whole list is refused if one address has
// no peer ID, because edgevpn crashes on such a bootstrap address.
func SetExtraPeers(peers string) error {
	fields := strings.Fields(peers)
	for _, p := range fields {
		if _, err := peer.AddrInfoFromString(p); err != nil {
			return fmt.Errorf("extra peer %q: %w", p, err)
		}
	}
	extraPeersMu.Lock()
	defer extraPeersMu.Unlock()
	extraPeers = fields
	return nil
}

// bootstrapPeers returns the bootstrap list for the node config: none when
// there are no extra peers, so edgevpn uses the public DHT bootstrap peers,
// and otherwise those public peers followed by the extra ones, because a
// non-empty list replaces edgevpn's default.
func bootstrapPeers() []string {
	extraPeersMu.Lock()
	defer extraPeersMu.Unlock()
	if len(extraPeers) == 0 {
		return nil
	}
	out := make([]string, 0, len(dht.DefaultBootstrapPeers)+len(extraPeers))
	for _, p := range dht.DefaultBootstrapPeers {
		out = append(out, p.String())
	}
	return append(out, extraPeers...)
}
