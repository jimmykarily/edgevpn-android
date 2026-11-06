package edgevpn

import (
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/p2p/muxer/yamux"
)

// muxerTransport returns a yamux transport with a shorter keepalive than the
// libp2p default. A connection that drops silently (Wi-Fi lost, handover to
// mobile data) is noticed within 15 seconds: one 10 second keepalive interval
// plus the 5 second write timeout. Step 8 of the reliability epic (issue #1)
// measures the battery cost of this interval.
//
// The result is a copy; yamux.DefaultTransport is shared and left untouched.
func muxerTransport() *yamux.Transport {
	t := *yamux.DefaultTransport
	cfg := t.Config()
	cfg.EnableKeepAlive = true
	cfg.KeepAliveInterval = 10 * time.Second
	cfg.ConnectionWriteTimeout = 5 * time.Second
	return &t
}

// muxerOption replaces libp2p's default muxers. Yamux is the only default
// muxer, so nothing else is lost.
func muxerOption() libp2p.Option {
	return libp2p.Muxer(yamux.ID, muxerTransport())
}
