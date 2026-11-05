package edgevpn

import (
	"testing"
	"time"
)

func TestCheckInIntervalIsTheCLIDefaultUnlessSet(t *testing.T) {
	t.Cleanup(func() { SetCheckInSeconds(0) })

	if got := checkInInterval(); got != aliveAnnounce {
		t.Errorf("default check-in interval = %v, want %v", got, aliveAnnounce)
	}

	SetCheckInSeconds(15)
	if got := checkInInterval(); got != 15*time.Second {
		t.Errorf("check-in interval after SetCheckInSeconds(15) = %v, want 15s", got)
	}

	for _, reset := range []int{0, -5} {
		SetCheckInSeconds(15)
		SetCheckInSeconds(reset)
		if got := checkInInterval(); got != aliveAnnounce {
			t.Errorf("check-in interval after SetCheckInSeconds(%d) = %v, want the default %v", reset, got, aliveAnnounce)
		}
	}
}
