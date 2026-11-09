# Architecture

## EdgeVPN in short

EdgeVPN is a peer-to-peer VPN written in Go on top of libp2p. Nodes find each other through mDNS on the local network and through the public libp2p DHT, exchange routing state over GossipSub, and forward IP packets to each other over libp2p streams. A node reads IP packets from a TUN interface, looks up the destination address in a shared ledger that maps VPN addresses to libp2p peer IDs, and sends each packet over a stream to that peer. All nodes of a network share one network key, a base64 encoded YAML document from which the discovery rendezvous point and the GossipSub topic keys are derived.

## Two layers

**Go library** (`golib/`). `gomobile bind` compiles it into an AAR with JNI bindings, so the Kotlin code calls exported Go functions like ordinary Java methods, and the Go runtime runs inside the app's process. gomobile limits the exported API to basic types (int, string, bool, []byte, simple structs and interfaces with methods over those types). The main entry points are `Start(fd, token, nodeKey, address, name)`, `Stop()`, `Snapshot()` (the network's devices as JSON for the main screen) and `RecentLogs()`. The library wraps the tunnel's file descriptor as an `*os.File` and passes it to edgevpn with `vpn.WithInterface`; Android support for this is part of edgevpn since v0.33.0.

**Android app** (`app/`, Kotlin). `EdgeVpnService` is an Android `VpnService`: it builds the tunnel with `VpnService.Builder` (the phone's VPN address with a /24 route), runs as a foreground service with a notification, and hands the file descriptor to Go. It also gives Go a socket protector, so the libp2p sockets go through `VpnService.protect` and leave over Wi-Fi or mobile data instead of looping into the tunnel, and it tells Go when the underlying network changes (`UnderlyingNetworkWatcher`). The screens are `MainActivity` (the network map and state), `JoinActivity` (entering the network key), `SettingsActivity` and `LogsActivity`.

## Keys on the phone

The network key and the node key (the libp2p identity, which fixes the phone's peer ID) are both kept in the app's encrypted shared preferences (`TokenStore`, using `EncryptedSharedPreferences` from AndroidX Security). The node key is generated the first time the VPN starts and then reused (`NodeKeyStore`), so the phone keeps the same peer ID across restarts and its ledger entries stay its own.

## Settings that differ from edgevpn's defaults

The library starts edgevpn in low profile mode, announces and syncs the ledger every 10 seconds, and enforces ledger ownership, so a node can only change entries it signed itself. The libp2p relay service, which lets other peers pass connections through this node, is off unless the user turns on "Relay for other devices" in Settings. When on, it uses go-libp2p's default limits (128 KiB and 2 minutes per relayed connection, 16 circuits per peer, reservations for 1 hour) instead of edgevpn's much larger ones, and it is open to any peer. The comments in `golib/edgevpn.go` give the reason for each setting.
