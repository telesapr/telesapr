package sketch_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// cylinderBody is a solid cylinder of radius r on the z axis from z = 0
// to height, and its round face.
func cylinderBody(d *api.Doc, r, height float64) (api.Group, api.SolidFace) {
	d.Sketch.Circle(api.V(0, 0, 0), r)
	ex := d.Features.Extrude(height, "")
	return ex, ex.Solid().FaceAt(api.V(r, 0, height/2))
}

// wrappedBand is the volume of the band a profile of area a makes on a
// cylinder of radius r between the heights h0 and h1 off it: the wrap
// keeps areas, and the band's sections scale with the radius.
func wrappedBand(a, r, h0, h1 float64) float64 {
	return a * ((r+h1)*(r+h1) - (r+h0)*(r+h0)) / (2 * r)
}

// bandTol is how near the volume a band adds to a body's mesh comes to
// the band's exact volume: the body's curved face is meshed in other
// chords once the band is cut into it, which moves the volume by as much
// as 2% of a small band. The kernel's tests hold the exact volumes at a
// finer mesh (pkg/srf, wrap_test.go).
const bandTol = 0.025

func TestWrapEmbossRectangle(t *testing.T) {
	t.Parallel()
	d := api.New()
	ex, face := cylinderBody(d, 10, 40)
	base := ex.Analyze().Check().Volume
	d.Features.SketchIn(d.Workplane("ZX"), "")
	d.Sketch.Rect(api.V(-5, 0, 10), api.V(5, 0, 30))
	step(d)
	before := saved(t, d)
	w := d.Features.Emboss(face, 1, "")
	if p := w.Problem(); p != "" {
		t.Fatal(p)
	}
	r := w.Analyze().Check()
	if !r.Sound() || !nearRel(r.Volume-base, wrappedBand(200, 10, 0, 1), bandTol) || w.Kind() != "wrap" ||
		w.AsWrap().Engraved() || w.AsWrap().Height() != 1 {
		t.Fatalf("embossed: %s, added %g, want %g", r.String(), r.Volume-base, wrappedBand(200, 10, 0, 1))
	}
	undoRedo(t, d, before)
	// It depends on the extrusion whose face it lies on.
	dependsOnEx := false
	for _, g := range w.History().DependsOn() {
		dependsOnEx = dependsOnEx || g.Name() == ex.Name()
	}
	if !dependsOnEx {
		t.Fatalf("the wrap depends on %v, not on %s", w.History().DependsOn(), ex.Name())
	}
	// The height takes an expression.
	if err := w.SetExpr(sketch.GroupPitch, "1+1"); err != nil {
		t.Fatal(err)
	}
	d.Edit.Regen()
	if v := w.Analyze().Check().Volume; w.AsWrap().Height() != 2 || !nearRel(v-base, wrappedBand(200, 10, 0, 2), bandTol) {
		t.Fatalf("at height %g the band is %g", w.AsWrap().Height(), v-base)
	}
	if err := w.SetExpr(sketch.GroupPitch, "0"); err == nil {
		t.Fatal("a height of 0 was taken")
	}
	back := roundTrip(t, d)
	bw := back.Group(w.Name())
	if bw.Kind() != "wrap" || bw.Expr(sketch.GroupPitch) != "1+1" || !nearRel(bw.Analyze().Check().Volume-base, wrappedBand(200, 10, 0, 2), bandTol) {
		t.Fatalf("reopened: %s, %q", bw.Kind(), bw.Expr(sketch.GroupPitch))
	}
	// The band's edges have names.
	s := w.Solid()
	if len(s.Problems()) != 0 {
		t.Fatalf("topology: %v", s.Problems())
	}
	for _, e := range s.Edges() {
		if e.Name() == "" {
			t.Fatal("an edge with no name")
		}
	}
	// Engraved instead, through the combine mode.
	w.SetCombine(api.Difference)
	d.Edit.Regen()
	if v := w.Analyze().Check(); !v.Sound() || !w.AsWrap().Engraved() || !nearRel(base-v.Volume, wrappedBand(200, 10, -2, 0), bandTol) {
		t.Fatalf("engraved: %s, took %g", v.String(), base-v.Volume)
	}
}

