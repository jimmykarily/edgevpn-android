SHELL := /bin/bash
.DEFAULT_GOAL := all

# ── Configuration ────────────────────────────────────────────────
BINARIES       := $(CURDIR)/binaries
ANDROID_HOME   ?= $(BINARIES)/android-sdk

GRADLE_VER     := 8.7
CMDTOOLS_VER   := 11076708
PLATFORM_VER   := android-34
BUILDTOOLS_VER := 34.0.0
NDK_VER        := 26.1.10909125
MIN_SDK        := 26

# JDK 17 for Android builds. Always use our own to avoid version mismatches.
# Override on the command line if you have JDK 17+: make JAVA_HOME=/path/to/jdk17
JAVA_HOME := $(BINARIES)/jdk-17
export JAVA_HOME
export ANDROID_HOME

# ── Tool resolution (prefer PATH, fall back to binaries/) ───────
GRADLE     := $(BINARIES)/gradle-$(GRADLE_VER)/bin/gradle
SDKMANAGER := $(ANDROID_HOME)/cmdline-tools/latest/bin/sdkmanager

ADB      := $(or $(shell command -v adb 2>/dev/null),$(ANDROID_HOME)/platform-tools/adb)

# gomobile and gobind generate code against golang.org/x/mobile/bind, so they
# must be the same version as the one golib/go.mod requires. They are
# installed into a directory named for that version, which reinstalls them
# whenever go.mod moves to another one.
MOBILE_VER := $(shell cd golib && go list -m -f '{{.Version}}' golang.org/x/mobile)
MOBILE_BIN := $(BINARIES)/gomobile-$(MOBILE_VER)
GOMOBILE   := $(MOBILE_BIN)/gomobile

