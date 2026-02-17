package edgevpn

import (
	"net"
	"sync"
	"syscall"
)

// SocketProtector is implemented on the Kotlin/Android side.
// gomobile generates a Java interface that wraps VpnService.protect(fd).
type SocketProtector interface {
	Protect(fd int) bool
}

var (
	protectMu   sync.RWMutex
	protector   SocketProtector
)

// SetSocketProtector registers the Android VpnService.protect callback.
// Must be called from Kotlin before Start().
func SetSocketProtector(p SocketProtector) {
	protectMu.Lock()
	defer protectMu.Unlock()
	protector = p
}

// protectSocket calls VpnService.protect(fd) if a protector is registered.
func protectSocket(fd uintptr) {
	protectMu.RLock()
	p := protector
	protectMu.RUnlock()
	if p != nil {
		p.Protect(int(fd))
	}
}

// controlFunc is a net.Dialer / net.ListenConfig Control function that
// calls VpnService.protect(fd) on every newly created socket.
func controlFunc(network, address string, c syscall.RawConn) error {
	var err error
	c.Control(func(fd uintptr) {
		protectSocket(fd)
	})
	return err
}

// ProtectedDialer returns a net.Dialer whose Control function calls
// VpnService.protect on every socket before connect.
func ProtectedDialer() *net.Dialer {
	return &net.Dialer{
		Control: controlFunc,
	}
}

// ProtectedListenConfig returns a net.ListenConfig whose Control function
// calls VpnService.protect on every socket before bind.
func ProtectedListenConfig() *net.ListenConfig {
	return &net.ListenConfig{
		Control: controlFunc,
	}
}
