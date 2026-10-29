package srf

import (
	"errors"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// stackedBoxes is a 4 by 3 box from z 0 to 10, and on it one from 10 to
// 20 whose base is the rectangle side by side, assembled, the top of the
// first and the base of the second given origins 1 and 2.
func stackedBoxes(side float64) *SShell {
	col := core.NewRGBA(100, 100, 100, 255)
	pts := func(s float64) []geom.Vector {
		return []geom.Vector{{}, {X: 4 * s}, {X: 4 * s, Y: 3}, {Y: 3}}
	}
	var a, b SShell
	a.MakePrism(pts(1), geom.Vector{}, geom.Vector{Z: 10}, col)
	b.MakePrism(pts(side), geom.Vector{Z: 10}, geom.Vector{Z: 20}, col)
	mark := func(s *SShell, z float64, o uint32) {
		for i := range s.Surface {
			ss := &s.Surface[i]
			if ss.DegM == 1 && ss.DegN == 1 && ss.PointAt(0.5, 0.5).Z == z && ss.NormalAt(0.5, 0.5).Z != 0 {
				ss.Origin = o
			}
		}
	}
	mark(&a, 10, 1)
	mark(&b, 10, 2)
	var w SShell
	w.MakeFromAssemblyOf(&a, &b)
	return &w
}

// Two boxes standing on each other with the same outline are one box
// once the planes between them are taken out; with outlines that differ
// they are refused.
func TestJoinedAtCaps(t *testing.T) {
	t.Parallel()
	j, err := stackedBoxes(1).JoinedAtCaps(1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(j.Surface); n != 10 {
		t.Errorf("%d surfaces, want 10", n)
	}
	checkVolume(t, "the boxes joined", shellVolume(j), 240)
	if _, err := stackedBoxes(0.5).JoinedAtCaps(1, 2); !errors.Is(err, ErrCapsDiffer) {
		t.Errorf("outlines that differ: %v", err)
	}
	if _, err := stackedBoxes(1).JoinedAtCaps(1, 7); err == nil {
		t.Error("a cap that isn't there joined")
	}
}
