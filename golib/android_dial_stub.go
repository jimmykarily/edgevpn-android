//go:build !android

package edgevpn

import (
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
)

// newAndroidProtectedDialer is only used when GOOS=android. Stub for non-Android builds.
func newAndroidProtectedDialer() tcp.ContextDialer {
	return nil
}
