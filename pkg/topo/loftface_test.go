package topo_test

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// A box's sides are square to its top: along them a loft from the top
// leaves straight up, at an edge as at a corner; from no edge, or from
// the bottom, which has no side below it, it can't.
func TestSideTangents(t *testing.T) {
	t.Parallel()
	var box srf.SShell
	box.MakePrism([]geom.Vector{v(0, 0, 0), v(10, 0, 0), v(10, 10, 0), v(0, 10, 0)}, geom.Vector{}, v(0, 0, 10),
		core.NewRGBA(100, 100, 100, 255))
	up, err := topo.SideTangents(&box, v(0, 0, 1))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []geom.Vector{v(5, 0, 10), v(10, 4, 10), v(0, 0, 10)} {
		d, err := up(p)
		if err != nil {
			t.Fatalf("at %v: %v", p, err)
		}
		if d.Minus(v(0, 0, 1)).Magnitude() > 1e-9 {
			t.Errorf("at %v the sides lead %v, want straight up", p, d)
		}
	}
	if _, err := up(v(5, 5, 10)); err == nil {
		t.Error("a point inside the top found a side")
	}
	// From the bottom, looking down: the sides rise from it, away from
	// the loft.
	down, err := topo.SideTangents(&box, v(0, 0, -1))
	if err != nil {
		t.Fatal(err)
	}
	if d, err := down(v(5, 0, 0)); err != nil || math.Abs(d.Z+1) > 1e-9 {
		t.Errorf("from the bottom the sides lead %v, %v; want straight down", d, err)
	}
	if _, err := up(v(5, 0, 0)); err == nil {
		t.Error("a loft up from the bottom edge found a side below it")
	}
}
