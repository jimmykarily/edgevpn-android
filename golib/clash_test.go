package edgevpn

import (
	"io"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/hub"
	"github.com/mudler/edgevpn/pkg/protocol"
	"github.com/mudler/edgevpn/pkg/types"
)

func fixClock(t *testing.T, at time.Time) {
	t.Helper()
	clock = func() time.Time { return at }
	t.Cleanup(func() {
		clock = time.Now
		forgetRejectedEntries()
	})
}

func TestLoggerRecordsRejectedMachineEntriesByAddress(t *testing.T) {
	at := time.Unix(1793890000, 0)
	fixClock(t, at)
	l := &bufferLogger{}

	l.Warnf(violationRejectedFormat, protocol.MachinesLedgerKey, "10.1.0.4", "12D3KooWPeer", rejectedClaimReason)
	if got := rejectedAt("10.1.0.4"); !got.Equal(at) {
		t.Fatalf("rejectedAt(10.1.0.4) = %v, want %v", got, at)
	}

	l.Warnf("ownership violation (observe, accepting): %s/%s from %s: %s", protocol.MachinesLedgerKey, "10.1.0.5", "p", "r")
	l.Warnf(violationRejectedFormat, protocol.ServicesLedgerKey, "10.1.0.6", "p", rejectedClaimReason)
	l.Warnf(violationRejectedFormat, protocol.MachinesLedgerKey, "10.1.0.7", "p", "tombstone by non-owner of a live entry")
	// Arguments of other types are ignored, not a panic.
	l.Warnf(violationRejectedFormat, []interface{}{1, 2}...)
	for _, addr := range []string{"10.1.0.5", "10.1.0.6", "10.1.0.7"} {
		if got := rejectedAt(addr); !got.IsZero() {
			t.Errorf("rejectedAt(%s) = %v, want none", addr, got)
		}
	}
}

func TestStopForgetsRejectedEntries(t *testing.T) {
	fixClock(t, time.Unix(1793890000, 0))
	(&bufferLogger{}).Warnf(violationRejectedFormat, protocol.MachinesLedgerKey, "10.1.0.4", "p", rejectedClaimReason)
	Stop()
	if got := rejectedAt("10.1.0.4"); !got.IsZero() {
		t.Fatalf("rejectedAt after Stop = %v", got)
	}
}

func testSigner(t *testing.T) blockchain.Signer {
	t.Helper()
	priv, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 0)
	if err != nil {
		t.Fatal(err)
	}
	s, err := blockchain.NewSigner(priv)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// twoLedgers returns the ledgers of a phone and a peer with ownership
// enforced, as edgevpn runs them, where the phone holds a live entry at
// 10.1.0.4. The peer's broadcasts go to out, and the phone's ledger logs
// through bufferLogger.
func twoLedgers(t *testing.T) (phone *blockchain.Ledger, phoneSigner blockchain.Signer, peer *blockchain.Ledger, peerSigner blockchain.Signer, out *capture) {
	t.Helper()
	ttl := time.Minute
	phoneSigner, peerSigner = testSigner(t), testSigner(t)
	phone = blockchain.New(io.Discard, &blockchain.MemoryStore{},
		blockchain.WithSigner(phoneSigner),
		blockchain.WithEnforcedOwnership(blockchain.DefaultRegistry(ttl), ttl),
		blockchain.WithViolationLogger((&bufferLogger{}).Warnf),
	)
	out = &capture{}
	peer = blockchain.New(out, &blockchain.MemoryStore{},
		blockchain.WithSigner(peerSigner),
		blockchain.WithEnforcedOwnership(blockchain.DefaultRegistry(ttl), ttl),
	)
	phone.Add(protocol.HealthCheckKey, map[string]interface{}{phoneSigner.ID(): time.Now().UTC().Format(time.RFC3339)})
	phone.Add(protocol.MachinesLedgerKey, map[string]interface{}{
		"10.1.0.4": types.Machine{PeerID: phoneSigner.ID(), Address: "10.1.0.4", Hostname: "phone"},
	})
	return
}

// deliver hands the peer's last broadcast to the phone's ledger, as gossip would.
func deliver(t *testing.T, out *capture, phone *blockchain.Ledger) {
	t.Helper()
	if out.last == nil {
		t.Fatal("the peer's ledger broadcast nothing")
	}
	if err := phone.Update(nil, hub.NewMessage(string(out.last)), nil); err != nil {
		t.Fatal(err)
	}
}

// capture keeps the last block a ledger broadcasts.
type capture struct{ last []byte }

func (c *capture) Write(p []byte) (int, error) {
	c.last = append([]byte(nil), p...)
	return len(p), nil
}

// Two ledgers with ownership enforced, as edgevpn runs them, both claim
// 10.1.0.4. The phone's ledger rejects the peer's entry, and the warning it
// logs for that is recorded as a rejected write at the address. A change of
// the warning in a new edgevpn release fails here.
func TestLedgerRejectionOfAnotherPeersEntryIsRecorded(t *testing.T) {
	fixClock(t, time.Unix(1793890000, 0))
	phone, phoneSigner, peer, peerSigner, out := twoLedgers(t)
	peer.Add(protocol.MachinesLedgerKey, map[string]interface{}{
		"10.1.0.4": types.Machine{PeerID: peerSigner.ID(), Address: "10.1.0.4", Hostname: "testpeer"},
	})
	deliver(t, out, phone)

	if rejectedAt("10.1.0.4").IsZero() {
		t.Fatal("the phone's ledger rejected the peer's entry, but no rejected write was recorded")
	}
	var m types.Machine
	v, _ := phone.GetKey(protocol.MachinesLedgerKey, "10.1.0.4")
	if err := v.Unmarshal(&m); err != nil || m.PeerID != phoneSigner.ID() {
		t.Fatalf("the phone's entry = %+v (%v), want its own", m, err)
	}
}

// A device that comes back after the other side dropped its entry receives
// that side's deletion of the entry and rejects it. That is no clash.
func TestLedgerRejectionOfADeletionIsNotRecorded(t *testing.T) {
	fixClock(t, time.Unix(1793890000, 0))
	phone, _, peer, peerSigner, out := twoLedgers(t)
	peer.Add(protocol.MachinesLedgerKey, map[string]interface{}{
		"10.1.0.4": types.Machine{PeerID: peerSigner.ID(), Address: "10.1.0.4", Hostname: "testpeer"},
	})
	peer.Delete(protocol.MachinesLedgerKey, "10.1.0.4")
	deliver(t, out, phone)

	if got := rejectedAt("10.1.0.4"); !got.IsZero() {
		t.Fatalf("a rejected deletion was recorded as a clash at %v", got)
	}
}
