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

## Current status

Skeleton only. The app compiles and installs but doesn't do anything yet -- the Go library exports stub functions and the Android UI shows a static "Not connected" label.
