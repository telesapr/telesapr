package sketch_test

import (
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// madeName names what group hg made from in as copy n.
func madeName(t *testing.T, sk *sketch.Sketch, hg core.HGroup, in core.HEntity, n int) string {
	t.Helper()
	id, ok := sk.Group(hg).Remap[core.EntityKey{Input: in, CopyNumber: n}]
	if !ok {
		t.Fatalf("g%03x made nothing from %08x as copy %d", hg.V, in.V, n)
	}
	return sk.EntityName(hg.Entity(int(id.V)))
}

// TestEntityNameExtrusion: what an extrusion makes is named by what it
// is, where on the solid, and what it was made from; no handle shows.
func TestEntityNameExtrusion(t *testing.T) {
	t.Parallel()
	d := api.New()
	ex := block(d, 0, 0, 20, 10, 10)
	top := ex.Solid().FaceAt(api.V(10, 5, 10))
	side := ex.Solid().FaceAt(api.V(10, 0, 5))
	d.With(func(sk *sketch.Sketch) {
		line := core.HRequest{V: 4}.Entity(0)
		for _, c := range []struct{ got, want string }{
			{sk.FaceName(srf.FaceID(top.ID())), "top face of g003-extrude"},
			{sk.FaceName(srf.FaceID(side.ID())), "side face of g003-extrude from r005-line-segment"},
			{madeName(t, sk, ex.H, line, sketch.RemapTop), "top edge of g003-extrude from r004-line-segment"},
			{madeName(t, sk, ex.H, line, sketch.RemapBottom), "bottom edge of g003-extrude from r004-line-segment"},
			{madeName(t, sk, ex.H, core.HRequest{V: 4}.Entity(1), sketch.RemapPtToLine),
				"side edge of g003-extrude from point 1 of r004-line-segment"},
			{madeName(t, sk, ex.H, core.HRequest{V: 4}.Entity(2), sketch.RemapTop),
				"top point of g003-extrude from point 2 of r004-line-segment"},
			{sk.EntityName(line), "r004-line-segment of g002-sketch-in-plane"},
			{sk.EntityName(core.HGroup{V: 2}.Entity(2)), "origin of g002-sketch-in-plane"},
			{sk.EntityName(core.HGroup{V: 2}.Entity(0)), "workplane of g002-sketch-in-plane"},
		} {
			if c.got != c.want {
				t.Errorf("got %q, want %q", c.got, c.want)
			}
		}
	})
}

// TestEntityNameCopies: a step and repeat's entities say which copy
// they are in and what they copy, itself named by where it is.
func TestEntityNameCopies(t *testing.T) {
	t.Parallel()
	d := api.New()
	ex := block(d, 0, 0, 20, 10, 10)
	tr := d.Features.Translate(3, api.V(30, 0, 0), "")
	if p := tr.Problem(); p != "" {
		t.Fatal(p)
	}
	d.With(func(sk *sketch.Sketch) {
		topEdge := sk.Group(ex.H).RemapEntity(core.HRequest{V: 4}.Entity(0), sketch.RemapTop)
		got := madeName(t, sk, tr.H, topEdge, 0)
		if want := "edge of g004-translate from top edge of g003-extrude from r004-line-segment (copy 1)"; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
		got = madeName(t, sk, tr.H, topEdge, sketch.RemapLast)
		if !strings.HasSuffix(got, "(last copy)") {
			t.Errorf("the last copy is %q", got)
		}
	})
}

// TestEntityNameLatheFaces: a lathe's faces are named by the lines
// they were turned from, and a face no entity names by its number in
// the group that made it, with "face" said once.
func TestEntityNameLatheFaces(t *testing.T) {
	t.Parallel()
	d := api.New()
	l := lathedCylinder(d, 10, 10)
	if p := l.Problem(); p != "" {
		t.Fatal(p)
	}
	d.With(func(sk *sketch.Sketch) {
		g := sk.Group(l.H)
		group := g.DescriptionString()
		sides := 0
		for _, s := range g.RunningShell.Surface {
			if s.Face == 0 {
				continue
			}
			name := sk.FaceName(s.Face)
			if !strings.HasPrefix(name, "side face of "+group+" from r0") {
				t.Errorf("face %08x is %q", uint32(s.Face), name)
			}
			sides++
		}
		if sides == 0 {
			t.Errorf("no face of the lathe has an identity")
		}
		id := srf.FaceID(1<<30 | l.H.V<<16 | 4)
		if got, want := sk.FaceName(id), "face 4 of "+group; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

// TestEntityNameGone: an entity whose group is gone and was never seen
// is named by its number in that group, not by its handle.
func TestEntityNameGone(t *testing.T) {
	t.Parallel()
	sk := &sketch.Sketch{}
	if got, want := sk.EntityName(core.HGroup{V: 0x55}.Entity(3)), "entity 3 of g055"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
