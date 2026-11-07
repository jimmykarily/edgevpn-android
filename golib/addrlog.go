package edgevpn

import (
	"context"
	"fmt"
	"strings"

	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/node"
)

// addrLogService is registered as a network service after hostReadyService,
// so it runs once n's host exists. It logs the addresses the host
// advertises to peers, now and after every change, for as long as ctx
// lives. It returns at once.
func addrLogService(ctx context.Context, _ node.Config, n *node.Node, _ *blockchain.Ledger) error {
	if err := watchAdvertisedAddrs(ctx, n.Host()); err != nil {
		emit("WARN ", fmt.Sprintf("cannot follow the advertised addresses: %v", err))
	}
	return nil
}

// watchAdvertisedAddrs subscribes to h's EvtLocalAddressesUpdated and logs
// each list in the background until ctx ends. The event is stateful, so the
// current list is logged first. libp2p emits it only when the list changes,
// and its Current field is the final list after the address factory, relay
// addresses included.
func watchAdvertisedAddrs(ctx context.Context, h host.Host) error {
	sub, err := h.EventBus().Subscribe(new(event.EvtLocalAddressesUpdated))
	if err != nil {
		return err
	}
	go func() {
		defer sub.Close()
		logAdvertisedAddrs(ctx, sub.Out())
	}()
	return nil
}

// logAdvertisedAddrs logs one info line per EvtLocalAddressesUpdated read
// from events, until ctx ends or events is closed.
func logAdvertisedAddrs(ctx context.Context, events <-chan interface{}) {
	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-events:
			if !ok {
				return
			}
			evt, isUpdate := e.(event.EvtLocalAddressesUpdated)
			if !isUpdate {
				continue
			}
			addrs := make([]string, 0, len(evt.Current))
			for _, a := range evt.Current {
				addrs = append(addrs, a.Address.String())
			}
			emit("INFO ", "advertised addresses: ["+strings.Join(addrs, " ")+"]")
		}
	}
}
