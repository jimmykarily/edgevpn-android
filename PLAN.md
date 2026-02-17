# EdgeVPN Android

An Android client for [EdgeVPN](https://github.com/mudler/edgevpn), enabling mobile devices to join EdgeVPN networks. The Go core (libp2p, ledger, packet handling) cross-compiles cleanly to Android -- the main work is bridging Android's VpnService API with EdgeVPN's TUN interface layer.

## How it works

EdgeVPN is a Go p2p VPN built on libp2p. Nodes discover each other via DHT/mDNS, exchange routing state over GossipSub, and forward IP packets between peers using libp2p streams. The VPN layer creates a TUN interface, reads IP packets from it, looks up the destination peer in a distributed ledger, and sends the packet over a libp2p stream to that peer.

On Android, Go code runs as a shared library (`.so`) inside a normal Android app. `gomobile bind` compiles the Go code into an `.aar` with JNI bindings. The Kotlin app calls exported Go functions like regular Java methods. The Go runtime (goroutines, GC) lives inside the `.so`. Exported function signatures are limited to basic types: int, string, bool, []byte, simple structs, and interfaces with methods using those types. No channels, generics, or function values in the API surface.

## Architecture

The app has two layers:

**Go library** (`golib/`) -- compiled via `gomobile bind`, exposes a clean API to start/stop the VPN and manage configuration. Contains an Android-specific TUN adapter that wraps a file descriptor received from the Java side, replacing the `water` library's Linux-specific `/dev/net/tun` codepath.

**Android app** (`app/`) -- Kotlin. Handles VpnService lifecycle, permissions, and UI. Passes the TUN file descriptor down to the Go library after `VpnService.Builder.establish()`.

## Key upstream files

These are the edgevpn source files relevant to the Android port. Understanding them is essential before making changes. All paths relative to the edgevpn repo root (`../edgevpn/`).

`pkg/vpn/vpn.go` -- The main VPN network service. `VPNNetworkService()` calls `createInterface(c)` to get a `*water.Interface`, then sets up stream handlers and enters the packet read loop. The `Register()` function wraps this as a `node.Option`. **This is where fd injection needs to happen** -- either by honoring the existing `WithInterface` config option or adding a new path that skips `createInterface`.

`pkg/vpn/config.go` -- VPN config and options. Already has `WithInterface(i *water.Interface)` (line 75) that sets `cfg.Interface`, but `VPNNetworkService` in `vpn.go` ignores it and always calls `createInterface(c)`. Also has all the other option functions: MTU, timeout, concurrency, etc.

`pkg/vpn/interface.go` -- Linux-specific (build tag `!windows,!darwin,!freebsd`). Creates TUN via `water.New()` which opens `/dev/net/tun` + ioctl. Uses `vishvananda/netlink` for IP/MTU/link-up. **This file will be excluded on Android builds** via build tags, replaced by our `interface_android.go`.

`pkg/vpn/interface_{darwin,windows,freebsd}.go` -- Other platform implementations. Same pattern: `createInterface()` + `prepareInterface()`. Shows the expected interface.

`pkg/node/node.go` -- Node creation and startup. `New()` creates the node, `Start()` joins the p2p network, starts the ledger syncer, and runs all registered NetworkServices (including the VPN service).

`pkg/node/connection.go` -- libp2p host creation. `genHost()` builds the libp2p host with transports, discovery, connection gating. Also handles the private key generation and subnet blocking.

`pkg/node/config.go` -- Node-level config. `Config` struct has ExchangeKey, RoomName, discovery settings, stream handlers, etc. `NetworkService` is defined as `func(context.Context, Config, *Node, *blockchain.Ledger) error`.

`pkg/config/config.go` -- High-level config that maps CLI flags to node+vpn options. `ToOpts()` converts a `Config` into `[]node.Option` and `[]vpn.Option`. This is what we'll use from the Go library to set up everything from a token string.

`pkg/hub/hub.go` -- GossipSub message hub. Creates pubsub topics with TOTP-rotated keys. The topic key rotates based on `crypto.TOTP()` of the exchange key, checked every 1 second.

`pkg/node/options.go` -- Node option constructors. `FromBase64()` decodes a base64 token into config. This is the entry point for token-based configuration.

### The `water.Interface` type

The `water` library's `Interface` struct (in the `github.com/mudler/water` module) is simple:

```go
type Interface struct {
    isTAP bool
    io.ReadWriteCloser
    name string
}
```

It embeds `io.ReadWriteCloser`. The VPN code reads/writes ethernet frames through this. On Linux, the underlying `ReadWriteCloser` is an `*os.File` opened from `/dev/net/tun`. For Android, we can wrap `os.NewFile(uintptr(fd), "tun")` to get the same thing. The catch: the struct fields are unexported, so we can't construct a `water.Interface` directly from outside the package. We either need to use `WithInterface` with a real `water.Interface` (impossible without `/dev/net/tun`), or bypass `water` entirely and refactor `VPNNetworkService` to accept an `io.ReadWriteCloser`.

## Upstream dependency strategy

The cleanest approach is a small patch to edgevpn itself. Specifically, make `VPNNetworkService` in `pkg/vpn/vpn.go` check if `c.Interface` is already set (via `WithInterface`) before calling `createInterface(c)`. This is a ~5 line change. If `c.Interface != nil`, use it; otherwise create one. Same for `prepareInterface` -- skip it if the interface was externally provided.

If upstreaming is slow, the fallback is to copy `pkg/vpn/` into this project and modify it. It's a small package (~6 files) but creates ongoing maintenance overhead.

Either way, we also need a way to construct something compatible with `*water.Interface` from a raw fd. Since `water.Interface` has unexported fields, the options are:
1. Add a constructor to the `water` fork (`mudler/water`) -- e.g. `water.NewFromFd(fd int) *Interface`
2. Refactor edgevpn's VPN service to use `io.ReadWriteCloser` instead of `*water.Interface`

Option 2 is cleaner long-term. The VPN code only uses `Read()`, `Write()`, and `Close()` on the interface.

## Implementation Plan

### Step 1: Go library skeleton

Set up the Go module under `golib/`. Create a minimal API surface: `Start(fd int, config string) error`, `Stop()`, `Status() string`. The config is the same base64 token EdgeVPN already uses. Import edgevpn as a dependency and wire up `node.New()` + `node.Start()`. Verify it cross-compiles with `GOOS=android GOARCH=arm64` before touching Android at all.

### Step 2: TUN fd adapter

The core problem. We need to:

1. Patch edgevpn's `VPNNetworkService` to accept a pre-created interface (honor `WithInterface`, or better, accept `io.ReadWriteCloser`). This avoids the `water` library and `vishvananda/netlink` entirely on Android.
2. Skip `prepareInterface()` when the interface is externally provided.
3. In this project's Go library, wrap the Android fd as `os.NewFile(uintptr(fd), "tun")` and pass it through.

This step decides whether we patch edgevpn upstream or vendor the vpn package.

### Step 3: Android project setup

Standard Android project with a `VpnService` subclass. The service:
- Requests VPN permission via `VpnService.prepare()`
- Configures the tunnel with `Builder.addAddress()`, `.addRoute()`, `.setMtu(1200)`, `.establish()`
- Passes the resulting ParcelFileDescriptor's fd (via `detachFd()`) to the Go library
- Runs as a foreground service with a persistent notification

### Step 4: Configuration flow

The user provides an EdgeVPN token (base64-encoded YAML). Start with paste and file import. QR code scanning can come later. The token contains the exchange key, room name, and bootstrap peers -- everything needed to join a network.

### Step 5: Battery and lifecycle

EdgeVPN defaults are tuned for always-on servers. On mobile:
- Ledger sync at 10s is aggressive; increase to 30-60s
- Handle Android doze mode (VpnService is partially exempt but background network is restricted)
- Clean shutdown: cancel the Go context when the user disconnects or the service is killed
- Consider exposing edgevpn's existing `LowProfile` option which reduces connection overhead

### Step 6: Testing and NAT traversal

Mobile networks sit behind carrier-grade NAT. EdgeVPN has relay and hole-punching via libp2p which helps, but needs real-device testing. Wi-Fi should work well; cellular depends on the carrier. Key thing to test: can the phone discover and connect to peers that are behind home NATs.

## Concerns

**Upstream coupling.** The cleanest path requires a small refactor to edgevpn (see upstream strategy above). If that can't happen quickly, we vendor `pkg/vpn/` (~6 files, ~400 lines).

**`vishvananda/netlink` and build tags.** This cgo dependency is in `pkg/vpn/interface.go` under build tag `!windows,!darwin,!freebsd`. Android counts as Linux so it would be included. We need an `interface_android.go` with a build tag that excludes the netlink path, or the upstream refactor needs to make the netlink import conditional on whether the interface is externally provided. Simplest: add `!android` to `interface.go`'s build tag and provide `interface_android.go`.

**`gopacket`.** Pure Go for basic decoding (which is all edgevpn uses -- `layers.IPv4` and `layers.IPv6`). Some sub-packages pull in libpcap/cgo but edgevpn doesn't import those. Should be fine but verify the build.

**APK size.** The Go shared library with the full libp2p stack will be 30-50MB. Normal for a VPN app.

**Android API level.** Targeting API 26+ (Android 8.0) minimum to keep foreground service handling simple.

## Build requirements

- Go 1.24+ (matching edgevpn's go.mod)
- `gomobile` (`go install golang.org/x/mobile/cmd/gomobile@latest && gomobile init`)
- Android SDK (API 26+)
- Android NDK (required by gomobile for cross-compilation)
- Android Studio (for the Kotlin app and emulator)
