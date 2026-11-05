//go:build linux

package edgevpn

import (
	"context"
	"net"
	"syscall"
	"testing"
	"time"
)

// startConnect begins a non-blocking connect to addr and returns the socket.
func startConnect(t *testing.T, addr *net.TCPAddr) int {
	t.Helper()
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM|syscall.SOCK_NONBLOCK, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { syscall.Close(fd) })
	sa := &syscall.SockaddrInet4{Port: addr.Port}
	copy(sa.Addr[:], addr.IP.To4())
	if err := syscall.Connect(fd, sa); err != nil && err != syscall.EINPROGRESS {
		t.Fatal(err)
	}
	return fd
}

func TestWaitConnectedReportsAConnection(t *testing.T) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	fd := startConnect(t, ln.Addr().(*net.TCPAddr))
	if err := waitConnected(context.Background(), fd, "tcp4", ln.Addr().String()); err != nil {
		t.Fatalf("waitConnected = %v, want nil", err)
	}
}

func TestWaitConnectedReportsARefusedConnection(t *testing.T) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().(*net.TCPAddr)
	ln.Close()
	fd := startConnect(t, addr)
	if err := waitConnected(context.Background(), fd, "tcp4", addr.String()); err == nil {
		t.Fatal("waitConnected = nil for a closed port, want an error")
	}
}

// A long-running node holds many sockets, so a dial's descriptor can be
// above 1024, the most a select descriptor set holds.
func TestWaitConnectedWorksForADescriptorAbove1024(t *testing.T) {
	var lim syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &lim); err != nil {
		t.Fatal(err)
	}
	if lim.Cur < 1200 {
		t.Skipf("the open file limit is %d, too low for this test", lim.Cur)
	}
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	fd := startConnect(t, ln.Addr().(*net.TCPAddr))
	const high = 1100
	if err := syscall.Dup3(fd, high, syscall.O_CLOEXEC); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { syscall.Close(high) })
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := waitConnected(ctx, high, "tcp4", ln.Addr().String()); err != nil {
		t.Fatalf("waitConnected = %v, want nil", err)
	}
}
