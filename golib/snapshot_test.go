package edgevpn

import (
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/mudler/edgevpn/pkg/protocol"
	"github.com/mudler/edgevpn/pkg/types"
	ma "github.com/multiformats/go-multiaddr"
)

func mustMA(t *testing.T, s string) ma.Multiaddr {
	t.Helper()
	m, err := ma.NewMultiaddr(s)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestSnapshotExcludesThisPhoneByIDAndByAddress(t *testing.T) {
	snap := buildSnapshot(snapshotInput{
		State:       "connected",
		SelfID:      "self",
		SelfAddress: "10.1.0.2",
		SelfName:    "phone",
		Machines: []types.Machine{
			{PeerID: "self", Address: "10.1.0.2", Hostname: "phone"},
			{PeerID: "old-identity", Address: "10.1.0.2", Hostname: "phone"},
			{PeerID: "desktop", Address: "10.1.0.1", Hostname: "desktop"},
		},
	})
	if len(snap.Devices) != 1 || snap.Devices[0].ID != "desktop" {
		t.Fatalf("devices = %+v, want only the desktop", snap.Devices)
	}
	if snap.Self.ID != "self" || snap.Self.Address != "10.1.0.2" || snap.Self.Name != "phone" {
		t.Fatalf("self = %+v", snap.Self)
	}
}

func TestSnapshotDoesNotCountTheVPNSubnetAsSameNetwork(t *testing.T) {
	_, tun, _ := net.ParseCIDR("10.1.0.2/24")
	snap := buildSnapshot(snapshotInput{
		State:       "connected",
		Now:         time.Unix(1793890000, 0),
		SelfAddress: "10.1.0.2",
		Machines:    []types.Machine{{PeerID: "desktop", Address: "10.1.0.1", Hostname: "desktop"}},
		Conns: map[string][]connInfo{
			"desktop": {{Remote: mustMA(t, "/ip4/10.1.0.9/tcp/4001"), Opened: time.Unix(1793884950, 0)}},
		},
		LocalNets: []*net.IPNet{tun},
	})
	if len(snap.Devices) != 1 || snap.Devices[0].SameNetwork {
		t.Fatalf("devices = %+v, want one device not on the same network", snap.Devices)
	}
}

func TestSnapshotDeviceFields(t *testing.T) {
	now := time.Unix(1793890000, 0)
	opened := time.Unix(1793884950, 0)
	_, lan, _ := net.ParseCIDR("192.168.1.50/24")
	snap := buildSnapshot(snapshotInput{
		State: "connected",
		Now:   now,
		Machines: []types.Machine{
			{PeerID: "desktop", Address: "10.1.0.1", Hostname: "desktop"},
			{PeerID: "laptop", Address: "10.1.0.3", Hostname: "laptop"},
			{PeerID: "ghost", Address: "10.1.0.4", Hostname: "ghost"},
		},
		HealthChecks: map[string]time.Time{"laptop": now.Add(-2 * time.Hour)},
		Conns: map[string][]connInfo{
			"desktop": {{Remote: mustMA(t, "/ip4/192.168.1.73/tcp/4001"), Opened: opened}},
		},
		LocalNets: []*net.IPNet{lan},
	})
	byID := map[string]deviceJSON{}
	for _, d := range snap.Devices {
		byID[d.ID] = d
	}
	d := byID["desktop"]
	if d.Reach != "direct" || d.Remote != "192.168.1.73:4001" || d.Transport != "TCP" ||
		!d.SameNetwork || d.RemoteAddr != "/ip4/192.168.1.73/tcp/4001" || d.OpenedAt != opened.Unix() || d.Limited {
		t.Fatalf("desktop = %+v", d)
	}
	l := byID["laptop"]
	if l.Reach != "offline" || l.LastSeen != now.Add(-2*time.Hour).Unix() {
		t.Fatalf("laptop = %+v", l)
	}
	raw, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	var generic struct {
		Devices []map[string]any `json:"devices"`
	}
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatal(err)
	}
	for _, dev := range generic.Devices {
		_, has := dev["lastSeen"]
		switch dev["id"] {
		case "ghost":
			if has {
				t.Fatalf("ghost has lastSeen: %v", dev)
			}
		case "laptop":
			if !has {
				t.Fatalf("laptop lacks lastSeen: %v", dev)
			}
		}
	}
}

