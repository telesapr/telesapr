package api

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// plateDoc is a 40 by 20 by 4 plate, with holes holes of 3 mm across
// along its middle.
func plateDoc(t *testing.T, holes int) (*Doc, Group) {
	t.Helper()
	d := New()
	sides := d.Sketch.Rect(V(0, 0, 0), V(40, 20, 0))
	d.Constrain.Coincident(sides[0].Start(), d.Origin())
	d.Constrain.Length(sides[0], 40)
	d.Constrain.Length(sides[1], 20)
	for i := range holes {
		x := 40 * (float64(i) + 0.5) / float64(holes)
		c := d.Sketch.Circle(V(x, 10, 0), 0.5)
		d.Constrain.Diameter(c, 0.5)
	}
	return d, d.Features.Extrude(4, "plate")
}

// faceLine is a face's line: its ID and the point of it.
var faceLine = regexp.MustCompile(`(?m)^    (0x[0-9a-f]+) .*; at \(([-0-9.e]+), ([-0-9.e]+), ([-0-9.e]+)\); (\d+) edges?$`)

// edgeLine is an edge's line: its name.
var edgeLine = regexp.MustCompile(`(?m)^    ("edge [^"]+"): `)

// The faces and edges of a plate, by the names and points that find
// them back.
func TestDescribeSolidPlate(t *testing.T) {
	t.Parallel()
	_, plate := plateDoc(t, 0)
	text := plate.DescribeSolid()
	if !strings.HasPrefix(text, "solid at g003-plate: 6 faces, 12 edges, 8 vertices; sound\n") {
		t.Fatalf("head:\n%s", text)
	}
	s := plate.Solid()
	faces := faceLine.FindAllStringSubmatch(text, -1)
	if len(faces) != 6 {
		t.Fatalf("%d face lines:\n%s", len(faces), text)
	}
	for _, f := range faces {
		var p [3]float64
		for i := range p {
			p[i], _ = strconv.ParseFloat(f[2+i], 64)
		}
		got := s.FaceAt(V(p[0], p[1], p[2]))
		if id := fmt.Sprintf("%#x", got.ID()); id != f[1] {
			t.Errorf("FaceAt %v is %s, the line says %s", p, id, f[1])
		}
		if f[5] != "4" {
			t.Errorf("face %s has %s edges", f[1], f[5])
		}
	}
	if n := strings.Count(text, ": flat, normal "); n != 6 {
		t.Errorf("%d flat faces:\n%s", n, text)
	}
	if !strings.Contains(text, "flat, normal (0, 0, 1)") || !strings.Contains(text, "flat, normal (0, 0, -1)") {
		t.Errorf("no top and bottom:\n%s", text)
	}
	edges := edgeLine.FindAllStringSubmatch(text, -1)
	if len(edges) != 12 {
		t.Fatalf("%d edge lines:\n%s", len(edges), text)
	}
	for _, e := range edges {
		name, _ := strconv.Unquote(e[1])
		if got := s.Edge(name).Name(); got != name {
			t.Errorf("Edge(%q) is %q", name, got)
		}
	}
	if !strings.Contains(text, ": straight, 40 mm, ") || !strings.Contains(text, ": straight, 4 mm, ") {
		t.Errorf("lengths:\n%s", text)
	}
}

// A boss's side is a cylinder and its rim a circle.
func TestDescribeSolidBoss(t *testing.T) {
	t.Parallel()
	d := New()
	c := d.Sketch.Circle(V(0, 0, 0), 5)
	d.Constrain.Coincident(c.Center(), d.Origin())
	d.Constrain.Diameter(c, 10)
	boss := d.Features.Extrude(8, "boss")
	text := boss.DescribeSolid()
	for _, want := range []string{
		": 3 faces, 2 edges, ",
		"cylinder, axis (0, 0, ",
		"through (0, 0, 0), radius 5; at (",
		": circle about (0, 0, 8), radius 5, 31.416 mm, closed",
		"mm, closed; faces ",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("no %q in:\n%s", want, text)
		}
	}
	// The side found back by its point.
	m := faceLine.FindAllStringSubmatch(text, -1)
	found := false
	for _, f := range m {
		if strings.Contains(f[0], "cylinder") {
			var p [3]float64
			for i := range p {
				p[i], _ = strconv.ParseFloat(f[2+i], 64)
			}
			found = fmt.Sprintf("%#x", boss.Solid().FaceAt(V(p[0], p[1], p[2])).ID()) == f[1]
		}
	}
	if !found {
		t.Errorf("the cylinder isn't found back by its point:\n%s", text)
	}
}

