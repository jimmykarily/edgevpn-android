# Network changes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When the phone moves between Wi-Fi and mobile data, the VPN works again within seconds without user action.

**Architecture:** Kotlin watches the best non-VPN network and calls `Edgevpn.networkChanged(onLocalNetwork)`. Go merges bursts of calls, closes connections on addresses that are gone, restarts its own mDNS runner (Wi-Fi or Ethernet only) and redials known peers past the dial backoff. A shorter yamux keepalive catches drops that come without a network change.

**Tech Stack:** Go (golib, gomobile, go-libp2p v0.48.0, edgevpn v0.35.5, go-yamux v5), Kotlin with Android Views, bash (`scripts/network-test`).

**Spec:** `docs/superpowers/specs/2026-11-06-network-changes-design.md`

## Global Constraints

- Debounce: the work runs 500 ms after the last `NetworkChanged` call, with the last call's value; a call during a run gives exactly one more run.
- Redial: bootstrap peers and every peer owning a machines ledger entry, in parallel, 10 second timeout each, context wrapped with `network.WithForceDirectDial`; skip peers that are still connected.
- yamux: keepalive interval 10 seconds, ping timeout (`ConnectionWriteTimeout`) 5 seconds.
- mDNS: edgevpn's own mDNS off (`Discovery.MDNS: false`); the app runs `discovery.MDNS{DiscoveryServiceTag: <token mdns field>}` itself, only while the last report says Wi-Fi or Ethernet. Never on mobile data, tethered setups included. The tag is a secret and never goes to the log.
- Kotlin network choice: API 31+ `registerBestMatchingNetworkCallback`; API 26 to 30 `registerNetworkCallback` and a pure picker: validated first, then Wi-Fi, then Ethernet, then cellular. Request: `NET_CAPABILITY_INTERNET` and `NET_CAPABILITY_NOT_VPN`.
- Emulator test: Wi-Fi off, Wi-Fi on, airplane mode on for 10 seconds then off; each step fails above 60 seconds and logs the measured time.
- Repository rules: test first; commits with `git commit --no-signoff --no-gpg-sign` and only the `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>` trailer; never push; no em dashes or emojis in any text; comments describe current code, never history; temporary keys only, never `my.token`; emulator-5556 only.

## Review Focus

- The VPN is turned off while a debounced run is pending or running: the run must not touch the closed host, and must not act on a newer session started meanwhile. Test in Task 4.
- `NetworkChanged` arrives before the host is ready (right after Start): it records the value (so mDNS starts when the host is ready if local) and does nothing else. Test in Tasks 2 and 4.
- Network flapping (many changes in a row): no mDNS server or goroutine is left behind; exactly one mDNS instance runs at most. Test in Task 2.
- No network at all (airplane mode): the run finishes quickly, logs, and the next change still works. Test in Task 4 (unit) and Task 5 (emulator).
- A new session after Stop does not inherit the old session's network state or mDNS runner. Test in Task 2.

---

### Task 1: Pick connections on gone addresses

**Files:**
- Create: `golib/netchange.go`, `golib/netchange_test.go`

**Interfaces:**
- Produces: `func staleConns(localIPs []net.IP, conns []ma.Multiaddr) []int` (indexes into `conns` of connections whose local address has an IP not in `localIPs`); `func closeStaleConns(h host.Host)` which reads `anet.InterfaceAddrs()`, calls `staleConns` over `h.Network().Conns()` local multiaddrs and closes those, logging one INFO line with the count.

- [ ] **Step 1: Write failing tests** `TestStaleConns` table: IPv4 local address gone → index returned; IPv4 kept → not returned; IPv6 gone and kept likewise; local address without an IP component (for example `/p2p-circuit`) → not returned; empty `localIPs` → every IP-based connection returned.
- [ ] **Step 2: Run** `cd golib && go test -run TestStaleConns ./...`, expect a compile failure (undefined `staleConns`).
- [ ] **Step 3: Implement** both functions. Use `manet.ToIP` for the IP and `net.IP.Equal` for the comparison.
- [ ] **Step 4: Run** the test, expect PASS; then `make test`, expect PASS.
- [ ] **Step 5: Commit.**

