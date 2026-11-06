# EdgeVPN Android

An Android client for [EdgeVPN](https://github.com/mudler/edgevpn), enabling mobile devices to join EdgeVPN networks.

See [PLAN.md](PLAN.md) for architecture details and the full implementation roadmap.

## Prerequisites

- Go 1.26+
- Linux x86_64 (build host)
- USB-connected Android device (for `make install`)

Everything else (JDK 17, Android SDK/NDK, Gradle, gomobile) is downloaded automatically on first build.

## Project structure

```
edgevpn-android/
├── Makefile                        # Build orchestration
├── golib/                          # Go library (compiled to AAR via gomobile)
│   ├── go.mod
│   └── edgevpn.go                  # Exported API: Start(), Stop(), Status()
├── app/                            # Android app (Kotlin)
│   ├── build.gradle.kts
│   └── src/main/
│       ├── AndroidManifest.xml
│       └── kotlin/com/kairos/edgevpn/
│           └── MainActivity.kt
├── build.gradle.kts                # Root Gradle config
├── settings.gradle.kts
└── gradle.properties
```

## Build

```bash
make              # full build (downloads tools on first run, then builds AAR + APK)
make install      # build + push APK to USB-connected device
```

## Make targets

| Target | Description |
|--------|-------------|
| `make` / `make all` | Full build: download tools, build Go AAR, build APK |
| `make setup` | Download and install all tools only (JDK, SDK, Gradle, gomobile) |
| `make golib` | Build the Go library AAR (`app/libs/edgevpn.aar`) |
| `make app` | Build the Android debug APK |
| `make preview` | Build the preview APK (`com.kairos.edgevpn.preview`, installs next to a release) |
| `make release` | Build the release APK (signed only when the signing variables below are set) |
| `make test` | Run the scrub-key self-test, the Go tests and the app's JVM unit tests |
| `make lint` | Run Android lint on the debug build; fails on errors, warnings go to `app/build/reports/` |
| `make install` | Build + install APK on USB-connected device via adb |
| `make clean` | Remove build outputs |
| `make distclean` | Remove build outputs and all downloaded tools (`binaries/`) |

The **Unit tests** workflow (`.github/workflows/test.yml`) runs `make test` and `make lint` on every pull request and every push to `main`. It needs no device and no secrets.

## Emulator

A headless x86_64 emulator (AVD `edgevpn`, serial `emulator-5556`) runs the preview app without a phone. It uses the emulator and the Android 35 system image from `~/Android/Sdk` (override with `EDGEVPN_EMULATOR_SDK`) and this repo's own adb. The port can be changed with `EDGEVPN_AVD_PORT`.

| Target | Description |
|--------|-------------|
| `make emu-setup` | Install the emulator and system image, create the AVD (once) |
| `make emu-up` / `make emu-down` / `make emu-status` | Start headless and wait for boot, stop (saves a snapshot), report state |
| `make emu-run` | Build the preview APK with the emulator architecture, install and launch it |
| `make emu-shot` | Write `screenshot.png` |
| `make e2e` | Run the screen tests on the emulator (see below) |
| `make network-test` | Run the real-network check against the test peer (see below) |

`scripts/emulator` has more commands: `up-windowed`, `reset`, `install`, `launch`, `stop-app`, `shot [file]`, `ui` (view hierarchy as text; the text of editable fields is redacted to its length) and `logs`. The emulator reaches the internet through the host's NAT, never the host's local network.

The Go library is built for the architectures in `GOMOBILE_TARGETS` (default `android/arm64`, which is what phones and CI need). `make emu-run` sets it to `android/arm64,android/amd64`. Changing the value rebuilds the AAR, so building again with a different value takes a full gomobile bind.

## Screen tests

The tests in `app/src/androidTest` open the real screens on an emulator and check what a person sees: the main screen when the VPN is off, looking, connected and failing, a device sheet and a help sheet, a cut-off key on the Join screen, the address check and the Detailed logs switch in Settings, and an empty Logs screen. They show the main screen from a chosen network snapshot (set through `NetworkSource`) instead of a running node, so they need no network key and never join a network. The Join test checks a made-up key that it builds itself.

```bash
make emu-up   # start the emulator, once
make e2e      # build with android/amd64 added and run the tests on emulator-5556
```

`make e2e` runs `connectedDebugAndroidTest` on the device named by `ANDROID_SERIAL` (default `emulator-5556`) and turns off its animations first. It installs the debug build, `com.kairos.edgevpn`. Results are in `app/build/reports/androidTests/connected/`. The tests clear the debug app's stored settings, network key included, and Gradle uninstalls the app when they finish, so do not point `ANDROID_SERIAL` at a phone whose debug app you want to keep.

The **E2E tests** workflow (`.github/workflows/e2e.yml`) runs the same target on every pull request and every push to `main`, on an API 35 x86_64 emulator, and uploads the reports when a test fails. It uses no secrets.

## Test peer

Real-network checks need a second device in the network. `scripts/testpeer` runs one: a disposable QEMU VM (Debian 12 cloud image, KVM, 512 MB, headless) with edgevpn inside, at VPN address `10.1.0.10`. It needs `qemu-system-x86_64`, `qemu-img`, `xorriso`, `ssh` and a usable `/dev/kvm`, and no root: the VM uses QEMU user-mode networking.

| Target | Description |
|--------|-------------|
| `make peer-up` | Start the VM with a new temporary network key and wait until edgevpn is up |
| `make peer-down` | Stop the VM and delete the run directory, including the key |
| `make peer-status` | Whether the VM and edgevpn are running, and the VPN interface |

`scripts/testpeer` also has `ping <addr>` (ping from inside the VM), `logs` (the VM's edgevpn log), `serial` (its console log), `scrub` (stdin to stdout without the key), `key` (the path of the key file, never its contents) and `addr` (the VM's libp2p address as the emulator reaches it).

The first `up` downloads the cloud image and builds edgevpn at the version `golib/go.mod` requires, both into `binaries/testpeer/`. Each run boots from a fresh overlay disk and generates its own key into `binaries/testpeer/run/token`; the owner's network key is never used. edgevpn in the VM runs with an ownership TTL of 60 seconds instead of the default 8 minutes, so it forgets a device that has been silent for about a minute. The host forwards `127.0.0.1:40022` to the VM's SSH and `127.0.0.1:40001` to its libp2p port (override with `TESTPEER_SSH_PORT` and `TESTPEER_P2P_PORT`).

## Network test

`make network-test` checks the app on a real network: the debug app on the emulator joins the test peer's network, the VPN network has no unreachable IPv6 route (which would block IPv6 for every app), both sides ping each other, and the VPN is turned off for 90 seconds (longer than the peer's ownership TTL, so the peer drops the app from its machine list) and on again, after which pings must work again and the app must still have exactly two listening sockets. With Android's "Always-on VPN, block connections without VPN" IPv6 may now bypass the VPN; that is acceptable because the VPN carries only 10.1.0.0/24, and it is untested. The off and on part guards against an old libp2p host staying alive after the VPN stops.

