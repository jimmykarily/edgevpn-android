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
	protectMu sync.RWMutex
	protector SocketProtector
)

// SetSocketProtector registers the Android VpnService.protect callback.
// Must be called from Kotlin before Start().
func SetSocketProtector(p SocketProtector) {
	protectMu.Lock()
	defer protectMu.Unlock()
	protector = p
}

// protectSocket calls VpnService.protect(fd) if a protector is registered.
// It returns true if the socket is protected, false otherwise (no protector or protect failed).
func protectSocket(fd uintptr) bool {
	protectMu.RLock()
	p := protector
	protectMu.RUnlock()
	if p == nil {
		return false
	}
	return p.Protect(int(fd))
}

// controlFunc is a net.Dialer / net.ListenConfig Control function that
// calls VpnService.protect(fd) on every newly created socket.
// If protect fails, we return an error so the socket is not used (avoids "operation not permitted").
func controlFunc(network, address string, c syscall.RawConn) error {
	var protected bool
	controlErr := c.Control(func(fd uintptr) {
		protected = protectSocket(fd)
	})
	if controlErr != nil {
		return controlErr
	}
	if !protected {
		return &protectError{network: network, address: address}
	}
	return nil
}

// protectError is returned when VpnService.protect(fd) fails or no protector is set.
type protectError struct{ network, address string }

func (e *protectError) Error() string {
	return "socket protect failed (VpnService.protect returned false or not set): " + e.network + " " + e.address
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
