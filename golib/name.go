package edgevpn

import (
	"context"
	"time"

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

// announceName keeps the phone's entry in the machines bucket named with the
// chosen name until ctx is done.
func announceName(ctx context.Context, n *node.Node) error {
	ledger, err := n.Ledger()
	if err != nil {
		return err
	}
	ledger.Announce(ctx, nameAnnounceInterval, func() {
		host := n.Host()
		if host == nil {
			return
		}
		selfID := host.ID().String()
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
				return
			}
		}
		if updated, ok := nameUpdate(machine, found, selfID, name); ok {
			ledger.Add(protocol.MachinesLedgerKey, map[string]interface{}{address: updated})
		}
	})
	return nil
}
