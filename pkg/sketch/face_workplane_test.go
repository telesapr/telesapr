package sketch_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// nearPoint reports whether two points are within a micrometre.
func nearPoint(p, q api.Vec) bool { return p.Minus(q).Magnitude() < 1e-6 }

// frameOf is the origin, u and normal of a group's workplane.
func frameOf(d *api.Doc, g api.Group) (origin, u, n api.Vec) {
	return frameIn(d.Host().Sketch(), g.H)
}

// frameIn is frameOf read from the sketch as it is, regenerating
// nothing.
func frameIn(sk *sketch.Sketch, hg core.HGroup) (origin, u, n api.Vec) {
	wp := sk.MustEntity(hg.Entity(0))
	nn := wp.NormalEntity(sk)
	return sk.MustEntity(wp.Point[0]).PointGetNum(sk), nn.NormalU(sk), nn.NormalN(sk)
}

// A sketch on the top of a 20 x 10 x 5 block lies in it, its normal up
// and its origin on a corner with the face in its first quadrant; a
// boss drawn there stands on the block, and when the block grows the
// sketch and the boss go up with its top.
func TestSketchOnFaceFollowsFace(t *testing.T) {
	t.Parallel()
	d := api.New()
	ex := block(d, 0, 0, 20, 10, 5)
	top := ex.Solid().FaceAt(api.V(10, 5, 5))
	step(d)
	before := saved(t, d)
	sk := d.SketchOnFace(top, "")
	if p := sk.Problem(); p != "" || sk.Kind() != "sketch-in-plane" || sk.Name() != "sketch-on-face" {
		t.Fatalf("problem %q, kind %s, name %s", p, sk.Kind(), sk.Name())
	}
	undoRedo(t, d, before)
	o, u, n := frameOf(d, sk)
	if !nearPoint(n, api.V(0, 0, 1)) || o.Z != 5 || math.Abs(u.Z) > 1e-12 {
		t.Fatalf("frame: origin %v, u %v, normal %v", o, u, n)
	}
	// The face lies on the side of v from the origin's edge.
	v := n.Cross(u)
	if inside := o.Plus(u.ScaledBy(0.1)).Plus(v.ScaledBy(0.1)); inside.X <= 0 || inside.X >= 20 || inside.Y <= 0 || inside.Y >= 10 {
		t.Fatalf("the face is not in the first quadrant: origin %v, u %v, v %v", o, u, v)
	}
	if f := sk.Face(); f.ID() != top.ID() {
		t.Fatalf("the sketch lies in face %#x, not %#x", f.ID(), top.ID())
	}

	d.Rect(api.V(2, 2, 5), api.V(6, 6, 5))
	boss := d.Extrude(3, "boss")
	if r := boss.Check(); !r.Sound() || math.Abs(r.Volume-(1000+48)) > 1e-6 {
		t.Fatalf("block with a boss: %s", r.String())
	}

	// Ten deep: the top, the sketch and the boss move up 5.
	ex.Param(2).Set(5)
	d.Regen()
	o2, u2, n2 := frameOf(d, sk)
	if !nearPoint(o2, o.Plus(api.V(0, 0, 5))) || !nearPoint(u2, u) || !nearPoint(n2, n) || sk.Problem() != "" {
		t.Fatalf("after the edit: origin %v, u %v, normal %v, problem %q", o2, u2, n2, sk.Problem())
	}
	if r := boss.Check(); !r.Sound() || math.Abs(r.Volume-(2000+48)) > 1e-6 || math.Abs(r.Max.Z-13) > 1e-9 {
		t.Fatalf("after the edit: %s", r.String())
	}

	back := roundTrip(t, d)
	if r := back.Group(boss.Name()).Check(); !r.Sound() || math.Abs(r.Volume-(2000+48)) > 1e-6 {
		t.Fatalf("reopened: %s", r.String())
	}
}

// When the face goes, the workplane stays where it was and the group
// says why; when it comes back, the workplane follows it again.
func TestSketchOnFaceLost(t *testing.T) {
	t.Parallel()
	d := api.New()
	ex := block(d, 0, 0, 20, 10, 5)
	sk := d.SketchOnFace(ex.Solid().FaceAt(api.V(10, 5, 5)), "")
	o, _, _ := frameOf(d, sk)
	ex.SetSuppress(api.SuppressSolid)
	p := sk.Problem()
	if !strings.Contains(p, "stays where it was") {
		t.Fatalf("problem %q", p)
	}
	if o2, _, _ := frameOf(d, sk); !nearPoint(o2, o) {
		t.Fatalf("the workplane moved to %v from %v", o2, o)
	}
	ex.SetSuppress(api.Unsuppressed)
	if p := sk.Problem(); p != "" {
		t.Fatalf("problem with the face back: %q", p)
	}
}