func TestSnapshotRelayedDeviceUsesRelayEndpoint(t *testing.T) {
	snap := buildSnapshot(snapshotInput{
		Machines: []types.Machine{{PeerID: "desktop", Address: "10.1.0.1"}},
		Conns: map[string][]connInfo{
			"desktop": {{Remote: mustMA(t, "/ip4/203.0.113.9/tcp/4001/p2p-circuit"), Limited: true}},
		},
	})
	d := snap.Devices[0]
	if d.Reach != "helper" || d.Remote != "203.0.113.9:4001" || !d.Limited || d.SameNetwork {
		t.Fatalf("device = %+v", d)
	}
}

func TestSnapshotHelpersCountsOnlyNonDevices(t *testing.T) {
	snap := buildSnapshot(snapshotInput{
		SelfID:         "self",
		Machines:       []types.Machine{{PeerID: "desktop", Address: "10.1.0.1"}},
		ConnectedPeers: []string{"desktop", "relay1", "relay2", "self"},
	})
	if snap.Helpers != 2 {
		t.Fatalf("helpers = %d, want 2", snap.Helpers)
	}
}

func TestSnapshotLastHeardIsNewestOtherDevice(t *testing.T) {
	now := time.Unix(1793890000, 0)
	snap := buildSnapshot(snapshotInput{
		SelfID: "self",
		Machines: []types.Machine{
			{PeerID: "self", Address: "10.1.0.2", Hostname: "phone"},
			{PeerID: "desktop", Address: "10.1.0.1", Hostname: "desktop"},
			{PeerID: "laptop", Address: "10.1.0.3", Hostname: "laptop"},
		},
		HealthChecks: map[string]time.Time{
			"self":    now,
			"desktop": now.Add(-10 * time.Minute),
			"laptop":  now.Add(-5 * time.Minute),
		},
	})
	if snap.LastHeard == nil || snap.LastHeard.Name != "laptop" || snap.LastHeard.At != now.Add(-5*time.Minute).Unix() {
		t.Fatalf("lastHeard = %+v", snap.LastHeard)
	}
}

func resetSnapshotState(t *testing.T) {
	t.Cleanup(func() {
		Stop()
		ClearLogs()
	})
}

func TestSnapshotWhileStopped(t *testing.T) {
	resetSnapshotState(t)
	var got snapshotJSON
	if err := json.Unmarshal([]byte(Snapshot()), &got); err != nil {
		t.Fatal(err)
	}
	if got.State != "off" || len(got.Devices) != 0 || got.Helpers != 0 {
		t.Fatalf("snapshot = %+v", got)
	}
	if !strings.Contains(Snapshot(), `"devices":[]`) {
		t.Fatalf("devices must marshal as an empty array: %s", Snapshot())
	}

	ReportStartFailure("x")
	if err := json.Unmarshal([]byte(Snapshot()), &got); err != nil {
		t.Fatal(err)
	}
	if got.State != "problem" || got.Error != "x" {
		t.Fatalf("snapshot = %+v", got)
	}
	if strings.Contains(Snapshot(), `"detail"`) {
		t.Fatalf("a failure without details must leave out detail: %s", Snapshot())
	}
}