### Task 2: Restartable mDNS runner

**Files:**
- Create: `golib/mdns.go`, `golib/mdns_test.go`
- Modify: `golib/edgevpn.go` (`networkConfig`: `MDNS: false` with a comment that the app runs mDNS itself; `Start`: register the service; `Stop`: stop the runner), `golib/edgevpn_test.go` (`TestNetworkConfigMatchesCLIDefaults` now expects MDNS false), `golib/name.go` only if the service is chained there

**Interfaces:**
- Produces:
  - `func mdnsTag(token string) string`: the `mdns` field of the decoded token (same decoding as `ValidateToken`), "" when it cannot be decoded.
  - `type mdnsRunner struct` with `func (r *mdnsRunner) setLocal(local bool)`, `func (r *mdnsRunner) hostReady(ctx context.Context, h host.Host)`, `func (r *mdnsRunner) restart()`, `func (r *mdnsRunner) stop()`. A package variable `runMDNS = func(ctx context.Context, h host.Host, tag string) error` that calls `(&discovery.MDNS{DiscoveryServiceTag: tag}).Run(logger, ctx, h)`; tests replace it.
  - Behaviour: mDNS runs only when the host is ready and the last `setLocal` was true. `restart()` cancels the running child context and, if still local, starts a new one. `setLocal(false)` stops it. `stop()` stops it and makes later calls no-ops. All methods are safe from any goroutine.
  - `var currentMDNS *mdnsRunner` set by `Start` (guarded by `mu`) for Task 4.
  - A network service `mdnsService(ctx, cfg, n, ledger) error` that calls `currentMDNS.hostReady(ctx, n.Host())` and returns at once. It must be registered after `hostReadyService` and before the VPN service options (see the comment in `Start` about the VPN service blocking).

- [ ] **Step 1: Write failing tests** with a fake `runMDNS` that records starts and watches its context:
  - `TestMDNSTagReadsTheTokenField`: a token built from a YAML with `mdns: secret-tag` gives "secret-tag"; garbage gives "".
  - `TestMDNSRunnerWaitsForALocalNetwork`: `hostReady` alone starts nothing; then `setLocal(true)` starts once.
  - `TestMDNSRunnerStartsWhenLocalWasReportedFirst`: `setLocal(true)` then `hostReady` starts once.
  - `TestMDNSRunnerRestartReplacesTheInstance`: after `restart()` the first context is done and exactly one new start happened; after 20 restarts, exactly one context is not done.
  - `TestMDNSRunnerStopsOnMobileData`: `setLocal(false)` cancels the running context; `restart()` while not local starts nothing.
  - `TestMDNSRunnerStopEndsEverything`: after `stop()`, the context is done and `setLocal(true)`, `restart()`, `hostReady` start nothing.
  - `TestNetworkConfigMatchesCLIDefaults`: expects `Discovery.MDNS == false`.
- [ ] **Step 2: Run** `cd golib && go test -run 'TestMDNS|TestNetworkConfig' ./...`, expect failures.
- [ ] **Step 3: Implement.** Pass the tag from `Start` (`mdnsTag(normalizeToken(token))`). The upstream Android `Run` logs `mdns(android): starting, service=<tag>`, which `logbuffer.go` already scrubs; keep it that way and do not log the tag anywhere else. A failing `runMDNS` logs a WARN without the tag.
- [ ] **Step 4: Run** the focused tests, then `make test`, expect PASS.
- [ ] **Step 5: Commit.**

### Task 3: Shorter yamux keepalive

**Files:**
- Create: `golib/muxer.go`, `golib/muxer_test.go`
- Modify: `golib/edgevpn.go` (add the option to `node.WithLibp2pAdditionalOptions`)

