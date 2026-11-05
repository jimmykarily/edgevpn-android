package edgevpn

import "runtime/debug"

// LibraryVersion returns the version of the edgevpn library this build links
// against, or "unknown" when the build carries no module information.
func LibraryVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, dep := range info.Deps {
		if dep.Path == "github.com/mudler/edgevpn" && dep.Version != "" {
			return dep.Version
		}
	}
	return "unknown"
}
