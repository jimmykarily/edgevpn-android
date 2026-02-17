//go:build android

package edgevpn

import (
	"context"
	"net"
	"os"
	"syscall"
)

// androidProtectedDialer creates a TCP socket, calls VpnService.protect(fd) before
// connect(), then connects. This guarantees protect runs on the correct fd before
// any network syscall, avoiding "operation not permitted" when the standard
// net.Dialer Control callback is not honored on some devices.
type androidProtectedDialer struct{}

func newAndroidProtectedDialer() *androidProtectedDialer {
	return &androidProtectedDialer{}
}

func (d *androidProtectedDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	raddr, err := net.ResolveTCPAddr(network, address)
	if err != nil {
		return nil, err
	}

	var domain int
	var sa syscall.Sockaddr
	if raddr.IP.To4() != nil {
		domain = syscall.AF_INET
		var addr [4]byte
		copy(addr[:], raddr.IP.To4())
		sa = &syscall.SockaddrInet4{Port: raddr.Port, Addr: addr}
	} else {
		domain = syscall.AF_INET6
		var addr [16]byte
		copy(addr[:], raddr.IP.To16())
		sa = &syscall.SockaddrInet6{Port: raddr.Port, Addr: addr}
	}

	fd, err := syscall.Socket(domain, syscall.SOCK_STREAM, 0)
	if err != nil {
		return nil, err
	}
	// Ensure we close fd on any error path.
	success := false
	defer func() {
		if !success {
			syscall.Close(fd)
		}
	}()

	if !protectSocket(uintptr(fd)) {
		return nil, &protectError{network: network, address: address}
	}

	if err := syscall.Connect(fd, sa); err != nil {
		LogDialError(network+" "+address, err)
		return nil, err
	}

	// Wrap fd in net.Conn. FileConn duplicates the fd; we close the original file.
	file := os.NewFile(uintptr(fd), "tcp")
	conn, err := net.FileConn(file)
	file.Close() // release our duplicate; conn holds the other
	if err != nil {
		return nil, err
	}
	success = true
	return conn, nil
}
