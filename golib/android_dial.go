//go:build android

package edgevpn

import (
	"context"
	"fmt"
	"net"
	"os"
	"syscall"
	"time"

	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"golang.org/x/sys/unix"
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
	emit("INFO ", fmt.Sprintf("protect fd=%d %s %s: %v", fd, network, address, protected))
	if !protected {
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

// waitConnected uses pselect6 rather than select: Android's seccomp filter
// kills the process on the plain select syscall on x86_64.
// It polls fd for writability in 100ms chunks so that context
// cancellation is respected promptly. Falls back to 5s if ctx has no deadline.
func waitConnected(ctx context.Context, fd int, network, address string) error {
	deadline := time.Now().Add(5 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}

	const pollInterval = 100 * time.Millisecond
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			LogDialError(network+" "+address, fmt.Errorf("timeout"))
			return fmt.Errorf("connect timeout: %s %s", network, address)
		}
		wait := pollInterval
		if remaining < wait {
			wait = remaining
		}
		ts := unix.NsecToTimespec(wait.Nanoseconds())

		var wfds, efds unix.FdSet
		fdSetBit(&wfds, fd)
		fdSetBit(&efds, fd)

		n, err := unix.Pselect(fd+1, nil, &wfds, &efds, &ts, nil)
		if err == syscall.EINTR {
			continue
		}
		if err != nil {
			return err
		}
		if n == 0 {
			continue
		}
		nerr, err := syscall.GetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_ERROR)
		if err != nil {
			return err
		}
		if nerr != 0 {
			connErr := syscall.Errno(nerr)
			LogDialError(network+" "+address, connErr)
			return connErr
		}
		return nil
	}
}

func fdSetBit(p *unix.FdSet, fd int) {
	p.Set(fd)
}
