package edgevpn

import (
	"context"
	"testing"
	"time"

	"github.com/mudler/edgevpn/pkg/config"
	"github.com/mudler/edgevpn/pkg/services"
)

// newSessionPause sets a session with p as its discovery pause.
func newSessionPause(t *testing.T) *services.DiscoveryPause {
	t.Helper()
	p := services.NewDiscoveryPause(&bufferLogger{}, checkInInterval())
	setSession(newTestNode(t), true, nil, context.Background())
	mu.Lock()
	currentPause = p
	mu.Unlock()
	return p
}

func TestANetworkChangeStartsADiscoveryRound(t *testing.T) {
	t.Cleanup(Stop)
	calls := installNetChange(t)
	p := newSessionPause(t)
	NetworkChanged(false)
	select {
	case <-p.Wake():
	case <-time.After(2 * time.Second):
		t.Fatal("no discovery round after a network change")
	}
	if len(calls()) != 1 {
		t.Fatalf("the network change steps ran %d times, want 1", len(calls()))
	}
}

func TestNetworkConfigRefreshesTheDHTRoutingTableLessOften(t *testing.T) {
	if got := len(networkConfig("some-token", "10.1.0.2").Discovery.DHTOptions); got != 1 {
		t.Fatalf("%d DHT options, want 1 (the routing table refresh period)", got)
	}
}

func TestLookForDevicesStartsADiscoveryRound(t *testing.T) {
	t.Cleanup(Stop)
	p := newSessionPause(t)
	if !LookForDevices() {
		t.Fatal("LookForDevices reported no session")
	}
	select {
	case <-p.Wake():
	default:
		t.Fatal("no discovery round after LookForDevices")
	}
}

func TestLookForDevicesWithoutASession(t *testing.T) {
	Stop()
	if LookForDevices() {
		t.Fatal("LookForDevices reported a session after Stop")
	}
}

func TestSaveMobileDataIsOnByDefault(t *testing.T) {
	if !SaveMobileData() {
		t.Fatal("SaveMobileData is off before anyone set it")
	}
}

func TestDiscoveryPausesOnlyWhenSavingMobileData(t *testing.T) {
	p := services.NewDiscoveryPause(&bufferLogger{}, checkInInterval())
	var on, off config.Discovery
	configureDiscoveryPause(&on, p, true)
	configureDiscoveryPause(&off, p, false)
	if on.Pause == nil {
		t.Fatal("Pause is not set with Save mobile data on")
	}
	if off.Pause != nil {
		t.Fatal("Pause is set with Save mobile data off")
	}
	// A round asked for by the user or by a network change starts in both cases.
	if on.Wake == nil || off.Wake == nil {
		t.Fatal("Wake is not set, so Look for devices now would do nothing")
	}
}
