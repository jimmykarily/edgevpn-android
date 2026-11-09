package edgevpn

import (
	"sync"

	relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	"github.com/mudler/edgevpn/pkg/config"
)

var (
	relayForOthersMu sync.Mutex
	relayForOthers   bool
)

// SetRelayForOthers turns the circuit relay v2 service on or off for the
// next Start. With it on, any libp2p peer may reserve a slot on this phone
// and pass connections through it. go-libp2p starts the service only while
// AutoNAT reports the phone as publicly reachable.
func SetRelayForOthers(on bool) {
	relayForOthersMu.Lock()
	defer relayForOthersMu.Unlock()
	relayForOthers = on
}

// RelayForOthers reports whether the next Start offers the relay service.
func RelayForOthers() bool {
	relayForOthersMu.Lock()
	defer relayForOthersMu.Unlock()
	return relayForOthers
}

// relayServiceConfig returns the relay service settings for Start. edgevpn
// replaces every zero limit with its own defaults, sized for servers (1 GiB
// and 30 minutes per circuit), so each limit is set to go-libp2p's default,
// which bounds a relayed connection to 128 KiB or 2 minutes. That is enough
// for two peers to start a hole punch and not enough to carry their traffic
// on the user's data plan.
func relayServiceConfig() config.RelayService {
	if !RelayForOthers() {
		return config.RelayService{Disabled: true}
	}
	res := relayv2.DefaultResources()
	return config.RelayService{
		Disabled:       false,
		NetworkOnly:    false,
		MaxData:        res.Limit.Data,
		MaxDuration:    res.Limit.Duration,
		MaxCircuits:    res.MaxCircuits,
		ReservationTTL: res.ReservationTTL,
		BufferSize:     res.BufferSize,
	}
}