# ── Outputs ──────────────────────────────────────────────────────
GOLIB_AAR  := app/libs/edgevpn.aar
APK        := app/build/outputs/apk/debug/app-debug.apk
PREVIEW_APK := app/build/outputs/apk/preview/app-preview.apk
GO_SOURCES  := $(wildcard golib/*.go) golib/go.mod golib/go.sum
APP_SOURCES := $(shell find app/src -type f 2>/dev/null)
SDK_STAMP   := $(ANDROID_HOME)/platform-tools/adb

# Delete targets whose recipes fail, so stale/empty files don't block rebuilds.
.DELETE_ON_ERROR:

# ── Phony targets ────────────────────────────────────────────────
.PHONY: all golib app preview release test install logs setup clean distclean

all: app

# ── JDK 17 (Eclipse Temurin) ────────────────────────────────────
$(JAVA_HOME)/bin/java:
	@echo "==> Downloading JDK 17 (Eclipse Temurin)..."
	@mkdir -p $(BINARIES)
	curl -sSL -o /tmp/_jdk17.tar.gz \
	  "https://api.adoptium.net/v3/binary/latest/17/ga/linux/x64/jdk/hotspot/normal/eclipse?project=jdk"
	@mkdir -p /tmp/_jdk17
	tar xzf /tmp/_jdk17.tar.gz -C /tmp/_jdk17/
	@rm -rf $(JAVA_HOME)
	mv /tmp/_jdk17/jdk-17.* $(JAVA_HOME)
	@rm -rf /tmp/_jdk17 /tmp/_jdk17.tar.gz

# ── Android SDK ──────────────────────────────────────────────────
$(SDKMANAGER):
	@echo "==> Downloading Android command-line tools..."
	@mkdir -p $(ANDROID_HOME)/cmdline-tools
	curl -sSL -o /tmp/_cmdtools.zip \
	  "https://dl.google.com/android/repository/commandlinetools-linux-$(CMDTOOLS_VER)_latest.zip"
	@mkdir -p /tmp/_cmdtools
	unzip -q /tmp/_cmdtools.zip -d /tmp/_cmdtools
	mv /tmp/_cmdtools/cmdline-tools $(ANDROID_HOME)/cmdline-tools/latest
	@rm -rf /tmp/_cmdtools /tmp/_cmdtools.zip

$(SDK_STAMP): | $(SDKMANAGER) $(JAVA_HOME)/bin/java
	@echo "==> Installing SDK packages..."
	@yes | $(SDKMANAGER) --sdk_root=$(ANDROID_HOME) --licenses >/dev/null 2>&1 || true
	$(SDKMANAGER) --sdk_root=$(ANDROID_HOME) \
	  "platform-tools" \
	  "build-tools;$(BUILDTOOLS_VER)" \
	  "platforms;$(PLATFORM_VER)" \
	  "ndk;$(NDK_VER)"

# ── Gradle ───────────────────────────────────────────────────────
$(GRADLE):
	@echo "==> Downloading Gradle $(GRADLE_VER)..."
	@mkdir -p $(BINARIES)
	curl -sSL -o /tmp/_gradle.zip \
	  "https://services.gradle.org/distributions/gradle-$(GRADLE_VER)-bin.zip"
	unzip -q /tmp/_gradle.zip -d $(BINARIES)/
	@rm -f /tmp/_gradle.zip

# ── gomobile ─────────────────────────────────────────────────────
$(GOMOBILE):
	@echo "==> Installing gomobile and gobind $(MOBILE_VER)..."
	GOBIN=$(MOBILE_BIN) go install golang.org/x/mobile/cmd/gomobile@$(MOBILE_VER)
	GOBIN=$(MOBILE_BIN) go install golang.org/x/mobile/cmd/gobind@$(MOBILE_VER)

# ── Go library → AAR ────────────────────────────────────────────
# -checklinkname=0: the anet library (transitive dep via libp2p) uses
# //go:linkname to patch Go's internal net.zoneCache for Android DNS fixes.
# Go 1.23+ rejects this by default. Disable the check until anet is fixed
# upstream: https://github.com/wlynxg/anet/issues/11
$(GOLIB_AAR): $(GO_SOURCES) | $(GOMOBILE) $(SDK_STAMP)
	@echo "==> Building Go library (gomobile bind)..."
	@mkdir -p app/libs
	cd golib && \
	  PATH=$(MOBILE_BIN):$(JAVA_HOME)/bin:$$PATH \
	  ANDROID_NDK_HOME=$(ANDROID_HOME)/ndk/$(NDK_VER) \
	  $(GOMOBILE) bind -target android/arm64 -androidapi $(MIN_SDK) \
	    -ldflags="-checklinkname=0" \
	    -o $(CURDIR)/$(GOLIB_AAR) .

golib: $(GOLIB_AAR)

# ── Android APK ──────────────────────────────────────────────────
$(APK): $(GOLIB_AAR) $(APP_SOURCES) build.gradle.kts app/build.gradle.kts settings.gradle.kts | $(GRADLE) $(SDK_STAMP)
	@echo "sdk.dir=$(ANDROID_HOME)" > local.properties
	@echo "==> Building Android APK..."
	$(GRADLE) -p $(CURDIR) assembleDebug

app: $(APK)

# The preview and release APKs are always rebuilt rather than tracked as file
# targets, because what they contain depends on environment variables (the
# signing key and EDGEVPN_VERSION, see app/build.gradle.kts) that make cannot
# see change.
preview: $(GOLIB_AAR) | $(GRADLE) $(SDK_STAMP)
	@echo "sdk.dir=$(ANDROID_HOME)" > local.properties
	@echo "==> Building preview APK..."
	$(GRADLE) -p $(CURDIR) assemblePreview
	@echo "==> $(PREVIEW_APK)"

release: $(GOLIB_AAR) | $(GRADLE) $(SDK_STAMP)
	@echo "sdk.dir=$(ANDROID_HOME)" > local.properties
	@echo "==> Building release APK..."
	$(GRADLE) -p $(CURDIR) assembleRelease
	@ls app/build/outputs/apk/release/*.apk

# ── Tests ────────────────────────────────────────────────────────
# The Go tests run on the build host. The app's unit tests run on the JVM
# and need the AAR, because the app module links against it.
test: $(GOLIB_AAR) | $(GRADLE) $(SDK_STAMP)
	cd golib && go test ./...
	@echo "sdk.dir=$(ANDROID_HOME)" > local.properties
	$(GRADLE) -p $(CURDIR) testDebugUnitTest

# ── Install on device ────────────────────────────────────────────
install: $(APK)
	@echo "==> Installing APK on connected device..."
	-$(ADB) shell am force-stop com.kairos.edgevpn
	$(ADB) install -r $(APK)

# ── Logs from device ─────────────────────────────────────────────
logs:
	$(ADB) logcat -v time --pid=$$($(ADB) shell pidof -s com.kairos.edgevpn)

# ── Convenience ──────────────────────────────────────────────────
setup: $(JAVA_HOME)/bin/java $(SDK_STAMP) $(GRADLE) $(GOMOBILE)
	@echo "==> All tools ready."

clean:
	rm -rf $(GOLIB_AAR) app/build .gradle build local.properties
	@echo "==> Clean."

distclean: clean
	rm -rf $(BINARIES)
	@echo "==> Distclean (removed binaries/)."
