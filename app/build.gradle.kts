plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

// The path to the release keystore, read once so the signing config and the
// build types that use it agree on whether one is present. Only CI sets it
// (see .github/workflows and the README); a local build without it signs
// preview builds with the local debug key and leaves the release APK unsigned.
val releaseKeystorePath = System.getenv("EDGEVPN_RELEASE_KEYSTORE")

// The release version, "major.minor.patch", read from EDGEVPN_VERSION, which
// the release workflow sets from the git tag. Android only installs an update
// whose versionCode is higher than the installed one, so the code is computed
// from the same three numbers: major * 10000 + minor * 100 + patch, which
// makes 1.2.3 into 10203. That only keeps versions apart while every part is
// under 100, so a larger part fails the build instead of producing a code
// that collides with another version. Without the variable, or with a value
// that is not three numbers, the fallback below is used.
val fallbackVersionName = "0.1.0"
val fallbackVersionCode = 1

fun releaseVersion(): Pair<String, Int> {
    val raw = System.getenv("EDGEVPN_VERSION") ?: return fallbackVersionName to fallbackVersionCode
    val parts = raw.split(".")
    val numbers = parts.mapNotNull { it.toIntOrNull() }
    if (parts.size != 3 || numbers.size != 3) {
        return fallbackVersionName to fallbackVersionCode
    }
    val (major, minor, patch) = numbers
    for ((name, value) in listOf("major" to major, "minor" to minor, "patch" to patch)) {
        if (value !in 0..99) {
            error(
                "EDGEVPN_VERSION '$raw' has a $name part of $value. " +
                    "Each version part must be under 100, or its version code " +
                    "collides with another version."
            )
        }
    }
    return raw to (major * 10_000 + minor * 100 + patch)
}

val (releaseVersionName, releaseVersionCode) = releaseVersion()

android {
    namespace = "com.kairos.edgevpn"
    compileSdk = 34

    defaultConfig {
        applicationId = "com.kairos.edgevpn"
        minSdk = 26
        targetSdk = 34
        versionCode = releaseVersionCode
        versionName = releaseVersionName
        manifestPlaceholders["appLabel"] = "EdgeVPN"
    }

    signingConfigs {
        if (releaseKeystorePath != null) {
            create("release") {
                storeFile = file(releaseKeystorePath)
                storePassword = System.getenv("EDGEVPN_RELEASE_KEYSTORE_PASSWORD")
                keyAlias = System.getenv("EDGEVPN_RELEASE_KEY_ALIAS")
                keyPassword = System.getenv("EDGEVPN_RELEASE_KEY_PASSWORD")
            }
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            if (releaseKeystorePath != null) {
                signingConfig = signingConfigs.getByName("release")
            }
        }

        // The build CI makes for every pull request and every push to main,
        // to install on a phone from a browser. It is a debug build under its
        // own application id, so it sits next to the released app instead of
        // replacing it. It is signed with the release key so each preview
        // installs over the previous one: the phone refuses an update signed
        // with a different key, and every CI runner has its own debug key.
        create("preview") {
            initWith(getByName("debug"))
            applicationIdSuffix = ".preview"
            versionNameSuffix = "-preview"
            manifestPlaceholders["appLabel"] = "EdgeVPN preview"
            matchingFallbacks += listOf("debug")
            if (releaseKeystorePath != null) {
                signingConfig = signingConfigs.getByName("release")
            }
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    kotlinOptions {
        jvmTarget = "17"
    }
}

dependencies {
    implementation(files("libs/edgevpn.aar"))
    implementation("androidx.core:core-ktx:1.12.0")
    implementation("androidx.appcompat:appcompat:1.6.1")
    implementation("com.google.android.material:material:1.11.0")
    implementation("androidx.security:security-crypto:1.1.0-alpha06")

    testImplementation("junit:junit:4.13.2")
}
