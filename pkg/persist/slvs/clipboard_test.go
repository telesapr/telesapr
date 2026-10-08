package slvs

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// TestCopyPasteDelete copies a constrained line, pastes it moved,
// mirrored and rotated as SolveSpace's Paste Transformed does, and deletes the
// original.
func TestCopyPasteDelete(t *testing.T) {
	doc, err := LoadFile(fixture("request/line_segment/normal.slvs"))
	if err != nil {
		t.Fatal(err)
	}
	sk := doc.Sketch
	var line core.Entity
	for _, e := range sk.Entities {
		if e.Type == core.EntityLineSegment {
			line = e
		}
	}
	sk.Constrain(core.ConstraintHorizontal, core.NoEntity, core.NoEntity, line.H, core.NoEntity, false, false)
	sk.Refresh()
	pt := func(h core.HEntity) geom.Vector { return sk.MustEntity(h).PointGetNum(sk) }
	a, b := pt(line.Point[0]), pt(line.Point[1])

	sk.CopySelection([]sketch.Selection{{Entity: line.H}})
	if len(sk.Clipboard.Requests) != 1 || len(sk.Clipboard.Constraints) != 1 {
		t.Fatalf("clipboard: %d requests, %d constraints; want 1, 1",
			len(sk.Clipboard.Requests), len(sk.Clipboard.Constraints))
	}

	for _, tc := range []struct {
		name         string
		trans        geom.Vector
		theta, scale float64
		want         [2]geom.Vector
		constraint   core.ConstraintType
	}{
		{"moved", geom.Vector{X: 10}, 0, 1, [2]geom.Vector{a.Plus(geom.Vector{X: 10}), b.Plus(geom.Vector{X: 10})}, core.ConstraintHorizontal},
		{"mirrored", geom.Vector{}, 0, -1, [2]geom.Vector{{X: -a.X, Y: a.Y}, {X: -b.X, Y: b.Y}}, core.ConstraintHorizontal},
		{"rotated", geom.Vector{}, math.Pi / 2, 1, [2]geom.Vector{{X: -a.Y, Y: a.X}, {X: -b.Y, Y: b.X}}, core.ConstraintVertical},
	} {
		before := len(sk.Constraints)
		sel := sk.PasteClipboard(tc.trans, tc.theta, tc.scale)
		if len(sel) != 3 {
			t.Fatalf("%s: selected %d, want the line and its points", tc.name, len(sel))
		}
		sk.Refresh()
		for i := range 2 {
			if got := pt(sel[1+i].Entity); !got.EqualsTol(tc.want[i], 1e-9) {
				t.Errorf("%s: point %d at %v, want %v", tc.name, i, got, tc.want[i])
			}
		}
		if len(sk.Constraints) != before+1 {
			t.Fatalf("%s: %d constraints, want %d", tc.name, len(sk.Constraints), before+1)
		}
		c := sk.Constraints[len(sk.Constraints)-1]
		if c.Type != tc.constraint || c.EntityA != sel[0].Entity {
			t.Errorf("%s: pasted constraint %v on %v, want %v on %v", tc.name, c.Type, c.EntityA, tc.constraint, sel[0].Entity)
		}
	}

	// Deleting the original line deletes its constraint too.
	var msg string
	sk.Hooks = sketch.Messages(func(m string) { msg = m })
	n := len(sk.Constraints)
	sk.DeleteSelection([]sketch.Selection{{Entity: line.H}})
	if sk.Entity(line.H) != nil || len(sk.Constraints) != n-1 {
		t.Errorf("after delete: line %v, %d constraints (want %d)", sk.Entity(line.H) != nil, len(sk.Constraints), n-1)
	}
	if msg != "" {
		t.Errorf("a pruned horizontal constraint isn't worth a message, got %q", msg)
	}
}
