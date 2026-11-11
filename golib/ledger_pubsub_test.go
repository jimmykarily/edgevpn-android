package edgevpn

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	relayclient "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/client"
	relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	"github.com/mudler/edgevpn/pkg/hub"
	ma "github.com/multiformats/go-multiaddr"
)

// Ledger updates travel over edgevpn's message hub, which runs on
// go-libp2p-pubsub. These tests check the two pubsub behaviours the app
// depends on: delivery over a relayed connection, which is often all a phone
// on mobile data has, and delivery after a pubsub stream is reset while the
// connection stays up, which happens when a phone changes networks.

func newPubsubTestHost(t *testing.T, opts ...libp2p.Option) host.Host {
	t.Helper()
	h, err := libp2p.New(append([]libp2p.Option{
		libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"),
	}, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { h.Close() })
	return h
}

// hubPair starts a hub on each host, both in the same room, and has a
// publish a message every 100 ms. The returned function reports whether b
// received one of them after the given time.
func hubPair(t *testing.T, a, b host.Host) (heardSince func(time.Time) bool) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	hubA := hub.NewHub("ledger-pubsub-test", 1<<20, 43, 3600, false)
	hubB := hub.NewHub("ledger-pubsub-test", 1<<20, 43, 3600, false)
	go hubA.Start(ctx, a)
	go hubB.Start(ctx, b)

	var last atomic.Int64
	go func() {
		for {
			select {
			case <-hubB.Messages:
				last.Store(time.Now().UnixNano())
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		for i := 0; ctx.Err() == nil; i++ {
			hubA.PublishMessage(hub.NewMessage(fmt.Sprint(i)))
			time.Sleep(100 * time.Millisecond)
		}
	}()
	return func(since time.Time) bool { return last.Load() > since.UnixNano() }
}

func waitUntil(timeout time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return cond()
}

func TestLedgerMessagesFlowOverARelayedConnection(t *testing.T) {
	relay := newPubsubTestHost(t, libp2p.DisableRelay(), libp2p.ForceReachabilityPublic())
	if _, err := relayv2.New(relay); err != nil {
		t.Fatal(err)
	}
	relayInfo := peer.AddrInfo{ID: relay.ID(), Addrs: relay.Addrs()}

	a := newPubsubTestHost(t, libp2p.ForceReachabilityPrivate())
	b := newPubsubTestHost(t, libp2p.ForceReachabilityPrivate())
	ctx := context.Background()
	if err := a.Connect(ctx, relayInfo); err != nil {
		t.Fatal(err)
	}
	if _, err := relayclient.Reserve(ctx, a, relayInfo); err != nil {
		t.Fatal(err)
	}
	if err := b.Connect(ctx, relayInfo); err != nil {
		t.Fatal(err)
	}
	circuit := ma.StringCast("/p2p/" + relay.ID().String() + "/p2p-circuit")
	var viaRelay []ma.Multiaddr
	for _, addr := range relay.Addrs() {
		viaRelay = append(viaRelay, addr.Encapsulate(circuit))
	}
	if err := b.Connect(ctx, peer.AddrInfo{ID: a.ID(), Addrs: viaRelay}); err != nil {
		t.Fatal(err)
	}
	if got := b.Network().Connectedness(a.ID()); got != network.Limited {
		t.Fatalf("b and a should share only a relayed connection, connectedness is %s", got)
	}

	heardSince := hubPair(t, a, b)
	start := time.Now()
	if !waitUntil(10*time.Second, func() bool { return heardSince(start) }) {
		t.Fatal("b never received a message from a over the relayed connection")
	}
}

func TestLedgerMessagesFlowAfterAPubsubStreamIsReset(t *testing.T) {
	a, b := newPubsubTestHost(t), newPubsubTestHost(t)
	if err := a.Connect(context.Background(), peer.AddrInfo{ID: b.ID(), Addrs: b.Addrs()}); err != nil {
		t.Fatal(err)
	}

	heardSince := hubPair(t, a, b)
	start := time.Now()
	if !waitUntil(10*time.Second, func() bool { return heardSince(start) }) {
		t.Fatal("b never received a message from a before the reset")
	}

	reset := 0
	for _, c := range a.Network().ConnsToPeer(b.ID()) {
		for _, s := range c.GetStreams() {
			if s.Stat().Direction == network.DirOutbound && strings.HasPrefix(string(s.Protocol()), "/meshsub/") {
				s.Reset()
				reset++
			}
		}
	}
	if reset == 0 {
		t.Fatal("found no outbound pubsub stream from a to b to reset")
	}
	resetAt := time.Now()
	// Messages already queued before the reset may still arrive in the first
	// moments after it, so only those received a second later count.
	if !waitUntil(6*time.Second, func() bool { return heardSince(resetAt.Add(time.Second)) }) {
		t.Fatalf("b received nothing from a in the 5 s after a's pubsub stream to b was reset; still connected: %v",
			a.Network().Connectedness(b.ID()) == network.Connected)
	}
}
