package edgevpn

import (
	"os"
	"syscall"
	"testing"
	"time"

	crypto "github.com/libp2p/go-libp2p/core/crypto"
)

// The edgevpn CLI fills in these values from its flag defaults (cmd/util.go).
// The library's zero values differ, and a node built from them cannot work
// with CLI nodes: with zero intervals it never announces itself, and without
// relaying and hole punching it cannot reach peers behind NAT.
func TestNetworkConfigMatchesCLIDefaults(t *testing.T) {
	cfg := networkConfig("some-token")

	if cfg.NetworkToken != "some-token" {
		t.Errorf("NetworkToken = %q, want the token passed in", cfg.NetworkToken)
	}

	if !cfg.Connection.AutoRelay || !cfg.Connection.HolePunch {
		t.Errorf("AutoRelay = %v, HolePunch = %v, want both true", cfg.Connection.AutoRelay, cfg.Connection.HolePunch)
	}
	if cfg.Ledger.AnnounceInterval != 10*time.Second || cfg.Ledger.SyncInterval != 10*time.Second {
		t.Errorf("Ledger intervals = %v / %v, want 10s / 10s", cfg.Ledger.AnnounceInterval, cfg.Ledger.SyncInterval)
	}
	if !cfg.Discovery.DHT || !cfg.Discovery.MDNS {
		t.Errorf("Discovery DHT = %v, MDNS = %v, want both true", cfg.Discovery.DHT, cfg.Discovery.MDNS)
	}
}

func TestGenerateNodeKeyReturnsAnEd25519Key(t *testing.T) {
	raw, err := GenerateNodeKey()
	if err != nil {
		t.Fatalf("GenerateNodeKey: %v", err)
	}
	key, err := crypto.UnmarshalPrivateKey(raw)
	if err != nil {
		t.Fatalf("generated key does not unmarshal: %v", err)
	}
	if key.Type() != crypto.Ed25519 {
		t.Errorf("key type = %v, want Ed25519", key.Type())
	}

	other, err := GenerateNodeKey()
	if err != nil {
		t.Fatalf("GenerateNodeKey: %v", err)
	}
	if string(other) == string(raw) {
		t.Error("two calls returned the same key")
	}
}

// A pipe stands in for the TUN file descriptor. Start must reject a bad key
// before it touches the network, and must close the descriptor it was given,
// because the caller has handed over ownership of it.
func TestStartRejectsAMissingOrInvalidNodeKey(t *testing.T) {
	for name, key := range map[string][]byte{
		"missing": nil,
		"invalid": []byte("not a key"),
	} {
		t.Run(name, func(t *testing.T) {
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer w.Close()
			fd, err := syscall.Dup(int(r.Fd()))
			if err != nil {
				t.Fatal(err)
			}
			r.Close()

			if err := Start(fd, "some-token", key); err == nil {
				Stop()
				t.Fatal("Start succeeded, want an error")
			}
			if got := Status(); got != "disconnected" {
				t.Errorf("Status() = %q after a failed Start, want disconnected", got)
			}
			var st syscall.Stat_t
			if err := syscall.Fstat(fd, &st); err != syscall.EBADF {
				syscall.Close(fd)
				t.Errorf("fstat on the TUN descriptor = %v, want EBADF (closed)", err)
			}
		})
	}
}
