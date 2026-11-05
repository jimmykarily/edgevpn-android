package edgevpn

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/node"
	"github.com/mudler/edgevpn/pkg/protocol"
	"github.com/mudler/edgevpn/pkg/types"
)

const nameAnnounceInterval = 10 * time.Second

// SetName changes the name this phone is announced under. While the VPN
// runs, the next announcement tick publishes it.
func SetName(name string) {
	mu.Lock()
	defer mu.Unlock()
	selfName = name
}

// nameUpdate returns the phone's machines entry carrying name, and whether it
// must be written. edgevpn creates the entry (with the device hostname) and
// only replaces it when it is missing or belongs to another peer, so an entry
// of this peer that already has the name is left alone and an entry that does
// not exist yet or belongs to someone else is never touched.
func nameUpdate(existing types.Machine, found bool, selfID, name string) (types.Machine, bool) {
	if !found || name == "" || existing.PeerID != selfID || existing.Hostname == name {
		return existing, false
	}
	existing.Hostname = name
	return existing, true
}

// markHostReady records that edgevpn has created n's host, so the host can
// be read safely. It reports false when n is not the current session.
func markHostReady(n *node.Node) bool {
	mu.Lock()
	defer mu.Unlock()
	if currentNode != n {
		return false
	}
	hostReady = true
	return true
}

// hostReadyService is registered as a network service. edgevpn runs it after
// the host exists, on the goroutine that ran n.Start; it marks the host ready
// and starts the name announcement. When n's session was stopped before the
// host was ready, Stop could not close the host, so the service closes it and
// returns an error that keeps edgevpn from starting the services after it.
// It returns at once.
func hostReadyService(ctx context.Context, _ node.Config, n *node.Node, ledger *blockchain.Ledger) error {
	if !markHostReady(n) {
		closeNodeHost(n)
		return errSessionStopped
	}
	announceName(ctx, n, ledger)
	return nil
}

var errSessionStopped = errors.New("the VPN was turned off before the node started")

// announceName keeps the phone's entry in the machines bucket named with the
// chosen name until ctx is done. The host of n must be ready.
func announceName(ctx context.Context, n *node.Node, ledger *blockchain.Ledger) {
	ledger.Announce(ctx, nameAnnounceInterval, func() {
		selfID := n.Host().ID().String()
		mu.Lock()
		address, name := selfAddress, selfName
		mu.Unlock()
		if address == "" {
			return
		}
		data, found := ledger.GetKey(protocol.MachinesLedgerKey, address)
		var machine types.Machine
		if found {
			if err := data.Unmarshal(&machine); err != nil {
				emit("WARN ", fmt.Sprintf("cannot read the machines entry of %s: %v", address, err))
				return
			}
		}
		if updated, ok := nameUpdate(machine, found, selfID, name); ok {
			ledger.Add(protocol.MachinesLedgerKey, map[string]interface{}{address: updated})
		}
	})
}
