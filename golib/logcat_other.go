//go:build !android || !cgo

package edgevpn

// writeLogcat does nothing where there is no Android log device.
func writeLogcat(level, line string) {}
