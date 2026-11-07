package edgevpn

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/event"
	ma "github.com/multiformats/go-multiaddr"
)

func advertisedLines(logs string) []string {
	var out []string
	for _, l := range strings.Split(logs, "\n") {
		if strings.Contains(l, "advertised addresses:") {
			out = append(out, l)
		}
	}
	return out
}

func waitForAdvertisedLines(t *testing.T, n int) []string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		lines := advertisedLines(RecentLogs())
		if len(lines) >= n || time.Now().After(deadline) {
			return lines
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func addrsUpdate(addrs ...string) event.EvtLocalAddressesUpdated {
	var evt event.EvtLocalAddressesUpdated
	for _, a := range addrs {
		evt.Current = append(evt.Current, event.UpdatedAddress{Address: ma.StringCast(a)})
	}
	return evt
}

func TestLogAdvertisedAddrsLogsEachUpdate(t *testing.T) {
	SetMinLogLevel("info")
	t.Cleanup(func() { SetMinLogLevel("warn") })
	ClearLogs()
	events := make(chan interface{})
	done := make(chan struct{})
	go func() { logAdvertisedAddrs(context.Background(), events); close(done) }()

	events <- addrsUpdate("/ip4/192.168.1.35/tcp/4001", "/ip6/2001:db8:1::a/tcp/4002")
	events <- addrsUpdate("/ip4/10.106.78.112/tcp/4001")
	close(events)
	<-done

	lines := advertisedLines(RecentLogs())
	want := []string{
		"advertised addresses: [/ip4/192.168.1.35/tcp/4001 /ip6/2001:db8:1::a/tcp/4002]",
		"advertised addresses: [/ip4/10.106.78.112/tcp/4001]",
	}
	if len(lines) != len(want) {
		t.Fatalf("lines = %q", lines)
	}
	for i := range want {
		if !strings.Contains(lines[i], "INFO") || !strings.Contains(lines[i], want[i]) {
			t.Errorf("line %d = %q, want it to contain %q", i, lines[i], want[i])
		}
	}
}

func TestLogAdvertisedAddrsStopsWithTheContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { logAdvertisedAddrs(ctx, make(chan interface{})); close(done) }()
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("still running after the context ended")
	}
}

func TestWatchAdvertisedAddrsLogsARealHostsAddresses(t *testing.T) {
	SetMinLogLevel("info")
	t.Cleanup(func() { SetMinLogLevel("warn") })
	ClearLogs()
	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { h.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	if err := watchAdvertisedAddrs(ctx, h); err != nil {
		t.Fatal(err)
	}
	lines := waitForAdvertisedLines(t, 1)
	if len(lines) == 0 || !strings.Contains(lines[0], "/ip4/127.0.0.1/tcp/") {
		t.Fatalf("lines = %q, want the host's listen address", lines)
	}
}
