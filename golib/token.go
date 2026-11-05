package edgevpn

import (
	"encoding/base64"
	"strings"
	"unicode"

	"github.com/mudler/edgevpn/pkg/node"
	"gopkg.in/yaml.v2"
)

const (
	tokenEmptyMessage   = "Paste a network key first."
	tokenTruncatedMsg   = "This doesn't look like a complete key. It may have been cut off while copying."
	tokenMissingMessage = "This key is missing parts EdgeVPN needs. Copy it again from a device that is already in the network."
)

// ValidateToken returns an empty string when token is a network key that
// EdgeVPN can join with, and otherwise a message that tells the user what is
// wrong with it. Keys copied from a terminal often contain line breaks or
// spaces, so those are removed first.
func ValidateToken(token string) string {
	token = normalizeToken(token)
	if token == "" {
		return tokenEmptyMessage
	}

	raw, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return tokenTruncatedMsg
	}

	var cfg node.YAMLConnectionConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return tokenTruncatedMsg
	}

	if cfg.OTP.DHT.Key == "" || cfg.OTP.Crypto.Key == "" || cfg.RoomName == "" || cfg.Rendezvous == "" {
		return tokenMissingMessage
	}
	return ""
}

// normalizeToken removes every whitespace character from token.
func normalizeToken(token string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, token)
}
