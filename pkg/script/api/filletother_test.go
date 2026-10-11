package api

import (
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/sweep"
)

// filletTally is how fillets of every edge of a solid fared: made and
// sound, made but not sound, or refused, with the reasons; smooth edges,
// seams with no corner to round, are left out and counted apart.
type filletTally struct {
	sound, unsound, refused, smooth int
	reasons                         map[string]int
}

// filletEveryEdge fillets each of the edges of the solid build makes,
// up to max, alone in a document of its own, with radius r, and tallies
// how they fare.
func filletEveryEdge(t *testing.T, build func(d *Doc) Group, r float64, limit int) filletTally {
	tl := filletTally{reasons: map[string]int{}}
	n := 0
	var smooth []bool
	{
		d := New()
		d.Edit.SetChordTolerance(0.01)
		g := build(d)
		if p := g.Problem(); p != "" {
			t.Fatal(p)
		}
		n = min(len(g.Solid().Edges()), limit)
		for _, e := range g.Solid().Edges()[:n] {
			smooth = append(smooth, e.Smooth())
		}
	}
	for i := range n {
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
			d.Edit.SetChordTolerance(0.01)
			g := build(d)
			f := d.Features.FilletSolid([]SolidEdge{g.Solid().Edges()[i]}, r, "")
			if p := f.Problem(); p != "" {
				tl.refused++
				tl.reasons[reasonOf(p)]++
				return
			}
			if rep := f.Analyze().Check(); rep.Sound() {
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
	slices.Sort(rs)
	return fmt.Sprintf("%d sound, %d unsound, %d refused, %d smooth left out [%s]", tl.sound, tl.unsound, tl.refused, tl.smooth, strings.Join(rs, "; "))
}

// Fillets on lofts, sweeps and imported solids (roadmap 22.5): every edge
// of a smooth loft, a swept tube and STEP solids read as solids, each
// filleted alone, the rate measured. A fillet that is made must be
// sound; what is refused is counted, and the counts may only get better.
func TestFilletOnOtherSolids(t *testing.T) {
	if testing.Short() {
		t.Skip("STEP solids at a fine tolerance")
	}
	type tcase struct {
		name    string
		build   func(d *Doc) Group
		r       float64
		minMade int
	}
	step := func(file string) func(d *Doc) Group {
		return func(d *Doc) Group {
			g, err := d.Assembly.LinkStep(filepath.Join("..", "..", "..", "testdata", "step", file), "", true)
			if err != nil {
				t.Fatal(err)
			}
			return g
		}
	}
	cases := []tcase{
		{"loft", func(d *Doc) Group {
			d.Features.SketchIn(d.Workplane("XY"), "foot")
			d.Sketch.Rect(V(-18, -18, 0), V(18, 18, 0))
			var ps []Group
			ps = append(ps, d.Groups()[len(d.Groups())-1])
			for _, p := range []struct{ z, r float64 }{{24, 30}, {50, 10}, {60, 14}} {
				d.Features.Sketch3D("")
				o := d.Sketch.Point(V(0, 0, p.z))
				ps = append(ps, d.Features.SketchAt(o, V(1, 0, 0), V(0, 1, 0), ""))
				d.Sketch.Circle(V(0, 0, p.z), p.r)
			}
			return d.Features.LoftThrough(ps, "vase")
		}, 2, 0},
		{"sweep", func(d *Doc) Group {
			d.Features.SketchIn(d.Workplane("ZX"), "path")
			up := d.Sketch.Line(V(0, 0, 0), V(0, 0, 30))
			bend := d.Sketch.Arc(V(25, 0, 30), V(0, 0, 30), V(25, 0, 55))
			along := d.Sketch.Line(V(25, 0, 55), V(65, 0, 55))
			d.Constrain.Coincident(up.End(), bend.Start())
			d.Constrain.Coincident(bend.End(), along.Start())
			d.Constrain.Tangent(up, bend)
			d.Constrain.Tangent(bend, along)
			d.Features.SketchIn(d.Workplane("XY"), "ring")
			d.Sketch.Rect(V(-6, -6, 0), V(6, 6, 0))
			return d.Features.Sweep([]Entity{up, bend, along}, "bar")
		}, 1, 12},
		{"box_hole", step("shapes/box_hole.step"), 1, 14},
		{"cylinder", step("shapes/cylinder.step"), 1, 2},
		{"blob_extruded", step("shapes/blob_extruded.step"), 1, 3},
		{"sphere_bored", step("shapes/sphere_bored.step"), 1, 2},
		{"vase_revolved", step("shapes/vase_revolved.step"), 1, 2},
		{"lid_degree7", step("degree/lid_degree7.step"), 1, 12},
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
	d.Edit.SetChordTolerance(0.005)
	g, err := d.Assembly.LinkStep(filepath.Join("..", "..", "..", "testdata", "step", "shapes", "cylinder.step"), "", true)
	if err != nil {
		t.Fatal(err)
	}
	before := g.Analyze().Check().Volume
	f := d.Features.FilletSolid([]SolidEdge{g.Solid().EdgeAt(V(5, 0, 0))}, 1, "")
	if p := f.Problem(); p != "" {
		t.Fatal(p)
	}
	r := f.Analyze().Check()
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
	d.Features.SketchIn(d.Workplane("XY"), "outline")
	lines := d.Sketch.Polyline(true, V(0, 0, 0), V(20, 0, 0), V(20, 20, 0), V(0, 20, 0))
	d.Sketch.TangentArc(lines[0].End(), 5, true)
	g := d.Features.Extrude(10, "block")
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

// The upright edges of the lid read from a STEP file, between two flat
// walls, end at the top on its curved dome (roadmap 22.5): the rolling
// ball's end follows the dome, its section the quintic fitted to where
// the surface the ball sweeps meets it. Fillets of 1 take what
// OpenCASCADE's makeFillet takes from the same edges of the same file
// (scripts/fillet-step-volumes.py) within half a percent; the same after
// saving and loading, and gone with one undo. Ours take 0.1 to 0.2% more
// at this chord tolerance.
func TestFilletEndsOnCurvedFace(t *testing.T) {
	if testing.Short() {
		t.Skip("fillets over curved faces, slow")
	}
	lid := filepath.Join("..", "..", "..", "testdata", "step", "degree", "lid_degree7.step")
	for _, c := range []struct {
		at   Vec
		want float64
	}{{V(0, 0, 5), 2.173344}, {V(40, 0, 5), 2.292926}, {V(40, 30, 5), 2.025750}, {V(0, 30, 4), 1.804228}} {
		d := New()
		d.Edit.SetChordTolerance(0.002)
		g, err := d.Assembly.LinkStep(lid, "", true)
		if err != nil {
			t.Fatal(err)
		}
		before := g.Analyze().Check().Volume
		f := d.Features.FilletSolid([]SolidEdge{g.Solid().EdgeAt(c.at)}, 1, "")
		if p := f.Problem(); p != "" {
			t.Fatalf("at %v: %s", c.at, p)
		}
		rep := f.Analyze().Check()
		took := before - rep.Volume
		t.Logf("at %v: took %.6g, OpenCASCADE %g", c.at, took, c.want)
		if !rep.Sound() || math.Abs(took-c.want) > 0.005*c.want {
			t.Errorf("at %v: %s, took %g, OpenCASCADE %g", c.at, rep.Summary(), took, c.want)
		}
		if c.at.X != 0 || c.at.Y != 0 {
			continue
		}
		path := filepath.Join(t.TempDir(), "lid.tlspr")
		if err := d.File.Save(path); err != nil {
			t.Fatal(err)
		}
		l := New()
		if err := l.File.Load(path); err != nil {
			t.Fatal(err)
		}
		l.Edit.SetChordTolerance(0.002)
		gs := l.Groups()
		if r2 := gs[len(gs)-1].Analyze().Check(); !r2.Sound() || math.Abs(r2.Volume-rep.Volume) > 1e-6*rep.Volume {
			t.Errorf("loaded: %s, volume %g, saved %g", r2.Summary(), r2.Volume, rep.Volume)
		}
		n := len(d.Groups())
		if !d.Edit.Undo() || len(d.Groups()) >= n {
			t.Errorf("undo left %d groups of %d", len(d.Groups()), n)
		}
	}
}

// The lid's upright edges filleted at radius 2.5 where they end on its
// dome, too wide a corner for one quintic to follow: the sections are
// split in two spans (rollingBall.splitEnds), and take what OpenCASCADE's
// makeFillet takes within half a percent (ours 0.12 to 0.17% more at
// this chord tolerance, as at radius 1).
func TestFilletEndsOnCurvedFaceWide(t *testing.T) {
	if testing.Short() {
		t.Skip("fillets over curved faces, slow")
	}
	lid := filepath.Join("..", "..", "..", "testdata", "step", "degree", "lid_degree7.step")
	for _, c := range []struct {
		at   Vec
		want float64
	}{{V(0, 0, 4), 13.812062}, {V(40, 0, 4), 14.384156}, {V(40, 30, 4), 12.747386}, {V(0, 30, 4), 11.556165}} {
		d := New()
		d.Edit.SetChordTolerance(0.002)
		g, err := d.Assembly.LinkStep(lid, "", true)
		if err != nil {
			t.Fatal(err)
		}
		before := g.Analyze().Check().Volume
		f := d.Features.FilletSolid([]SolidEdge{g.Solid().EdgeAt(c.at)}, 2.5, "")
		if p := f.Problem(); p != "" {
			t.Fatalf("at %v: %s", c.at, p)
		}
		rep := f.Analyze().Check()
		if took := before - rep.Volume; !rep.Sound() || math.Abs(took-c.want) > 0.005*c.want {
			t.Errorf("at %v: %s, took %g, OpenCASCADE %g", c.at, rep.Summary(), took, c.want)
		}
	}
}

// The lid's upright edges filleted where they end on its dome, at radii
// up to 2.5 and two chord tolerances: none refused, none unsound. Until
// 2026-10-11 8 of 40 were refused, at radii of 1.5 to 2.5: the dome's
// height round a corner that wide is no quintic within half the chord
// tolerance (a polynomial in the angle round the corner misses it by
// 3e-4 mm at degree 5, 2e-5 at 6). Their sections are now split in two
// quintic spans (rollingBall.splitEnds).
func TestFilletEndsOnCurvedFaceSweep(t *testing.T) {
	lid := filepath.Join("..", "..", "..", "testdata", "step", "degree", "lid_degree7.step")
	g := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Strings("corner", "0,0", "40,0", "40,30", "0,30"),
			sweep.Floats("r", 0.5, 1, 1.5, 2, 2.5),
			sweep.Floats("chord", 0.002, 0.005),
		},
		Sample: 6,
	}
	g.Run(func(p sweep.Point) error {
		var x, y float64
		if _, err := fmt.Sscanf(p.String("corner"), "%g,%g", &x, &y); err != nil {
			return err
		}
		d := New()
		d.Edit.SetChordTolerance(p.Float("chord"))
		s, err := d.Assembly.LinkStep(lid, "", true)
		if err != nil {
			return err
		}
		f := d.Features.FilletSolid([]SolidEdge{s.Solid().EdgeAt(V(x, y, 4))}, p.Float("r"), "")
		if pr := f.Problem(); pr != "" {
			return errors.New(pr)
		}
		if rep := f.Analyze().Check(); !rep.Sound() {
			t.Errorf("%s: made, not sound: %s", p.Label(), rep.Summary())
		}
		return nil
	}).Require(t)
}

// The blob's top and bottom rims, each one closed edge that turns 130
// degrees where its B-spline wall has a crease that stays sharp: the
// ball rolling either way comes within its radius of the wall across
// the crease near it, and the two ends meet in a mitre (rollingmitre.go).
// The mitre runs from the crease, at the ball's height, to where the
// curves the fillet touches the cap along cross: OpenCASCADE's makeFillet
// of the same edges has its vertices there (0, 0, 4) and (0.0842189,
// 2.4007678, 5), and every point of our fillet's surfaces lies within
// 8e-5 mm of its result (sampled 2026-10-11). Both rims are sound and
// take alike; the top one comes back alike after saving and loading.
func TestFilletRimMitre(t *testing.T) {
	if testing.Short() {
		t.Skip("fillets over curved faces, slow")
	}
	blob := filepath.Join("..", "..", "..", "testdata", "step", "shapes", "blob_extruded.step")
	var took [2]float64
	for k, z := range []float64{5, 0} {
		d := New()
		d.Edit.SetChordTolerance(0.01)
		g, err := d.Assembly.LinkStep(blob, "", true)
		if err != nil {
			t.Fatal(err)
		}
		before := g.Analyze().Check().Volume
		var rim SolidEdge
		for _, e := range g.Solid().Edges() {
			if a, b := e.Ends(); a == b && math.Abs(a.Z-z) < 1e-9 {
				rim = e
			}
		}
		f := d.Features.FilletSolid([]SolidEdge{rim}, 1, "")
		if p := f.Problem(); p != "" {
			t.Fatalf("rim at z %g: %s", z, p)
		}
		rep := f.Analyze().Check()
		if !rep.Sound() {
			t.Errorf("rim at z %g: %s", z, rep.Summary())
		}
		took[k] = before - rep.Volume
		// The mitre: from the crease at the ball's height up to the cap.
		dz := 1.0
		if z == 0 {
			dz = -1
		}
		low, high := V(0, 0, z-dz), V(0.0842189, 2.4007678, z)
		var ends [2]bool
		for _, e := range f.Solid().Edges() {
			a, b := e.Ends()
			for i, q := range []Vec{low, high} {
				ends[i] = ends[i] || a.Minus(q).Magnitude() < 1e-4 || b.Minus(q).Magnitude() < 1e-4
			}
		}
		if !ends[0] || !ends[1] {
			t.Errorf("rim at z %g: the mitre's ends %v and %v are not both vertices (%v)", z, low, high, ends)
		}
		if k == 1 {
			continue
		}
		path := filepath.Join(t.TempDir(), "blob.tlspr")
		if err := d.File.Save(path); err != nil {
			t.Fatal(err)
		}
		l := New()
		if err := l.File.Load(path); err != nil {
			t.Fatal(err)
		}
		l.Edit.SetChordTolerance(0.01)
		gs := l.Groups()
		if r2 := gs[len(gs)-1].Analyze().Check(); !r2.Sound() || math.Abs(r2.Volume-rep.Volume) > 1e-6*rep.Volume {
			t.Errorf("rim at z %g loaded: %s, volume %g, saved %g", z, r2.Summary(), r2.Volume, rep.Volume)
		}
	}
	if math.Abs(took[0]-took[1]) > 1e-4*took[0] {
		t.Errorf("the rims take %g and %g", took[0], took[1])
	}
}
