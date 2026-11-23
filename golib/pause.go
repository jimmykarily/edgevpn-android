package edgevpn

import (
	"sync"
	"time"

	"github.com/mudler/edgevpn/pkg/config"
	"github.com/mudler/edgevpn/pkg/services"
)

// dhtRefreshPeriod is how often the DHT refreshes its routing table. The
// library default of 10 minutes costs 2 to 11 MB each time on a phone. The
// routing table only decides which DHT nodes a lookup asks first, and a
// lookup still gets fresh answers from the network.
const dhtRefreshPeriod = time.Hour

// LookForDevices starts a discovery round now, also while discovery is
// paused: it searches the current meeting point and announces this device
// there. A device that joins while every device that is on is paused is
// found this way without waiting for the hourly round. It reports false
// when the VPN is off.
func LookForDevices() bool {
	mu.Lock()
	p := currentPause
	mu.Unlock()
	if p == nil {
		return false
	}
	p.WakeUp("asked by the user")
	return true
}

var (
	saveMobileDataMu sync.Mutex
	saveMobileData   = true
)

// SetSaveMobileData turns the discovery pause on or off for the next Start.
// With it off, the phone searches for devices on the regular schedule, like
// an edgevpn desktop node with default flags.
func SetSaveMobileData(on bool) {
	saveMobileDataMu.Lock()
	defer saveMobileDataMu.Unlock()
	saveMobileData = on
}

// SaveMobileData reports whether the next Start pauses discovery while the
// phone is connected to every device that is on. It is on until
// SetSaveMobileData says otherwise.
func SaveMobileData() bool {
	saveMobileDataMu.Lock()
	defer saveMobileDataMu.Unlock()
	return saveMobileData
}

// configureDiscoveryPause connects p to d. Pause is set only when pausing is
// wanted. Wake is always set, so that LookForDevices and a network change
// start a round whether or not discovery pauses.
func configureDiscoveryPause(d *config.Discovery, p *services.DiscoveryPause, pausing bool) {
	if pausing {
		d.Pause = p.Pause
	}
	d.Wake = p.Wake()
}
