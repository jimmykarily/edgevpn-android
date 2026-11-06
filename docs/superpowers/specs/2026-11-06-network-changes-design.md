# Network changes: design

Date: 2026-11-06. Epic: issue #1, step 7. Branch: `feat/network-changes`.

## What the user sees today

A phone runs the VPN on home Wi-Fi and the owner walks out of the house. Wi-Fi drops and the phone moves to mobile data. The app does nothing about it. The VPN comes back only when libp2p happens to notice that its connections are dead and something dials again, which can take minutes. Coming home again, the phone never starts local discovery (mDNS) on the Wi-Fi if the VPN was started on mobile data.

None of this has been measured on the phone yet (epic step 2). The test below measures it on the emulator first, before any fix, so the change is judged against a number.

## Goal

After the phone switches between Wi-Fi and mobile data, VPN traffic works again within 10 seconds, without the user doing anything. The emulator test checks a looser bound (see Testing) and logs the measured time.

## Non-goals

- A seamless handover. libp2p has no QUIC connection migration, and QUIC is off anyway (epic step 6). A switch interrupts traffic for a few seconds.
- Faster discovery through the public DHT (a separate epic item).
- Stopping mDNS from advertising the VPN and mobile data addresses on Wi-Fi (epic step 3). The new mDNS runner below makes that easier later, but it stays a separate change.
- Battery tuning (epic step 8). The keepalive change below has a battery cost that step 8 measures.

## How it works today

- Kotlin (`EdgeVpnService`) builds the VPN, takes a Wi-Fi multicast lock and starts Go. It never listens for network changes.
- Go dials through protected sockets (`VpnService.protect`), so the dials bypass the VPN and leave through whatever network Android uses at that moment.
- libp2p recomputes the phone's own addresses every 5 seconds. Our `androidAddrsFactory` reads the interfaces each time, so the advertised addresses already follow the network, and libp2p tells connected peers about new addresses through identify push.
- edgevpn runs the Android mDNS discovery once, at node start (`pkg/discovery/mdns_android.go`). It picks the multicast interfaces at that moment and never looks again. Started on mobile data, it finds no Wi-Fi interface and skips mDNS for the whole session.
- A dead connection is noticed by the yamux keepalive: a ping every 30 seconds with a 10 second timeout, so up to 40 seconds. When Android itself removes an interface's address it usually also destroys the TCP sockets on it, which closes those connections at once (expected from Android's design, untested here).
- After a failed dial, libp2p's swarm keeps the peer's addresses in a dial backoff for a while, so a redial right after a switch can be refused without trying.

## Design

### Kotlin: watch the network

A new class `UnderlyingNetworkWatcher` in the service owns one network callback for the session. It is registered in `connect()` after `Edgevpn.start` succeeds and unregistered in `teardown()`.

Which network to watch: an app's default network, as reported by `registerDefaultNetworkCallback`, is the VPN itself when the VPN applies to the app, and it does apply to this app. So the watcher asks for the best network that is not a VPN:

- Android 12 (API 31) and later: `registerBestMatchingNetworkCallback` with a request for `NET_CAPABILITY_INTERNET` and `NET_CAPABILITY_NOT_VPN`. Android does the ranking.
- Android 8 to 11 (API 26 to 30): `registerNetworkCallback` with the same request, keeping the capabilities of every network it reports. A pure function picks the current one: validated before not validated, then Wi-Fi, then Ethernet, then cellular. This path is untested on a real device; the emulator image in CI is API 35.

When the chosen network changes (another `Network`, or the same network switching between Wi-Fi and something else), the watcher:

1. calls `setUnderlyingNetworks(arrayOf(network))`, or `setUnderlyingNetworks(null)` when no network is left. The VPN carries only 10.1.0.0/24, so this does not change routing; it makes Android report the right transport and metered state for the VPN to other apps and in Settings.
2. calls `Edgevpn.networkChanged(onLocalNetwork)`, where `onLocalNetwork` is true when the network is Wi-Fi or Ethernet. It is false on mobile data and when no network is left.

The first report after registering also goes through both steps, so Go learns the starting network type.

The multicast lock stays as it is: taken at start, released at teardown. A lock taken through `WifiManager` is expected to survive Wi-Fi going away and coming back (untested). The emulator test checks that mDNS finds the peer again after Wi-Fi returns; if it does not, the watcher also releases and takes the lock again on each change, and the design gets updated.

### Go: `NetworkChanged(onLocalNetwork bool)`

A new exported function. It returns at once; the work runs on its own goroutine, so the Kotlin callback thread never waits on network I/O. When the VPN is not running it does nothing.

Bursts of callbacks (Android often reports a change in several steps) are merged: the work runs 500 ms after the last call, with the `onLocalNetwork` value of the last call. A call that arrives while the work is running schedules one more run after it.

