package edgevpn

import (
	"encoding/base64"
	"testing"

	"github.com/mudler/edgevpn/pkg/node"
)

func TestValidateToken(t *testing.T) {
	valid := node.GenerateNewConnectionData().Base64()
	const cut = "This doesn't look like a complete key. It may have been cut off while copying."
	const empty = "Paste a network key first."
	cases := map[string]string{
		valid: "",
		"  " + valid[:40] + "\n" + valid[40:] + "\n": "", // whitespace from a terminal
		"":                    empty,
		valid[:len(valid)-22]: cut, // what adb input truncation produced
		"not a key at all":    cut,
	}
	for in, want := range cases {
		if got := ValidateToken(in); got != want {
			t.Errorf("ValidateToken(%.20q) = %q, want %q", in, got, want)
		}
	}
}

// A key without an mDNS name cannot find devices on the same Wi-Fi, and one
// without a positive message size gives pubsub a limit of zero bytes, so the
// ledger never syncs. edgevpn always writes both into the keys it generates.
func TestValidateTokenRejectsAKeyWithoutMDNSOrMessageSize(t *testing.T) {
	const missing = "This key is missing parts EdgeVPN needs. Copy it again from a device that is already in the network."
	noMDNS := node.GenerateNewConnectionData()
	noMDNS.MDNS = ""
	noSize := node.GenerateNewConnectionData()
	noSize.MaxMessageSize = 0
	negativeSize := node.GenerateNewConnectionData()
	negativeSize.MaxMessageSize = -1
	cases := map[string]*node.YAMLConnectionConfig{
		"no mdns":                   noMDNS,
		"no max_message_size":       noSize,
		"negative max_message_size": negativeSize,
	}
	for name, cfg := range cases {
		if got := ValidateToken(cfg.Base64()); got != missing {
			t.Errorf("%s: ValidateToken = %q, want %q", name, got, missing)
		}
	}
}

func TestValidateTokenRejectsYAMLWithoutKeys(t *testing.T) {
	tok := base64.StdEncoding.EncodeToString([]byte("room: x\n"))
	if got := ValidateToken(tok); got != "This key is missing parts EdgeVPN needs. Copy it again from a device that is already in the network." {
		t.Fatalf("got %q", got)
	}
}
