package edgevpn

import (
	"sync"
	"time"
)

var (
	checkInMu      sync.Mutex
	checkInSeconds int
)

// SetCheckInSeconds sets how often the next Start writes this device's
// heartbeat into the ledger, in seconds; zero or less means the CLI default
// of 120. A peer forgets a device whose heartbeat is older than the peer's
// ownership TTL, so a peer with a TTL shorter than 120 seconds needs a
// shorter interval here. The network tests use one with a 60 second TTL.
func SetCheckInSeconds(seconds int) {
	checkInMu.Lock()
	defer checkInMu.Unlock()
	checkInSeconds = seconds
}

func checkInInterval() time.Duration {
	checkInMu.Lock()
	defer checkInMu.Unlock()
	if checkInSeconds <= 0 {
		return aliveAnnounce
	}
	return time.Duration(checkInSeconds) * time.Second
}
