# Status screen, settings and logs: design

Date: 2026-11-05. Approved mockups: `2026-11-05-status-screen/main-screen.html`, `2026-11-05-status-screen/settings.html` and `2026-11-05-status-screen/help-icons.html` (open them in a browser; they are self-contained HTML fragments, so unstyled bits are expected).

## Why

Today the app shows one line, "Status: connected", a Disconnect button, and a log panel that is empty until it is opened. "Connected" is set before anything has connected and stays even when the node has failed. The only way to know whether the phone reaches the other devices is to read logs on the phone or run the edgevpn API on a desktop.

The owner wants to see on the phone itself which devices of their network are reachable and whether each one is reached directly or through a relay, as a debugging aid that replaces log reading for most questions. The screen must make sense to someone who does not know VPN or EdgeVPN terms.

## Goals

- One main screen that answers: is the VPN working, which of my devices can I reach right now, and how.
- Plain language on every screen except the logs. Words to use: "network key" (token), "address in your network" (VPN IP), "direct" and "via helper" (direct and relayed connection), "public helpers" (other libp2p peers on the internet), "phone ID" (libp2p peer ID).
- A truthful status: "connected" only when at least one device is reachable.
- Logs that exist whether or not a screen was open when they were written.

## Non-goals

- Managing other devices (removing them from the network, renaming them).
- Bandwidth graphs or traffic counters.
- Automatic address assignment (edgevpn DHCP). The phone's VPN address must be known before Android's `VpnService.Builder.establish()`, and DHCP assigns it after the node joins, so it needs a reconnect dance. Later.
- The IPv6 and mDNS issues listed in issue #1.

## Anti-goals

- Peer IDs, multiaddrs, "DHT", "ledger", "relay circuit" or "libp2p" on the main screen or in settings labels. They belong in the logs and in the copyable phone ID only.
- Any badge or status that claims more than the code knows. If a fact is unknown (for example "last seen" for a device that never checked in), leave it out instead of guessing.

## Constraints

- Existing stack: Kotlin with Android Views and Material Components (`com.google.android.material:material:1.11.0`), minSdk 26, targetSdk 34, Go library bound with gomobile. Stay on Views; no Jetpack Compose (listed under the calls at the end).
- gomobile can only pass basic types between Go and Kotlin (int, string, bool, []byte, interfaces with such methods). A `[]byte` passed from Kotlin is only valid during the call; Go must copy it if it keeps it.
- Everything runs on one phone with arm64 only (`-target android/arm64`).
- Code is written test first (Go `testing`, Kotlin JUnit 4 already set up under `app/src/test`).

## Screens

### Main screen

Top to bottom, as in the "Connected" mockup:

1. **Status card**, coloured by state:
   - **Off** (grey): VPN service not running. "VPN is off", one sentence on what that means, button "Turn on".
   - **Looking for your devices** (orange): node running, zero devices reachable. Shows how long ago it started and that this can take a few minutes on mobile data.
   - **Connected** (green): node running, at least one device reachable. "You can reach N of your M devices."
   - **Problem** (red, not in the mockups; same layout): the VPN could not start or stopped with an error. Shows a plain sentence plus "See logs", and the button "Try again". The raw error goes to the logs. Covers every failure, not only background ones: Go's `Start` returning an error (bad key, config error), `establish()` returning null, the node key failing to load, and `node.Start` returning an error later. In all of them the VPN is torn down completely (tunnel closed, foreground notification removed, service stopped), and the reason is remembered so the screen can show it. "Try again" starts the VPN like "Turn on". The Problem state lasts until the next start attempt or "Turn off".
   - In every state except Off: two pills, "This phone" with its address in the network, and "Using" with `Wi-Fi` or `Mobile data` (from `ConnectivityManager` network capabilities of the active underlying network). The mockup shows the Wi-Fi name; reading it needs the location permission, so it is left out (design call, see the end).
   - Button "Turn off" / "Turn on" in the card.
