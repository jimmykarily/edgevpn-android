package edgevpn

import (
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/p2p/muxer/yamux"
)

func TestMuxerKeepalive(t *testing.T) {
	defaultInterval := yamux.DefaultTransport.Config().KeepAliveInterval
	defaultWriteTimeout := yamux.DefaultTransport.Config().ConnectionWriteTimeout

	cfg := muxerTransport().Config()
	if !cfg.EnableKeepAlive {
		t.Error("keepalive is disabled")
	}
	if cfg.KeepAliveInterval != 10*time.Second {
		t.Errorf("KeepAliveInterval = %v, want 10s", cfg.KeepAliveInterval)
	}
	if cfg.ConnectionWriteTimeout != 5*time.Second {
		t.Errorf("ConnectionWriteTimeout = %v, want 5s", cfg.ConnectionWriteTimeout)
	}

	d := yamux.DefaultTransport.Config()
	if d.KeepAliveInterval != defaultInterval || d.ConnectionWriteTimeout != defaultWriteTimeout {
		t.Error("yamux.DefaultTransport was modified")
	}
	if muxerTransport() == yamux.DefaultTransport {
		t.Error("muxerTransport returned the shared default")
	}
}