func TestSnapshotCarriesTheErrorDetails(t *testing.T) {
	snap := buildSnapshot(snapshotInput{State: "problem", Failure: reasonStopped, Detail: "node.Start returned error: boom"})
	out, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"detail":"node.Start returned error: boom"`) {
		t.Fatalf("snapshot = %s", out)
	}
}

func TestPeerIDFromKey(t *testing.T) {
	key, err := GenerateNodeKey()
	if err != nil {
		t.Fatal(err)
	}
	id := PeerIDFromKey(key)
	if id == "" || PeerIDFromKey(key) != id {
		t.Fatalf("id = %q", id)
	}
	if PeerIDFromKey([]byte("junk")) != "" {
		t.Fatal("invalid key must give an empty id")
	}
}

func TestLibraryVersionIsNeverEmpty(t *testing.T) {
	if LibraryVersion() == "" {
		t.Fatal("empty version")
	}
}

func TestSnapshotKeepsSmallestAddressForARepeatedPeerID(t *testing.T) {
	for i := 0; i < 50; i++ {
		machines := []types.Machine{
			{PeerID: "desktop", Address: "10.1.0.9", Hostname: "desktop"},
			{PeerID: "desktop", Address: "10.1.0.1", Hostname: "desktop"},
			{PeerID: "desktop", Address: "10.1.0.5", Hostname: "desktop"},
		}
		if i%2 == 1 {
			machines[0], machines[2] = machines[2], machines[0]
		}
		snap := buildSnapshot(snapshotInput{Machines: machines})
		if len(snap.Devices) != 1 || snap.Devices[0].Address != "10.1.0.1" {
			t.Fatalf("devices = %+v, want one at 10.1.0.1", snap.Devices)
		}
	}
}

func TestSnapshotSkipsTheHostUntilItIsReady(t *testing.T) {
	resetSnapshotState(t)
	n := newTestNode(t)
	ledger, err := n.Ledger()
	if err != nil {
		t.Fatal(err)
	}
	ledger.Add(protocol.MachinesLedgerKey, map[string]interface{}{
		"10.1.0.9": types.Machine{PeerID: "12D3KooWPeer", Hostname: "laptop", Address: "10.1.0.9"},
	})
	mu.Lock()
	currentNode, cancelFn, hostReady = n, func() {}, false
	mu.Unlock()

	in := collectSnapshot()
	if in.SelfID != "" || len(in.ConnectedPeers) != 0 || len(in.Conns) != 0 {
		t.Errorf("network read despite host not ready: %+v", in)
	}
	var got snapshotJSON
	if err := json.Unmarshal([]byte(Snapshot()), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Devices) != 1 || got.Devices[0].Reach != "offline" {
		t.Errorf("devices = %+v, want the ledger device offline", got.Devices)
	}
}

func TestSnapshotReportsAnotherPeerAtThisPhonesAddress(t *testing.T) {
	snap := buildSnapshot(snapshotInput{
		State:       "looking",
		SelfID:      "self",
		SelfAddress: "10.1.0.2",
		Machines: []types.Machine{
			{PeerID: "emulator", Address: "10.1.0.2", Hostname: "emulator"},
			{PeerID: "desktop", Address: "10.1.0.1", Hostname: "desktop"},
		},
	})
	want := &addressInUseJSON{ID: "emulator", Name: "emulator"}
	if snap.AddressInUse == nil || *snap.AddressInUse != *want {
		t.Fatalf("addressInUse = %+v, want %+v", snap.AddressInUse, want)
	}
	raw, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"addressInUse":{"id":"emulator","name":"emulator"}`) {
		t.Fatalf("snapshot = %s", raw)
	}
}

func TestSnapshotLeavesOutAddressInUseWithoutAnotherPeerAtTheAddress(t *testing.T) {
	cases := map[string]snapshotInput{
		"our own entry": {
			SelfID: "self", SelfAddress: "10.1.0.2",
			Machines: []types.Machine{{PeerID: "self", Address: "10.1.0.2", Hostname: "phone"}},
		},
		"no entry at the address": {
			SelfID: "self", SelfAddress: "10.1.0.2",
			Machines: []types.Machine{{PeerID: "desktop", Address: "10.1.0.1", Hostname: "desktop"}},
		},
		"own ID not known yet": {
			SelfAddress: "10.1.0.2",
			Machines:    []types.Machine{{PeerID: "emulator", Address: "10.1.0.2", Hostname: "emulator"}},
		},
		"own address not known": {
			SelfID:   "self",
			Machines: []types.Machine{{PeerID: "emulator", Address: "", Hostname: "emulator"}},
		},
	}
	for name, in := range cases {
		snap := buildSnapshot(in)
		if snap.AddressInUse != nil {
			t.Errorf("%s: addressInUse = %+v, want none", name, snap.AddressInUse)
		}
		raw, err := json.Marshal(snap)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "addressInUse") {
			t.Errorf("%s: snapshot has addressInUse: %s", name, raw)
		}
	}
}