```bash
make emu-up         # start the emulator, once
make network-test   # build, start the peer if needed, run the check
```

The script, `scripts/network-test`, stores the peer's temporary key, the address `10.1.0.4` and the peer's libp2p address in the app through an instrumented helper (`NetworkTestSetup`), grants the VPN permission with `appops`, and drives the main screen through `adb`. The app dials the peer directly at `10.0.2.2` (the host as the emulator sees it) instead of looking for it through the public DHT, so it connects in seconds. Because the peer forgets a device after 60 seconds without a heartbeat, the test also makes the app send one every 15 seconds instead of every 120. Neither setting has a screen in the app.

It writes screenshots, the app's log and the peer's edgevpn and console logs into `build/network-test/`. Every log it shows or writes (not the screenshots) goes through `scripts/scrub-key` first, which removes the key, the secrets inside it and any 12-character piece of either, and which prints nothing if it has no key to scrub with (`make test` runs its self-test). Screenshots are not scrubbed. They show only the app's screens for this run's throwaway key, and the Join screen, the one screen a key can be typed into, blocks screenshots. It clears the app's data at the end so the key does not stay on the device. A peer it started is stopped again. `NETWORK_TEST_LONG=0` skips the off/on part for a quick local run; CI always runs it. Before that part, the script switches the emulator's network: Wi-Fi off, Wi-Fi on, and airplane mode on for 10 seconds then off. After each step both sides must ping each other again, and the script logs `switch <step>: <n>s`. A step fails above `NETWORK_TEST_SWITCH_TIMEOUT` seconds (default 60), and `NETWORK_TEST_SWITCH=0` skips these steps. The script also checks the app's mDNS, the local discovery that finds peers on the same Wi-Fi: while Wi-Fi is off and only mobile data is left, no fresh `mdns(android): using interfaces:` log line may appear, and after Wi-Fi is back a fresh one must list `wlan0`. These checks run by default; `NETWORK_TEST_MDNS_CHECK=0` skips them. Wi-Fi is switched on and airplane mode off again when the script ends, also after a failure.

The test also guards the app's calls to `VpnService.protect`, which keeps the app's own sockets out of the VPN. They run on the dialing thread. In the network test runs protect never failed off the main thread on an API 35 emulator; vendor phone builds are untested. The script enlarges the emulator's log buffer and fails if the app logged `VpnService.protect(...) returned false`, once after the first connect and once at the end. That log line is also the sign to look for on a real phone.

The **Network tests** workflow (`.github/workflows/network.yml`) runs `make network-test` on every pull request and every push to `main`, and uploads `build/network-test/` when it fails. It uses no secrets.

