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

func TestValidateTokenRejectsYAMLWithoutKeys(t *testing.T) {
	tok := base64.StdEncoding.EncodeToString([]byte("room: x\n"))
	if got := ValidateToken(tok); got != "This key is missing parts EdgeVPN needs. Copy it again from a device that is already in the network." {
		t.Fatalf("got %q", got)
	}
}
