# Status Screen, Settings and Logs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the app's single "Status: connected" line with a main screen that shows the VPN's real state, a map and list of the network's devices with how each is reached, plus Settings, Join and Logs screens, all in plain language with (?) help.

**Architecture:** The Go library (`golib/`) keeps the node, tracks a truthful state, keeps a log ring buffer, and exposes one JSON `Snapshot()` that the app polls every 2 s. Pure Go functions (reach rule, same-network rule, snapshot builder, name decision) carry the logic and the tests; thin adapters read the live libp2p host and edgevpn ledger. On the Kotlin side a pure presentation layer turns the snapshot JSON into typed view models (unit tested on the JVM), and the activities only map those to views and string resources.

**Tech Stack:** Go 1.26 with gomobile, edgevpn v0.35.5, go-libp2p v0.48.0, go-log v2.9.2; Kotlin, Android Views, Material Components 1.11.0, minSdk 26, targetSdk 34, JUnit 4; zxing-android-embedded for QR.

**Spec:** `docs/superpowers/specs/2026-11-05-status-screen-design.md` (read it, including the help glossary table and the "Calls made" list). Mockups: `docs/superpowers/specs/2026-11-05-status-screen/*.html`.

## Global Constraints

- Plain language on every screen except Logs. Never show "peer ID", "multiaddr", "DHT", "ledger", "relay circuit" or "libp2p" in main-screen or settings labels; these appear only in help sheets' "Technical details", the device details sheet, the copyable phone ID and the logs.
- Words: "network key" (token), "address in your network" (VPN IP), "Direct" / "Via helper" / "Offline" (badges), "public helpers" (non-network peers), "phone ID" (peer ID), "Unnamed device" (hostname empty or "localhost").
- State values, everywhere: `off`, `looking`, `connected`, `problem`.
- A `[]byte` received from Kotlin is copied before Go keeps it.
- Polling: snapshot every 2 s while the main screen is visible; logs every 1 s while the Logs screen is visible; stop when not visible.
- Ring buffer: last 2000 lines of edgevpn's own log lines (not libp2p's).
- Detailed logs off: app level `info`, libp2p level `error`. On: both `debug`. Default off.
- Address default `10.1.0.2`, route is the address's /24 network.
- Name default `android.os.Build.MODEL`.
- Map: up to 8 devices; beyond 8, 7 devices plus a "+N more" bubble.
- (?) icon: inline after the last word of its term, vertically centred on that line, about cap height, at least 13 dp visible, touch target at least 48 dp.
- Tests first for every Go and pure-Kotlin unit. Go: `cd golib && go test ./...`. Kotlin: `make test` (runs Go tests and `testDebugUnitTest`).
- Commits: `git commit --no-signoff --no-gpg-sign`, message ends with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`, no other trailer.

## Review Focus

1. **A stale entry at the phone's own address** (after "Start fresh", the old identity still owns `machines/<address>` for up to 8 minutes): the device list must not show the phone itself as "Unnamed device". Exclude entries whose address equals the phone's address as well as entries with the phone's peer ID. Test in Task 5.
2. **Snapshot while the node is stopped, starting, or just failed**: `Snapshot()` must return valid JSON with state `off` / `looking` / `problem` and empty devices, never panic on a nil node or ledger. Test in Task 5.
3. **A device with a direct and a relayed connection at the same time** (normal right after hole punching): it is `direct`. Test in Task 4.
4. **A check-in time in the future** (clock skew between devices): relative time shows "just now", never a negative number. Test in Task 9.
5. **A pasted key with spaces or line breaks** (copied from a terminal): validation ignores whitespace and the stored key has none. Test in Task 3.

---

### Task 1: Truthful state, check-ins and new Start parameters (Go)

**Files:**
- Modify: `golib/edgevpn.go` (state variables, `Start`, `Stop`, `Status`, node option list)
- Create: `golib/state.go`
- Test: `golib/state_test.go`, `golib/edgevpn_test.go`

**Interfaces:**
- Consumes: existing `networkConfig(token string) config.Config`, `closeTunOnError`.
- Produces:
  - `func Start(fd int, token string, nodeKey []byte, address string, name string) error` (address like `10.1.0.2`; Go sets `cfg.Address = address + "/24"`; name is stored for Task 6)
  - `func Stop()`; `func Status() string` returning one of the four states
  - `func ReportStartFailure(reason string)`
  - `type StatusListener interface { OnStopped(reason string) }`, `func SetStatusListener(l StatusListener)`
  - `func computeState(running bool, failure string, reachable int) string`
  - package vars for Task 5: `currentNode *node.Node`, `startedAt time.Time`, `lastFailure string`, `failedAt time.Time`, `selfAddress string`, all guarded by `mu`
  - `const aliveAnnounce, aliveScrub, aliveMax = 120 * time.Second, 600 * time.Second, 900 * time.Second`

- [ ] **Step 1: Write the failing tests**

```go
func TestComputeState(t *testing.T) {
	cases := []struct{ running bool; failure string; reachable int; want string }{
		{false, "", 0, "off"},
		{false, "bad key", 0, "problem"},
		{true, "", 0, "looking"},
		{true, "", 2, "connected"},
		{true, "boom", 2, "problem"},
	}
	for _, c := range cases {
		if got := computeState(c.running, c.failure, c.reachable); got != c.want {
			t.Errorf("computeState(%v,%q,%d) = %q, want %q", c.running, c.failure, c.reachable, got, c.want)
		}
	}
}

