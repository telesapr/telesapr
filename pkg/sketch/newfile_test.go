package sketch_test

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/fixtures"
	"github.com/telesapr/telesapr/pkg/internal/golden"
	"github.com/telesapr/telesapr/pkg/persist/tlspr"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// TestNewFile checks that a new file, generated as AfterNewFile does,
// saves to the golden, testdata/newfile.tlspr.
func TestNewFile(t *testing.T) {
	t.Parallel()
	sk := sketch.NewSketch()
	sk.NewFile()
	sk.GenerateAll()
	golden.Check(t, fixtures.Path("newfile.tlspr"),
		tlspr.Encode(sk, tlspr.Options{}))
}
