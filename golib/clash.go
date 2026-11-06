package edgevpn

import (
	"sync"
	"time"

	"github.com/mudler/edgevpn/pkg/protocol"
)

// addressInUseWindow is how long after the last rejected write the
// snapshot still reports the address as in use. The device that holds the
// address sends its entry again every few seconds, so the warning stays up
// while the clash lasts and clears by itself once it is over.
const addressInUseWindow = 60 * time.Second

// violationRejectedFormat is the format edgevpn's ledger passes to the
// node logger when it drops a write that the writer does not own
// (pkg/blockchain/ledger.go in v0.35.5). Its arguments are the bucket, the
// key, the writer's peer ID and the reason.
const violationRejectedFormat = "ownership violation (rejected): %s/%s from %s: %s"

// rejectedClaimReason is the reason edgevpn gives when another peer tries
// to take an entry this node holds as live. The other reasons are about
// deletions and bad signatures, which say nothing about a shared address.
const rejectedClaimReason = "overwrite of a live entry owned by another peer"

// clock is the time source for recording rejected writes; tests replace it.
var clock = time.Now

var (
	rejectedMu sync.Mutex
	// rejectedMachines maps an address to the last time this node rejected
	// another peer's machines entry for it.
	rejectedMachines = map[string]time.Time{}
)

// rejectedAt returns when another peer's machines entry for addr was last
// rejected, or the zero time.
func rejectedAt(addr string) time.Time {
	rejectedMu.Lock()
	defer rejectedMu.Unlock()
	return rejectedMachines[addr]
}

// recordRejection notes a rejected write from the arguments of a warning
// logged with violationRejectedFormat. Only another peer's claim on a
// machines entry counts. It
// takes its own lock, never mu, because the ledger calls it with the
// ledger's lock held.
func recordRejection(args []interface{}) {
	if len(args) < 4 {
		return
	}
	bucket, ok1 := args[0].(string)
	key, ok2 := args[1].(string)
	reason, ok3 := args[3].(string)
	if !ok1 || !ok2 || !ok3 || bucket != protocol.MachinesLedgerKey || reason != rejectedClaimReason {
		return
	}
	rejectedMu.Lock()
	rejectedMachines[key] = clock()
	rejectedMu.Unlock()
}

// forgetRejectedEntries clears what recordRejection has recorded.
func forgetRejectedEntries() {
	rejectedMu.Lock()
	rejectedMachines = map[string]time.Time{}
	rejectedMu.Unlock()
}