func TestReportStartFailureThenStop(t *testing.T) {
	ReportStartFailure("VPN interface could not be created")
	if Status() != "problem" { t.Fatalf("Status() = %q, want problem", Status()) }
	Stop()
	if Status() != "off" { t.Fatalf("Status() = %q after Stop, want off", Status()) }
}

func TestAliveIntervalsMatchCLIDefaults(t *testing.T) {
	// edgevpn cmd/util.go: aliveness-healthcheck-interval 120, -scrub-interval 600, -max-interval 900
	if aliveAnnounce != 120*time.Second || aliveScrub != 600*time.Second || aliveMax != 900*time.Second {
		t.Fatal("check-in intervals differ from the edgevpn CLI defaults")
	}
}
```

Update `TestStartRejectsAMissingOrInvalidNodeKey` in `golib/edgevpn_test.go`: call `Start(fd, "some-token", key, "10.1.0.2", "test")`, and expect `Status() == "problem"` after the failed Start (then `Stop()` and expect `"off"`). Add to `TestNetworkConfigMatchesCLIDefaults`: `cfg.Address == "10.1.0.2/24"` when built via the helper Start uses (change `networkConfig` to `networkConfig(token, address string)`).

- [ ] **Step 2: Run the tests and see them fail**

Run: `cd golib && go test ./...`
Expected: build failure on `computeState`, `ReportStartFailure`, the new `Start` arity.

- [ ] **Step 3: Implement**

`computeState` in `golib/state.go`. In `Start`: copy `nodeKey` (already done), reset `lastFailure`, set `startedAt`, `selfAddress`, append `services.Alive(aliveAnnounce, aliveScrub, aliveMax)...` (import `github.com/mudler/edgevpn/pkg/services`) to the node options, keep `currentNode = n`. Every early error return records `lastFailure` and `failedAt`. When `n.Start` returns an error in the goroutine: close the tun file, clear `currentNode` and `cancelFn`, record the failure, and call the registered `StatusListener.OnStopped(err.Error())` outside `mu`. `Stop` clears `lastFailure`, `currentNode`, `startedAt`. `Status()` calls `computeState(cancelFn != nil, lastFailure, reachableCount())` where `reachableCount()` returns 0 until Task 5 replaces it.

- [ ] **Step 4: Run the tests and see them pass**

Run: `cd golib && go test -race ./... && GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build ./...`
Expected: `ok` and no build errors.

- [ ] **Step 5: Commit** (`golib/`), message "Track the VPN state truthfully and run check-ins like the CLI".

Note: the app does not compile after this task until Task 8 updates the `Edgevpn.start` call. To keep every commit buildable, Task 1's commit also changes the one call in `EdgeVpnService.kt` to `Edgevpn.start(fd.toLong(), token, nodeKey, "10.1.0.2", android.os.Build.MODEL)`; run `make test` before committing.

---

### Task 2: Log ring buffer, detailed logs and logcat from Go (Go)

**Files:**
- Modify: `golib/logbuffer.go` (replace the `LogReceiver` push model)
- Create: `golib/logcat_android.go` (`//go:build android`, cgo, `#cgo LDFLAGS: -llog`, `__android_log_write` with tag `EdgeVPN`), `golib/logcat_other.go` (`//go:build !android`, no-op)
- Modify: `golib/edgevpn.go` (`networkConfig` log levels)
- Test: `golib/logbuffer_test.go`

**Interfaces:**
- Produces:
  - `func RecentLogs() string` (lines joined with `\n`, oldest first)
  - `func ClearLogs()`
  - `func SetDetailedLogs(on bool)` (app min level `DEBUG`/`INFO `; go-log `logging.SetAllLoggers(logging.LevelDebug / logging.LevelError)` from `github.com/ipfs/go-log/v2`)
  - `func DetailedLogs() bool`
  - `const logBufferLines = 2000`
  - `networkConfig` sets `LogLevel` to `debug`/`info` and `Libp2pLogLevel` to `debug`/`error` from `DetailedLogs()`
- Removes: `LogReceiver`, `SetLogReceiver` (Task 10 removes the Kotlin user).

- [ ] **Step 1: Write the failing tests**

```go
func TestRingBufferKeepsTheNewest2000Lines(t *testing.T) {
	ClearLogs(); SetDetailedLogs(false)
	for i := 0; i < 2100; i++ { emit("INFO ", fmt.Sprintf("line %d", i)) }
	lines := strings.Split(RecentLogs(), "\n")
	if len(lines) != 2000 { t.Fatalf("got %d lines, want 2000", len(lines)) }
	if !strings.HasSuffix(lines[0], "line 100") || !strings.HasSuffix(lines[1999], "line 2099") {
		t.Fatalf("first/last = %q / %q", lines[0], lines[1999])
	}
}

func TestDetailedLogsControlsDebugLines(t *testing.T) {
	ClearLogs(); SetDetailedLogs(false)
	emit("DEBUG", "hidden")
	SetDetailedLogs(true)
	emit("DEBUG", "shown")
	SetDetailedLogs(false)
	got := RecentLogs()
	if strings.Contains(got, "hidden") || !strings.Contains(got, "shown") { t.Fatalf("logs = %q", got) }
}

func TestNetworkConfigLogLevelsFollowDetailedLogs(t *testing.T) {
	SetDetailedLogs(false)
	if c := networkConfig("t", "10.1.0.2"); c.LogLevel != "info" || c.Libp2pLogLevel != "error" { t.Fatal(c.LogLevel, c.Libp2pLogLevel) }
	SetDetailedLogs(true); defer SetDetailedLogs(false)
	if c := networkConfig("t", "10.1.0.2"); c.LogLevel != "debug" || c.Libp2pLogLevel != "debug" { t.Fatal(c.LogLevel, c.Libp2pLogLevel) }
}
```

