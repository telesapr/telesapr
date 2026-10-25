package srf

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

func TestSameShell(t *testing.T) {
	t.Parallel()
	// A tube centred on x = 0 reflected in x is itself; moved off the
	// centre, it isn't, nor is a box of other size.
	tube := lathedTube(1, 5, 0, 20, Tolerance{})
	// Scaled by -1, then turned half a turn about x: reflected in x = 0.
	var refl SShell
	refl.MakeFromTransformationOf(tube, geom.Vector{}, geom.QuaternionFromAxisAngle(geom.Vector{X: 1}, math.Pi), -1)
	if !SameShell(tube, &refl) {
		t.Error("a tube reflected in a plane through its axis is not the same")
	}
	if !SameShell(tube, tube) {
		t.Error("a shell is not the same as itself")
	}
	var moved SShell
	moved.MakeFromTransformationOf(tube, geom.Vector{X: 1}, geom.QuaternionIdentity, 1)
	if SameShell(tube, &moved) {
		t.Error("a tube moved is the same")
	}
	if SameShell(extrudedBox(geom.Vector{}, geom.Vector{X: 1, Y: 1, Z: 1}, Tolerance{}), extrudedBox(geom.Vector{}, geom.Vector{X: 2, Y: 1, Z: 1}, Tolerance{})) {
		t.Error("boxes of other sizes are the same")
	}
	if SameShell(tube, extrudedBox(geom.Vector{}, geom.Vector{X: 1, Y: 1, Z: 1}, Tolerance{})) || SameShell(&SShell{}, &SShell{}) {
		t.Error("shells of other counts, or empty ones, are the same")
	}
}
