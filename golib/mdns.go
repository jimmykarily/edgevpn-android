package edgevpn

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/discovery"
	"github.com/mudler/edgevpn/pkg/node"
	"gopkg.in/yaml.v2"
)

// mdnsTag returns the mDNS service name stored in the network key, or ""
// when the key cannot be decoded. The name is a secret: it must never be
// logged.
func mdnsTag(token string) string {
	raw, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return ""
	}
	var cfg node.YAMLConnectionConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return ""
	}
	return cfg.MDNS
}

// runMDNS starts edgevpn's mDNS discovery on h under the service name tag.
// It returns at once; the discovery stops when ctx is done. Tests replace it.
var runMDNS = func(ctx context.Context, h host.Host, tag string) error {
	return (&discovery.MDNS{DiscoveryServiceTag: tag}).Run(&bufferLogger{}, ctx, h)
}

// mdnsRunner runs mDNS discovery for one VPN session. mDNS runs only while
// the host is ready and the last network report said the phone is on Wi-Fi
// or Ethernet, and at most one instance runs at a time. Every method is safe
// to call from any goroutine.
type mdnsRunner struct {
	mu      sync.Mutex
	tag     string
	local   bool
	parent  context.Context // the node's context; nil until the host is ready
	h       host.Host
	cancel  context.CancelFunc // cancels the running instance; nil when none runs
	stopped bool
}

func newMDNSRunner(tag string) *mdnsRunner {
	return &mdnsRunner{tag: tag}
}

// setLocal records whether the phone is on Wi-Fi or Ethernet. It starts mDNS
// when it becomes local and the host is ready, and stops it otherwise.
func (r *mdnsRunner) setLocal(local bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopped {
		return
	}
	r.local = local
	if !local {
		r.cancelLocked()
		return
	}
	if r.cancel == nil {
		r.startLocked()
	}
}

// hostReady records the node's context and host, and starts mDNS when the
// phone is already known to be local. Instances derive their context from
// ctx, so they end with the node.
func (r *mdnsRunner) hostReady(ctx context.Context, h host.Host) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopped {
		return
	}
	r.parent, r.h = ctx, h
	r.cancelLocked()
	r.startLocked()
}

// restart replaces the running instance with a new one, so mDNS advertises
// the addresses of the current network. It does nothing while not local.
func (r *mdnsRunner) restart() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopped {
		return
	}
	r.cancelLocked()
	r.startLocked()
}

// stop ends mDNS for good: every later call is a no-op.
func (r *mdnsRunner) stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopped = true
	r.cancelLocked()
}

// cancelLocked stops the running instance, if any. The caller holds r.mu.
func (r *mdnsRunner) cancelLocked() {
	if r.cancel != nil {
		r.cancel()
		r.cancel = nil
	}
}

// startLocked starts an instance when the phone is local and the host is
// ready. The caller holds r.mu and has cancelled any running instance.
// runMDNS returns at once, so holding r.mu across it keeps two calls from
// starting two instances without blocking callers for long.
func (r *mdnsRunner) startLocked() {
	if !r.local || r.parent == nil {
		return
	}
	ctx, cancel := context.WithCancel(r.parent)
	if err := runMDNS(ctx, r.h, r.tag); err != nil {
		cancel()
		emit("WARN ", fmt.Sprintf("mdns: cannot start local discovery: %s", hideTag(err.Error(), r.tag)))
		return
	}
	r.cancel = cancel
}

// hideTag replaces every occurrence of tag in msg with "<hidden>".
func hideTag(msg, tag string) string {
	if tag == "" {
		return msg
	}
	return strings.ReplaceAll(msg, tag, "<hidden>")
}

// mdnsService is registered as a network service after hostReadyService, so
// it runs once n's host exists. It hands the host to the session's mDNS
// runner and returns at once. A stale session is ignored.
func mdnsService(ctx context.Context, _ node.Config, n *node.Node, _ *blockchain.Ledger) error {
	mu.Lock()
	var r *mdnsRunner
	if currentNode == n {
		r = currentMDNS
	}
	mu.Unlock()
	if r != nil {
		r.hostReady(ctx, n.Host())
	}
	return nil
}