// The cap of a cylinder, which has no straight edge, puts the origin at
// its centre; a chamfer's face, which no entity names, is named by the
// group and its place.
func TestSketchOnFaceCapAndChamfer(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Circle(api.V(3, 4, 0), 2)
	cyl := d.Extrude(6, "")
	top := d.SketchOnFace(cyl.Solid().FaceAt(api.V(3, 4, 6)), "")
	if o, _, n := frameOf(d, top); !nearPoint(o, api.V(3, 4, 6)) || !nearPoint(n, api.V(0, 0, 1)) || top.Problem() != "" {
		t.Fatalf("cap: origin %v, normal %v, problem %q", o, n, top.Problem())
	}

	d2 := api.New()
	ex := block(d2, 0, 0, 10, 10, 10)
	d2.Chamfer([]api.Entity{lineAlong(t, ex, api.V(0, 0, 10), api.V(10, 0, 10))}, 1, "")
	slope := d2.SketchOnFace(d2.Group("chamfer").Solid().FaceAt(api.V(5, 0.5, 9.5)), "")
	o, u, n := frameOf(d2, slope)
	if !nearPoint(n, api.V(0, -1, 1).WithMagnitude(1)) || math.Abs(o.Minus(api.V(0, 0, 9)).Dot(n)) > 1e-9 ||
		math.Abs(u.Dot(n)) > 1e-12 || slope.Problem() != "" {
		t.Fatalf("chamfer face: origin %v, u %v, normal %v, problem %q", o, u, n, slope.Problem())
	}
	if id := srf.FaceID(slope.Face().ID()); id&(1<<30) == 0 {
		t.Fatalf("the chamfer's face %#x is not named by its group", uint32(id))
	}

	// The floor of a shelled box's cavity, its normal up into the cavity.
	d3 := api.New()
	box := block(d3, 0, 0, 20, 10, 10)
	sh := d3.Shell([]api.Entity{box.Solid().FaceAt(api.V(10, 5, 10)).Entity()}, 1, "")
	floor := d3.SketchOnFace(sh.Solid().FaceAt(api.V(10, 5, 1)), "")
	if o, _, n := frameOf(d3, floor); !nearPoint(n, api.V(0, 0, 1)) || o.Z != 1 || floor.Problem() != "" {
		t.Fatalf("cavity floor: origin %v, normal %v, problem %q", o, n, floor.Problem())
	}
}

// A face that is not flat, a solid that isn't there or is a mesh, are
// errors; the reasons say which.
func TestSketchOnFaceRefused(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Circle(api.V(0, 0, 0), 2)
	cyl := d.Extrude(6, "")
	side := cyl.Solid().FaceAt(api.V(2, 0, 3))
	d.With(func(sk *sketch.Sketch) {
		if _, err := sk.NewSketchOnFace(srf.FaceID(side.ID()), ""); err == nil || !strings.Contains(err.Error(), "not flat") {
			t.Errorf("the cylinder's side: %v", err)
		}
		if _, err := sk.NewSketchOnFace(0x7777, ""); err == nil || !strings.Contains(err.Error(), "no longer in the solid") {
			t.Errorf("a face there isn't: %v", err)
		}
	})
	forceToMesh(d, cyl)
	d.With(func(sk *sketch.Sketch) {
		if _, err := sk.NewSketchOnFace(srf.FaceID(side.ID()), ""); err == nil || !strings.Contains(err.Error(), "triangle mesh") {
			t.Errorf("a mesh: %v", err)
		}
	})
	empty := api.New()
	empty.With(func(sk *sketch.Sketch) {
		if _, err := sk.NewSketchOnFace(1, ""); err == nil || !strings.Contains(err.Error(), "no solid") {
			t.Errorf("no solid: %v", err)
		}
	})
}

// A sketch on a drafted side tilts with it when the draft changes, and
// a boss on it stands square to the side, in one regeneration: the
// solve placed the boss against the side as the last regeneration had
// it, and is made again.
func TestSketchOnFaceTilts(t *testing.T) {
	t.Parallel()
	d := api.New()
	ex := block(d, 0, 0, 20, 10, 10)
	ex.SetDraft(5)
	lean := math.Tan(5 * math.Pi / 180)
	side := d.SketchOnFace(ex.Solid().FaceAt(api.V(10, 5*lean, 5)), "")
	// The rectangle is square to the workplane's u, which runs along
	// the side's leaning edge; its size in the plane stays as drawn.
	ls := d.Rect(api.V(8, 0, 4), api.V(12, 0, 6))
	area := ls[0].Length() * ls[1].Length()
	boss := d.Extrude(1, "boss")
	for _, deg := range []float64{10, -3} {
		ex.SetDraft(deg)
		d.Regen()
		rad := deg * math.Pi / 180
		want := api.V(0, -math.Cos(rad), math.Sin(rad))
		if _, _, n := frameOf(d, side); !nearPoint(n, want) || side.Problem() != "" {
			t.Fatalf("draft %g: normal %v, want %v; problem %q", deg, n, want, side.Problem())
		}
		// The boss's depth is free, so the solve keeps its vector as
		// near as it can: square to the side, a little shorter.
		half := api.V(boss.Param(0).Val(), boss.Param(1).Val(), boss.Param(2).Val())
		if half.Cross(want).Magnitude() > 1e-9 {
			t.Fatalf("draft %g: the boss runs along %v, not %v", deg, half, want)
		}
		block, all := ex.Check(), boss.Check()
		if !all.Sound() || math.Abs(all.Volume-block.Volume-area*2*half.Magnitude()) > 1e-6 {
			t.Fatalf("draft %g: block %g, with the boss %s", deg, block.Volume, all.String())
		}
	}
}

