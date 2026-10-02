package check

import (
	"math"
	"strings"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/srf"
)

// box is a closed mesh of an axis-aligned box, outward facing.
func box(min, max geom.Vector) *srf.SMesh {
	v := func(i int) geom.Vector {
		p := min
		if i&1 != 0 {
			p.X = max.X
		}
		if i&2 != 0 {
			p.Y = max.Y
		}
		if i&4 != 0 {
			p.Z = max.Z
		}
		return p
	}
	// Each face as four corners, counter-clockwise seen from outside.
	faces := [][4]int{
		{0, 2, 3, 1}, // z = min
		{4, 5, 7, 6}, // z = max
		{0, 1, 5, 4}, // y = min
		{2, 6, 7, 3}, // y = max
		{0, 4, 6, 2}, // x = min
		{1, 3, 7, 5}, // x = max
	}
	var m srf.SMesh
	for _, f := range faces {
		m.AddTriangle3(srf.STriMeta{}, v(f[0]), v(f[1]), v(f[2]))
		m.AddTriangle3(srf.STriMeta{}, v(f[0]), v(f[2]), v(f[3]))
	}
	return &m
}

func near(a, b float64) bool { return math.Abs(a-b) <= 1e-12*math.Max(1, math.Abs(b)) }

func TestSolidBox(t *testing.T) {
	m := box(geom.Vector{X: 1, Y: 2, Z: 3}, geom.Vector{X: 3, Y: 5, Z: 7})
	r := Solid(nil, m)
	if !r.Sound() || r.Empty() {
		t.Fatalf("a closed box is not sound: %s", r.String())
	}
	if r.Triangles != 12 || r.Faces != 0 || r.NakedEdges != 0 || r.SelfIntersecting != 0 {
		t.Errorf("counts: %s", r.String())
	}
	if !near(r.Volume, 24) {
		t.Errorf("volume %v, want 24", r.Volume)
	}
	c := r.CenterOfMass
	if !near(c.X, 2) || !near(c.Y, 3.5) || !near(c.Z, 5) {
		t.Errorf("centre of mass %v, want (2, 3.5, 5)", c)
	}
	if r.Min != (geom.Vector{X: 1, Y: 2, Z: 3}) || r.Max != (geom.Vector{X: 3, Y: 5, Z: 7}) {
		t.Errorf("bounding box %v %v", r.Min, r.Max)
	}
	if len(r.Problems.Edges) != 0 {
		t.Errorf("%d problem edges on a closed box", len(r.Problems.Edges))
	}
}

func TestSolidOpen(t *testing.T) {
	m := box(geom.Vector{}, geom.Vector{X: 1, Y: 1, Z: 1})
	m.Triangles = m.Triangles[1:]
	r := Solid(nil, m)
	if r.Sound() {
		t.Fatal("a box missing a triangle is sound")
	}
	if r.NakedEdges != 3 || r.SelfIntersecting != 0 || len(r.Problems.Edges) != 3 {
		t.Errorf("want the three edges of the hole naked: %s", r.String())
	}
	if !strings.Contains(r.Summary(), "3 naked edges") {
		t.Errorf("summary does not name the naked edges:\n%s", r.Summary())
	}
}

func TestSolidSelfIntersecting(t *testing.T) {
	// Two boxes that overlap, each closed: no naked edge, but edges of
	// each pass through the other.
	a := box(geom.Vector{}, geom.Vector{X: 2, Y: 2, Z: 2})
	b := box(geom.Vector{X: 1, Y: 1, Z: 1}, geom.Vector{X: 3, Y: 3, Z: 3})
	a.Triangles = append(a.Triangles, b.Triangles...)
	r := Solid(nil, a)
	if r.Sound() || r.SelfIntersecting == 0 || r.NakedEdges != 0 {
		t.Errorf("want self-intersecting and closed: %s", r.String())
	}
	if len(r.Problems.Edges) != r.SelfIntersecting {
		t.Errorf("%d problem edges, %d self-intersecting", len(r.Problems.Edges), r.SelfIntersecting)
	}
	if !strings.Contains(r.Summary(), "self-intersecting edges") {
		t.Errorf("summary does not name the intersections:\n%s", r.Summary())
	}
}

func TestSolidShell(t *testing.T) {
	sh := &srf.SShell{Surface: make([]srf.SSurface, 6), BooleanFailed: true}
	r := Solid(sh, box(geom.Vector{}, geom.Vector{X: 1, Y: 1, Z: 1}))
	if r.Faces != 6 || !r.BooleanFailed || r.Sound() {
		t.Errorf("shell not reported: %s", r.String())
	}
	if !strings.Contains(r.Summary(), "the Boolean failed") {
		t.Errorf("summary does not name the failed Boolean:\n%s", r.Summary())
	}
}

func TestSolidEmpty(t *testing.T) {
	r := Solid(nil, &srf.SMesh{})
	if !r.Empty() || !r.Sound() {
		t.Errorf("empty: %s", r.String())
	}
	if r.String() != "tri=0 faces=0 naked=0 inter=0 failed=false" {
		t.Errorf("String() = %q", r.String())
	}
	if r.Summary() != "The group has no solid." {
		t.Errorf("Summary() = %q", r.Summary())
	}
}

func TestStringReadsBack(t *testing.T) {
	r := Solid(nil, box(geom.Vector{X: 0.1, Y: 0.2, Z: 0.3}, geom.Vector{X: 1, Y: 1, Z: 1}))
	want := "tri=12 faces=0 naked=0 inter=0 failed=false vol=" + g17(r.Volume) +
		" com=" + v17(r.CenterOfMass) + " min=0.10000000000000001,0.20000000000000001,0.29999999999999999 max=1,1,1"
	if r.String() != want {
		t.Errorf("String() =\n%s\nwant\n%s", r.String(), want)
	}
	s := r.Summary()
	for _, part := range []string{"sound", "Faces: 0", "Triangles: 12", "Volume: 0.504 mm^3", "Extent: 0.9 x 0.8 x 0.7 mm"} {
		if !strings.Contains(s, part) {
			t.Errorf("summary lacks %q:\n%s", part, s)
		}
	}
}