**Interfaces:**
- Produces: `func muxerTransport() *yamux.Transport` (from `github.com/libp2p/go-libp2p/p2p/muxer/yamux`, a copy of `yamux.DefaultTransport` with `KeepAliveInterval = 10 * time.Second` and `ConnectionWriteTimeout = 5 * time.Second`) and `func muxerOption() libp2p.Option` returning `libp2p.Muxer(yamux.ID, muxerTransport())`.

- [ ] **Step 1: Write failing test** `TestMuxerKeepalive`: `muxerTransport().Config()` has keepalive enabled, interval 10 s, write timeout 5 s, and `yamux.DefaultTransport` is unchanged afterwards.
- [ ] **Step 2: Run** it, expect a compile failure.
- [ ] **Step 3: Implement** and wire the option into `Start`. Comment why: a silent drop is noticed within 15 seconds; step 8 of the epic measures the battery cost.
- [ ] **Step 4: Run** the test and `make test`, expect PASS. Also `GOOS=android GOARCH=arm64 go build ./...` in golib, expect success.
- [ ] **Step 5: Commit.**

### Task 4: `NetworkChanged`

**Files:**
- Modify: `golib/netchange.go`, `golib/netchange_test.go`

**Interfaces:**
- Consumes: `closeStaleConns` (Task 1), `currentMDNS` and its methods (Task 2), `bootstrapPeers()` (`golib/peers.go`), the machines bucket read the snapshot uses (`golib/snapshot.go`, `readNetwork`).
- Produces: exported `func NetworkChanged(onLocalNetwork bool)`; internal `type netChangeDebouncer` with an injectable delay and an injectable `work func(onLocal bool)`, so tests run fast; `func redialTargets(selfID peer.ID, bootstrap []string, machinePeerIDs []string) []peer.AddrInfo` (deduplicated, without self); `func redial(ctx context.Context, h host.Host, targets []peer.AddrInfo)`.
- Behaviour: `NetworkChanged` always calls `currentMDNS.setLocal(onLocal)` at once when a session exists (so a value reported before the host is ready is kept), then hands the call to the debouncer. The work, for the session that was current when it was scheduled and only if that session is still current and its host is ready: `closeStaleConns`, `currentMDNS.restart()`, `redial`. It logs one INFO line per run: whether local, how many connections closed, how many peers dialled, how many succeeded.

- [ ] **Step 1: Write failing tests:**
  - `TestDebounceMergesABurst`: 5 calls 10 ms apart with a 50 ms delay run the work once, with the last value.
  - `TestDebounceRunsOnceMoreAfterACallDuringARun`: a call while `work` blocks gives exactly one more run after it.
  - `TestRedialTargets`: dedupes, drops self, drops unparsable entries, merges bootstrap addresses with machine peer IDs.
  - `TestNetworkChangedWithoutASessionDoesNothing`: before Start and after Stop, the work function is never called and nothing panics.
  - `TestNetworkChangedSkipsAStaleSession`: work scheduled for session A does nothing when session B is current when it fires (use the session check function, not a real host).
  - `TestNetworkChangedBeforeHostReadyOnlyRecordsTheValue`: the mDNS runner receives `setLocal(true)`; the work does not run its network steps.
- [ ] **Step 2: Run** `cd golib && go test -run 'TestDebounce|TestRedial|TestNetworkChanged' ./...`, expect failures.
- [ ] **Step 3: Implement.** The 500 ms delay and 10 s dial timeout from Global Constraints. Each dial context: `network.WithForceDirectDial(ctx, "network changed")`. Skip targets with `h.Network().Connectedness(id) == network.Connected`. `Stop` must stop the debouncer's pending timer.
- [ ] **Step 4: Run** the focused tests with `-race`, then `make test`, expect PASS. `GOOS=android GOARCH=arm64 go build ./...` succeeds.
- [ ] **Step 5: Commit.**

### Task 5: Emulator network-switch test, with a baseline

**Files:**
- Modify: `scripts/network-test` (new part after the first successful two-way ping, before the off/on part), `Makefile` comment if the env vars change, `README.md` testing section

