package edgevpn

import (
	"encoding/json"
	"net"
	"sort"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
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
	FailedAt  int64          `json:"failedAt"`
	Self      selfJSON       `json:"self"`
	Devices   []deviceJSON   `json:"devices"`
	Helpers   int            `json:"helpers"`
	LastHeard *lastHeardJSON `json:"lastHeard,omitempty"`
}

// snapshotInput is everything buildSnapshot needs, gathered by the caller.
type snapshotInput struct {
	State                    string
	StartedAt, FailedAt, Now time.Time
	Failure                  string
	SelfID, SelfAddress      string
	SelfName                 string
	Machines                 []types.Machine
	HealthChecks             map[string]time.Time
	Conns                    map[string][]connInfo
	ConnectedPeers           []string
	LocalNets                []*net.IPNet
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
		FailedAt:  unixOrZero(in.FailedAt),
		Self:      selfJSON{ID: in.SelfID, Address: in.SelfAddress, Name: in.SelfName},
		Devices:   []deviceJSON{},
	}

	deviceIDs := map[string]bool{}
	var newest time.Time
	for _, m := range in.Machines {
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
			d.SameNetwork = sameNetwork(ip, in.LocalNets)
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
	in := snapshotInput{
		StartedAt:   startedAt,
		FailedAt:    failedAt,
		Now:         time.Now(),
		Failure:     lastFailure,
		SelfAddress: selfAddress,
		SelfName:    selfName,
	}
	mu.Unlock()

	if n != nil {
		readNetwork(n, &in)
	}
	reachable := 0
	for _, d := range buildSnapshot(in).Devices {
		if d.Reach != "offline" {
			reachable++
		}
	}
	in.State = computeState(running, in.Failure, reachable)
	return in
}

// readNetwork fills in the ledger, connection and interface facts of a
// running node. Each read that fails leaves its part of in empty.
// A node that is not fully initialised can panic inside the library, which
// leaves whatever was read before the panic.
func readNetwork(n *node.Node, in *snapshotInput) {
	defer func() { _ = recover() }()
	if h := n.Host(); h != nil {
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
	if h := n.Host(); h != nil {
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

// Snapshot returns the state of the network as JSON for the app.
func Snapshot() string {
	raw, err := json.Marshal(buildSnapshot(collectSnapshot()))
	if err != nil {
		return "{}"
	}
	return string(raw)
}