// tubeBody is a tube of outer radius r and wall wall on the z axis from
// z = 0 to height, and its outer face.
func tubeBody(d *api.Doc, r, wall, height float64) (api.Group, api.SolidFace) {
	d.Sketch.Circle(api.V(0, 0, 0), r)
	d.Sketch.Circle(api.V(0, 0, 0), r-wall)
	ex := d.Features.Extrude(height, "")
	return ex, ex.Solid().FaceAt(api.V(r, 0, height/2))
}

// A band reaches through the face it is joined to by a part of its
// height; on a wall thinner than that it stood out of the wall's inside.
// It reaches no further than half the wall, and where the wall is too
// thin even for that the group says so.
func TestWrapEmbossThinWall(t *testing.T) {
	t.Parallel()
	d := api.New()
	ex, face := tubeBody(d, 10, 0.3, 40)
	base := ex.Analyze().Check().Volume
	d.Features.SketchIn(d.Workplane("ZX"), "")
	d.Sketch.Rect(api.V(-5, 0, 10), api.V(5, 0, 30))
	w := d.Features.Emboss(face, 4, "")
	if p := w.Problem(); p != "" {
		t.Fatal(p)
	}
	if r := w.Analyze().Check(); !r.Sound() || !nearRel(r.Volume-base, wrappedBand(200, 10, 0, 4), bandTol) {
		t.Fatalf("embossed on a thin wall: %s, added %g, want %g", r.String(), r.Volume-base, wrappedBand(200, 10, 0, 4))
	}

	d = api.New()
	_, face = tubeBody(d, 10, 0.004, 40)
	d.Features.SketchIn(d.Workplane("ZX"), "")
	d.Sketch.Rect(api.V(-5, 0, 10), api.V(5, 0, 30))
	if w := d.Features.Emboss(face, 1, ""); !strings.Contains(w.Problem(), "wall under the emboss") {
		t.Fatalf("on a wall of 0.004: %q", w.Problem())
	}
}

// A profile wider than half the face's way round has caps of several
// patches, which meet at seams across it.
func TestWrapRoundMostOfTheWay(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name    string
		engrave bool
		hole    bool
	}{{"raised", false, false}, {"sunk", true, false}, {"raised with a hole", false, true}, {"sunk with a hole", true, true}} {
		t.Run(c.name, func(t *testing.T) {
			d := api.New()
			// The narrow ring left round the hole is small beside the
			// cylinder's chords.
			d.Edit.SetChordTolerance(0.005)
			ex, face := cylinderBody(d, 10, 40)
			base := ex.Analyze().Check().Volume
			d.Features.SketchIn(d.Workplane("ZX"), "")
			// 50 mm across a radius of 10: 286° round.
			d.Sketch.Rect(api.V(-25, 0, 10), api.V(25, 0, 30))
			area := 1000.0
			if c.hole {
				d.Sketch.Rect(api.V(-20, 0, 15), api.V(20, 0, 25))
				area -= 400
			}
			var w api.Group
			want := wrappedBand(area, 10, 0, 1)
			if c.engrave {
				w = d.Features.Engrave(face, 1, "")
				want = wrappedBand(area, 10, -1, 0)
			} else {
				w = d.Features.Emboss(face, 1, "")
			}
			if p := w.Problem(); p != "" {
				t.Fatal(p)
			}
			r := w.Analyze().Check()
			if got := math.Abs(r.Volume - base); !r.Sound() || !nearRel(got, want, bandTol) {
				t.Fatalf("%s: %s, band %g, want %g", c.name, r.String(), got, want)
			}
		})
	}
	d := api.New()
	_, face := cylinderBody(d, 10, 40)
	d.Features.SketchIn(d.Workplane("ZX"), "")
	d.Sketch.Rect(api.V(-32, 0, 10), api.V(32, 0, 30))
	if !refused(func() { d.Features.Emboss(face, 1, "") }) {
		t.Fatal("a profile wrapped more than round the face was taken")
	}
}

