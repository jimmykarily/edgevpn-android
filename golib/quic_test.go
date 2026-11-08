package edgevpn

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type countingProtector struct{ calls atomic.Int32 }

func (p *countingProtector) Protect(int) bool { p.calls.Add(1); return true }

var _ = Describe("The QUIC transport", func() {
	var protector *countingProtector

	BeforeEach(func() {
		protector = &countingProtector{}
		SetSocketProtector(protector)
		DeferCleanup(func() { SetSocketProtector(nil) })
	})

	// newAppHost builds a host with the app's transports: TCP and QUIC with
	// protected UDP sockets.
	newAppHost := func(listen ...string) host.Host {
		opts := []libp2p.Option{libp2p.NoTransports, libp2p.Transport(tcp.NewTCPTransport), libp2p.ListenAddrStrings(listen...)}
		h, err := libp2p.New(append(opts, quicTransportOptions()...)...)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(h.Close)
		return h
	}

	// A desktop running the edgevpn CLI uses libp2p's default transports.
	newDesktop := func() host.Host {
		h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/udp/0/quic-v1"))
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(h.Close)
		return h
	}

	connect := func(from, to host.Host) error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return from.Connect(ctx, peer.AddrInfo{ID: to.ID(), Addrs: to.Addrs()})
	}

	It("connects to a default libp2p host in both directions", func() {
		app, desktop := newAppHost("/ip4/127.0.0.1/udp/0/quic-v1"), newDesktop()
		Expect(connect(app, desktop)).To(Succeed())
		Expect(connect(desktop, app)).To(Succeed())
		Expect(app.Network().ConnsToPeer(desktop.ID())[0].RemoteMultiaddr().String()).To(ContainSubstring("/quic-v1"))
		Expect(protector.calls.Load()).To(BeNumerically(">", 0), "no UDP socket went through protect")
	})

	It("dials over QUIC without a UDP listener", func() {
		app, desktop := newAppHost("/ip4/127.0.0.1/tcp/0"), newDesktop()
		Expect(connect(app, desktop)).To(Succeed())
		Expect(app.Network().ConnsToPeer(desktop.ID())[0].RemoteMultiaddr().String()).To(ContainSubstring("/quic-v1"))
		Expect(protector.calls.Load()).To(BeNumerically(">", 0), "no UDP socket went through protect")
	})
})
