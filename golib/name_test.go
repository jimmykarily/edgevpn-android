package edgevpn

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mudler/edgevpn/pkg/types"
)

func TestNameUpdate(t *testing.T) {
	self := "12D3KooWB2mB"
	e := types.Machine{PeerID: self, Hostname: "localhost", Address: "10.1.0.2", OS: "android"}
	got, ok := nameUpdate(e, true, self, "Redmi Note 9 Pro")
	if !ok || got.Hostname != "Redmi Note 9 Pro" || got.Address != "10.1.0.2" || got.OS != "android" {
		t.Fatalf("%+v %v", got, ok)
	}
	if _, ok := nameUpdate(got, true, self, "Redmi Note 9 Pro"); ok {
		t.Error("rewrites an entry that already has the name")
	}
	if _, ok := nameUpdate(e, false, self, "x"); ok {
		t.Error("writes before edgevpn created the entry")
	}
	if _, ok := nameUpdate(types.Machine{PeerID: "other"}, true, self, "x"); ok {
		t.Error("touches another peer's entry")
	}
}

func TestNameUpdateIgnoresEmptyName(t *testing.T) {
	e := types.Machine{PeerID: "me", Hostname: "localhost"}
	if _, ok := nameUpdate(e, true, "me", ""); ok {
		t.Error("blanks the hostname")
	}
}

func TestSetNameStoresName(t *testing.T) {
	t.Cleanup(func() { mu.Lock(); selfName = ""; mu.Unlock() })
	SetName("Pixel")
	mu.Lock()
	defer mu.Unlock()
	if selfName != "Pixel" {
		t.Errorf("selfName = %q", selfName)
	}
}

func TestStopClearsNameAndAddress(t *testing.T) {
	mu.Lock()
	selfName, selfAddress = "Pixel", "10.1.0.2"
	mu.Unlock()
	t.Cleanup(func() { mu.Lock(); selfName, selfAddress = "", ""; mu.Unlock() })
	Stop()
	mu.Lock()
	defer mu.Unlock()
	if selfName != "" || selfAddress != "" {
		t.Errorf("name %q address %q survive Stop", selfName, selfAddress)
	}
}

// edgevpn's VPN service rewrites the phone's machines entry only when it is
// missing or has another peer ID. The name announcement relies on that: if a
// new edgevpn version also compares the hostname, both would keep overwriting
// each other. This test fails on such an upgrade.
func TestEdgevpnOnlyRewritesMissingOrForeignEntries(t *testing.T) {
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/mudler/edgevpn").Output()
	if err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(filepath.Join(strings.TrimSpace(string(out)), "pkg", "vpn", "vpn.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "if !found || machine.PeerID != n.Host().ID().String() {") {
		t.Fatal("edgevpn changed when it rewrites the machines entry; recheck the name announcement in name.go")
	}
}
