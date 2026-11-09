package edgevpn

import (
	"testing"

	relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	"github.com/mudler/edgevpn/pkg/config"
)

func TestRelayForOthersIsOffByDefault(t *testing.T) {
	if RelayForOthers() {
		t.Fatal("RelayForOthers() = true before any call to SetRelayForOthers, want false")
	}
	rs := networkConfig("some-token", "10.1.0.2").Connection.RelayService
	if !rs.Disabled {
		t.Error("RelayService.Disabled = false, want true so the phone relays for nobody unless the user asks")
	}
}

// edgevpn replaces every zero limit with its own server-sized default
// (1 GiB and 30 minutes per circuit), so the phone sets each limit itself.
func TestRelayForOthersUsesTheLibp2pDefaultLimits(t *testing.T) {
	t.Cleanup(func() { SetRelayForOthers(false) })
	SetRelayForOthers(true)
	if !RelayForOthers() {
		t.Fatal("RelayForOthers() = false after SetRelayForOthers(true)")
	}

	rs := networkConfig("some-token", "10.1.0.2").Connection.RelayService
	if rs.Disabled {
		t.Error("RelayService.Disabled = true, want false after SetRelayForOthers(true)")
	}
	if rs.NetworkOnly {
		t.Error("RelayService.NetworkOnly = true, want false so any libp2p peer can use the relay")
	}

	got := config.RelayServiceResources(rs)
	want := relayv2.DefaultResources()
	if got.Limit.Data != 1<<17 || got.Limit.Data != want.Limit.Data {
		t.Errorf("data limit = %d, want 128 KiB (%d)", got.Limit.Data, want.Limit.Data)
	}
	if got.Limit.Duration != want.Limit.Duration {
		t.Errorf("duration limit = %v, want %v", got.Limit.Duration, want.Limit.Duration)
	}
	if got.MaxCircuits != want.MaxCircuits {
		t.Errorf("MaxCircuits = %d, want %d", got.MaxCircuits, want.MaxCircuits)
	}
	if got.ReservationTTL != want.ReservationTTL {
		t.Errorf("ReservationTTL = %v, want %v", got.ReservationTTL, want.ReservationTTL)
	}
	if got.BufferSize != want.BufferSize {
		t.Errorf("BufferSize = %d, want %d", got.BufferSize, want.BufferSize)
	}

	SetRelayForOthers(false)
	if rs := networkConfig("some-token", "10.1.0.2").Connection.RelayService; !rs.Disabled {
		t.Error("RelayService.Disabled = false after SetRelayForOthers(false), want true")
	}
}
