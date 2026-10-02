package api

import (
	"math"
	"strings"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/sketch"
)

// Mirror, Chamfer and Sweep, and what each reads and changes.
func TestSolidFeatures(t *testing.T) {
	d := New()
	d.SketchIn(d.Workplane("ZX"), "path")
	up := d.Line(V(20, 0, 0), V(20, 0, 10))
	d.SketchIn(d.Workplane("XY"), "profile")
	d.Rect(V(15, -5, 0), V(25, 5, 0))
	sw := d.Sweep([]Entity{up}, "post")
	if sw.Kind() != "sweep" || len(sw.Edges()) != 1 || sw.Problem() != "" {
		t.Fatalf("sweep %s, %d edges, problem %q", sw.Kind(), len(sw.Edges()), sw.Problem())
	}
	if v := sw.Volume(); math.Abs(v-1000) > 1e-6 {
		t.Fatalf("the post's volume is %g", v)
	}

	var edge Entity
	for _, e := range sw.Entities() {
		if e.Kind() == "line" && e.Start().Pos().Z == 10 && e.End().Pos().Z == 10 {
			edge = e
			break
		}
	}
	c := d.Chamfer([]Entity{edge}, 1, "")
	if c.Kind() != "chamfer" || c.Distance() != 1 || c.Edges()[0].H != edge.H {
		t.Fatalf("chamfer %s at %g", c.Kind(), c.Distance())
	}
	if err := c.SetExpr(sketch.GroupPitch, "2"); err != nil || c.Distance() != 2 {
		t.Fatalf("distance %g, %v", c.Distance(), err)
	}
	if v := c.Volume(); math.Abs(v-(1000-0.5*2*2*10)) > 1e-6 {
		t.Fatalf("the chamfered post's volume is %g", v)
	}

	// A mirror reflects the active group: of the chamfer, its cut.
	m := d.Mirror(d.Workplane("YZ"), "")
	if m.Kind() != "mirror" || m.Plane().H != d.Workplane("YZ").H || !m.KeepOriginal() {
		t.Fatalf("mirror %s", m.Kind())
	}
	if v := m.Volume(); math.Abs(v-(1000-20)) > 1e-6 {
		t.Fatalf("the post with the mirrored cut has volume %g", v)
	}
	// Of the sweep, the post.
	m.Delete()
	d.Activate(sw)
	m = d.Mirror(d.Workplane("YZ"), "")
	if v := m.Volume(); math.Abs(v-2000) > 1e-6 {
		t.Fatalf("the mirrored posts' volume is %g", v)
	}
	m.SetKeepOriginal(false)
	if r := m.Check(); math.Abs(r.Volume-1000) > 1e-6 || !r.Sound() {
		t.Fatalf("the reflection alone: %s", r.String())
	}
	// The chamfer's edge has gone with the original: the chamfer says
	// so, and cuts nothing.
	d.Activate(c)
	if v := c.Volume(); math.Abs(v-1000) > 1e-6 || !strings.Contains(c.Problem(), "not an edge") {
		t.Fatalf("the chamfer after the mirror leaves %g, problem %q", v, c.Problem())
	}
}

// What can't be done panics with an error that says why.
func TestSolidFeatureMisuse(t *testing.T) {
	d := New()
	line := d.Line(V(0, 0, 0), V(10, 0, 0))
	d.Rect(V(20, 0, 0), V(30, 10, 0))
	ex := d.Extrude(5, "")
	for name, c := range map[string]struct {
		fn   func()
		want string
	}{
		"mirror in a line":    {func() { d.Mirror(line, "") }, "plane"},
		"chamfer nothing":     {func() { d.Chamfer(nil, 1, "") }, "no edges"},
		"negative distance":   {func() { d.Chamfer([]Entity{line}, -1, "") }, "more than zero"},
		"sweep from extrude":  {func() { d.Sweep([]Entity{line}, "") }, "sketch in a workplane"},
		"edges of an extrude": {func() { ex.Edges() }, "chamfer or a sweep"},
		"plane of an extrude": {func() { ex.Plane() }, "mirror"},
		"keep on an extrude":  {func() { ex.SetKeepOriginal(false) }, "mirror"},
		"distance to zero":    {func() { ex.SetDistance(1) }, "chamfer"},
	} {
		func() {
			defer func() {
				err, ok := recover().(error)
				if !ok || !strings.Contains(err.Error(), c.want) {
					t.Errorf("%s: %v, want an error saying %q", name, err, c.want)
				}
			}()
			c.fn()
		}()
	}
}