- [ ] **Step 2: Run and see them fail** (`cd golib && go test ./...`: undefined `RecentLogs`, `ClearLogs`, `SetDetailedLogs`).

- [ ] **Step 3: Implement.** A fixed-size ring (slice of 2000 strings plus head index) under its own mutex; `emit` appends the formatted line and calls `writeLogcat(level, line)`. `SetMinLogLevel` stays internal and is driven by `SetDetailedLogs`. Run `gofmt -w golib/logbuffer.go` (it is currently not gofmt-clean).

- [ ] **Step 4: Run and see them pass**, plus the Android cross-build with cgo through gomobile: `make golib`. Expected: AAR builds.

- [ ] **Step 5: Commit** (`golib/` and the minimal `MainActivity.kt` change that drops `setLogReceiver` so `make test` passes), message "Keep logs in a ring buffer and write them to logcat from Go".

---

### Task 3: Network key validation (Go)

**Files:**
- Create: `golib/token.go`
- Test: `golib/token_test.go`

**Interfaces:**
- Produces: `func ValidateToken(token string) string` (empty when valid, otherwise one of the three messages below); `func normalizeToken(token string) string` (removes all whitespace); `Start` uses `normalizeToken`.

- [ ] **Step 1: Write the failing tests**

```go
func TestValidateToken(t *testing.T) {
	valid := node.GenerateNewConnectionData().Base64()
	const cut = "This doesn't look like a complete key. It may have been cut off while copying."
	const empty = "Paste a network key first."
	cases := map[string]string{
		valid:                                  "",
		"  " + valid[:40] + "\n" + valid[40:] + "\n": "", // whitespace from a terminal
		"":                                     empty,
		valid[:len(valid)-22]:                  cut,  // what adb input truncation produced
		"not a key at all":                     cut,
	}
	for in, want := range cases {
		if got := ValidateToken(in); got != want { t.Errorf("ValidateToken(%.20q) = %q, want %q", in, got, want) }
	}
}

func TestValidateTokenRejectsYAMLWithoutKeys(t *testing.T) {
	tok := base64.StdEncoding.EncodeToString([]byte("room: x\n"))
	if got := ValidateToken(tok); got != "This key is missing parts EdgeVPN needs. Copy it again from a device that is already in the network." {
		t.Fatalf("got %q", got)
	}
}
```

- [ ] **Step 2: Run and see them fail.**
- [ ] **Step 3: Implement**: normalize, base64-decode (`StdEncoding`), `yaml.Unmarshal` into `node.YAMLConnectionConfig`; require non-empty `OTP.DHT.Key`, `OTP.Crypto.Key`, `RoomName`, `Rendezvous`. Decode or YAML errors give the "cut off" message; missing fields give the "missing parts" message.
- [ ] **Step 4: Run and see them pass.**
- [ ] **Step 5: Commit**, message "Validate network keys before they are saved".

---

### Task 4: Reach and same-network rules (Go, pure)

**Files:**
- Create: `golib/reach.go`
- Test: `golib/reach_test.go`

**Interfaces:**
- Produces:
  - `type connInfo struct { Remote ma.Multiaddr; Limited bool; Opened time.Time }`
  - `func classifyReach(conns []connInfo) (reach string, best *connInfo)`: `reach` is `direct`, `helper` or `offline`; `best` is the oldest direct connection, else the oldest relayed one, else nil
  - `func isRelayed(c connInfo) bool` (limited, or the multiaddr has a `/p2p-circuit` component)
  - `func sameNetwork(remote net.IP, local []*net.IPNet) bool` (remote is private (`IsPrivate()`) or IPv6 link-local or ULA, and contained in one of `local`)
  - `func transportOf(m ma.Multiaddr) string` returning `TCP`, `QUIC` or `other`

- [ ] **Step 1: Write the failing tests**

```go
func TestClassifyReach(t *testing.T) {
	direct := connInfo{Remote: ma.StringCast("/ip4/192.168.21.73/tcp/4001")}
	relayed := connInfo{Remote: ma.StringCast("/ip4/203.0.113.17/tcp/4001/p2p/12D3KooWF3aRcSwpSy36Z7S4gatUWpBtnyHdtGdYY3WmEjeYxqsJ/p2p-circuit"), Limited: true}
	unlimitedCircuit := connInfo{Remote: relayed.Remote}
	for name, c := range map[string]struct{ in []connInfo; want string }{
		"none": {nil, "offline"}, "direct": {[]connInfo{direct}, "direct"},
		"relayed": {[]connInfo{relayed}, "helper"}, "circuit without limit": {[]connInfo{unlimitedCircuit}, "helper"},
		"both, after hole punching": {[]connInfo{relayed, direct}, "direct"},
	} {
		if got, _ := classifyReach(c.in); got != c.want { t.Errorf("%s: got %q, want %q", name, got, c.want) }
	}
}

func TestSameNetwork(t *testing.T) {
	_, wifi, _ := net.ParseCIDR("192.168.20.0/23")
	_, v6, _ := net.ParseCIDR("2001:db8:5d1e:1::/64")
	local := []*net.IPNet{wifi, v6}
	if !sameNetwork(net.ParseIP("192.168.21.73"), local) { t.Error("LAN IPv4 not same network") }
	if sameNetwork(net.ParseIP("10.200.7.113"), local) { t.Error("other private subnet counted as same network") }
	if sameNetwork(net.ParseIP("198.51.100.168"), local) { t.Error("public IPv4 counted as same network") }
	if sameNetwork(net.ParseIP("2001:db8:5d1e:1::5"), local) { t.Error("global IPv6 counted as same network") }
}
```

