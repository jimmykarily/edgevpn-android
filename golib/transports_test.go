package edgevpn

import (
	"github.com/libp2p/go-libp2p"
	ma "github.com/multiformats/go-multiaddr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("hostOptions", func() {
	BeforeEach(func() {
		SetSocketProtector(&countingProtector{})
		DeferCleanup(func() { SetSocketProtector(nil) })
	})

	It("listens on TCP and QUIC, over IPv4 and IPv6", func() {
		h, err := libp2p.New(hostOptions("10.1.0.4")...)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(h.Close)

		var kinds []string
		for _, a := range h.Network().ListenAddresses() {
			family, rest := ma.SplitFirst(a)
			if code := family.Protocol().Code; code != ma.P_IP4 && code != ma.P_IP6 {
				continue // the relay transport's /p2p-circuit
			}
			transport := "tcp"
			if _, err := rest.ValueForProtocol(ma.P_QUIC_V1); err == nil {
				transport = "quic-v1"
			}
			kinds = append(kinds, family.Protocol().Name+" "+transport)
		}
		Expect(kinds).To(ConsistOf("ip4 tcp", "ip6 tcp", "ip4 quic-v1", "ip6 quic-v1"))
	})
})
