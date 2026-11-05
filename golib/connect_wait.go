//go:build linux

package edgevpn

import (
	"context"
	"fmt"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// waitConnected waits for a non-blocking connect on fd to finish. It uses
// ppoll: Android's seccomp filter kills the process on the plain select
// syscall on x86_64, and a select descriptor set cannot hold a descriptor
// above 1024. It polls in 100ms chunks so that context cancellation is
// respected promptly. Falls back to 5s if ctx has no deadline.
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

		// POLLERR and POLLHUP are always reported, so a failed connect
		// wakes the wait as well.
		fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLOUT}}
		n, err := unix.Ppoll(fds, &ts, nil)
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
