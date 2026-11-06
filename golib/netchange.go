package edgevpn

import (
	"fmt"
	"net"

	"github.com/libp2p/go-libp2p/core/host"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"github.com/wlynxg/anet"
)

// staleConns returns the indexes of conns whose address carries an IP that is
// not in localIPs. Addresses without an IP component (such as relay circuits)
// are never reported.
func staleConns(localIPs []net.IP, conns []ma.Multiaddr) []int {
	var stale []int
	for i, c := range conns {
		ip, err := manet.ToIP(c)
		if err != nil {
			continue
		}
		found := false
		for _, l := range localIPs {
			if l.Equal(ip) {
				found = true
				break
			}
		}
		if !found {
			stale = append(stale, i)
		}
	}
	return stale
}

// closeStaleConns closes every connection of h whose local address no longer
// exists on any interface.
func closeStaleConns(h host.Host) {
	var local []net.IP
	addrs, err := anet.InterfaceAddrs()
	if err != nil {
		emit("WARN ", fmt.Sprintf("anet.InterfaceAddrs failed: %v; keeping connections", err))
		return
	}
	for _, a := range addrs {
		if ipn, ok := a.(*net.IPNet); ok {
			local = append(local, ipn.IP)
		}
	}
	conns := h.Network().Conns()
	locals := make([]ma.Multiaddr, len(conns))
	for i, c := range conns {
		locals[i] = c.LocalMultiaddr()
	}
	stale := staleConns(local, locals)
	for _, i := range stale {
		_ = conns[i].Close()
	}
	emit("INFO ", fmt.Sprintf("closed %d connections on addresses that no longer exist", len(stale)))
}