// A unit boss on each face of a drafted block, drawn a unit from the
// sketch's origin along u and v, and the block then made deeper: every
// solid sound, the boss adding exactly its volume. None may fail (fixed
// before the first run).
func TestSketchOnFaceSweep(t *testing.T) {
	t.Parallel()
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Floats("draft", -10, 0, 5, 15),
			sweep.Strings("face", "top", "bottom", "front", "back", "left", "right"),
			sweep.Floats("deeper", 0, 3),
		},
		MaxFailRate: 0,
		Sample:      16,
	}
	grid.Run(func(p sweep.Point) error {
		d := api.New()
		ex := block(d, 0, 0, 20, 10, 10)
		lean := 5 * math.Tan(p.Float("draft")*math.Pi/180)
		if p.Float("draft") != 0 {
			ex.SetDraft(p.Float("draft"))
		}
		on := map[string]api.Vec{
			"top": api.V(10, 5, 10), "bottom": api.V(10, 5, 0),
			"front": api.V(10, lean, 5), "back": api.V(10, 10-lean, 5),
			"left": api.V(lean, 5, 5), "right": api.V(20-lean, 5, 5),
		}[p.String("face")]
		sk := d.SketchOnFace(ex.Solid().FaceAt(on), "")
		o, u, n := frameOf(d, sk)
		v := n.Cross(u)
		d.Rect(o.Plus(u).Plus(v), o.Plus(u.ScaledBy(2)).Plus(v.ScaledBy(2)))
		boss := d.Extrude(1, "boss")
		if deeper := p.Float("deeper"); deeper != 0 {
			ex.Param(2).Set(5 + deeper/2)
			d.Regen()
		}
		block, all := ex.Check(), boss.Check()
		switch {
		case sk.Problem() != "" || boss.Problem() != "":
			return fmt.Errorf("problems %q, %q", sk.Problem(), boss.Problem())
		case !all.Sound():
			return fmt.Errorf("unsound: %s", all.String())
		case math.Abs(all.Volume-block.Volume-1) > 1e-6:
			return fmt.Errorf("the boss adds %g, not 1", all.Volume-block.Volume)
		}
		return nil
	}).Require(t)
}

// A boss on a block's top, the face found again at each regeneration
// (F1); BenchmarkSketchOnFaceMoved has the face move each time, which
// solves the sketch and the boss a second time.
func BenchmarkSketchOnFace(b *testing.B) {
	benchRegen(b, func(d *api.Doc) {
		ex := block(d, 0, 0, 20, 10, 5)
		d.SketchOnFace(ex.Solid().FaceAt(api.V(10, 5, 5)), "")
		d.Rect(api.V(2, 2, 5), api.V(6, 6, 5))
		d.Extrude(3, "")
	})
}

func BenchmarkSketchOnFaceMoved(b *testing.B) {
	d := api.New()
	ex := block(d, 0, 0, 20, 10, 5)
	d.SketchOnFace(ex.Solid().FaceAt(api.V(10, 5, 5)), "")
	d.Rect(api.V(2, 2, 5), api.V(6, 6, 5))
	d.Extrude(3, "")
	deep := 2.5
	for b.Loop() {
		deep = 5 - deep
		ex.Param(2).Set(deep)
		d.Regen()
	}
}

// With the solids made off the thread, a regeneration places a sketch on
// a face from the solid as it was; when the job finds the face moved, it
// brings the sketch's entities with the solids, so the workplane and the
// boss on it are where a regeneration in full puts them.
func TestSketchOnFaceShellJob(t *testing.T) {
	t.Parallel()
	d := api.New()
	ex := block(d, 0, 0, 20, 10, 5)
	sk := d.SketchOnFace(ex.Solid().FaceAt(api.V(10, 5, 5)), "")
	d.Rect(api.V(2, 2, 5), api.V(6, 6, 5))
	boss := d.Extrude(3, "boss")
	s := d.Host().Sketch()
	ex.Param(2).Set(5)
	s.DeferShells = true
	s.GenerateAllTyped(sketch.GenerateDirty)
	s.DeferShells = false
	if o, _, _ := frameIn(s, sk.H); o.Z != 5 {
		t.Fatalf("before the job the sketch is at %v, not on the old top", o)
	}
	job := s.NewShellJob()
	job.Run()
	if !s.Apply(job) {
		t.Fatal("the job wasn't applied")
	}
	if o, _, _ := frameIn(s, sk.H); o.Z != 10 {
		t.Fatalf("after the job the sketch is at %v, not on the new top", o)
	}
	if r := boss.Check(); !r.Sound() || math.Abs(r.Volume-(2000+48)) > 1e-6 {
		t.Fatalf("after the job: %s", r.String())
	}
}
