package api

import (
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// filletTally is how fillets of every edge of a solid fared: made and
// sound, made but not sound, or refused, with the reasons; smooth edges,
// seams with no corner to round, are left out and counted apart.
type filletTally struct {
	sound, unsound, refused, smooth int
	reasons                 map[string]int
}

// filletEveryEdge fillets each of the edges of the solid build makes,
// up to max, alone in a document of its own, with radius r, and tallies
// how they fare.
func filletEveryEdge(t *testing.T, build func(d *Doc) Group, r float64, max int) filletTally {
	tl := filletTally{reasons: map[string]int{}}
	n := 0
	var smooth []bool
	{
		d := New()
		d.SetChordTolerance(0.01)
		g := build(d)
		if p := g.Problem(); p != "" {
			t.Fatal(p)
		}
		n = min(len(g.Solid().Edges()), max)
		for _, e := range g.Solid().Edges()[:n] {
			smooth = append(smooth, e.Smooth())
		}
	}
	for i := 0; i < n; i++ {
		if smooth[i] {
			tl.smooth++
			continue
		}
		func() {
			defer func() {
				if x := recover(); x != nil {
					tl.refused++
					tl.reasons[reasonOf(fmt.Sprint(x))]++
				}
			}()
			d := New()
			d.SetChordTolerance(0.01)
			g := build(d)
			f := d.FilletSolid([]SolidEdge{g.Solid().Edges()[i]}, r, "")
			if p := f.Problem(); p != "" {
				tl.refused++
				tl.reasons[reasonOf(p)]++
				return
			}
			if rep := f.Check(); rep.Sound() {
				tl.sound++
			} else {
				tl.unsound++
			}
		}()
	}
	return tl
}

// reasonOf is a refusal's words up to its first colon.
func reasonOf(msg string) string {
	if i := strings.IndexAny(msg, ":("); i > 0 {
		msg = msg[:i]
	}
	return strings.TrimSpace(msg)
}

func (tl filletTally) String() string {
	var rs []string
	for r, n := range tl.reasons {
		rs = append(rs, fmt.Sprintf("%d× %s", n, r))
	}
	sort.Strings(rs)
	return fmt.Sprintf("%d sound, %d unsound, %d refused, %d smooth left out [%s]", tl.sound, tl.unsound, tl.refused, tl.smooth, strings.Join(rs, "; "))
}

// Fillets on lofts, sweeps and imported solids (roadmap 22.5): every edge
// of a smooth loft, a swept tube and STEP solids read as solids, each
// filleted alone, the rate measured. A fillet that is made must be
// sound; what is refused is counted, and the counts may only get better.
func TestFilletOnOtherSolids(t *testing.T) {
	type tcase struct {
		name    string
		build   func(d *Doc) Group
		r       float64
		minMade int
	}
	step := func(file string) func(d *Doc) Group {
		return func(d *Doc) Group {
			g, err := d.LinkStep(filepath.Join("..", "..", "..", "testdata", "step", file), "", true)
			if err != nil {
				t.Fatal(err)
			}
			return g
		}
	}
	cases := []tcase{
		{"loft", func(d *Doc) Group {
			d.SketchIn(d.Workplane("XY"), "foot")
			d.Rect(V(-18, -18, 0), V(18, 18, 0))
			var ps []Group
			ps = append(ps, d.Groups()[len(d.Groups())-1])
			for _, p := range []struct{ z, r float64 }{{24, 30}, {50, 10}, {60, 14}} {
				d.Sketch3D("")
				o := d.Point(V(0, 0, p.z))
				ps = append(ps, d.SketchAt(o, V(1, 0, 0), V(0, 1, 0), ""))
				d.Circle(V(0, 0, p.z), p.r)
			}
			return d.LoftThrough(ps, "vase")
		}, 2, 0},
		{"sweep", func(d *Doc) Group {
			d.SketchIn(d.Workplane("ZX"), "path")
			up := d.Line(V(0, 0, 0), V(0, 0, 30))
			bend := d.Arc(V(25, 0, 30), V(0, 0, 30), V(25, 0, 55))
			along := d.Line(V(25, 0, 55), V(65, 0, 55))
			d.Coincident(up.End(), bend.Start())
			d.Coincident(bend.End(), along.Start())
			d.Tangent(up, bend)
			d.Tangent(bend, along)
			d.SketchIn(d.Workplane("XY"), "ring")
			d.Rect(V(-6, -6, 0), V(6, 6, 0))
			return d.Sweep([]Entity{up, bend, along}, "bar")
		}, 1, 12},
		{"box_hole", step("shapes/box_hole.step"), 1, 13},
		{"cylinder", step("shapes/cylinder.step"), 1, 2},
		{"blob_extruded", step("shapes/blob_extruded.step"), 1, 1},
		{"sphere_bored", step("shapes/sphere_bored.step"), 1, 2},
		{"vase_revolved", step("shapes/vase_revolved.step"), 1, 2},
		{"lid_degree7", step("degree/lid_degree7.step"), 1, 8},
	}
	total, made := 0, 0
	for _, c := range cases {
		tl := filletEveryEdge(t, c.build, c.r, 16)
		t.Logf("%s, r %g: %s", c.name, c.r, tl)
		if tl.unsound > 0 {
			t.Errorf("%s: %d fillets made but not sound", c.name, tl.unsound)
		}
		if tl.sound < c.minMade {
			t.Errorf("%s: %d fillets made, %d were", c.name, tl.sound, c.minMade)
		}
		total += tl.sound + tl.unsound + tl.refused
		made += tl.sound
	}
	t.Logf("in all: %d of %d edges filleted soundly (%.0f%%)", made, total, 100*float64(made)/float64(total))
}

// A fillet of radius 1 round the foot of a cylinder of radius 5 read
// from a STEP file, whose side is four patches trimmed apart along its
// cells: by Pappus, the corner's r²(1 - π/4) turned about the axis at its
// centroid, r(10 - 3π)/(12 - 3π) in from the side.
func TestFilletStepCylinder(t *testing.T) {
	d := New()
	d.SetChordTolerance(0.005)
	g, err := d.LinkStep(filepath.Join("..", "..", "..", "testdata", "step", "shapes", "cylinder.step"), "", true)
	if err != nil {
		t.Fatal(err)
	}
	before := g.Check().Volume
	f := d.FilletSolid([]SolidEdge{g.Solid().EdgeAt(V(5, 0, 0))}, 1, "")
	if p := f.Problem(); p != "" {
		t.Fatal(p)
	}
	r := f.Check()
	const rad, fr = 5.0, 1.0
	want := 2 * math.Pi * (rad - fr*(10-3*math.Pi)/(12-3*math.Pi)) * fr * fr * (1 - math.Pi/4)
	if took := before - r.Volume; !r.Sound() || math.Abs(took-want) > 0.01*want {
		t.Errorf("%s: took %g, want %g", r.Summary(), took, want)
	}
}

// An extruded outline whose corner is rounded by a tangent arc has
// smooth edges where the arc's face goes on into the lines' faces, and
// creased ones round its top and foot.
func TestSolidEdgeSmooth(t *testing.T) {
	d := New()
	d.SketchIn(d.Workplane("XY"), "outline")
	lines := d.Polyline(true, V(0, 0, 0), V(20, 0, 0), V(20, 20, 0), V(0, 20, 0))
	d.TangentArc(lines[0].End(), 5, true)
	g := d.Extrude(10, "block")
	if p := g.Problem(); p != "" {
		t.Fatal(p)
	}
	smooth, creased := 0, 0
	for _, e := range g.Solid().Edges() {
		if e.Smooth() {
			smooth++
			a, b := e.Ends()
			if !e.Straight() || !near(a.X, b.X) || !near(a.Y, b.Y) {
				t.Errorf("%s is smooth but doesn't run up the side", e.Describe())
			}
		} else {
			creased++
		}
	}
	if smooth != 2 {
		t.Errorf("%d smooth edges, want the 2 where the arc meets the lines (%d creased)", smooth, creased)
	}
}
