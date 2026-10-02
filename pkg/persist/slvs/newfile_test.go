package slvs_test

import (
	"path/filepath"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/internal/golden"
	"github.com/spacesolve/spacesolve/pkg/persist/spcs"
	"github.com/spacesolve/spacesolve/pkg/sketch"
)

// TestNewFile checks that a new file, generated as AfterNewFile does,
// saves to the golden (testdata/spcs/golden/newfile.spcs; until the C++
// comparison was frozen, the .slvs save matched C++'s byte for byte).
func TestNewFile(t *testing.T) {
	sk := sketch.NewSketch()
	sk.NewFile()
	sk.GenerateAll()
	golden.Check(t, filepath.Join("..", "..", "..", "testdata", "spcs", "golden", "newfile.spcs"),
		spcs.Encode(sk, spcs.Options{}))
}
