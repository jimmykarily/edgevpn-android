package edgevpn

import (
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/protocol/holepunch"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("The hole punch log", func() {
	remote, err := peer.Decode(relayID)
	if err != nil {
		panic(err)
	}
	event := func(typ string, evt any) *holepunch.Event {
		return &holepunch.Event{Remote: remote, Type: typ, Evt: evt}
	}

	DescribeTable("describeHolePunch",
		func(e *holepunch.Event, want string) {
			Expect(describeHolePunch(e)).To(Equal(want))
		},
		Entry("a direct dial that worked",
			event(holepunch.DirectDialEvtT, &holepunch.DirectDialEvt{Success: true, EllapsedTime: 120 * time.Millisecond}),
			"hole punch with "+relayID+": direct dial succeeded in 120ms"),
		Entry("a direct dial that failed",
			event(holepunch.DirectDialEvtT, &holepunch.DirectDialEvt{EllapsedTime: 5 * time.Second, Error: "timeout"}),
			"hole punch with "+relayID+": direct dial failed after 5s: timeout"),
		Entry("the start of a hole punch",
			event(holepunch.StartHolePunchEvtT, &holepunch.StartHolePunchEvt{RemoteAddrs: []string{"/ip4/203.0.113.7/udp/4001/quic-v1"}, RTT: 80 * time.Millisecond}),
			"hole punch with "+relayID+": started, round trip 80ms, remote addresses [/ip4/203.0.113.7/udp/4001/quic-v1]"),
		Entry("an attempt",
			event(holepunch.HolePunchAttemptEvtT, &holepunch.HolePunchAttemptEvt{Attempt: 2}),
			"hole punch with "+relayID+": attempt 2"),
		Entry("a hole punch that worked",
			event(holepunch.EndHolePunchEvtT, &holepunch.EndHolePunchEvt{Success: true, EllapsedTime: 300 * time.Millisecond}),
			"hole punch with "+relayID+": succeeded in 300ms"),
		Entry("a hole punch that failed",
			event(holepunch.EndHolePunchEvtT, &holepunch.EndHolePunchEvt{EllapsedTime: 2 * time.Second, Error: "all retries failed"}),
			"hole punch with "+relayID+": failed after 2s: all retries failed"),
		Entry("a protocol error",
			event(holepunch.ProtocolErrorEvtT, &holepunch.ProtocolErrorEvt{Error: "bad message"}),
			"hole punch with "+relayID+": protocol error: bad message"),
		Entry("an event type it does not know",
			event("Something", nil),
			"hole punch with "+relayID+": Something"),
	)

	It("is logged at info level", func() {
		SetMinLogLevel("info")
		DeferCleanup(SetMinLogLevel, "warn")
		ClearLogs()
		holePunchLogger{}.Trace(event(holepunch.HolePunchAttemptEvtT, &holepunch.HolePunchAttemptEvt{Attempt: 1}))
		Expect(RecentLogs()).To(ContainSubstring("INFO  hole punch with " + relayID + ": attempt 1"))
	})

	It("is attached to the host's hole punching", func() {
		// edgevpn applies its own options, with a plain EnableHolePunching,
		// before the app's.
		var cfg libp2p.Config
		Expect(cfg.Apply(libp2p.EnableHolePunching())).To(Succeed())
		Expect(cfg.Apply(hostOptions("10.1.0.4")...)).To(Succeed())
		Expect(cfg.EnableHolePunching).To(BeTrue())
		Expect(cfg.HolePunchingOptions).To(HaveLen(1))
	})
})
