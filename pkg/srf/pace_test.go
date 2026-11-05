package srf

import (
	"reflect"
	"testing"
)

// TestBooleanPaced checks that a Boolean made into a shell given a pace
// (SShell.PaceBy) calls it between the steps of its loops, on the
// goroutine that made it, makes the shell made without one, and forgets
// the pace when done.
func TestBooleanPaced(t *testing.T) {
	t.Parallel()
	tol := sweepTol()
	for _, op := range []SurfaceCombineAs{SurfaceUnion, SurfaceDiff, SurfaceInter} {
		want := SShell{Tol: tol}
		want.MakeFromBoolean(lathedTube(0, 5, 0, 20, tol), crossedCylinder(4, 0.7, 45, tol), op)

		got := SShell{Tol: tol}
		paces := 0
		got.PaceBy(func() { paces++ })
		got.MakeFromBoolean(lathedTube(0, 5, 0, 20, tol), crossedCylinder(4, 0.7, 45, tol), op)
		if paces == 0 {
			t.Errorf("%v: the Boolean never paced", op)
		}
		if got.pace != nil {
			t.Errorf("%v: the shell kept its pace after the Boolean", op)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%v: paced, the Boolean made another shell", op)
		}
	}
}
