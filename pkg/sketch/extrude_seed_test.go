package sketch

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// TestExtrudeSeed starts a new extrusion's solve along the view's
// normal, unless its workplane is seen edge on: then along the
// workplane's normal, of the same length.
func TestExtrudeSeed(t *testing.T) {
	t.Parallel()
	sk := NewSketch()
	sk.NewFile()
	sk.GenerateAll()
	view := geom.Vector{Z: 7} // the default view looks down z
	for _, c := range []struct {
		name    string
		plane   core.HRequest
		subtype GroupSubtype
		want    geom.Vector
	}{
		{"facing the view", HRequestReferenceXY, GroupOneSided, view},
		{"edge on", HRequestReferenceZX, GroupOneSided, geom.Vector{Y: 7}},
		{"edge on, skewed", HRequestReferenceZX, GroupOneSkewed, view},
	} {
		g := Group{Type: GroupExtrude, Subtype: c.subtype}
		g.Predef.EntityB = c.plane.Entity(0)
		got := g.extrudeSeed(sk, view)
		// Either way along the normal will do.
		got.Y = max(got.Y, -got.Y)
		if !got.Equals(c.want) {
			t.Errorf("%s: seed %v, want %v", c.name, got, c.want)
		}
	}
}
