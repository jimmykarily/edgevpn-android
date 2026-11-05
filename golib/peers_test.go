package edgevpn

import (
	"testing"

	dht "github.com/libp2p/go-libp2p-kad-dht"
)

const vmPeer = "/ip4/10.0.2.2/tcp/40001/p2p/12D3KooWD7dVUX9WL1NbxASy1GHkKqT7teqCZq7H6HBHrP6EqsdJ"

func resetExtraPeers(t *testing.T) {
	t.Cleanup(func() {
		if err := SetExtraPeers(""); err != nil {
			t.Errorf("reset extra peers: %v", err)
		}
	})
}

// With no extra peers the bootstrap list stays empty, which makes edgevpn
// use the public DHT bootstrap peers, as the CLI does.
func TestNetworkConfigHasNoBootstrapPeersByDefault(t *testing.T) {
	resetExtraPeers(t)
	cfg := networkConfig("some-token", "10.1.0.2")
	if len(cfg.Discovery.BootstrapPeers) != 0 {
		t.Errorf("BootstrapPeers = %v, want none", cfg.Discovery.BootstrapPeers)
	}
}

// An extra peer is dialled directly at start, next to the public bootstrap
// peers, so the DHT keeps working as it does without one.
func TestExtraPeersAreAddedAfterThePublicBootstrapPeers(t *testing.T) {
	resetExtraPeers(t)
	if err := SetExtraPeers(" " + vmPeer + "\n"); err != nil {
		t.Fatalf("SetExtraPeers: %v", err)
	}

	got := networkConfig("some-token", "10.1.0.2").Discovery.BootstrapPeers

	want := len(dht.DefaultBootstrapPeers) + 1
	if len(got) != want {
		t.Fatalf("got %d bootstrap peers, want %d: %v", len(got), want, got)
	}
	for i, p := range dht.DefaultBootstrapPeers {
		if got[i] != p.String() {
			t.Errorf("bootstrap peer %d = %q, want %q", i, got[i], p.String())
		}
	}
	if got[want-1] != vmPeer {
		t.Errorf("last bootstrap peer = %q, want %q", got[want-1], vmPeer)
	}
}

// edgevpn dereferences the peer ID of every bootstrap address, so one
// without /p2p/<id> would crash the node; it is refused up front instead.
func TestSetExtraPeersRefusesAnAddressWithoutAPeerID(t *testing.T) {
	resetExtraPeers(t)
	for _, bad := range []string{"/ip4/10.0.2.2/tcp/40001", "not an address"} {
		if err := SetExtraPeers(bad); err == nil {
			t.Errorf("SetExtraPeers(%q) succeeded, want an error", bad)
		}
	}
	if got := networkConfig("some-token", "10.1.0.2").Discovery.BootstrapPeers; len(got) != 0 {
		t.Errorf("BootstrapPeers after refused calls = %v, want none", got)
	}
}