## Auto-downloaded tools

On first build, the Makefile downloads these into `binaries/` (gitignored):

- **JDK 17** (Eclipse Temurin) -- Android Gradle Plugin requires JDK 17+
- **Android SDK** -- command-line tools, platform-tools (adb), build-tools 34.0.0, platform API 34, NDK 26.1
- **Gradle 8.7**
- **gomobile + gobind** -- installed to `binaries/gomobile-<version>/`, at the `golang.org/x/mobile` version that `golib/go.mod` requires

To use your own Android SDK instead of downloading: `make ANDROID_HOME=/path/to/sdk`

## APKs from CI

Two GitHub Actions workflows build APKs that install on a phone:

- **Preview** (`.github/workflows/preview.yml`) runs on every pull request and every push to `main`. The run's summary page has an "Install on a phone" link to the APK, which a phone's browser can download and install directly. The preview is a debug build named "EdgeVPN preview" with the application id `com.kairos.edgevpn.preview`, so it installs next to a released app instead of replacing it. Previews are kept for 14 days.
- **Release** (`.github/workflows/release.yml`) runs when a tag starting with `v` is pushed, for example `v0.2.0`. It builds a release APK whose version comes from the tag (`0.2.0`, version code `200`), and attaches it to a GitHub release named after the tag, using the tag's message as release notes. Each part of the version must be under 100.

```bash
git tag -a v0.2.0 -m "What changed in this release"
git push origin v0.2.0
```

Both workflows run the same `make` targets as a local build, and cache `binaries/` between runs.

## Signing

Android only installs an update over an installed app when both are signed with the same key. Every CI build is signed with one release key, so each preview installs over the previous preview, and each release over the previous release.

The key is a PKCS12 keystore with one RSA 4096 key under the alias `edgevpn`. It was generated on 2026-11-05 and is valid for 10000 days. Its certificate's SHA-256 fingerprint is:

```
F0:31:8D:E9:12:35:C1:72:1E:58:DB:2C:61:A6:56:AC:1D:04:1A:55:21:4A:5B:7B:A3:D0:A6:9B:A0:12:31:30
```

The original copy lives outside the repository, in `~/.local/share/edgevpn-android/signing/` on the machine it was generated on:

| File | Contents |
|------|----------|
| `release.keystore` | The keystore |
| `keystore-password` | The password of both the keystore and the key (PKCS12 uses one password for both) |
| `key-alias` | `edgevpn` |

**Back this directory up somewhere safe.** If the key is lost, a new key cannot produce updates for installed apps: every phone has to uninstall the app, losing its settings, before installing a build signed with a new key.

CI reads the key from these repository secrets (Settings, Secrets and variables, Actions):

| Secret | Value |
|--------|-------|
| `EDGEVPN_RELEASE_KEYSTORE_BASE64` | `release.keystore`, base64 encoded |
| `EDGEVPN_RELEASE_KEYSTORE_PASSWORD` | Contents of `keystore-password` |
| `EDGEVPN_RELEASE_KEY_PASSWORD` | Contents of `keystore-password` |

The alias is set in the workflows as a plain value rather than a secret. GitHub replaces every occurrence of a secret's value in the logs with `***`, and `edgevpn` appears in package names, paths and APK names.

To set the secrets again, for example after moving the key to a new machine, without printing any value:

```bash
D=~/.local/share/edgevpn-android/signing
R=jimmykarily/edgevpn-android
base64 -w0 "$D/release.keystore" | gh secret set EDGEVPN_RELEASE_KEYSTORE_BASE64 -R $R
gh secret set EDGEVPN_RELEASE_KEYSTORE_PASSWORD -R $R < "$D/keystore-password"
gh secret set EDGEVPN_RELEASE_KEY_PASSWORD -R $R < "$D/keystore-password"
```

A pull request that gets no secrets, such as one from Dependabot, still builds a preview, signed with the runner's debug key. That APK only installs on a phone that has no preview installed yet. The release workflow refuses to run without the secrets.

To build a signed APK locally, set the same values as environment variables:

```bash
D=~/.local/share/edgevpn-android/signing
export EDGEVPN_RELEASE_KEYSTORE=$D/release.keystore
export EDGEVPN_RELEASE_KEY_ALIAS=edgevpn
export EDGEVPN_RELEASE_KEYSTORE_PASSWORD="$(cat $D/keystore-password)"
export EDGEVPN_RELEASE_KEY_PASSWORD="$(cat $D/keystore-password)"
EDGEVPN_VERSION=0.2.0 make release
```

Without these variables, `make preview` signs with the local debug key and `make release` produces an unsigned APK.

## Current status

The phone joins an EdgeVPN network, appears in the web UI and routes traffic over Wi-Fi. Reliable operation on mobile data and across network changes is tracked in [issue #1](https://github.com/jimmykarily/edgevpn-android/issues/1).