// A sketch with nothing before it has no solid to tell of.
func TestDescribeSolidNone(t *testing.T) {
	t.Parallel()
	d := New()
	if got := d.Active().DescribeSolid(); got != "no solid yet at g002-sketch-in-plane\n" {
		t.Errorf("got %q", got)
	}
	// An extrusion of an outline that doesn't close says why.
	d.Sketch.Line(V(0, 0, 0), V(10, 0, 0))
	g := d.Features.Extrude(3, "open")
	if got := g.DescribeSolid(); !strings.HasPrefix(got, "no solid yet at g003-open; problem: ") {
		t.Errorf("got %q", got)
	}
}

// The solid at a sketch after a solid is that solid.
func TestDescribeSolidAfter(t *testing.T) {
	t.Parallel()
	d, _ := plateDoc(t, 0)
	sk := d.Features.SketchIn(d.Workplane("XY"), "next")
	if got := sk.DescribeSolid(); !strings.HasPrefix(got, "solid at g004-next: 6 faces") {
		t.Errorf("got %q", got)
	}
}

// A solid of many edges is listed in part, with how to narrow it.
func TestDescribeSolidCap(t *testing.T) {
	t.Parallel()
	_, plate := plateDoc(t, 30)
	text := plate.DescribeSolid()
	if !strings.Contains(text, ": 36 faces, 72 edges, ") {
		t.Fatalf("head:\n%s", text)
	}
	if n := len(edgeLine.FindAllString(text, -1)); n != describeMaxEdges {
		t.Errorf("%d edges listed", n)
	}
	if !strings.Contains(text, "    … and 12 more edges; narrow with Solid().EdgesAlong(dir)") {
		t.Errorf("no tail:\n%s", text)
	}
	_, plate = plateDoc(t, 40)
	text = plate.DescribeSolid()
	if n := len(faceLine.FindAllString(text, -1)); n != describeMaxFaces {
		t.Errorf("%d faces listed", n)
	}
	if !strings.Contains(text, "    … and 6 more faces; ") {
		t.Errorf("no face tail:\n%s", text)
	}
}

// A turned ball, cone and ring: a sphere, a cone, cylinders, arcs and
// circles.
func TestDescribeSolidTurned(t *testing.T) {
	t.Parallel()
	d := New()
	axis := d.Sketch.Line(V(0, -10, 0), V(0, 10, 0))
	axis.SetConstruction(true)
	d.Sketch.Arc(V(0, 0, 0), V(0, -5, 0), V(0, 5, 0))
	d.Sketch.Line(V(0, 5, 0), V(0, -5, 0))
	d.Sketch.Polyline(true, V(0, 6, 0), V(3, 6, 0), V(0, 10, 0))
	d.Sketch.Polyline(true, V(7, -3, 0), V(9, -3, 0), V(9, 3, 0), V(7, 3, 0))
	g := d.Features.Lathe(axis, "turn")
	text := g.DescribeSolid()
	for _, want := range []string{
		"sphere, centre (0, 0, 0), radius 5",
		"cone, axis (0, -1, 0) from its apex (0, 10, 0), half angle 36.87°",
		"cylinder, axis (0, -1, 0) through (0, 3, 0), radius 7",
		": arc about (0, 6, 0), radius 3, 4.712 mm, ",
		": circle about (0, 3, 0), radius 9, 56.549 mm, closed",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("no %q in:\n%s", want, text)
		}
	}
}
