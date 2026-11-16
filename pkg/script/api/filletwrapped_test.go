package api

import (
	"math"
	"testing"
)

// wrappedTube is a tube along x, radius 7.3, wall 0.8 and 60 long, as a
// lathe makes it of four quarter patches with their seams at -z, y, z and
// -y, and the face its outside is.
func wrappedTube(d *Doc) SolidFace {
	d.Sketch.Polyline(true, V(0, 6.5, 0), V(60, 6.5, 0), V(60, 7.3, 0), V(0, 7.3, 0))
	axis := d.Sketch.Line(V(0, 0, 0), V(60, 0, 0))
	axis.SetConstruction(true)
	tube := d.Features.LatheAbout(axis.Start(), axis, "tube")
	return tube.Solid().FaceAt(V(30, 0, -7.3))
}

// reliefFoot is the edges where a band embossed 0.3 on the tube meets it:
// those on the tube's outside next to a face of the band reaching its top.
func reliefFoot(g Group) []SolidEdge {
	rad := func(p Vec) float64 { return math.Hypot(p.Y, p.Z) }
	near := func(e SolidEdge, r float64) bool {
		for _, p := range e.Points() {
			if math.Abs(rad(p)-r) < 1e-3 {
				return true
			}
		}
		return false
	}
	var out []SolidEdge
	for _, e := range g.Solid().Edges() {
		on := true
		for _, p := range e.Points() {
			on = on && math.Abs(rad(p)-7.3) < 1e-3
		}
		if !on {
			continue
		}
		for _, f := range e.Faces() {
			reaches := false
			for _, x := range f.Edges() {
				reaches = reaches || near(x, 7.6)
			}
			if reaches {
				out = append(out, e)
				break
			}
		}
	}
	return out
}

// A strip 1 by 30 wrapped onto a tube and raised 0.3, its foot rounded
// at 0.2: the concave edges between the tube and the strip's flat sides
// and ends (the rolling ball). Each end face is held as a patch of degree
// two, which the fillet's end grows down to the tube; at the strip's
// corners the balls lie outside the solid, round a convex crease, and
// the fillets meet as a pocket's do at an inside corner (concaveAt).
// OpenCASCADE's fillet of the same solid built from exact cylinders
// adds 0.247 along one long edge and 0.515 round all four; our faces
// lie on its within 0.003 mm along the edges, 0.016 mm at the corners,
// where it rounds the corner and ours is a mitre. The tube's mesh moves
// the volume by tenths of a mm³ as the fillet cuts its face, so the
// volume is checked loosely.
func TestFilletWrappedStrip(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		what   string
		chord  float64
		all    bool
		lo, hi float64
	}{
		{"one long edge", 0.005, false, 0.2, 0.32},
		{"the whole foot", 0, true, 0.3, 1.2},
	} {
		t.Run(c.what, func(t *testing.T) {
			t.Parallel()
			d := New()
			if c.chord > 0 {
				d.Edit.SetChordTolerance(c.chord)
			}
			face := wrappedTube(d)
			d.Features.SketchAt(d.Origin(), V(0, 1, 0), V(1, 0, 0), "")
			d.Sketch.Rect(d.Sketch.UV(2.5, 10), d.Sketch.UV(3.5, 40))
			band := d.Features.Emboss(face, 0.3, "")
			if p := band.Problem(); p != "" {
				t.Fatal(p)
			}
			before := band.Analyze().Check().Volume
			foot := reliefFoot(band)
			if len(foot) != 4 {
				t.Fatalf("the strip's foot is %d edges, want 4", len(foot))
			}
			if !c.all {
				foot = []SolidEdge{band.Solid().EdgeAt(V(25, 7.3*math.Sin(3.5/7.3), -7.3*math.Cos(3.5/7.3)))}
			}
			f := d.Features.FilletSolid(foot, 0.2, "")
			if p := f.Problem(); p != "" {
				t.Fatal(p)
			}
			r := f.Analyze().Check()
			added := r.Volume - before
			if !r.Sound() || added < c.lo || added > c.hi {
				t.Errorf("%s: %s, added %g, want %g to %g", c.what, r.Summary(), added, c.lo, c.hi)
			}
		})
	}
}