- [ ] **Step 2–4:** fail, implement, pass (`cd golib && go test ./...`).
- [ ] **Step 5: Commit**, message "Classify how each device is reached".

---

### Task 5: Snapshot (Go)

**Files:**
- Create: `golib/snapshot.go` (types, pure `buildSnapshot`, live adapter `Snapshot()`), `golib/version.go`
- Modify: `golib/state.go` (`reachableCount` uses the snapshot's device reach)
- Test: `golib/snapshot_test.go`

**Interfaces:**
- Consumes: Task 1 state vars and `computeState`, Task 4 functions.
- Produces:
  - `func Snapshot() string` (JSON below)
  - `func LibraryVersion() string` (`debug.ReadBuildInfo`, module `github.com/mudler/edgevpn`, its `Version`; `"unknown"` if absent)
  - `func PeerIDFromKey(nodeKey []byte) string` (empty string on error; copies the slice)
  - `type snapshotInput struct { State string; StartedAt, FailedAt, Now time.Time; Failure string; SelfID, SelfAddress, SelfName string; Machines []types.Machine; HealthChecks map[string]time.Time; Conns map[string][]connInfo; ConnectedPeers []string; LocalNets []*net.IPNet }`
  - `func buildSnapshot(in snapshotInput) snapshotJSON`
  - JSON (field names fixed; Task 9 parses them):

```json
{"state":"connected","startedAt":1793884943,"error":"","failedAt":0,
 "self":{"id":"12D3...","address":"10.1.0.2","name":"Pixel 8"},
 "devices":[{"id":"12D3...","name":"desktop","address":"10.1.0.1","reach":"direct",
   "remote":"192.168.21.73:4001","remoteAddr":"/ip4/192.168.21.73/tcp/4001","transport":"TCP",
   "limited":false,"openedAt":1793884950,"sameNetwork":true,"lastSeen":1793884900}],
 "helpers":143,"lastHeard":{"at":1793884900,"name":"desktop"}}
```

Times are Unix seconds; `0` or an omitted field means unknown. `lastSeen` and `lastHeard` are omitted when unknown.

- [ ] **Step 1: Write the failing tests** for `buildSnapshot`:
  - `TestSnapshotExcludesThisPhoneByIDAndByAddress`: machines contain self (same ID), an old entry at `SelfAddress` with another ID, and the desktop; `devices` has only the desktop.
  - `TestSnapshotDeviceFields`: one direct LAN connection gives `reach=direct`, `remote="192.168.21.73:4001"`, `transport="TCP"`, `sameNetwork=true`; a device with no connection and a health check 2 h old gives `reach=offline` and `lastSeen` set; a device with no connection and no health check has no `lastSeen` key in the marshalled JSON.
  - `TestSnapshotHelpersCountsOnlyNonDevices`: `ConnectedPeers` holds 3 IDs, one of them a device; `helpers == 2`.
  - `TestSnapshotLastHeardIsNewestOtherDevice`: health checks from self (newest), desktop, laptop; `lastHeard.name` is the newer of desktop and laptop.
  - `TestSnapshotWhileStopped`: `Snapshot()` with no node running unmarshals to `state=off`, empty `devices`, `helpers=0` (Review Focus 2). Also after `ReportStartFailure("x")`: `state=problem`, `error="x"`.
- [ ] **Step 2: Run and see them fail.**
- [ ] **Step 3: Implement.** `buildSnapshot` is pure. `Snapshot()` takes `mu` only to copy the state vars, then (if a node runs) reads `n.Ledger()` (`CurrentData()[protocol.MachinesLedgerKey]` unmarshalled into `types.Machine`, `[protocol.HealthCheckKey]` as RFC3339 strings), `n.Host().Network().ConnsToPeer(id)` mapped to `connInfo{c.RemoteMultiaddr(), c.Stat().Limited, c.Stat().Opened}`, `n.Host().Network().Peers()`, and the local networks from `anet.InterfaceAddrs()`; any error in a read leaves that part empty. The state comes from `computeState` with the number of devices whose reach is not `offline`; `Status()` uses the same.
- [ ] **Step 4: Run and see them pass**, plus `GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build ./...`.
- [ ] **Step 5: Commit**, message "Expose a snapshot of the network for the app".

---

### Task 6: Announce the chosen name (Go)

**Files:**
- Create: `golib/name.go`
- Modify: `golib/edgevpn.go` (start the name announcer after the node starts)
- Test: `golib/name_test.go`

**Interfaces:**
- Consumes: Task 1's stored name, `currentNode`.
- Produces: `func SetName(name string)`; `func nameUpdate(existing types.Machine, found bool, selfID, name string) (types.Machine, bool)`.

- [ ] **Step 1: Write the failing tests**

```go
func TestNameUpdate(t *testing.T) {
	self := "12D3KooWB2mB"
	e := types.Machine{PeerID: self, Hostname: "localhost", Address: "10.1.0.2", OS: "android"}
	got, ok := nameUpdate(e, true, self, "Pixel 8")
	if !ok || got.Hostname != "Pixel 8" || got.Address != "10.1.0.2" || got.OS != "android" { t.Fatalf("%+v %v", got, ok) }
	if _, ok := nameUpdate(got, true, self, "Pixel 8"); ok { t.Error("rewrites an entry that already has the name") }
	if _, ok := nameUpdate(e, false, self, "x"); ok { t.Error("writes before edgevpn created the entry") }
	if _, ok := nameUpdate(types.Machine{PeerID: "other"}, true, self, "x"); ok { t.Error("touches another peer's entry") }
}

// edgevpn's VPN service rewrites the phone's machines entry only when it is
// missing or has another peer ID. The name announcement relies on that: if a
// new edgevpn version also compares the hostname, both would keep overwriting
// each other. This test fails on such an upgrade.
func TestEdgevpnOnlyRewritesMissingOrForeignEntries(t *testing.T) {
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/mudler/edgevpn").Output()
	if err != nil { t.Fatal(err) }
	src, err := os.ReadFile(filepath.Join(strings.TrimSpace(string(out)), "pkg", "vpn", "vpn.go"))
	if err != nil { t.Fatal(err) }
	if !strings.Contains(string(src), "if !found || machine.PeerID != n.Host().ID().String() {") {
		t.Fatal("edgevpn changed when it rewrites the machines entry; recheck the name announcement in name.go")
	}
}
```

- [ ] **Step 2–4:** fail, implement, pass. The announcer is `ledger.Announce(ctx, 10*time.Second, func() { ... })` reading `ledger.GetKey(protocol.MachinesLedgerKey, selfAddress)` and calling `ledger.Add(protocol.MachinesLedgerKey, map[string]interface{}{selfAddress: updated})` when `nameUpdate` says so. `SetName` updates the stored name under `mu`; the next tick applies it.
- [ ] **Step 5: Commit**, message "Announce the phone under the name the user chose".

---

### Task 7: App settings storage and Start fresh (Kotlin)

**Files:**
- Create: `app/src/main/kotlin/com/kairos/edgevpn/AppSettings.kt`, `app/src/main/kotlin/com/kairos/edgevpn/AddressRules.kt`
- Test: `app/src/test/kotlin/com/kairos/edgevpn/AddressRulesTest.kt`

**Interfaces:**
- Produces:
  - `object AppSettings { fun address(c: Context): String; fun setAddress(c: Context, v: String); fun name(c: Context): String; fun setName(c: Context, v: String); fun detailedLogs(c: Context): Boolean; fun setDetailedLogs(c: Context, v: Boolean); fun startFresh(c: Context) }` stored in `TokenStore.prefs`; defaults `"10.1.0.2"`, `Build.MODEL`, `false`. `startFresh` clears every key in those prefs (token, node key, settings) with `commit()` and calls `Edgevpn.clearLogs()`.
  - `fun isValidAddress(s: String): Boolean` (four dot-separated decimal numbers 0–255, no leading zeros except `0`, no whitespace)
  - `fun routeFor(address: String): String` (same first three numbers, last `0`)

- [ ] **Step 1: Write the failing tests**: `isValidAddress` true for `10.1.0.2`, `192.168.5.1`; false for `""`, `10.1.0`, `10.1.0.256`, `10.1.0.02`, ` 10.1.0.2`, `a.b.c.d`, `10.1.0.2/24`. `routeFor("10.1.0.2") == "10.1.0.0"`, `routeFor("192.168.5.77") == "192.168.5.0"`.
- [ ] **Step 2–4:** fail, implement, pass (`make test`).
- [ ] **Step 5: Commit**, message "Store the phone's name, address and log setting".

---

### Task 8: Service uses settings and reports failures (Kotlin)

**Files:**
- Modify: `app/src/main/kotlin/com/kairos/edgevpn/EdgeVpnService.kt`

**Interfaces:**
- Consumes: Task 1 (`start`, `reportStartFailure`, `setStatusListener`, `StatusListener`), Task 2 (`setDetailedLogs`), Task 7 (`AppSettings`, `routeFor`).
- Produces: `EdgeVpnService.isRunning` stays; the notification text follows the state ("Connected" / "Looking for your devices").

- [ ] **Step 1:** Before establishing: `Edgevpn.setDetailedLogs(AppSettings.detailedLogs(this))`, `Edgevpn.setStatusListener(...)` whose `onStopped` posts to the main thread and runs the same teardown as `disconnect()` but keeps the remembered failure (do not call `Edgevpn.stop()`, which clears it). `Builder().addAddress(address, 24).addRoute(routeFor(address), 24)`. Every failure path (`token == null` excluded) calls `Edgevpn.reportStartFailure(<plain reason>)` before `stopSelf()`: key load "The phone's identity key could not be loaded.", establish null "Android did not allow the VPN to start.", start exception "The network key could not be used: " + message. `Edgevpn.start(fd.toLong(), token, nodeKey, address, AppSettings.name(this))`.
  The notification text follows `Edgevpn.status()`, checked every 5 s while the service runs: "Connected" for `connected`, "Looking for your devices" for `looking`.
- [ ] **Step 2: Verify on the phone**: `make preview`, install with the release key (see README "Signing"), connect with the app closed afterwards. Expected: `adb logcat -s EdgeVPN` shows the node ID and `mdns(android)` lines with no app screen open; `adb shell ip addr show tun0` shows the address from settings; the notification switches from "Looking for your devices" to "Connected" once the desktop is reached.
- [ ] **Step 3: Commit**, message "Start the VPN with the stored settings and report failures".

---

### Task 9: Presentation layer (Kotlin, pure)

**Files:**
- Create: `app/src/main/kotlin/com/kairos/edgevpn/ui/StatusModel.kt`
- Modify: `app/build.gradle.kts` (`testImplementation("org.json:json:20240303")`, since Android's `org.json` is a stub in JVM tests)
- Test: `app/src/test/kotlin/com/kairos/edgevpn/ui/StatusModelTest.kt`

**Interfaces:**
- Consumes: the Task 5 JSON.
- Produces:
  - `enum class VpnState { OFF, LOOKING, CONNECTED, PROBLEM }`
  - `enum class Badge { DIRECT, VIA_HELPER, OFFLINE }`
  - `sealed interface How { object SameWifi; object OverInternet; object ThroughHelper; data class LastSeen(val secondsAgo: Long); object NotReachable }`
  - `data class DeviceRow(val id: String, val name: String?, val address: String, val badge: Badge, val how: How, val remote: String?, val remoteAddr: String?, val transport: String?, val limited: Boolean, val openSeconds: Long?)` (`name == null` means show "Unnamed device")
  - `data class StatusModel(val state: VpnState, val reachable: Int, val total: Int, val selfAddress: String, val selfId: String, val runningSeconds: Long?, val error: String?, val devices: List<DeviceRow>, val helpers: Int, val lastHeardSecondsAgo: Long?, val lastHeardName: String?)`
  - `fun parseStatus(json: String, nowSeconds: Long): StatusModel`
  - `fun formatAgo(seconds: Long): Ago` with `sealed interface Ago { object JustNow; data class Seconds(val n: Long); data class Minutes(val n: Long); data class Hours(val n: Long); data class Days(val n: Long) }` (under 10 s and any negative value: `JustNow`)

- [ ] **Step 1: Write the failing tests**: sorting (direct before helper before offline, then by name case-insensitively, `null` names last); `"localhost"` and `""` names map to `null`; `reachable`/`total` counts; `How` per device (direct + sameNetwork, direct without, helper, offline with and without `lastSeen`); `formatAgo(-30) == JustNow`, `formatAgo(9) == JustNow`, `formatAgo(45) == Seconds(45)`, `formatAgo(7200) == Hours(2)`; `parseStatus` of the Task 5 example JSON; an `off` snapshot with no `devices` key parses to an empty list.
- [ ] **Step 2–4:** fail, implement, pass (`make test`).
- [ ] **Step 5: Commit**, message "Turn the network snapshot into what the screen shows".

---

### Task 10: Main screen (Kotlin, views)

**Files:**
- Replace: `app/src/main/res/layout/activity_main.xml`, `app/src/main/kotlin/com/kairos/edgevpn/MainActivity.kt`
- Create: `app/src/main/res/layout/item_device.xml`, `app/src/main/res/layout/sheet_device.xml`, `app/src/main/res/menu/main.xml`, drawables for the state gradients, badge backgrounds and device icons, `app/src/main/res/values/colors.xml`
- Modify: `app/src/main/res/values/strings.xml` (all main-screen texts), `app/src/main/res/values/themes.xml` (Material3 parent `Theme.Material3.Light.NoActionBar` with a `MaterialToolbar`, teal primary `#0D9488`)

**Interfaces:**
- Consumes: Task 9 `parseStatus`, Task 5 `Edgevpn.snapshot()`, Task 11 `NetworkMapView.setDevices(List<DeviceRow>)` (add the view in Task 11; until then leave a placeholder `View` with id `map`).
- Produces: `MainActivity` with menu items Settings and Logs (Task 13 and 15 add the target activities; until then the items show a toast).

- [ ] **Step 1:** Layout per the "Connected" mockup: status card (`MaterialCardView` with gradient background per state, title, subtitle, Turn off/Turn on/Try again button, two pills), map, "Your devices" header, a vertical `LinearLayout` of `item_device` rows (devices are few; no RecyclerView), "Finding your devices" header and two tiles. Off state hides the pills, the map and the lists. Looking state shows the pulsing ring (Task 11) and the "devices found so far" tile in place of "last heard". Problem state is red (`#DC2626` to `#B91C1C`), subtitle from `StatusModel.error`, button "Try again", and a "See logs" text button.
- [ ] **Step 2:** Poll `Edgevpn.snapshot()` every 2 s on the main thread with a `Handler` between `onStart` and `onStop`. "Using" from `ConnectivityManager.getNetworkCapabilities(activeNetwork)`: the VPN network is active while connected, so read `getAllNetworks()` and pick the one with `NET_CAPABILITY_INTERNET`, not `TRANSPORT_VPN`, preferring `TRANSPORT_WIFI`; text "Wi-Fi", "Mobile data" or "Other".
- [ ] **Step 3:** Tapping a device row opens a `BottomSheetDialog` (`sheet_device.xml`) with name, address in network, how it is reached, remote IP and port, transport, phone ID with a copy button.
- [ ] **Step 4:** Turn on keeps the existing VPN permission flow (`VpnService.prepare`), and goes to the Join screen when no key is stored (Task 14 adds it; until then Settings).
- [ ] **Step 5: Verify on the phone** with the desktop node from issue #1 running (`edgevpn api` with the same token): screenshot each of Off, Looking, Connected; the desktop row shows "Direct" and "same Wi-Fi as you". `make test` passes.
- [ ] **Step 6: Commit**, message "Show the network's devices and the real state on the main screen".

---

### Task 11: Network map view (Kotlin)

**Files:**
- Create: `app/src/main/kotlin/com/kairos/edgevpn/ui/NetworkMapView.kt`, `app/src/main/kotlin/com/kairos/edgevpn/ui/MapLayout.kt`
- Modify: `activity_main.xml` (replace the `map` placeholder)
- Test: `app/src/test/kotlin/com/kairos/edgevpn/ui/MapLayoutTest.kt`

**Interfaces:**
- Produces:
  - `data class MapSlot(val angleDegrees: Float, val deviceIndex: Int?, val moreCount: Int?)`
  - `fun mapSlots(deviceCount: Int, max: Int = 8): List<MapSlot>`: evenly spaced, first slot at -90° (top), clockwise; when `deviceCount > max`, `max - 1` device slots and a last slot with `moreCount = deviceCount - (max - 1)`
  - `class NetworkMapView : View { fun setDevices(rows: List<DeviceRow>); fun setLooking(looking: Boolean) }`

- [ ] **Step 1: Failing tests**: `mapSlots(0)` empty; `mapSlots(3)` angles `-90, 30, 150`; `mapSlots(8)` 8 device slots; `mapSlots(11)` 7 device slots plus `moreCount = 4`.
- [ ] **Step 2–3:** fail, implement `mapSlots`, pass.
- [ ] **Step 4:** Draw on `Canvas`: phone in the centre (teal circle), devices on a circle at 38% of the smaller side; lines solid green `#16A34A` 4 dp (direct), dashed orange `#D97706` 3 dp with 6 dp dashes (helper), dotted grey `#C4C8D0` 2 dp (offline); offline icons at 50% alpha; names under icons, ellipsized at 10 characters; legend below the view with (?) per entry (Task 12 adds the icons). Looking: a dashed orange ring around the phone animating scale 0.9 to 1.1 every 1.5 s. Tapping a device opens the same sheet as its list row.
- [ ] **Step 5: Verify on the phone** (screenshot with 1 and 3+ devices; use a second `edgevpn api` instance with another `--listen` port and its own `--privkey-cache-dir` to get more devices) and commit, message "Draw the network as a map".

---

### Task 12: Help (?) icons and sheets (Kotlin)

**Files:**
- Create: `app/src/main/kotlin/com/kairos/edgevpn/ui/Help.kt`, `app/src/main/res/layout/sheet_help.xml`, `app/src/main/res/drawable/ic_help.xml` (vector, circle with "?")
- Modify: `strings.xml` (three strings per term: `help_<term>_title`, `help_<term>_plain`, `help_<term>_tech`, wording from the spec's glossary table), `MainActivity.kt`, `NetworkMapView` legend, later the settings rows (Task 13)
- Test: `app/src/test/kotlin/com/kairos/edgevpn/ui/HelpTermTest.kt`

**Interfaces:**
- Produces:
  - `enum class HelpTerm { CONNECTED, LOOKING, OFF, PROBLEM, THIS_PHONE, USING, YOUR_DEVICES, DIRECT, VIA_HELPER, OFFLINE, SAME_NETWORK, FINDING, HELPERS, LAST_HEARD, NETWORK_KEY, NAME, ADDRESS, DETAILED_LOGS, PHONE_ID, START_FRESH }` (20 terms, one per glossary row)
  - `fun TextView.appendHelpIcon(term: HelpTerm, onClick: (HelpTerm) -> Unit)`: appends a ` ` and an `ImageSpan` (`ALIGN_CENTER` on API 29+, `ALIGN_BASELINE` with a computed vertical offset below) sized to the text's cap height (min 13 dp), plus a `ClickableSpan`; sets `movementMethod`; enlarges the touch area with a `TouchDelegate` on the parent to 48 dp around the icon.
  - `fun showHelp(activity: Activity, term: HelpTerm, live: () -> List<Pair<String, String>>)`: `BottomSheetDialog` with title, plain text, grey "Technical details" box, and a "This connection" / "This phone" section from `live()`, refreshed every 2 s while open.
  - `fun liveValues(term: HelpTerm, model: StatusModel, device: DeviceRow?): List<Pair<String, String>>` (pure)

- [ ] **Step 1: Failing tests**: `HelpTerm.values().size == 20`; `liveValues(VIA_HELPER, model, laptopRow)` contains `"Relay address" to <remoteAddr>`, `"Limited" to "yes"`, `"Open for" to "48 s"`; `liveValues(OFF, ...)` is empty; `liveValues(PHONE_ID, ...)` contains the full ID.
- [ ] **Step 2–3:** fail, implement `liveValues`, pass.
- [ ] **Step 4:** Add icons on the main screen at the places in `help-icons.html`, and add the strings.
- [ ] **Step 5: Verify on the phone**: screenshot with icons; check by eye that every icon is centred on its line, including the wrapped tile labels and the large status title (the mistake the mockup review caught). Commit, message "Explain every term with a (?) help sheet".

---

### Task 13: Settings screen (Kotlin)

**Files:**
- Replace: `app/src/main/res/layout/activity_settings.xml`, `app/src/main/kotlin/com/kairos/edgevpn/SettingsActivity.kt`

**Interfaces:**
- Consumes: Task 7 `AppSettings`, `isValidAddress`; Task 2 `setDetailedLogs`; Task 5 `PeerIDFromKey`, `LibraryVersion`; Task 6 `setName`; Task 12 `appendHelpIcon`; `NodeKeyStore.loadOrCreate`.

- [ ] **Step 1:** Layout per the "Joined a network" mockup with (?) on Network key, Name, Address in your network, Detailed logs, Phone ID, Start fresh.
- [ ] **Step 2:** Behaviour:
  - Replace key opens Join (Task 14).
  - Leave network: dialog text from the spec; on confirm stop the VPN, `TokenStore.clear`, finish to the main screen (Off).
  - Name: dialog with a text field; saves and calls `Edgevpn.setName` (no reconnect).
  - Address: dialog with a text field validated by `isValidAddress` (error "Use four numbers from 0 to 255, like 10.1.0.2."); if the VPN runs, ask "Reconnect now to use the new address?" and restart the service on yes.
  - Detailed logs: switch saves and calls `Edgevpn.setDetailedLogs`.
  - Phone ID: `Edgevpn.peerIDFromKey(NodeKeyStore.loadOrCreate(this))` shortened as first 8 + "…" + last 4 characters; tap copies the full ID to the clipboard with a toast "Phone ID copied".
  - Start fresh: dialog text from the spec; on confirm stop the VPN, `AppSettings.startFresh`, open Join and clear the back stack.
  - About: `BuildConfig.VERSION_NAME` (enable `buildFeatures { buildConfig = true }`) and `Edgevpn.libraryVersion()`.
- [ ] **Step 3: Verify on the phone**: change the name and see it on the desktop (`curl -s 127.0.0.1:18080/api/machines`) within 20 s; change the address and see `tun0` with the new address after reconnecting; Start fresh shows a new phone ID afterwards. Commit, message "Rebuild the settings screen".

---

### Task 14: Join screen with QR scanning (Kotlin)

**Files:**
- Create: `app/src/main/kotlin/com/kairos/edgevpn/JoinActivity.kt`, `app/src/main/res/layout/activity_join.xml`
- Modify: `AndroidManifest.xml` (activity, `android.permission.CAMERA`), `app/build.gradle.kts` (`implementation("com.journeyapps:zxing-android-embedded:4.3.0")`), `MainActivity.kt` (open Join when no key)

**Interfaces:**
- Consumes: Task 3 `Edgevpn.validateToken`.

- [ ] **Step 1:** Layout per the "First time" mockup. "Join network": run `validateToken`; on a non-empty result show it under the field; otherwise save the whitespace-free key with `TokenStore.save` and return to the main screen, which starts the VPN.
- [ ] **Step 2:** "Scan QR code instead": `registerForActivityResult(ScanContract())` with `ScanOptions().setDesiredBarcodeFormats(ScanOptions.QR_CODE).setBeepEnabled(false).setOrientationLocked(true)`; the scanned text fills the field and runs validation.
- [ ] **Step 3: Verify on the phone**: paste a truncated key (drop the last 22 characters) and see the "cut off" message; scan a QR of the token (`qrencode -o /tmp/claude-*/token.png < my.token`, shown on the desktop screen) and join. Commit, message "Add a join screen that checks the key and can scan it".

---

### Task 15: Logs screen (Kotlin)

**Files:**
- Create: `app/src/main/kotlin/com/kairos/edgevpn/LogsActivity.kt`, `app/src/main/res/layout/activity_logs.xml`
- Modify: `AndroidManifest.xml`, `MainActivity.kt` (menu item), `SettingsActivity.kt` (View logs)

**Interfaces:**
- Consumes: Task 2 `Edgevpn.recentLogs()`, `Edgevpn.detailedLogs()`.

- [ ] **Step 1:** Monospace text in a `ScrollView`, polled every 1 s while visible; keep scrolled to the bottom only if the user was at the bottom before the update; toolbar action "Copy all" (clipboard, toast "Logs copied"); when detailed logs are on, a line at the top: "Network library details are in the developer log (logcat)."
- [ ] **Step 2: Verify on the phone**: open Logs after the VPN has run for a minute with the screen off; startup lines (node ID, mDNS) are there. Commit, message "Add a logs screen".

---

### Task 16: Finish

- [ ] **Step 1:** `make test`; `make preview`; install; screenshots of Off, Looking, Connected, Problem, a device sheet, a help sheet, Settings, Join, Logs.
  For Problem, build once with `EdgeVpnService.connect()` calling `Edgevpn.reportStartFailure("Test failure")` and returning before `establish()`; take the screenshot and discard that change (do not commit it).
- [ ] **Step 2:** Remove strings and layouts no longer used (`no_token_message`, `show_logs`, `hide_logs`, ...), check `./gradlew lint` shows no new errors.
- [ ] **Step 3:** Update issue #1: tick "Make `Status()` report the real state", the logs items in "3a. Logs", and link the PR.
- [ ] **Step 4:** Commit any leftovers and stop. The owner pushes and opens the PR.
