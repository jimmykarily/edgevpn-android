package edgevpn

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/net/swarm"
	"github.com/mudler/edgevpn/pkg/node"
	"github.com/mudler/edgevpn/pkg/protocol"
	"github.com/mudler/edgevpn/pkg/types"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"github.com/wlynxg/anet"
)

// staleConns returns the indexes of conns whose address carries an IP that is
// not in localIPs. Addresses without an IP component (such as relay circuits)
// are never reported.
func staleConns(localIPs []net.IP, conns []ma.Multiaddr) []int {
	var stale []int
	for i, c := range conns {
		ip, err := manet.ToIP(c)
		if err != nil {
			continue
		}
		found := false
		for _, l := range localIPs {
			if l.Equal(ip) {
				found = true
				break
			}
		}
		if !found {
			stale = append(stale, i)
		}
	}
	return stale
}

// closeStaleConns closes every connection of h whose local address no longer
// exists on any interface and returns how many it closed.
func closeStaleConns(h host.Host) int {
	var local []net.IP
	addrs, err := anet.InterfaceAddrs()
	if err != nil {
		emit("WARN ", fmt.Sprintf("anet.InterfaceAddrs failed: %v; keeping connections", err))
		return 0
	}
	for _, a := range addrs {
		if ipn, ok := a.(*net.IPNet); ok {
			local = append(local, ipn.IP)
		}
	}
	conns := h.Network().Conns()
	locals := make([]ma.Multiaddr, len(conns))
	for i, c := range conns {
		locals[i] = c.LocalMultiaddr()
	}
	stale := staleConns(local, locals)
	for _, i := range stale {
		_ = conns[i].Close()
	}
	return len(stale)
}

const (
	// netChangeDelay is how long the network must stay quiet after a
	// NetworkChanged call before the work runs.
	netChangeDelay = 500 * time.Millisecond
	// redialTimeout bounds the dial of each peer after a network change.
	redialTimeout = 10 * time.Second
)

// netChangeDebouncer merges bursts of network reports: work runs delay after
// the last trigger, with the last value. A trigger while work runs gives one
// more run after it ends. Every method is safe from any goroutine.
type netChangeDebouncer struct {
	delay time.Duration
	work  func(onLocal bool)

	mu      sync.Mutex
	value   bool
	timer   *time.Timer
	gen     int // invalidates timers that fire after a newer trigger or stop
	running bool
	again   bool
}

func newNetChangeDebouncer(delay time.Duration, work func(onLocal bool)) *netChangeDebouncer {
	return &netChangeDebouncer{delay: delay, work: work}
}

// trigger records onLocal and schedules a run.
func (d *netChangeDebouncer) trigger(onLocal bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.value = onLocal
	if d.running {
		d.again = true
		return
	}
	d.scheduleLocked()
}

// stop drops the pending run, if any. A run in progress finishes, without a
// further run. Later triggers schedule runs as usual.
func (d *netChangeDebouncer) stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.gen++
	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}
	d.again = false
}

// scheduleLocked (re)starts the timer. The caller holds d.mu.
func (d *netChangeDebouncer) scheduleLocked() {
	d.gen++
	if d.timer != nil {
		d.timer.Stop()
	}
	gen := d.gen
	d.timer = time.AfterFunc(d.delay, func() { d.fire(gen) })
}

func (d *netChangeDebouncer) fire(gen int) {
	d.mu.Lock()
	if gen != d.gen {
		d.mu.Unlock()
		return
	}
	d.timer = nil
	d.running = true
	v := d.value
	d.mu.Unlock()

	d.work(v)

	d.mu.Lock()
	defer d.mu.Unlock()
	d.running = false
	if d.again {
		d.again = false
		d.scheduleLocked()
	}
}

// netChange is the process's debouncer for NetworkChanged. Tests replace it.
var netChange = newNetChangeDebouncer(netChangeDelay, runNetChange)

// NetworkChanged tells the VPN that the phone moved to another network.
// onLocalNetwork is true on Wi-Fi or Ethernet, where mDNS can find peers.
// The value goes to mDNS at once; the rest runs after the reports settle:
// connections on addresses that no longer exist are closed, mDNS restarts
// and known peers are dialled again, past the dial backoff. Without a
// session it does nothing.
func NetworkChanged(onLocalNetwork bool) {
	mu.Lock()
	n, r, d := currentNode, currentMDNS, netChange
	if n != nil {
		netChangeNode = n
	}
	mu.Unlock()
	if n == nil {
		return
	}
	if r != nil {
		r.setLocal(onLocalNetwork)
	}
	d.trigger(onLocalNetwork)
}

