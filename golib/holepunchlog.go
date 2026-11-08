package edgevpn

import (
	"fmt"
	"strings"

	"github.com/libp2p/go-libp2p/p2p/protocol/holepunch"
)

// holePunchLogger is the event tracer of libp2p's hole punching service
// (DCUtR). It logs each step at info level, so the log shows whether a
// direct connection came from a hole punch and whether it worked. libp2p's
// own hole punching log does not reach the app's log.
type holePunchLogger struct{}

func (holePunchLogger) Trace(e *holepunch.Event) {
	emit("INFO ", describeHolePunch(e))
}

// describeHolePunch renders one hole punching event as a log line. Before
// punching, libp2p tries a plain direct dial ("direct dial"); a punch then
// has a start, one or more attempts and an end.
func describeHolePunch(e *holepunch.Event) string {
	prefix := "hole punch with " + e.Remote.String() + ": "
	switch evt := e.Evt.(type) {
	case *holepunch.DirectDialEvt:
		return prefix + outcome("direct dial ", evt.Success, evt.EllapsedTime.String(), evt.Error)
	case *holepunch.StartHolePunchEvt:
		return prefix + fmt.Sprintf("started, round trip %s, remote addresses [%s]", evt.RTT, strings.Join(evt.RemoteAddrs, " "))
	case *holepunch.HolePunchAttemptEvt:
		return prefix + fmt.Sprintf("attempt %d", evt.Attempt)
	case *holepunch.EndHolePunchEvt:
		return prefix + outcome("", evt.Success, evt.EllapsedTime.String(), evt.Error)
	case *holepunch.ProtocolErrorEvt:
		return prefix + "protocol error: " + evt.Error
	}
	return prefix + e.Type
}

func outcome(what string, success bool, took, err string) string {
	if success {
		return what + "succeeded in " + took
	}
	return what + "failed after " + took + ": " + err
}
