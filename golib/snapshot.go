package edgevpn

import (
	"encoding/json"
	"net"
	"sort"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mudler/edgevpn/pkg/node"
	"github.com/mudler/edgevpn/pkg/protocol"
	"github.com/mudler/edgevpn/pkg/types"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"github.com/wlynxg/anet"
)

type selfJSON struct {
	ID      string `json:"id"`
	Address string `json:"address"`
	Name    string `json:"name"`
}

type deviceJSON struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Address     string `json:"address"`
	Reach       string `json:"reach"`
	Remote      string `json:"remote"`
	RemoteAddr  string `json:"remoteAddr"`
	Transport   string `json:"transport"`
	Limited     bool   `json:"limited"`
	OpenedAt    int64  `json:"openedAt"`
	SameNetwork bool   `json:"sameNetwork"`
	LastSeen    int64  `json:"lastSeen,omitempty"`
}

type lastHeardJSON struct {
	At   int64  `json:"at"`
	Name string `json:"name"`
}

type snapshotJSON struct {
	State     string         `json:"state"`
	StartedAt int64          `json:"startedAt"`
	Error     string         `json:"error"`
	Detail    string         `json:"detail,omitempty"`
	FailedAt  int64          `json:"failedAt"`
	Self      selfJSON       `json:"self"`
	Devices   []deviceJSON   `json:"devices"`
	Helpers   int            `json:"helpers"`
	LastHeard *lastHeardJSON `json:"lastHeard,omitempty"`
	// AddressInUse is true while another device claims this phone's address
	// (see addressInUseWindow).
	AddressInUse bool `json:"addressInUse,omitempty"`
	// LocalNetworkClash is true while a Wi-Fi or Ethernet network the phone
	// is on uses addresses in the VPN's range (see SetLocalNetworkClash).
	LocalNetworkClash bool `json:"localNetworkClash,omitempty"`
}

// snapshotInput is everything buildSnapshot needs, gathered by the caller.
type snapshotInput struct {
	State                    string
	StartedAt, FailedAt, Now time.Time
	Failure, Detail          string
	SelfID, SelfAddress      string
	SelfName                 string
	Machines                 []types.Machine
	HealthChecks             map[string]time.Time
	Conns                    map[string][]connInfo
	ConnectedPeers           []string
	LocalNets                []*net.IPNet
	// AddressRejectedAt is when this node last rejected another peer's
	// machines entry at SelfAddress, zero if never.
	AddressRejectedAt time.Time
	LocalNetworkClash bool
}

