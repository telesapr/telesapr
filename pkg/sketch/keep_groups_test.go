package sketch_test

import (
	"math"
	"path/filepath"
	"slices"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// A model built a step at a time, each step regenerating from the
// group it changes and keeping the entities and params of the groups
// before (those of its requests, its constraints and its own), has the entities and params a regeneration of the whole file
// read back makes: the same handles, and where they are to the last
// digits a solve again may move.
func TestKeptGroupsAsRegenerated(t *testing.T) {
	t.Parallel()
	d := api.New()
	var lengths []api.Constraint
	for i := range 6 {
		x := float64(i) * 20
		d.Features.SketchIn(d.Workplane("XY"), "")
		sides := d.Sketch.Polyline(true, api.V(x, 0, 0), api.V(x+10, 0, 0), api.V(x+10, 10, 0), api.V(x, 10, 0))
		hole := d.Sketch.Circle(api.V(x+5, 5, 0), 2)
		// A point on a line is a constraint with a param of its own.
		diagonal := d.Sketch.Line(api.V(x, 0, 0), api.V(x+10, 10, 0))
		diagonal.SetConstruction(true)
		d.Constrain.On(hole.Center(), diagonal)
		lengths = append(lengths, d.Constrain.Length(sides[0], 10))
		d.Features.Extrude(float64(3+i), "").SetCombine(api.Assemble)
	}
	// An edit in the middle of the history, then one at the end.
	lengths[2].Set(14)
	lengths[5].Set(12)
	sk := d.Host().Sketch()
	sk.Refresh()
	// The save regenerates everything, so the lists are taken first.
	entities, params := slices.Clone(sk.Entities), slices.Clone(sk.Params)

	path := filepath.Join(t.TempDir(), "steps.tlspr")
	if err := d.File.Save(path); err != nil {
		t.Fatal(err)
	}
	read, err := api.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	want := read.Host().Sketch()
	if len(entities) != len(want.Entities) || len(params) != len(want.Params) {
		t.Fatalf("%d entities and %d params, want %d and %d", len(entities), len(params), len(want.Entities), len(want.Params))
	}
	for i := range entities {
		g, w := &entities[i], &want.Entities[i]
		if g.H != w.H || g.Type != w.Type || g.Group != w.Group || g.ActPoint.Minus(w.ActPoint).Magnitude() > 1e-9 {
			t.Fatalf("entity %d: %08x %v at %v, want %08x %v at %v", i, g.H.V, g.Type, g.ActPoint, w.H.V, w.Type, w.ActPoint)
		}
	}
	for i := range params {
		g, w := params[i], want.Params[i]
		if g.H != w.H || math.Abs(g.Val-w.Val) > 1e-9 {
			t.Fatalf("param %d: %08x = %g, want %08x = %g", i, g.H.V, g.Val, w.H.V, w.Val)
		}
	}
}
