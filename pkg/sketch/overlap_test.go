package sketch_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// What lies at a point: the other points there, and the curves through
// it with no point of their own there.
func TestOverlap(t *testing.T) {
	t.Parallel()
	d := api.New()
	a := d.Sketch.Line(api.V(0, 0, 0), api.V(20, 0, 0))
	b := d.Sketch.Line(api.V(20, 0, 0), api.V(20, 10, 0))
	arc := d.Sketch.Arc(api.V(30, 0, 0), api.V(20, 0, 0), api.V(30, -10, 0))
	across := d.Sketch.Line(api.V(10, -5, 0), api.V(10, 5, 0))
	circle := d.Sketch.Circle(api.V(20, 20, 0), 10)
	mid := d.Sketch.Point(api.V(10, 0, 0))

	handles := func(es []api.Entity) []core.HEntity {
		var hs []core.HEntity
		for _, e := range es {
			hs = append(hs, e.H)
		}
		return hs
	}
	for _, tc := range []struct {
		name           string
		at             api.Entity
		points, curves []api.Entity
	}{
		// Three ends meet; the curves that end there are no rows of
		// their own.
		{"corner", a.End(), []api.Entity{b.Start(), arc.Start()}, nil},
		{"arc start", arc.Start(), []api.Entity{a.End(), b.Start()}, nil},
		// The datum point lies on two lines that don't end there.
		{"crossing", mid, nil, []api.Entity{a, across}},
		{"on circle", b.End(), nil, []api.Entity{circle}},
		{"alone", across.Start(), nil, nil},
	} {
		points, curves := tc.at.Overlap()
		if !slices.Equal(handles(points), handles(tc.points)) {
			t.Errorf("%s: points %v, want %v", tc.name, points, tc.points)
		}
		if !slices.Equal(handles(curves), handles(tc.curves)) {
			t.Errorf("%s: curves %v, want %v", tc.name, curves, tc.curves)
		}
	}

	// The origin of the sketch's workplane, and of the three reference
	// planes, are points too.
	if points, _ := a.Start().Overlap(); len(points) != 4 {
		t.Errorf("at the origin: %v", points)
	}

	sk := d.Host().(*api.SketchHost).Sketch()
	if _, err := sk.Overlap(a.H); !errors.Is(err, sketch.ErrNotPoint) {
		t.Errorf("a line: %v, want ErrNotPoint", err)
	}
	if _, err := sk.Overlap(core.HEntity{V: 0x7fff0000}); err == nil {
		t.Error("no such entity: no error")
	}
}

// What a later group than the active one draws isn't listed: it isn't
// drawn either.
func TestOverlapAfterActive(t *testing.T) {
	t.Parallel()
	d := api.New()
	a := d.Sketch.Line(api.V(0, 0, 0), api.V(20, 0, 0))
	first := d.Active()
	d.Features.SketchIn(d.Sketch.ActiveWorkplane(), "second")
	b := d.Sketch.Line(api.V(20, 0, 0), api.V(20, 10, 0))
	if points, _ := a.End().Overlap(); len(points) != 1 || points[0].H != b.Start().H {
		t.Fatalf("with the second group active: %v", points)
	}
	d.Activate(first)
	if points, _ := a.End().Overlap(); len(points) != 0 {
		t.Errorf("with the first group active: %v", points)
	}
}