func unixOrZero(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

// buildSnapshot turns the gathered facts into the JSON model. It reads no
// package state and does no I/O.
func buildSnapshot(in snapshotInput) snapshotJSON {
	snap := snapshotJSON{
		State:     in.State,
		StartedAt: unixOrZero(in.StartedAt),
		Error:     in.Failure,
		Detail:    in.Detail,
		FailedAt:  unixOrZero(in.FailedAt),
		Self:      selfJSON{ID: in.SelfID, Address: in.SelfAddress, Name: in.SelfName},
		Devices:   []deviceJSON{},
		AddressInUse: !in.AddressRejectedAt.IsZero() &&
			in.Now.Sub(in.AddressRejectedAt) < addressInUseWindow,
		LocalNetworkClash: in.LocalNetworkClash,
	}

	deviceIDs := map[string]bool{}
	var newest time.Time
	// The ledger keys machines by address, so one peer can appear under
	// several addresses; the smallest address is shown.
	machines := append([]types.Machine(nil), in.Machines...)
	sort.SliceStable(machines, func(i, j int) bool { return machines[i].Address < machines[j].Address })
	for _, m := range machines {
		if m.PeerID == "" || (in.SelfID != "" && m.PeerID == in.SelfID) ||
			(in.SelfAddress != "" && m.Address == in.SelfAddress) || deviceIDs[m.PeerID] {
			continue
		}
		deviceIDs[m.PeerID] = true
		d := deviceFor(m, in)
		snap.Devices = append(snap.Devices, d)
		if seen, ok := in.HealthChecks[m.PeerID]; ok && seen.After(newest) {
			newest = seen
			snap.LastHeard = &lastHeardJSON{At: seen.Unix(), Name: m.Hostname}
		}
	}
	sort.Slice(snap.Devices, func(i, j int) bool {
		a, b := snap.Devices[i], snap.Devices[j]
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.ID < b.ID
	})

	for _, id := range in.ConnectedPeers {
		if id != in.SelfID && !deviceIDs[id] {
			snap.Helpers++
		}
	}
	return snap
}

func deviceFor(m types.Machine, in snapshotInput) deviceJSON {
	d := deviceJSON{ID: m.PeerID, Name: m.Hostname, Address: m.Address}
	if seen, ok := in.HealthChecks[m.PeerID]; ok && !seen.IsZero() {
		d.LastSeen = seen.Unix()
	}
	reach, best := classifyReach(in.Conns[m.PeerID])
	d.Reach = reach
	if best == nil {
		return d
	}
	d.Limited = best.Limited
	d.OpenedAt = unixOrZero(best.Opened)
	if best.Remote == nil {
		d.Transport = "other"
		return d
	}
	d.RemoteAddr = best.Remote.String()
	d.Transport = transportOf(best.Remote)
	d.Remote = endpointOf(best.Remote)
	if reach == "direct" {
		if ip, err := manet.ToIP(best.Remote); err == nil {
			d.SameNetwork = sameNetwork(ip, in.LocalNets, net.ParseIP(in.SelfAddress))
		}
	}
	return d
}

// endpointOf returns "IP:port" from the first IP and port components of a
// connection address. For a relayed address that is the relay's endpoint.
// It returns an empty string when the address has no IP and port.
func endpointOf(m ma.Multiaddr) string {
	var ip, port string
	ma.ForEach(m, func(c ma.Component) bool {
		switch c.Protocol().Code {
		case ma.P_IP4, ma.P_IP6:
			if ip == "" {
				ip = c.Value()
			}
		case ma.P_TCP, ma.P_UDP:
			if ip != "" && port == "" {
				port = c.Value()
			}
		}
		return port == ""
	})
	if ip == "" || port == "" {
		return ""
	}
	return net.JoinHostPort(ip, port)
}

// PeerIDFromKey returns the node ID that belongs to a key from
// GenerateNodeKey, or an empty string when the key cannot be read.
func PeerIDFromKey(nodeKey []byte) string {
	key, err := crypto.UnmarshalPrivateKey(append([]byte(nil), nodeKey...))
	if err != nil {
		return ""
	}
	id, err := peer.IDFromPrivateKey(key)
	if err != nil {
		return ""
	}
	return id.String()
}

// collectSnapshot copies the shared state under mu and then reads the live
// network without holding it. A node that is missing or fails a read leaves
// that part of the input empty.
func collectSnapshot() snapshotInput {
	mu.Lock()
	n := currentNode
	running := cancelFn != nil
	ready := hostReady
	in := snapshotInput{
		StartedAt:   startedAt,
		FailedAt:    failedAt,
		Now:         clock(),
		Failure:     lastFailure,
		Detail:      failureDetail,
		SelfAddress: selfAddress,
		SelfName:    selfName,

		LocalNetworkClash: localNetworkClash,
	}
	mu.Unlock()
	if in.SelfAddress != "" {
		in.AddressRejectedAt = rejectedAt(in.SelfAddress)
	}

	if n != nil {
		readNetwork(n, ready, &in)
	}
	in.State = computeState(running, in.Failure, reachableCount(buildSnapshot(in).Devices))
	return in
}

// reachableCount is the number of devices with a direct connection.
// edgevpn opens VPN and ledger streams without allowing limited (relayed)
// connections, so libp2p waits for a direct one and nothing flows to a
// device reached only through a relay.
func reachableCount(devices []deviceJSON) int {
	n := 0
	for _, d := range devices {
		if d.Reach == "direct" {
			n++
		}
	}
	return n
}

// readNetwork fills in the ledger, connection and interface facts of a
// running node. The host is read only when ready says edgevpn has
// created it. Each read that fails leaves its part of in empty.
func readNetwork(n *node.Node, ready bool, in *snapshotInput) {
	var h host.Host
	if ready {
		h = n.Host()
	}
	if h != nil {
		in.SelfID = h.ID().String()
		for _, id := range h.Network().Peers() {
			in.ConnectedPeers = append(in.ConnectedPeers, id.String())
		}
	}
	if ledger, err := n.Ledger(); err == nil && ledger != nil {
		data := ledger.CurrentData()
		for _, v := range data[protocol.MachinesLedgerKey] {
			var m types.Machine
			if v.Unmarshal(&m) == nil {
				in.Machines = append(in.Machines, m)
			}
		}
		in.HealthChecks = map[string]time.Time{}
		for id, v := range data[protocol.HealthCheckKey] {
			var s string
			if v.Unmarshal(&s) != nil {
				continue
			}
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				in.HealthChecks[id] = t
			}
		}
	}
	if h != nil {
		in.Conns = map[string][]connInfo{}
		for _, m := range in.Machines {
			id, err := peer.Decode(m.PeerID)
			if err != nil {
				continue
			}
			for _, c := range h.Network().ConnsToPeer(id) {
				in.Conns[m.PeerID] = append(in.Conns[m.PeerID], connInfo{
					Remote:  c.RemoteMultiaddr(),
					Limited: c.Stat().Limited,
					Opened:  c.Stat().Opened,
				})
			}
		}
	}
	if addrs, err := anet.InterfaceAddrs(); err == nil {
		for _, a := range addrs {
			if ipNet, ok := a.(*net.IPNet); ok {
				in.LocalNets = append(in.LocalNets, ipNet)
			}
		}
	}
}

// localNetworkClash is what the app last reported through
// SetLocalNetworkClash for this session. It is guarded by mu.
var localNetworkClash bool

// SetLocalNetworkClash tells the core whether a Wi-Fi or Ethernet network
// the phone is on uses addresses in the VPN's /24. The app works this out
// from Android's view of the networks, which knows which interface is
// Wi-Fi; the core only passes it on in the snapshot. Start and Stop
// clear it.
func SetLocalNetworkClash(clash bool) {
	mu.Lock()
	localNetworkClash = clash
	mu.Unlock()
}

// Snapshot returns the state of the network as JSON for the app.
func Snapshot() string {
	raw, err := json.Marshal(buildSnapshot(collectSnapshot()))
	if err != nil {
		return "{}"
	}
	return string(raw)
}
