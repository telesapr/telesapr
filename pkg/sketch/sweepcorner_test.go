package sketch_test

import (
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// cornerPath draws, in the ZX workplane, up from the origin by up, then
// at angle degrees from the vertical for along; a corner between them.
func cornerPath(d *api.Doc, up, along, angle float64) []api.Entity {
	d.Features.SketchIn(d.Workplane("ZX"), "path")
	th := angle * math.Pi / 180
	top := api.V(0, 0, up)
	return []api.Entity{
		d.Sketch.Line(api.V(0, 0, 0), top),
		d.Sketch.Line(top, top.Plus(api.V(math.Sin(th), 0, math.Cos(th)).ScaledBy(along))),
	}
}

// A sweep along a path with a corner (roadmap 28.4): the pieces are
// mitred, each cut by the plane halving the corner, so the volume is the
// profile's area times the path's length (the mitre takes from one
// piece what it gives the other).
func TestSweepCorner(t *testing.T) {
	t.Parallel()
	d := api.New()
	path := cornerPath(d, 10, 12, 90)
	d.Features.SketchIn(d.Workplane("XY"), "profile")
	d.Sketch.Rect(api.V(-1, -2, 0), api.V(1, 2, 0))
	step(d)
	before := saved(t, d)
	s := d.Features.Sweep(path, "")
	if p := s.Problem(); p != "" {
		t.Fatal(p)
	}
	r := s.Analyze().Check()
	if want := 8.0 * 22; !r.Sound() || math.Abs(r.Volume-want) > 1e-6*want {
		t.Fatalf("an L: %s, want %g", r.String(), want)
	}
	undoRedo(t, d, before)
	roundTrip(t, d)
	// A round profile, mitred: elliptical joints.
	d2 := api.New()
	d2.Edit.SetChordTolerance(0.002)
	path2 := cornerPath(d2, 10, 12, 60)
	d2.Features.SketchIn(d2.Workplane("XY"), "profile")
	d2.Sketch.Circle(api.V(0, 0, 0), 2)
	s2 := d2.Features.Sweep(path2, "")
	r2 := s2.Analyze().Check()
	if want := math.Pi * 4 * 22; s2.Problem() != "" || !r2.Sound() || math.Abs(r2.Volume-want) > 0.005*want {
		t.Fatalf("a round elbow: %q %s, want %g", s2.Problem(), r2.String(), want)
	}
}

func TestSweepCornerRefused(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		path func(d *api.Doc) []api.Entity
		want string
	}{
		"too short": {func(d *api.Doc) []api.Entity {
			d.Features.SketchIn(d.Workplane("ZX"), "path")
			return []api.Entity{
				d.Sketch.Line(api.V(0, 0, 0), api.V(0, 0, 10)),
				d.Sketch.Line(api.V(0, 0, 10), api.V(0.5, 0, 10)),
				d.Sketch.Line(api.V(0.5, 0, 10), api.V(0.5, 0, 0)),
			}
		}, "too short"},
	} {
		d := api.New()
		path := c.path(d)
		d.Features.SketchIn(d.Workplane("XY"), "profile")
		d.Sketch.Rect(api.V(-1, -2, 0), api.V(1, 2, 0))
		func() {
			defer func() {
				err, ok := recover().(error)
				if !ok || !strings.Contains(err.Error(), c.want) {
					t.Errorf("%s: %v, want an error saying %q", name, err, c.want)
				}
			}()
			d.Features.Sweep(path, "")
		}()
	}
}

// A sweep along edges of the solid by name (roadmap 28.4): a bead along
// the top edges of a block, round its corner, by their names.
func TestSweepSolidEdges(t *testing.T) {
	t.Parallel()
	d := api.New()
	block(d, 0, 0, 20, 10, 5)
	s := d.Active().Solid()
	front := s.EdgeAt(api.V(10, 0, 5))
	right := s.EdgeAt(api.V(20, 5, 5))
	// The profile: a small square on the block's left end, across the
	// front top edge's start.
	d.Features.SketchIn(d.Workplane("YZ"), "profile")
	d.Sketch.Rect(api.V(0, -0.5, 4.5), api.V(0, 0.5, 5.5))
	step(d)
	before := saved(t, d)
	g := d.Features.SweepEdges([]api.SolidEdge{right, front}, nil, "")
	if p := g.Problem(); p != "" {
		t.Fatal(p)
	}
	if len(g.SolidEdges()) != 2 {
		t.Errorf("the path's edges: %v", g.SolidEdges())
	}
	r := g.Analyze().Check()
	// The block and the bead, mitred at the corner: the bead lies a
	// quarter in the block along each edge, a quarter of its 1 by 1
	// section, so it adds three quarters of a square mm per mm, along
	// 30 mm of path less the overlap at the corner (a quarter square's
	// worth either way).
	if !r.Sound() || r.Volume < 1000+0.75*30-1 || r.Volume > 1000+0.75*30+1 {
		t.Fatalf("a bead along two edges: %s", r.String())
	}
	undoRedo(t, d, before)
	roundTrip(t, d)
}
