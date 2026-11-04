package sketch_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// foldPath draws, in the ZX workplane, up from the origin by lead, then
// at a sharp corner of 120° to an arc of radius 15 turning by turn
// degrees the same way as the corner: at 90° the arc comes back to
// within 5.5 of the line, at 270° it loops round across it.
func foldPath(d *api.Doc, lead, turn float64) []api.Entity {
	return turnPath(d, false, 120, turn, lead)
}

// A sweep whose path comes back within the profile's reach of where it
// has been would pass through itself: the arc past a sharp corner
// turning back over the line before it, or looping round across it.
// Each piece is swept sound and the corner joined, so only the solid's
// own mesh shows it. A sweep along lines and arcs is made in stretches
// each clear of itself, joined: the solid the profile passes through,
// sound, the hook's end grown into its shaft. Its volume is the
// crossing solid's less what that counted twice, the line's tube within
// the arc's beyond its first quarter turn, integrated on a grid (the
// first quarter's own overlap, near the corner, the joint trims and
// the crossing solid counts once). Held to the arc's axis, a fitted
// sweep, it is refused, naming it, with no solid. One step of undo takes
// either back.
func TestSweepCrossesItself(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name       string
		turn       float64
		held, made bool
	}{
		{"folds back", 90, false, true},
		{"loops round", 270, false, true},
		{"folds back, held to the arc's axis", 90, true, false},
	} {
		d := api.New()
		path := foldPath(d, 25, c.turn)
		d.Features.SketchIn(d.Workplane("XY"), "profile")
		d.Sketch.Circle(api.V(0, 0, 0), 3)
		step(d)
		groups := len(d.Groups())
		s := d.Features.Sweep(path, "")
		if c.held {
			s.AsSweep().SetSweepUp(api.V(0, 1, 0))
		}
		p, r := s.Problem(), s.Analyze().Check()
		switch {
		case c.made:
			if p != "" || !r.Sound() {
				t.Errorf("%s: %q %s, want made sound", c.name, p, r.String())
				continue
			}
			want := crossingVolume(t, d, s) - foldOverlap(3, c.turn)
			if math.Abs(r.Volume-want) > 1e-3*want {
				t.Errorf("%s: volume %.3f, want %.3f", c.name, r.Volume, want)
			}
		case !strings.Contains(p, sketch.ErrSweepCrossesItself.Error()) || !r.Empty():
			t.Errorf("%s: %q %s, want refused as passing through itself, with no solid", c.name, p, r.String())
		}
		// The arc's end is no round number, so the file after an undo
		// can differ from the one before in its last bits, as the sketch
		// is solved again: the group's count says the undo took it.
		if !c.held {
			if !d.Edit.Undo() || len(d.Groups()) != groups {
				t.Errorf("%s: undo left %d groups, want %d", c.name, len(d.Groups()), groups)
			}
			if !d.Edit.Redo() || len(d.Groups()) != groups+1 {
				t.Errorf("%s: redo didn't bring the sweep back", c.name)
			}
		}
	}
}

// crossingVolume is the volume of the mesh of the sweep s made in one
// piece, crossing itself: the overlap counted twice.
func crossingVolume(t *testing.T, d *api.Doc, s api.Group) float64 {
	t.Helper()
	v := 0.0
	d.With(func(sk *sketch.Sketch) {
		g := sk.Group(s.H)
		steps, err := g.SweepSteps(sk)
		if err != nil {
			t.Fatal(err)
		}
		src := sk.Group(g.OpA)
		sh := srf.SShell{Tol: sk.Tol()}
		sh.MakeFromSweepOf(&src.BezierLoops.Sets[0], steps, g.Color, 1, 2)
		var m srf.SMesh
		sh.TriangulateInto(&m)
		v = m.CalculateVolume()
	})
	return v
}