func TestWrapEngraveCircle(t *testing.T) {
	t.Parallel()
	d := api.New()
	// The circle's pocket is small beside the cylinder's chords.
	d.Edit.SetChordTolerance(0.005)
	ex, face := cylinderBody(d, 10, 40)
	base := ex.Analyze().Check().Volume
	d.Features.SketchIn(d.Workplane("ZX"), "")
	d.Sketch.Circle(api.V(3, 0, 20), 4)
	w := d.Features.Engrave(face, 0.5, "")
	r := w.Analyze().Check()
	if want := wrappedBand(16*math.Pi, 10, -0.5, 0); !r.Sound() || !nearRel(base-r.Volume, want, bandTol) {
		t.Fatalf("engraved: %s, took %g, want %g", r.String(), base-r.Volume, want)
	}
}

// On a cone a sheet of the band at height h is the face's area grown by
// 1 + h/R, R = rho tan(alpha) the face's distance from the axis along
// its normal, rho the slant distance from the apex: the band raised h
// over a profile of area A is A h + h²/2 times the integral of 1/R over
// it, taken here by the midpoint rule over the development, where the
// sketch's middle lies on the generator at its own distance along the
// axis from the apex.
func TestWrapEmbossCone(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Features.SketchIn(d.Workplane("ZX"), "")
	axis := d.Sketch.Line(api.V(0, 0, -2), api.V(0, 0, 32))
	axis.SetConstruction(true)
	d.Sketch.Polyline(true, api.V(0, 0, 0), api.V(10, 0, 0), api.V(4, 0, 30), api.V(0, 0, 30))
	cone := d.Features.Lathe(axis, "")
	base := cone.Analyze().Check().Volume
	face := cone.Solid().FaceAt(api.V(7, 0, 15))
	d.Features.SketchIn(d.Workplane("ZX"), "")
	d.Sketch.Rect(api.V(-3, 0, 10), api.V(3, 0, 20))
	w := d.Features.Emboss(face, 1, "")
	if p := w.Problem(); p != "" {
		t.Fatal(p)
	}
	r := w.Analyze().Check()
	// The apex is at z = 50; the rectangle runs 30 to 40 from it along
	// the axis, its middle 35.
	alpha := math.Atan(6.0 / 30)
	rhoC := 35 / math.Cos(alpha)
	const n = 200
	sum := 0.0
	for i := range n {
		for j := range n {
			sx := -3 + 6*(float64(i)+0.5)/n
			z := 30 + 10*(float64(j)+0.5)/n
			sum += 1 / (math.Hypot(sx, rhoC+z-35) * math.Tan(alpha))
		}
	}
	want := 60*1 + 0.5*sum*(6.0/n)*(10.0/n)
	if !r.Sound() || !nearRel(r.Volume-base, want, bandTol) {
		t.Fatalf("embossed cone: %s, added %g, want %g", r.String(), r.Volume-base, want)
	}
}

func TestWrapRefusals(t *testing.T) {
	t.Parallel()
	d := api.New()
	ex, face := cylinderBody(d, 10, 40)
	top := ex.Solid().FaceAt(api.V(0, 0, 40))
	refused := func(what, want string, f func()) {
		t.Helper()
		defer func() {
			t.Helper()
			err, ok := recover().(error)
			if !ok || !strings.Contains(err.Error(), want) {
				t.Fatalf("%s: %v, want an error saying %q", what, err, want)
			}
		}()
		f()
	}
	d.Features.SketchIn(d.Workplane("XY"), "")
	d.Sketch.Rect(api.V(-2, -2, 0), api.V(2, 2, 0))
	refused("a sketch square to the axis", "not parallel", func() { d.Features.Emboss(face, 1, "") })
	d.Features.SketchIn(d.Workplane("ZX"), "")
	d.Sketch.Rect(api.V(-2, 0, 10), api.V(2, 0, 20))
	refused("a flat face", "neither a cylinder nor a cone", func() { d.Features.Emboss(top, 1, "") })
	refused("no height", "more than zero", func() { d.Features.Emboss(face, 0, "") })
	refused("an engraving through the axis", "axis", func() { d.Features.Engrave(face, 12, "") })
	d.Features.SketchIn(d.Workplane("ZX"), "")
	d.Sketch.Rect(api.V(-32, 0, 10), api.V(32, 0, 20))
	refused("a loop round 367°", "meet itself", func() { d.Features.Emboss(face, 1, "") })
	d.Features.SketchIn(d.Workplane("ZX"), "")
	d.Sketch.Line(api.V(-2, 0, 10), api.V(2, 0, 20))
	refused("an open sketch", "closed loops", func() { d.Features.Emboss(face, 1, "") })
}

