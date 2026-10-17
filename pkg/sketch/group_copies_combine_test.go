package sketch_test

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// With flip, an extrude with no constraints turns round when it passes
// between adding and cutting; without flip it stays; a mode that does
// not exist is refused and changes nothing.
func TestSetGroupCombineFlip(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Sketch.Rect(api.V(0, 0, 0), api.V(2, 2, 0))
	ex := d.Features.Extrude(2, "")
	sk := d.Host().Sketch()
	g := sk.Group(ex.H)
	z := func() float64 { return g.ExtrusionGetVector(sk).Z }
	before := z()
	steps := []struct {
		as   sketch.CombineAs
		flip bool
		want float64
	}{
		{sketch.CombineDifference, false, before},
		{sketch.CombineUnion, true, -before},
		{sketch.CombineDifference, true, before},
		{sketch.CombineIntersection, true, before},
		{sketch.CombineAssemble, true, -before},
	}
	for _, s := range steps {
		if err := sk.SetGroupCombine(g.H, s.as, s.flip); err != nil {
			t.Fatal(err)
		}
		if got := z(); got != s.want || g.MeshCombine != s.as {
			t.Errorf("mode %d flip %v: vector z %g, want %g", s.as, s.flip, got, s.want)
		}
	}
	if err := sk.SetGroupCombine(g.H, sketch.CombineAs(9), true); err == nil || g.MeshCombine != sketch.CombineAssemble {
		t.Errorf("an unknown mode gave %v, mode %d", err, g.MeshCombine)
	}
}
