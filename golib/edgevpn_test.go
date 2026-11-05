package edgevpn

import (
	"encoding/base64"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	crypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/node"
)

// The edgevpn CLI fills in these values from its flag defaults (cmd/util.go).
// The library's zero values differ, and a node built from them cannot work
// with CLI nodes: with ownership off it writes unsigned ledger entries that
// CLI nodes reject, and with zero intervals it never announces itself.
func TestNetworkConfigMatchesCLIDefaults(t *testing.T) {
	cfg := networkConfig("some-token", "10.1.0.2")

	if cfg.NetworkToken != "some-token" {
		t.Errorf("NetworkToken = %q, want the token passed in", cfg.NetworkToken)
	}

	mode, err := blockchain.ParseOwnershipMode(cfg.Ownership.Mode)
	if err != nil {
		t.Fatalf("Ownership.Mode %q does not parse: %v", cfg.Ownership.Mode, err)
	}
	if mode != blockchain.OwnershipEnforce {
		t.Errorf("Ownership.Mode = %q, want enforce like the CLI default", cfg.Ownership.Mode)
	}
	if cfg.Ownership.TTL != node.DefaultOwnershipTTL {
		t.Errorf("Ownership.TTL = %v, want %v", cfg.Ownership.TTL, node.DefaultOwnershipTTL)
	}

	if !cfg.Connection.AutoRelay || !cfg.Connection.HolePunch {
		t.Errorf("AutoRelay = %v, HolePunch = %v, want both true", cfg.Connection.AutoRelay, cfg.Connection.HolePunch)
	}
	if cfg.Ledger.AnnounceInterval != 10*time.Second || cfg.Ledger.SyncInterval != 10*time.Second {
		t.Errorf("Ledger intervals = %v / %v, want 10s / 10s", cfg.Ledger.AnnounceInterval, cfg.Ledger.SyncInterval)
	}
	if cfg.Address != "10.1.0.2/24" {
		t.Errorf("Address = %q, want 10.1.0.2/24", cfg.Address)
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
			t.Cleanup(func() { Stop(); ClearLogs() })
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

			if err := Start(fd, "some-token", key, "10.1.0.2", "test"); err == nil {
				Stop()
				t.Fatal("Start succeeded, want an error")
			}
			if got := Status(); got != "problem" {
				t.Errorf("Status() = %q after a failed Start, want problem", got)
			}
			if got := rememberedFailure(); got != reasonKey {
				t.Errorf("remembered failure = %q, want %q", got, reasonKey)
			}
			if got := rememberedDetail(); got != "" {
				t.Errorf("error details = %q for a key failure, want none", got)
			}
			if !strings.Contains(RecentLogs(), "ERROR invalid node key") {
				t.Errorf("logs do not hold the raw error:\n%s", RecentLogs())
			}
			Stop()
			if got := Status(); got != "off" {
				t.Errorf("Status() = %q after Stop, want off", got)
			}
			var st syscall.Stat_t
			if err := syscall.Fstat(fd, &st); err != syscall.EBADF {
				syscall.Close(fd)
				t.Errorf("fstat on the TUN descriptor = %v, want EBADF (closed)", err)
			}
		})
	}
}

func TestStartWithAnUnusableDescriptorRemembersAPlainReason(t *testing.T) {
	t.Cleanup(func() { Stop(); ClearLogs() })
	key, err := GenerateNodeKey()
	if err != nil {
		t.Fatal(err)
	}
	if err := Start(-1, "some-token", key, "10.1.0.2", "test"); err == nil {
		Stop()
		t.Fatal("Start succeeded with fd -1")
	}
	if got := rememberedFailure(); got != reasonStart {
		t.Errorf("remembered failure = %q, want %q", got, reasonStart)
	}
	if got := rememberedDetail(); got != "invalid file descriptor: -1" {
		t.Errorf("error details = %q, want the raw error", got)
	}
	Stop()
	if got := rememberedDetail(); got != "" {
		t.Errorf("error details = %q after Stop, want none", got)
	}
}

// A token that edgevpn cannot read fails when the node is created, and the
// error text can quote what the token holds (YAML quotes the start of a
// value it cannot convert), so none of it may reach the error details.
func TestStartWithABadTokenKeepsItOutOfTheErrorDetails(t *testing.T) {
	t.Cleanup(func() { Stop(); ClearLogs() })
	key, err := GenerateNodeKey()
	if err != nil {
		t.Fatal(err)
	}
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
	const secret = "secret-room-name"
	token := base64.StdEncoding.EncodeToString([]byte("otp:\n  dht:\n    key: k\n    interval: " + secret))
	if err := Start(fd, token, key, "10.1.0.2", "test"); err == nil {
		Stop()
		t.Fatal("Start accepted a token that is not valid YAML")
	}
	if !strings.Contains(RecentLogs(), "ERROR") {
		t.Errorf("logs do not hold the raw error:\n%s", RecentLogs())
	}
	if got := rememberedDetail(); got != "" {
		t.Errorf("error details = %q for a config failure, want none", got)
	}
	if strings.Contains(Snapshot(), "secret") || strings.Contains(Snapshot(), `"detail"`) {
		t.Errorf("snapshot exposes the config error: %s", Snapshot())
	}
}

func TestStartWhileRunningClosesDescriptor(t *testing.T) {
	mu.Lock()
	cancelFn = func() {}
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		cancelFn = nil
		mu.Unlock()
	})

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

	if err := Start(fd, "some-token", make([]byte, 32), "10.1.0.2", "test"); err == nil {
		t.Fatal("Start succeeded while already running")
	}
	var st syscall.Stat_t
	if err := syscall.Fstat(fd, &st); err != syscall.EBADF {
		syscall.Close(fd)
		t.Errorf("fstat on the rejected descriptor = %v, want EBADF (closed)", err)
	}
}
