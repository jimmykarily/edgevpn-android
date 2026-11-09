package edgevpn

import (
	"context"
	"fmt"
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
	// failureDetail is the raw error behind lastFailure, for the Problem
	// help sheet. It stays empty for failures whose raw error can quote the
	// network key.
	failureDetail string
	failedAt      time.Time
	selfAddress   string
	selfName      string
	// hostReady is true once edgevpn has created the current session's
	// libp2p host. Before that, Node.Host is written by the node goroutine
	// without synchronization and must not be read.
	hostReady bool
	// sessionCtx is the current session's context; Stop cancels it.
	sessionCtx context.Context
	// netChangeNode is the session of the last NetworkChanged call.
	netChangeNode *node.Node
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
	failureDetail = ""
	failedAt = time.Now()
}

// ReportStartFailure lets the app record a failure that happens before Start
// is called, such as the VPN interface not being created.
func ReportStartFailure(reason string) {
	mu.Lock()
	defer mu.Unlock()
	recordFailure(reason)
}

// The reasons the app shows when the VPN cannot run. The raw errors go to
// the logs only, and errors that can quote the network key do not go
// anywhere (see failWithoutDetail).
const (
	reasonKey     = "The network key could not be used."
	reasonStart   = "The VPN could not start."
	reasonStopped = "The VPN stopped unexpectedly."
)

// fail logs the raw error and remembers the plain reason for the app, and
// the raw error as its details. The caller must hold mu.
func fail(reason, raw string) {
	emit("ERROR", raw)
	recordFailure(reason)
	failureDetail = raw
}

// failWithoutDetail is fail for errors that can quote the network key, such
// as those of reading the key and creating the node from it. The log buffer
// feeds the Logs screen, its "Copy all" button and logcat, so it gets only
// what failed and the Go type of err, never the error text. The caller must
// hold mu.
func failWithoutDetail(reason, what string, err error) {
	emit("ERROR", fmt.Sprintf("%s (%T)", what, err))
	recordFailure(reason)
}
