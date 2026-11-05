package edgevpn

import (
	"sync"
	"time"

	"github.com/mudler/edgevpn/pkg/node"
)

// Check-in intervals, equal to the edgevpn CLI defaults
// (cmd/util.go: aliveness-healthcheck-interval, -scrub-interval, -max-interval).
const aliveAnnounce, aliveScrub, aliveMax = 120 * time.Second, 600 * time.Second, 900 * time.Second

// State shared with the status code. All of it is guarded by mu.
var (
	currentNode *node.Node
	startedAt   time.Time
	lastFailure string
	failedAt    time.Time
	selfAddress string
	selfName    string
	// hostReady is true once edgevpn has created the current session's
	// libp2p host. Before that, Node.Host is written by the node goroutine
	// without synchronization and must not be read.
	hostReady bool
)

// StatusListener is told when the VPN stops without Stop being called.
type StatusListener interface {
	OnStopped(reason string)
}

var (
	listenerMu     sync.Mutex
	statusListener StatusListener
)

// SetStatusListener registers the listener for unexpected stops, or removes
// it when l is nil.
func SetStatusListener(l StatusListener) {
	listenerMu.Lock()
	defer listenerMu.Unlock()
	statusListener = l
}

func notifyStopped(reason string) {
	listenerMu.Lock()
	l := statusListener
	listenerMu.Unlock()
	if l != nil {
		l.OnStopped(reason)
	}
}

// computeState maps the facts about the VPN to one of "off", "looking",
// "connected" and "problem".
func computeState(running bool, failure string, reachable int) string {
	switch {
	case failure != "":
		return "problem"
	case !running:
		return "off"
	case reachable == 0:
		return "looking"
	default:
		return "connected"
	}
}

// recordFailure remembers why the VPN could not run. The caller must hold mu.
func recordFailure(reason string) {
	lastFailure = reason
	failedAt = time.Now()
}

// ReportStartFailure lets the app record a failure that happens before Start
// is called, such as the VPN interface not being created.
func ReportStartFailure(reason string) {
	mu.Lock()
	defer mu.Unlock()
	recordFailure(reason)
}
