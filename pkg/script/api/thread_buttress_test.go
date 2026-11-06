package api

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/fixtures"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
)

// buttressNut saves an S20x4 nut, a Ø40 disc 8 mm high with its hole
// threaded through from the bottom up, its load flank turned back or
// not, its crest as named, and returns its path. Two turns of thread
// are enough to collide or not, and half the cost of four.
func buttressNut(t *testing.T, back bool, crest string) string {
	t.Helper()
	nut := New()
	disc := nut.Sketch.Circle(V(0, 0, 0), 20)
	nut.Constrain.Coincident(disc.Center(), nut.Origin())
	nut.Features.Extrude(8, "disc")
	nut.Features.SketchIn(nut.Workplane("XY"), "hole")
	hole := nut.Sketch.Circle(V(0, 0, 0), 10)
	nut.Constrain.Coincident(hole.Center(), nut.Origin())
	nut.Features.Thread(hole, ThreadOptions{Designation: "S20x4", Internal: true, Through: true, Length: 8,
		Clearance: 0.1, LoadFlankBack: back, Crest: crest})
	path := filepath.Join(t.TempDir(), "nut.tlspr")
	if err := nut.File.Save(path); err != nil {
		t.Fatal(err)
	}
	return path
}

// buttressRod is a Ø20 rod 24 mm long threaded S20x4 down from its
// top, its crest as named, and the circle there that the thread is on.
func buttressRod(d *Doc, crest string) Entity {
	shank := d.Sketch.Circle(V(0, 0, 0), 10)
	d.Constrain.Coincident(shank.Center(), d.Origin())
	rod := d.Features.Extrude(24, "rod")
	var end Entity
	for _, e := range rod.Entities() {
		if e.Kind() == "circle" && e.Center().Pos().Z > 12 {
			end = e
		}
	}
	d.Features.Thread(end, ThreadOptions{Designation: "S20x4", Length: 24, Reverse: true, Through: true, Clearance: 0.1, Crest: crest})
	return end
}

// buttressLowest is g's circle of the hole lowest down.
func buttressLowest(g Group) Entity {
	var best Entity
	for _, e := range g.Entities() {
		if e.Kind() == "circle" && math.Abs(e.Radius()-10) < 1e-6 &&
			(best.H.V == 0 || e.Center().Pos().Z < best.Center().Pos().Z) {
			best = e
		}
	}
	return best
}

// TestScrewFitsButtressThreads screws a buttress nut onto a buttress
// rod. The rod's thread runs down and the nut's up, so their load
// flanks face the same way and the screw mate refuses them; the nut
// made with its load flank back fits: the mate moves it to where its
// ridges lie in the rod's grooves, leaning the same way, and it fits at
// every turn after, where half a pitch off it collides.
func TestScrewFitsButtressThreads(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("interferences of threads by their meshes, a few seconds")
	}
	d := New()
	end := buttressRod(d, "")
	g, err := d.Assembly.Link(buttressNut(t, false, ""), "ahead")
	if err != nil {
		t.Fatal(err)
	}
	d.Sketch.Anywhere3D()
	mustFail(t, "load flanks the same way", "load flanks face the same way", func() {
		d.Assembly.Screw(end, buttressLowest(g), 0)
	})

	screwButtress(t, "")
}

// TestScrewFitsRoundedButtressThreads screws a buttress nut with a
// rounded crest onto a rod whose crest is rounded too: the mate holds
// the ridges by their arcs' centres, which lie off the ridges' middles,
// and still puts the nut's ridges in the rod's grooves.
func TestScrewFitsRoundedButtressThreads(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("interferences of threads by their meshes, a few seconds")
	}
	screwButtress(t, "rounded")
}

// screwButtress screws the nut, its load flank turned back, onto the
// rod, both with the crest named, and requires it fits at every turn
// and collides half a pitch off.
func screwButtress(t *testing.T, crest string) {
	t.Helper()
	d := New()
	end := buttressRod(d, crest)
	g, err := d.Assembly.Link(buttressNut(t, true, crest), "back")
	if err != nil {
		t.Fatal(err)
	}
	link := g.AsLink()
	o := link.Offset()
	link.Move(V(-o.X, -o.Y, 14-o.Z))
	d.Sketch.Anywhere3D()
	mate := d.Assembly.Screw(end, buttressLowest(g), 0)
	if lead, _ := d.Params.Get(mate.Lead); math.Abs(lead.Value-4) > 1e-9 {
		t.Errorf("lead %v, want the S20x4's 4", lead.Value)
	}
	for _, turn := range []string{"0", "90", "-200"} {
		if err := d.Params.Set(mate.Turn, turn); err != nil {
			t.Fatal(err)
		}
		if found, _ := d.Assembly.Interferences(); len(found) != 0 {
			t.Errorf("turned %s: %+v", turn, found)
		}
	}
	// Half a turn on, a lead of 4 and half a pitch: the ridges collide.
	if err := d.Params.Set(mate.Lead, "0"); err != nil {
		t.Fatal(err)
	}
	if err := d.Params.Set(mate.Turn, "180"); err != nil {
		t.Fatal(err)
	}
	if found, _ := d.Assembly.Interferences(); len(found) != 1 {
		t.Errorf("half a pitch off: %+v, want the nut to collide", found)
	}
}

