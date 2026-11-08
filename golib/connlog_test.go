package edgevpn

import (
	"context"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/protocol"
	ma "github.com/multiformats/go-multiaddr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const relayID = "12D3KooWJBY2ABCjkzzfDmA8w8svvZ58tc3AJoYvwQS2xBCcJpZ6"

func connLines(logs string) []string {
	var out []string
	for _, l := range strings.Split(logs, "\n") {
		if i := strings.Index(l, "connections to network peers:"); i >= 0 {
			out = append(out, l[i:])
		}
	}
	return out
}

func newQUICHost() host.Host {
	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/udp/0/quic-v1"))
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(h.Close)
	return h
}

var _ = Describe("The connection log", func() {
	Describe("describeConns", func() {
		It("names the address, the direction and a relay for each connection", func() {
			Expect(describeConns(map[string][]peerConn{
				"12D3KooWB": {
					{Remote: ma.StringCast("/ip4/192.168.21.141/udp/41002/quic-v1"), Dir: network.DirOutbound},
				},
				"12D3KooWA": {
					{Remote: ma.StringCast("/ip6/2001:db8::1/tcp/4001"), Dir: network.DirInbound},
					{Remote: ma.StringCast("/ip4/203.0.113.7/tcp/4001/p2p/" + relayID + "/p2p-circuit"), Dir: network.DirOutbound, Limited: true},
				},
			})).To(Equal("connections to network peers: " +
				"12D3KooWA in /ip6/2001:db8::1/tcp/4001, " +
				"12D3KooWA out /ip4/203.0.113.7/tcp/4001/p2p/" + relayID + "/p2p-circuit (relayed); " +
				"12D3KooWB out /ip4/192.168.21.141/udp/41002/quic-v1"))
		})

		It("says none when no peer has a connection", func() {
			Expect(describeConns(nil)).To(Equal("connections to network peers: none"))
			Expect(describeConns(map[string][]peerConn{"12D3KooWA": nil})).To(Equal("connections to network peers: none"))
		})
	})

	Describe("connLogger", func() {
		BeforeEach(func() {
			SetMinLogLevel("info")
			DeferCleanup(SetMinLogLevel, "warn")
			ClearLogs()
		})

		It("logs a list only when it differs from the last one", func() {
			quic := map[string][]peerConn{"12D3KooWA": {{Remote: ma.StringCast("/ip4/192.168.21.141/udp/41002/quic-v1"), Dir: network.DirOutbound}}}
			var l connLogger
			l.report(nil)
			l.report(quic)
			l.report(quic)
			l.report(nil)

			Expect(connLines(RecentLogs())).To(Equal([]string{
				"connections to network peers: none",
				"connections to network peers: 12D3KooWA out /ip4/192.168.21.141/udp/41002/quic-v1",
				"connections to network peers: none",
			}))
		})
	})

	Describe("peerConns", func() {
		It("reads the host's connections to the given peers", func() {
			a, b := newQUICHost(), newQUICHost()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			Expect(a.Connect(ctx, peer.AddrInfo{ID: b.ID(), Addrs: b.Addrs()})).To(Succeed())

			// A network peer without a connection, and an ID that does not decode.
			stranger := newQUICHost()
			got := peerConns(a, []string{b.ID().String(), stranger.ID().String(), "not-a-peer-id"})

			Expect(got).To(HaveLen(1))
			Expect(got[b.ID().String()]).To(ConsistOf(SatisfyAll(
				HaveField("Dir", network.DirOutbound),
				HaveField("Limited", false),
				HaveField("Remote", WithTransform(ma.Multiaddr.String, ContainSubstring("/quic-v1"))),
			)))
		})
	})

	Describe("networkPeerIDs", func() {
		It("merges the machine entries and the health checks, without this node", func() {
			data := map[string]map[string]blockchain.Data{
				protocol.MachinesLedgerKey: {
					"10.1.0.4":  blockchain.Data(`{"PeerID":"12D3KooWSelf"}`),
					"10.1.0.10": blockchain.Data(`{"PeerID":"12D3KooWDesktop"}`),
					"10.1.0.11": blockchain.Data(`not json`),
				},
				// A node without a VPN address has a health check and no machine entry.
				protocol.HealthCheckKey: {
					"12D3KooWSelf":    blockchain.Data(`"2026-11-08T17:00:00Z"`),
					"12D3KooWDesktop": blockchain.Data(`"2026-11-08T17:00:00Z"`),
					"12D3KooWApiNode": blockchain.Data(`"2026-11-08T17:00:00Z"`),
				},
			}
			Expect(networkPeerIDs(data, "12D3KooWSelf")).To(Equal([]string{"12D3KooWApiNode", "12D3KooWDesktop"}))
		})
	})
})