// foldOverlap is the volume foldPath's sweep of a circle of radius rad
// passes through twice: the line's tube, along z from 0 to 25, within
// the arc's tube past its first quarter turn, on a grid of cells 0.05.
func foldOverlap(rad, turn float64) float64 {
	y := api.V(0, 1, 0)
	top := api.V(0, 0, 25)
	c := top.Plus(api.V(0, 0, 1).RotatedAroundAxis(y, 120*math.Pi/180).RotatedAroundAxis(y, math.Pi/2).ScaledBy(15))
	a0 := math.Atan2(top.X-c.X, top.Z-c.Z)
	next := c.Plus(top.Minus(c).RotatedAroundAxis(y, 0.01))
	way := 1.0
	if math.Remainder(math.Atan2(next.X-c.X, next.Z-c.Z)-a0, 2*math.Pi) < 0 {
		way = -1
	}
	const h = 0.05
	v := 0.0
	for x := -rad + h/2; x < rad; x += h {
		for yy := -rad + h/2; yy < rad; yy += h {
			if x*x+yy*yy >= rad*rad {
				continue
			}
			for z := h / 2; z < 25; z += h {
				px, pz := x-c.X, z-c.Z
				if rho := math.Hypot(px, pz); (rho-15)*(rho-15)+yy*yy >= rad*rad {
					continue
				}
				if a := math.Mod(way*(math.Atan2(px, pz)-a0)+4*math.Pi, 2*math.Pi); a < math.Pi/2 || a > turn*math.Pi/180 {
					continue
				}
				v += h * h * h
			}
		}
	}
	return v
}

// turnPath draws, in the ZX workplane, a line of length lead and an arc
// of radius 15 turning by turn degrees, about a sharp corner of corner
// degrees between them; the line first, or the arc.
func turnPath(d *api.Doc, arcFirst bool, corner, turn, lead float64) []api.Entity {
	d.Features.SketchIn(d.Workplane("ZX"), "path")
	y := api.V(0, 1, 0)
	way := math.Copysign(math.Pi/2, turn)
	arc := func(c, from, to api.Vec) api.Entity {
		if turn < 0 {
			return d.Sketch.Arc(c, to, from)
		}
		return d.Sketch.Arc(c, from, to)
	}
	if !arcFirst {
		top := api.V(0, 0, lead)
		dir := api.V(0, 0, 1).RotatedAroundAxis(y, corner*math.Pi/180)
		c := top.Plus(dir.RotatedAroundAxis(y, way).ScaledBy(15))
		end := c.Plus(top.Minus(c).RotatedAroundAxis(y, turn*math.Pi/180))
		return []api.Entity{d.Sketch.Line(api.V(0, 0, 0), top), arc(c, top, end)}
	}
	o := api.V(0, 0, 0)
	c := api.V(math.Copysign(15, turn), 0, 0)
	end := c.Plus(o.Minus(c).RotatedAroundAxis(y, turn*math.Pi/180))
	along := end.Minus(c).RotatedAroundAxis(y, way).WithMagnitude(1).RotatedAroundAxis(y, corner*math.Pi/180)
	return []api.Entity{arc(c, o, end), d.Sketch.Line(end, end.Plus(along.ScaledBy(lead)))}
}

// Round a line and an arc about a sharp corner, every sweep is sound or
// a problem: none is made crossing itself. On 2026-11-04, of the grid's
// 1440 points, 160 passed through themselves, all with the arc turning
// back the way of a sharp corner; 81 of them are made in stretches,
// sound, and 79 are ErrSweepCrossesItself. The 1088 sound before are
// made as they were.
func TestSweepCrossesItselfSweep(t *testing.T) {
	t.Parallel()
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Bools("arcFirst"),
			sweep.Floats("corner", -150, -120, -90, -60, -30, 30, 60, 90, 120, 150),
			sweep.Floats("turn", -270, -180, -90, -45, 45, 90, 180, 270),
			sweep.Floats("lead", 12, 25, 40),
			sweep.Floats("profile", 0, 1, 2),
		},
		MaxFailRate: 0,
		Sample:      24,
	}
	grid.Run(func(p sweep.Point) error {
		d := api.New()
		path := turnPath(d, p.Bool("arcFirst"), p.Float("corner"), p.Float("turn"), p.Float("lead"))
		d.Features.SketchIn(d.Workplane("XY"), "profile")
		switch p.Float("profile") {
		case 0:
			d.Sketch.Circle(api.V(0, 0, 0), 3)
		case 1:
			d.Sketch.Rect(api.V(-3, -2, 0), api.V(3, 2, 0))
		default:
			d.Sketch.Circle(api.V(0, 0, 0), 1)
		}
		s := d.Features.Sweep(path, "")
		if s.Problem() != "" {
			return nil
		}
		if r := s.Analyze().Check(); !r.Sound() {
			return fmt.Errorf("made unsound, with no problem: %s", r.String())
		}
		return nil
	}).Require(t)
}
