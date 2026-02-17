// Package edgevpn provides the Go core for the EdgeVPN Android app.
// gomobile bind exports these functions as Java methods in the edgevpn.Edgevpn class.
package edgevpn

// Start initializes the VPN with the given base64-encoded configuration token.
func Start(config string) error {
	return nil
}

// Stop shuts down the VPN.
func Stop() {
}

// Status returns the current VPN status as a string.
func Status() string {
	return "disconnected"
}
