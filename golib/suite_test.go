package edgevpn

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// TestEdgevpn runs the Ginkgo specs of this package. The plain Go tests in
// the other files run next to them under go test.
func TestEdgevpn(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "edgevpn golib")
}
