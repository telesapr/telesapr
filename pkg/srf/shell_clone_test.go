package srf

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

// A copy is followed at its original's tolerance: left unset, a Boolean
// or a transformation of it falls back to DefaultTolerance, and a link
// read from the cache gave its surfaces 0.1 mm where the file read the
// first time gave them its own.
func TestCloneKeepsTolerance(t *testing.T) {
	t.Parallel()
	tol := Tol(0.003, 40)
	sh := extrudedBox(geom.Vector{}, geom.Vector{X: 1, Y: 1, Z: 1}, tol)
	c := sh.Clone()
	if c.Tol != tol {
		t.Fatalf("the copy's tolerance is %+v, the original's %+v", c.Tol, tol)
	}
	var moved SShell
	moved.MakeFromTransformationOf(&c, geom.Vector{X: 1}, geom.QuaternionIdentity, 1)
	for i := range moved.Surface {
		if got := moved.Surface[i].Tol.ChordTolMm(); got != 0.003 {
			t.Fatalf("surface %d of the moved copy at %g mm", i, got)
		}
	}
}