// wrappedArch embosses h on face a pointed arch of a band 1 wide, as a
// Gothic window's moulding: legs 20 long along the tube from 10 going on
// smoothly into arcs of radius 2a - w and 2a, about centres a either
// side, which meet at an outer apex 20 + sqrt(3)a and an inner one.
func wrappedArch(d *Doc, face SolidFace, h float64) Group {
	d.Features.SketchAt(d.Origin(), V(0, 1, 0), V(1, 0, 0), "")
	const a, w, vs, base = 5.2, 1.0, 20.0, 10.0
	outer := vs + math.Sqrt(3)*a
	inner := vs + math.Sqrt((2*a-w)*(2*a-w)-a*a)
	uv := func(u, v float64) Vec { return d.Sketch.UV(u, v+base) }
	var first, prev Entity
	n := 0
	join := func(s, e Entity) {
		if n == 0 {
			first = s
		} else {
			d.Constrain.Coincident(prev, s)
		}
		n++
		prev = e
	}
	line := func(u1, v1, u2, v2 float64) {
		l := d.Sketch.Line(uv(u1, v1), uv(u2, v2))
		join(l.Start(), l.End())
	}
	// arc is counter-clockwise about (cu, cv) from (u1, v1) to (u2, v2),
	// walked backwards where back.
	arc := func(cu, cv, u1, v1, u2, v2 float64, back bool) {
		x := d.Sketch.Arc(uv(cu, cv), uv(u1, v1), uv(u2, v2))
		if back {
			join(x.End(), x.Start())
		} else {
			join(x.Start(), x.End())
		}
	}
	line(a, vs, a, 0)
	line(a, 0, a-w, 0)
	line(a-w, 0, a-w, vs)
	arc(-a, vs, a-w, vs, 0, inner, false)
	arc(a, vs, 0, inner, -a+w, vs, false)
	line(-a+w, vs, -a+w, 0)
	line(-a+w, 0, -a, 0)
	line(-a, 0, -a, vs)
	arc(a, vs, 0, outer, -a, vs, true)
	arc(-a, vs, a, vs, 0, outer, true)
	d.Constrain.Coincident(prev, first)
	return d.Features.Emboss(face, h, "")
}

// A pointed arch of a band 1 wide wrapped onto the tube, as a Gothic
// window's moulding, its foot rounded: straight legs going on smoothly
// into arcs, ends square at the feet, a convex crease at the outer apex
// and a concave one at the inner, where the balls lie in a wedge and the
// fillets meet in a mitre as a rim's do. Centred on the seam between the
// tube's quarters, both apexes stand on it, and each mitre's foot on the
// tube with them, where the seam is cut (splitMitreSeams); off it, the
// seam crosses the arcs.
func TestFilletWrappedArch(t *testing.T) {
	if testing.Short() {
		t.Skip("ten rolling-ball chains meeting in mitres, half a minute")
	}
	t.Parallel()
	d := New()
	band := wrappedArch(d, wrappedTube(d), 0.3)
	if p := band.Problem(); p != "" {
		t.Fatal(p)
	}
	before := band.Analyze().Check().Volume
	foot := reliefFoot(band)
	if len(foot) != 10 {
		t.Fatalf("the arch's foot is %d edges, want 10", len(foot))
	}
	f := d.Features.FilletSolid(foot, 0.2, "")
	if p := f.Problem(); p != "" {
		t.Fatal(p)
	}
	// About 110 of edge at 0.0086 a unit, and the mesh's tenths.
	r := f.Analyze().Check()
	if added := r.Volume - before; !r.Sound() || added < 0.3 || added > 4 {
		t.Errorf("%s, added %g", r.Summary(), added)
	}
}