**Interfaces:**
- Consumes: existing helpers in `scripts/network-test` (`ping_until`, `log`, `fail`, `adb_`).
- Produces: env `NETWORK_TEST_SWITCH=0` skips the part; `NETWORK_TEST_SWITCH_TIMEOUT` (default 60).

- [ ] **Step 1: Write the part.** For each of these steps, measure the seconds until `ping_until app` and `ping_until peer` both succeed, log `switch <name>: <n>s`, and `fail` above the timeout: `wifi-off` (`adb_ shell svc wifi disable`), `wifi-on` (`adb_ shell svc wifi enable`), `airplane` (`adb_ shell cmd connectivity airplane-mode enable`, sleep 10, `... disable`). The part always restores Wi-Fi on and airplane off on exit, also on failure (trap). After `wifi-on`, check the logcat for a fresh `mdns(android): using interfaces:` line that lists `wlan0`; that check is enabled only once Task 6 is in (an env flag the task sets to on by default in Task 6's commit is fine, or add the check in Task 6).
- [ ] **Step 2: Baseline (RED).** Before Kotlin calls `networkChanged`, the app behaves as on main. Run `NETWORK_TEST_LONG=0 make network-test` on emulator-5556 and record each measured time or the failure in the task report. Expected: at least one step above 60 s or failing. If every step already passes quickly, report DONE_WITH_CONCERNS with the numbers; the controller decides.
- [ ] **Step 3: Commit** the test part (it may fail on the branch at this commit; say so in the commit message).

### Task 6: Kotlin network watcher

**Files:**
- Create: `app/src/main/kotlin/com/kairos/edgevpn/NetworkRules.kt`, `app/src/test/kotlin/com/kairos/edgevpn/NetworkRulesTest.kt`, `app/src/main/kotlin/com/kairos/edgevpn/UnderlyingNetworkWatcher.kt`
- Modify: `app/src/main/kotlin/com/kairos/edgevpn/EdgeVpnService.kt`, `app/src/main/AndroidManifest.xml` only if `ACCESS_NETWORK_STATE` is missing, `scripts/network-test` (enable the mDNS check from Task 5)

**Interfaces:**
- Consumes: `Edgevpn.networkChanged(Boolean)` (Task 4).
- Produces:
  - `data class NetworkInfo(val id: Long, val validated: Boolean, val wifi: Boolean, val ethernet: Boolean, val cellular: Boolean)`.
  - `fun pickNetwork(networks: Collection<NetworkInfo>): NetworkInfo?` (validated first, then Wi-Fi, then Ethernet, then cellular, then the lowest `id` for a stable choice).
  - `fun isLocalNetwork(n: NetworkInfo?): Boolean` (Wi-Fi or Ethernet).
  - `class UnderlyingNetworkWatcher(context: Context, onChange: (Network?, Boolean) -> Unit)` with `start()` and `stop()`; it reports only when the chosen network or its local flag changes, and reports the first choice too.
  - Service: on change, `setUnderlyingNetworks(network?.let { arrayOf(it) })` then `Edgevpn.networkChanged(local)`. `start()` after `Edgevpn.start` succeeds; `stop()` in `teardown()`.

- [ ] **Step 1: Write failing tests** `NetworkRulesTest`: validated cellular beats unvalidated Wi-Fi; validated Wi-Fi beats validated cellular; Ethernet beats cellular; empty gives null; equal networks pick the lower id; `isLocalNetwork` true for Wi-Fi and Ethernet, false for cellular and null.
- [ ] **Step 2: Run** `make test` (the Kotlin unit tests), expect compile failures.
- [ ] **Step 3: Implement** the rules, the watcher (API 31+ best-matching callback, otherwise the listen callback plus `pickNetwork`; callbacks post to the main handler) and the service wiring. Log one line per change: `Underlying network: <wifi|ethernet|cellular|none>`.
- [ ] **Step 4: Run** `make test`, expect PASS. Then the full `make network-test` on emulator-5556 with the switch part and the mDNS check: every switch step passes; record each time in the report and compare with the Task 5 baseline.
- [ ] **Step 5: Commit.**
