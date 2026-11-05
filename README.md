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
| `make install` | Build + install APK on USB-connected device via adb |
| `make clean` | Remove build outputs |
| `make distclean` | Remove build outputs and all downloaded tools (`binaries/`) |

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