// TestScrewFitsButtressLoadWays screws a B1.9 nut onto a B1.9 rod by
// the way of the load their designations say. The rod's thread runs
// down from its top, which goes into the nut first, and the nut's up
// from its bottom, which meets the rod first: a screw and a nut that
// both push (PUSH-BUTT) or both pull (BUTT) mate, their load flanks
// facing each other, and one of each is refused. Only the way of each
// thread's load matters, so the rod is 10 mm long and the nut 6: the
// modelled threads were all the cost, at 40 and 15.
func TestScrewFitsButtressLoadWays(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("buttress threads screwed together, a few seconds")
	}
	const r = 12.7
	nutFile := func(designation string) string {
		nut := New()
		disc := nut.Sketch.Circle(V(0, 0, 0), 25)
		nut.Constrain.Coincident(disc.Center(), nut.Origin())
		nut.Features.Extrude(6, "disc")
		nut.Features.SketchIn(nut.Workplane("XY"), "hole")
		hole := nut.Sketch.Circle(V(0, 0, 0), r)
		nut.Constrain.Coincident(hole.Center(), nut.Origin())
		nut.Features.Thread(hole, ThreadOptions{Designation: designation, Internal: true, Through: true, Length: 6, Class: "2B"})
		path := filepath.Join(t.TempDir(), "nut.tlspr")
		if err := nut.File.Save(path); err != nil {
			t.Fatal(err)
		}
		return path
	}
	cases := []struct {
		rod, nut string
		mates    bool
	}{
		{"1-12 PUSH-BUTT", "1-12 PUSH-BUTT", true},
		{"1-12 BUTT", "1-12 PULL-BUTT", true},
		{"1-12 PUSH-BUTT", "1-12 BUTT", false},
		{"1-12 BUTT", "1-12 PUSH-BUTT", false},
	}
	if fixtures.RaceDetector {
		// Minutes each there: one pair that mates, one that doesn't.
		cases = []struct {
			rod, nut string
			mates    bool
		}{cases[0], cases[2]}
	}
	for _, c := range cases {
		d := New()
		shank := d.Sketch.Circle(V(0, 0, 0), r)
		d.Constrain.Coincident(shank.Center(), d.Origin())
		rod := d.Features.Extrude(10, "rod")
		var end Entity
		for _, e := range rod.Entities() {
			if e.Kind() == "circle" && e.Center().Pos().Z > 5 {
				end = e
			}
		}
		d.Features.Thread(end, ThreadOptions{Designation: c.rod, Length: 10, Reverse: true, Through: true, Class: "2A"})
		g, err := d.Assembly.Link(nutFile(c.nut), "nut")
		if err != nil {
			t.Fatal(err)
		}
		var hole Entity
		for _, e := range g.Entities() {
			if e.Kind() == "circle" && math.Abs(e.Radius()-r) < 1e-6 &&
				(hole.H.V == 0 || e.Center().Pos().Z < hole.Center().Pos().Z) {
				hole = e
			}
		}
		d.Sketch.Anywhere3D()
		if c.mates {
			d.Assembly.Screw(end, hole, 0)
			continue
		}
		mustFail(t, c.rod+" on "+c.nut, "load flanks face the same way", func() {
			d.Assembly.Screw(end, hole, 0)
		})
	}
}

// TestThreadRoundRootOnHeadedRod threads a rod with a head added after
// it, its roots rounded, at chord tolerances far coarser than the
// roots' arcs take: the rod and its head, two groups before the
// thread, are made at the thread's tolerance, and the screw comes out
// sound at the same volume at each. With only the group before the
// thread made at it, the rod's edges kept its coarse chords through
// the head's union, and the 3/4-10 BUTT screw had edges open along
// its root from 0.5%. The ordinary run takes 0.5 and 2, where the head
// broke it, and make test-sweeps the finer 0.1 too.
func TestThreadRoundRootOnHeadedRod(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("threads with rounded roots")
	}
	var first float64
	tols := []float64{0.5, 2}
	switch {
	case sweep.Full():
		tols = []float64{0.1, 0.5, 2}
	case fixtures.RaceDetector:
		// Each thread is made at its roots' own tolerance, minutes there;
		// the coarsest is the one the rod's head broke.
		tols = tols[1:]
	}
	for _, tol := range tols {
		d := New()
		d.Edit.SetChordTolerance(tol)
		shank := d.Sketch.Circle(V(0, 0, 0), 9.525)
		d.Constrain.Coincident(shank.Center(), d.Origin())
		rod := d.Features.Extrude(40, "rod")
		rod.AsExtrude().SetDepth(40)
		var bottom Entity
		for _, e := range rod.Entities() {
			if e.Kind() == "circle" && e.Center().Pos().Z < 1 {
				bottom = e
			}
		}
		d.Features.SketchOnFace(rod.Solid().FaceAt(V(0, 0, 40)), "head")
		head := d.Sketch.Circle(V(0, 0, 0), 15)
		d.Constrain.Coincident(d.Sketch.ActiveWorkplane().Points()[0], head.Center())
		d.Features.Extrude(6, "headx").AsExtrude().SetDepth(6)
		g := d.Features.Thread(bottom, ThreadOptions{Designation: "3/4-10 BUTT", Length: 25, RoundRoot: true})
		tg, ok := g.ThreadOf()
		if !ok {
			t.Fatalf("at %g%%: no thread", tol)
		}
		gs := tg.Groups()
		r := gs[len(gs)-1].Analyze().Check()
		if !r.Sound() {
			t.Errorf("at %g%%: not sound", tol)
		}
		if first == 0 {
			first = r.Volume
		} else if math.Abs(r.Volume-first) > 1e-6*first {
			t.Errorf("at %g%%: %.3f mm³, %.3f at %g%%", tol, r.Volume, first, tols[0])
		}
	}
}
