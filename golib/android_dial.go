//go:build android

package edgevpn

import (
	"context"
	"fmt"
	"net"
	"os"
	"syscall"

	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
)

// androidProtectedDialer creates a TCP socket, calls VpnService.protect(fd) before
// connect(), then connects. This guarantees protect runs on the correct fd before
// any network syscall, avoiding "operation not permitted" when the standard
// net.Dialer Control callback is not honored on some devices.
type androidProtectedDialer struct{}

func newAndroidProtectedDialer() tcp.ContextDialer {
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

	// SOCK_NONBLOCK so Connect() returns EINPROGRESS immediately instead of
	// blocking the OS thread (SO_SNDTIMEO does NOT apply to connect() on Linux).
	fd, err := syscall.Socket(domain, syscall.SOCK_STREAM|syscall.SOCK_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	success := false
	defer func() {
		if !success {
			syscall.Close(fd)
		}
	}()

	protected := protectSocket(uintptr(fd))
	emit("DEBUG", fmt.Sprintf("protect fd=%d %s %s: %v", fd, network, address, protected))
	if !protected {
		emit("WARN ", fmt.Sprintf("could not protect socket for %s %s", network, address))
		return nil, &protectError{network: network, address: address}
	}

	err = syscall.Connect(fd, sa)
	if err != nil && err != syscall.EINPROGRESS {
		LogDialError(network+" "+address, err)
		return nil, err
	}

	if err == syscall.EINPROGRESS {
		if connErr := waitConnected(ctx, fd, network, address); connErr != nil {
			return nil, connErr
		}
	}

	// Wrap fd in net.Conn. FileConn dups the fd; we close the original.
	file := os.NewFile(uintptr(fd), "tcp")
	conn, err := net.FileConn(file)
	file.Close()
	if err != nil {
		return nil, err
	}
	success = true
	return conn, nil
}