The work, in order:

1. **Close connections on gone addresses.** Read the current interface addresses with `anet.InterfaceAddrs()`. Close every connection whose local address has an IP that is not among them. Connections whose local address has no IP component are left alone. The decision is a pure function, `staleConns(localIPs []net.IP, conns []connAddr) []int`, with unit tests.
2. **Restart mDNS.** Stop the running mDNS discovery. When `onLocalNetwork` is true, start it again, so it picks the interfaces that exist now. On mobile data mDNS stays off, since there is no local network to find peers on.
3. **Redial.** Dial, in parallel and with a 10 second timeout each, the bootstrap peers from the token and every peer that owns an entry in the machines bucket of the ledger. The dial context carries `network.WithForceDirectDial`, which makes the swarm skip its dial backoff (`p2p/net/swarm/swarm_dial.go`). Peers already connected through a connection that survived step 1 are skipped.

Nothing extra is needed for the advertised addresses: libp2p recomputes them within 5 seconds, as described above. AutoRelay books a new relay slot by itself when its reservation's connection closes. The DHT routing table is left alone; edgevpn keeps its DHT object private, and kad-dht reconnects to more peers by itself when its table runs low (untested).

### Go: mDNS that can be restarted

edgevpn starts mDNS once with the node's context. To restart it, the app turns edgevpn's mDNS off in `networkConfig` (`Discovery.MDNS: false`) and runs the same upstream discovery itself:

- A network service, added next to `hostReadyService`, starts `(&discovery.MDNS{DiscoveryServiceTag: tag}).Run(logger, childCtx, host)` with a context derived from the node's context.
- `tag` is the `mdns` field of the token, decoded the same way as in `ValidateToken`. It is a secret, so it never goes to the log; the existing scrubbing of the mDNS start line in `logbuffer.go` keeps working because the upstream code logs the same line.
- A restart cancels `childCtx`, which shuts down the zeroconf server and the browse loop, and calls `Run` again with a new child context.
- The service starts mDNS only after Kotlin's first report says the phone is on a local network, so a session started on mobile data does not advertise on the mobile interface.

Unit tests use a fake run function to check: no start before a local network is reported, one stop and one start per restart, nothing left running after Stop.

### Go: notice silent drops faster

A connection can also die without any network change, for example when a NAT forgets the mapping. The app replaces libp2p's default muxer with yamux configured for a 10 second keepalive interval and a 5 second ping timeout (`libp2p.Muxer(yamux.ID, ...)`). edgevpn adds its default muxers only when none are set (`pkg/node/connection.go`), so this one wins. A silent drop is then noticed in at most 15 seconds instead of 40.

The cost is one small ping per connection every 10 seconds, also over mobile data with the screen off. The phone keeps a few dozen connections (DHT peers, relays, VPN peers), so step 8 measures what this does to the battery and may raise the interval.

## Testing

Test first, as everywhere in this repository.

Go unit tests (`make test`):

- `staleConns` for IPv4 and IPv6, for a gone address, for a kept address and for an address without an IP.
- The debounce: several calls within 500 ms run the work once, with the last value; a call during a run gives exactly one more run. A fake clock or a short test interval keeps the tests fast.
- `NetworkChanged` before Start and after Stop does nothing.
- The mDNS runner with a fake run function, as listed above.
- `networkConfig` turns edgevpn's mDNS off, and the muxer option carries the keepalive values.

Kotlin unit test: the pure function that picks the network on API 26 to 30.

Emulator test, a new part of `scripts/network-test` (and so of `make network-test` and the CI network job). The emulator has Wi-Fi (`wlan0`) and mobile data (`eth0`, shown as LTE), and both reach the QEMU test peer through the emulator's NAT:

1. Connect as today and wait until pings work both ways.
2. Run a ping from the peer to the app in the background, then `svc wifi disable`. Measure the time until pings work both ways again over mobile data.
3. `svc wifi enable`. Measure again, and check in the log that mDNS started again on `wlan0`.
4. Airplane mode on (`cmd connectivity airplane-mode enable`) for 10 seconds, then off. Measure again.

Each step fails above 60 seconds and logs the measured time. 60 seconds is looser than the 10 second goal because CI emulators are slow and the bound must not flake; the measured times on a local run go into the pull request. Before the fix, the same steps run once on `main` to record the baseline, which is the RED for this test.

A test of a Wi-Fi network without internet needs a second Wi-Fi network on the emulator, which it does not have. It stays for the phone.

## Open questions for the owner

- The 10 second keepalive is a guess at the right trade between noticing drops and battery. Fine to start there and let step 8 tune it?
- `NetworkChanged` turns mDNS off on mobile data. Is there a case where you want mDNS on mobile data (for example a USB or Bluetooth tethered setup)?