// wrapTextCase wraps text onto a cylinder or a frustum of a cone of
// radius r at the bottom, embossed or engraved by k of the letters'
// height, and checks the solid is sound.
func wrapTextCase(p sweep.Point) error {
	r, size := p.Float("radius"), p.Float("size")
	h := p.Float("k") * size
	text := p.String("text")
	d := api.New()
	var face api.SolidFace
	if p.String("face") == "cylinder" {
		d.Sketch.Circle(api.V(0, 0, 0), r)
		body := d.Features.Extrude(4*size, "")
		face = body.Solid().FaceAt(api.V(r, 0, 2*size))
	} else {
		d.Features.SketchIn(d.Workplane("ZX"), "")
		axis := d.Sketch.Line(api.V(0, 0, -1), api.V(0, 0, 4*size+1))
		axis.SetConstruction(true)
		d.Sketch.Polyline(true, api.V(0, 0, 0), api.V(r, 0, 0), api.V(0.7*r, 0, 4*size), api.V(0, 0, 4*size))
		body := d.Features.Lathe(axis, "")
		face = body.Solid().FaceAt(api.V(0.85*r, 0, 2*size))
	}
	// In a plane through the axis, running round it.
	d.Features.SketchAt(d.Origin(), api.V(1, 0, 0), api.V(0, 0, 1), "")
	d.Sketch.Text(text, "", api.V(-0.33*size*float64(len(text)), 0, 1.5*size), size)
	var w api.Group
	if p.Bool("engrave") {
		w = d.Features.Engrave(face, h, "")
	} else {
		w = d.Features.Emboss(face, h, "")
	}
	r2 := w.Analyze().Check()
	switch {
	case w.Problem() != "":
		return fmt.Errorf("problem: %s", w.Problem())
	case !r2.Sound():
		return fmt.Errorf("unsound: %s", r2.String())
	}
	return nil
}

// wrapTextGrid is the text sweep on one kind of face: strings of
// straight and curved letters, small to large on the face, raised or
// sunk by a tenth or a half of their height.
func wrapTextGrid(face string, maxFail, sampleMaxFail float64) sweep.Grid {
	return sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Strings("face", face),
			sweep.Strings("text", "TeleSAPR", "Ag 0.4%", "B&@q"),
			sweep.Floats("radius", 10, 20, 60),
			sweep.Floats("size", 3, 6),
			sweep.Floats("k", 0.1, 0.5),
			sweep.Bools("engrave"),
		},
		MaxFailRate:       maxFail,
		Sample:            8,
		SampleMaxFailRate: sampleMaxFail,
	}
}

// Text wrapped onto cylinders: none may fail, the rate fixed before the
// group was built.
func TestWrapTextCylinderSweep(t *testing.T) {
	t.Parallel()
	wrapTextGrid("cylinder", 0, 0).Run(wrapTextCase).Require(t)
}

// Text wrapped onto cones: none may fail. 14 of 72 did on 2026-10-22,
// every one a band half as deep as its letters are high: the cap of a
// band sunk deep reached less far along the cone than its letters, and
// marched curves kept vertices a hair from their ends.
func TestWrapTextConeSweep(t *testing.T) {
	t.Parallel()
	wrapTextGrid("cone", 0, 0).Run(wrapTextCase).Require(t)
}

// BenchmarkWrapText embosses a word onto a cylinder: the band, its
// Boolean with the cylinder, and the check.
func BenchmarkWrapText(b *testing.B) {
	for b.Loop() {
		d := api.New()
		_, face := cylinderBody(d, 20, 40)
		d.Features.SketchAt(d.Origin(), api.V(1, 0, 0), api.V(0, 0, 1), "")
		d.Sketch.Text("TeleSAPR", "", api.V(-15, 0, 15), 6)
		if w := d.Features.Emboss(face, 0.6, ""); w.Problem() != "" {
			b.Fatal(w.Problem())
		}
	}
}