// runNetChange is the debounced work. It runs the network steps only when
// the session of the last NetworkChanged call is still current, has a
// context and its host is ready.
func runNetChange(onLocal bool) {
	mu.Lock()
	n := netChangeNode
	ok := n != nil && currentNode == n && hostReady
	ctx, r := sessionCtx, currentMDNS
	mu.Unlock()
	if !ok || ctx == nil {
		return
	}
	netChangeSteps(ctx, n, hostOf(n), r, onLocal)
}

// netChangeSteps does the network work of one run. Tests replace it.
var netChangeSteps = networkChangeSteps

// networkChangeSteps closes the connections on gone addresses, restarts
// mDNS and redials the bootstrap peers and every machine of the network,
// then logs one line with the outcome. ctx ends with the session.
func networkChangeSteps(ctx context.Context, n *node.Node, h host.Host, r *mdnsRunner, onLocal bool) {
	if h == nil {
		return
	}
	closed := closeStaleConns(h)
	if r != nil {
		r.restart()
	}
	targets := redialTargets(h.ID(), bootstrapTargets(), machinePeerIDs(n))
	dialled, connected := redial(ctx, h, targets)
	emit("INFO ", fmt.Sprintf("network changed: local network: %t, closed %d connections, dialled %d peers, %d connected",
		onLocal, closed, dialled, connected))
}

// bootstrapTargets returns the bootstrap peers the node was configured with.
func bootstrapTargets() []string {
	if p := bootstrapPeers(); p != nil {
		return p
	}
	out := make([]string, 0, len(dht.DefaultBootstrapPeers))
	for _, a := range dht.DefaultBootstrapPeers {
		out = append(out, a.String())
	}
	return out
}

// machinePeerIDs returns the peer IDs of the machines bucket of n's ledger.
func machinePeerIDs(n *node.Node) []string {
	if n == nil {
		return nil
	}
	ledger, err := n.Ledger()
	if err != nil || ledger == nil {
		return nil
	}
	var ids []string
	for _, v := range ledger.CurrentData()[protocol.MachinesLedgerKey] {
		var m types.Machine
		if v.Unmarshal(&m) == nil {
			ids = append(ids, m.PeerID)
		}
	}
	return ids
}

// redialTargets merges the bootstrap addresses and the machine peer IDs into
// one list per peer, in first-seen order, without selfID. Entries that do not
// parse are dropped. Machines carry no address: the host finds one in its
// peerstore or through routing.
func redialTargets(selfID peer.ID, bootstrap []string, machinePeerIDs []string) []peer.AddrInfo {
	var out []peer.AddrInfo
	index := map[peer.ID]int{}
	add := func(info peer.AddrInfo) {
		if info.ID == selfID || info.ID == "" {
			return
		}
		if i, seen := index[info.ID]; seen {
			out[i].Addrs = append(out[i].Addrs, info.Addrs...)
			return
		}
		index[info.ID] = len(out)
		out = append(out, info)
	}
	for _, s := range bootstrap {
		if info, err := peer.AddrInfoFromString(s); err == nil {
			add(*info)
		}
	}
	for _, s := range machinePeerIDs {
		if id, err := peer.Decode(s); err == nil {
			add(peer.AddrInfo{ID: id})
		}
	}
	return out
}

// redial dials every target that is not connected, in parallel, each with
// its own timeout, and waits for all of them. It clears each target's dial
// backoff first, because failures on the old network would otherwise block
// the dial; relay addresses stay in play, since on mobile data a relay may be
// the only path. It returns how many it dialled and how many connected.
func redial(ctx context.Context, h host.Host, targets []peer.AddrInfo) (dialled, connected int) {
	var wg sync.WaitGroup
	var countMu sync.Mutex
	for _, t := range targets {
		if h.Network().Connectedness(t.ID) == network.Connected {
			continue
		}
		dialled++
		wg.Add(1)
		go func(t peer.AddrInfo) {
			defer wg.Done()
			if sw, ok := h.Network().(*swarm.Swarm); ok {
				sw.Backoff().Clear(t.ID)
			}
			dctx, cancel := context.WithTimeout(ctx, redialTimeout)
			defer cancel()
			if h.Connect(dctx, t) == nil {
				countMu.Lock()
				connected++
				countMu.Unlock()
			}
		}(t)
	}
	wg.Wait()
	return dialled, connected
}