2. **Map**: the phone in the centre, each device of the network around it. Line style per device: solid green for direct, dashed orange for via helper, faint dotted grey for offline. Device icon plus its name under it. Up to 8 devices placed evenly on a circle; beyond 8, show 7 and a "+N more" bubble. Hidden in the Off state; in the Looking state it shows only the phone with a pulsing ring. Drawn with a custom `View` on a `Canvas`.
3. **Your devices**: one row per device, sorted reachable first (direct, then via helper, then offline), then by name. Row: icon, name, second line "`<address in network>` · `<how>`", badge on the right.
   - `<how>`: "same Wi-Fi as you" (direct, and the remote IP is a private address in the same subnet as one of the phone's Wi-Fi addresses), "over the internet" (direct otherwise), "through a helper computer" (via helper), "last seen X ago" (offline, when a check-in time is known), "not reachable right now" (offline, no check-in time).
   - Badge: "Direct", "Via helper", "Offline".
   - Name: the device's announced hostname. When it is empty or "localhost" (what Android devices announce), show "Unnamed device" instead.
   - Tapping a row opens a small details sheet: name, address in network, how it is reached, the remote address the phone is connected to (as plain IP and port, not multiaddr), phone ID of that device with copy. This sheet is the one place where a technical value appears outside the logs.
4. **Finding your devices**: two tiles. "N public helpers available": the number of peers the phone currently has open connections to that are not devices of the network. "X ago, last heard from your network": the newest check-in time from any other device.
5. Overflow menu (⋮): Settings, Logs.

The screen refreshes every 2 seconds while visible, and stops refreshing when not visible.

### Settings

As in the "Joined a network" mockup:

- **Network card**: "You're part of a network", a sentence about keeping the key private, buttons "Replace key" (opens the Join screen) and "Leave network" (confirmation dialog, then disconnects and deletes the stored network key). The phone's identity key (`NodeKeyStore`) is kept, so if the phone joins the same network again it is the same device to the others and does not leave a second, locked entry behind. The dialog says: "This phone will disconnect and forget the network key. You can join again later with the key." The key is never displayed.
- **This phone**:
  - **Name**: what other devices see in their device lists. Defaults to the phone's model name (`Build.MODEL`, for example "Pixel 8"). Changing it takes effect while connected, without a reconnect.
  - **Address in your network**: an IPv4 address, default `10.1.0.2`. Validated as a dotted IPv4 address. Changing it while connected asks to reconnect. It replaces the hardcoded `10.1.0.2` in both `EdgeVpnService.kt` (`Builder.addAddress`) and `golib/edgevpn.go` (`Address`). The route stays `10.1.0.0/24` derived from the address with a /24 mask.
- **Troubleshooting**:
  - **View logs**: opens the Logs screen.
  - **Detailed logs** switch, off by default. Off: app log level info, libp2p log level error. On: both debug. Applies immediately to a running node and is remembered.
  - **Phone ID**: the libp2p peer ID, shortened in the middle, tap to copy the full value.
  - **Start fresh** (red text): deletes everything the app has stored, which is the network key, the phone's identity key, the name, the address, the detailed-logs choice and the log buffer, after disconnecting. Confirmation dialog: "This phone will forget the network and become a new device. Your other devices may show the old phone as a separate, unreachable device for a few minutes. You'll need the network key to join again." After it, the app shows the Join screen.
- **About**: app version name, and the edgevpn version the library was built with (from Go's `debug.ReadBuildInfo`).

### Join a network

Shown from the main screen when no key is stored, and from "Replace key". As in the "First time" mockup:

- Multi-line paste field, a sentence saying where the key comes from (the same token the user's computer uses to run EdgeVPN), "Join network" button.
- Validation before saving: the key, with whitespace removed, must decode as base64 to an edgevpn config that has the fields a node needs to start (the Go side decides this: add an exported `ValidateToken(token string) string` that returns an empty string when valid and a short plain-language reason otherwise). Shown under the field: "This doesn't look like a complete key. It may have been cut off while copying."
- "Scan QR code instead": scans a QR code containing the token. Uses `com.journeyapps:zxing-android-embedded` (works without Google Play services, which matters on LineageOS). Needs the camera permission, asked at the moment of scanning.

### Logs

A full screen with the most recent log lines, newest at the bottom, auto-scrolling unless the user has scrolled up, and a "Copy all" action. Lines are kept by the Go library in a ring buffer of the last 2000 lines, so they exist even when no screen was open. The Go library also writes every line to logcat itself (tag `EdgeVPN`), so `make logs` works regardless of what the UI is doing.

The buffer and the Logs screen hold edgevpn's own messages (the ones that go through `bufferLogger`), not libp2p's. libp2p logs through its own logging package to standard error, which gomobile forwards to logcat under the tag `GoLog` (seen on the phone). At debug level libp2p writes thousands of lines a minute and would push edgevpn's lines out of the buffer within seconds. So "Detailed logs" makes edgevpn's messages in the buffer more detailed (debug instead of info) and raises libp2p to debug in logcat only. The Logs screen says so in one line at the top when detailed logs are on: "Network library details are in the developer log (logcat)."

### Help: the (?) icons

Every plain-language term gets a small, faded (?) icon next to it, the first time the term appears on a screen: the status title, the "This phone" and "Using" pills, section titles, each badge, each tile, the map legend, and each settings item. Placement rule: the icon sits directly after the last word of its term, vertically centred on that line of text, and scales with the text size (about the height of a capital letter, never under 13 dp of visible circle; the touch target is at least 48 dp, extended around the icon). When a label wraps, the icon stays attached to its last word and never wraps onto a line of its own or drifts to the edge of its container. In Android, put the icon inline in the text (an image span or a compound drawable), not as a separate view aligned to the container. The mockup shows the intended result.

Tapping it opens a bottom sheet with:

1. the term, and the device name when the sheet is about one device;
2. the plain explanation;
3. a grey "Technical details" box with the real names of things;
4. where it applies, a "This connection" or "This phone" part with live values, refreshed while the sheet is open.

The texts live in string resources. Live values come from the snapshot (extend it where a value below is not already in it). The approved wording to start from, which the builder may polish without changing the meaning:

| Term | Plain explanation | Technical details | Live values |
|---|---|---|---|
| Connected | At least one of your devices can be reached from this phone right now. | The edgevpn node is running and has an open libp2p connection to at least one peer listed in the network's machines ledger. | Time since start. |
| Looking for your devices | The VPN is on, but none of your devices has been reached yet. On mobile data this can take a few minutes. | The node is running and searching the public DHT for peers under the network's rendezvous point, and listening for mDNS on Wi-Fi. No machine in the ledger has an open connection yet. | Time since start. |
| VPN is off | Your devices can't reach this phone, and it can't reach them. | No edgevpn node is running and no VPN interface exists. | None. |
| Problem | The VPN could not start, or stopped because of an error. | The exact error, as returned by the failing step, left out when it could contain part of the network key. | The error text, the exact error ("Error details") and when it happened. |
| This phone | The address your other devices use to reach this phone inside your network. | The IPv4 address of the phone's VPN (TUN) interface, announced in the machines ledger. | Address, phone ID. |
| Using | How this phone is connected to the internet right now. | The transport of the active underlying network, from Android's ConnectivityManager. | Wi-Fi or mobile data. |
| Your devices | The devices that have joined your network with the same network key. | Entries of the machines bucket in the edgevpn ledger, which every node keeps a copy of and syncs over GossipSub. | Number of devices. |
| Direct | This phone and the device talk straight to each other. This is the fastest way. | At least one open libp2p connection to the peer is not relayed (no /p2p-circuit in the remote multiaddr) and not limited. | Remote multiaddr, transport (TCP or QUIC), time open. |
| Via helper | Your phone and the device can't reach each other directly, usually because both are behind a home router or a mobile network. Their traffic passes through another computer on the internet that offered to help. It works, but it is slower, and the helper may limit how long the connection lasts. | The connection is a libp2p circuit relay v2 connection (multiaddr contains /p2p-circuit). Relays usually limit these (default: 2 minutes, 128 KiB). libp2p tries hole punching (DCUtR) to upgrade to a direct connection. | Relay multiaddr, limited yes or no, time open. |
| Offline | The device is in your network but can't be reached right now. It may be switched off or without internet. | The peer is in the machines ledger but there is no open libp2p connection to it. "Last seen" comes from its last health check in the ledger. | Last health check time, if any. |
| same Wi-Fi as you / over the internet | Whether the device is on the same local network as this phone. | The remote IP of the direct connection is a private address in the same subnet as one of the phone's Wi-Fi addresses, or it is not. | Remote IP. |
| Finding your devices | How well this phone can find your devices on the internet. | Discovery uses the public libp2p DHT (Kademlia) and, on Wi-Fi, mDNS. | None. |
| public helpers available | Other computers on the internet this phone is talking to. They help your devices find each other and can pass traffic along when a direct connection is impossible. | Number of connected libp2p peers that are not machines of the network: DHT servers, relays and bootstrap nodes. | Count. |
| last heard from your network | How long ago any of your devices last checked in. | The newest health check timestamp written to the ledger by another node (edgevpn's Alive service, every 2 minutes). | Timestamp and which device. |
| Network key | The secret that lets devices join your network. Anyone who has it can join, so keep it private. | The edgevpn network token: base64-encoded YAML holding the room name, the rendezvous string, two OTP keys (one rotates the DHT rendezvous point, one encrypts the messages between nodes), the mDNS service name and the maximum message size. | None (never shown). |
| Name | What your other devices see as this phone's name. | The Hostname field of this phone's entry in the machines ledger. | Current value. |
| Address in your network | Same as "This phone". | Same as "This phone", plus: the VPN routes 10.1.0.0/24 (the address with a /24 mask) through the tunnel. | Address, route. |
| Detailed logs | Writes much more detail to the logs, to help find problems. Uses more battery, so turn it off when done. | Sets the edgevpn log level to debug and the libp2p log level to debug. libp2p's lines go to logcat (tag GoLog), not to the in-app logs. | On or off. |
| Phone ID | A unique identifier for this phone in your network. | The libp2p peer ID, derived from the phone's Ed25519 identity key. It signs this phone's ledger entries. | Full peer ID. |
| Start fresh | Makes this phone forget everything and start over as a new device. | Deletes the network token, the Ed25519 identity key (so a new peer ID is generated) and all settings and logs. The old ledger entries stay owned by the old identity until the ownership TTL (8 minutes) expires. | None. |
| Address already in use | Another device in your network has the same address as this phone. Your network keeps only one device per address, so your other devices can't see this phone until one of the two moves to another address. | The machines ledger entry under this phone's address has another peer ID. Only the owner of an entry may change it while the owner's heartbeat is fresh (the ownership TTL, 8 minutes by default), so this phone's announcement is rejected. Right after Start fresh this also shows for up to that long, because the old identity still owns the entry. | Address, the other device's name and ID. |

## Go library changes

- **Check-ins**: add edgevpn's `services.Alive(120s, 600s, 900s)` to the node options, the same as the CLI does for VPN nodes (`cmd/main.go`). Without it other devices show the phone as offline, and the phone has no "last seen" data for them. Extend the existing CLI-defaults test.
- **Snapshot**: an exported `Snapshot() string` returning JSON, read by the app every 2 seconds. Contents:
  - `state`: `off`, `looking`, `connected`, `problem`, plus `startedAt` and, for `problem`, `error` and, when there is a raw error that cannot contain the network key, `detail`.
  - `self`: phone ID, address in network.
  - `devices`: for each entry of the ledger's machines bucket other than this phone: name (`Hostname`), address (`Address`), phone ID (`PeerID`), `reach` (`direct`, `helper`, `offline`), `remote` (IP and port of the best open connection, if any), `sameNetwork` (bool, per the rule above, computed in Go from the phone's interface addresses via `anet`), `lastSeen` (from the HealthCheck bucket, omitted if unknown).
  - `helpers`: count of connected peers that are not devices.
  - `lastHeard`: newest HealthCheck time from another device, omitted if none.
  - `addressInUse`: the peer ID (`id`) and name (`name`) of the other device whose machines entry holds this phone's address, omitted when the entry is this phone's, when there is none, and while the phone's own ID is unknown. The status card then says "Another device already uses 10.1.0.2. Pick another address in Settings." with a (?) and a "Change address" button that opens Settings.
- **Reach rule**: look at `Host().Network().ConnsToPeer(id)`. Any connection that is not limited (`Stat().Limited == false`) and whose remote multiaddr has no `/p2p-circuit` component: `direct`. Else, any connection at all: `helper`. Else: `offline`. Keep this a pure function over a small description of the connections so it can be unit tested without a network.
- **State rule**: `off` when no node runs and no failure is remembered, `problem` when the last start attempt failed at any stage (see the Problem state), `connected` when at least one device is `direct` or `helper`, otherwise `looking`. A failure is remembered until the next `Start` or `Stop`. The Kotlin side reports its own start failures (establish, key loading) to Go with an exported `ReportStartFailure(reason string)` so there is one source of truth. When `node.Start` fails in the background, Go closes the tunnel and notifies the service through a callback interface so it can stop itself. `Status()` follows the same rule.
- **Logs**: the ring buffer described above, an exported `RecentLogs() string`, logcat writing from Go, and `SetDetailedLogs(on bool)` which changes both the app log level and the libp2p log level at runtime.
- **Settings inputs**: `Start` gains the address and the name (strings), and `SetName(name string)` changes the name while running.
- **Name announcement**: edgevpn's VPN service announces the phone with `os.Hostname()`, which Android reports as "localhost", and has no option to change it. It only rewrites the phone's entry in the machines bucket when the entry is missing or has another peer ID (`pkg/vpn/vpn.go:109` in v0.35.5). The library runs its own announcement on the same interval (`Ledger.Announce`, 10 s): when the phone's entry under its address has this phone's peer ID and a different hostname, it writes the same entry with the chosen hostname, signed like every other write. edgevpn's loop then leaves it alone. A Go test pins this: it runs edgevpn's announce condition against an entry written by the library and fails if edgevpn would overwrite it, so a future edgevpn release that compares more fields is caught at upgrade time instead of as two writers fighting on the network. If no such test is possible without a running network, the builder writes the closest test that exercises the library's write and checks the code path manually on upgrade (note this in the plan).
- **Versions**: `LibraryVersion() string` with the edgevpn module version.

## App changes

- Replace `activity_main.xml`, `MainActivity`, `SettingsActivity` and `activity_settings.xml` with the screens above; add a Join screen and a Logs screen.
- A small presentation layer in plain Kotlin (no Android types) that turns the snapshot JSON into what each view shows: state colour and texts, sorted device rows with their texts and badges, tile values, relative times ("2 h ago"). This is where the Kotlin unit tests go.
- Settings values (address, detailed logs, name) live in the existing encrypted preferences next to the token and node key.
- The service no longer depends on `MainActivity` being open for logs.

## Testing

- Go: unit tests for the reach rule, the state rule, the same-network rule, snapshot JSON shape, the ring buffer, `ValidateToken`, and the CLI-defaults test extended with the Alive service.
- Kotlin: unit tests for the presentation layer.
- On a phone with Android 15 against a desktop edgevpn node on the same Wi-Fi: the desktop appears as "Direct, same Wi-Fi as you"; with the phone on mobile data the desktop appears as "Via helper" or "Direct, over the internet"; with the desktop node stopped it turns "Offline". Screenshots of each state go into the pull request.

## Calls made in this document (not discussed with the owner)

- Android Views, not Jetpack Compose, because the app already uses Views.
- The "Using" pill shows "Wi-Fi" or "Mobile data" without the Wi-Fi name, to avoid asking for the location permission.
- A red "Problem" state exists, although the mockups only show three states.
- "Public helpers available" counts peers with open connections, because edgevpn does not expose the DHT routing table.
- Up to 8 devices on the map, then "+N more".
- The address setting is manual IPv4 with a /24 route; no automatic assignment.
- The QR library is zxing-android-embedded.
- Every start failure shows the red Problem state, and the VPN is fully torn down in it; "Try again" restarts.
- "Leave network" keeps the phone's identity key, so rejoining makes it the same device again.
- The in-app log buffer holds edgevpn's messages only; libp2p's detailed output stays in logcat.
- Devices that announce "localhost" or no name show as "Unnamed device".
- The phone's name defaults to its model name.
- "Start fresh" removes everything the app stores, including the log buffer, not only the identity key.

## Possible follow-up upstream

edgevpn could take an option for the announced hostname (for example `vpn.WithHostname`). That would replace the library's own name announcement. Not needed for this work.
