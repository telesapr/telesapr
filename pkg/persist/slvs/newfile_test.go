package slvs_test

import (
	"path/filepath"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/golden"
	"github.com/telesapr/telesapr/pkg/persist/tlspr"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// TestNewFile checks that a new file, generated as AfterNewFile does,
// saves to the golden (testdata/tlspr/golden/newfile.tlspr; until the SolveSpace
// comparison was frozen, the .slvs save matched SolveSpace's byte for byte).
func TestNewFile(t *testing.T) {
	sk := sketch.NewSketch()
	sk.NewFile()
	sk.GenerateAll()
	golden.Check(t, filepath.Join("..", "..", "..", "testdata", "tlspr", "golden", "newfile.tlspr"),
		tlspr.Encode(sk, tlspr.Options{}))
}
