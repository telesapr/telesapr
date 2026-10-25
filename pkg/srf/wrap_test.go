package srf

import (
	"errors"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// roundFaceIn is the cylinder or cone among sh's surfaces.
func roundFaceIn(t *testing.T, sh *SShell) RoundFace {
	t.Helper()
	for i := range sh.Surface {
		if f, ok := RoundFaceOf(&sh.Surface[i]); ok {
			return f
		}
	}
	t.Fatal("no cylinder or cone in the shell")
	return RoundFace{}
}

func TestRoundFaceOf(t *testing.T) {
	t.Parallel()
	cyl := roundFaceIn(t, lathedTube(0, 5, 0, 20, Tolerance{}))
	if cyl.IsCone() || math.Abs(cyl.Radius-5) > 1e-9 || math.Abs(math.Abs(cyl.Axis.Z)-1) > 1e-9 ||
		math.Hypot(cyl.Point.X, cyl.Point.Y) > 1e-9 || cyl.Outward != 1 {
		t.Fatalf("lathed cylinder: %+v", cyl)
	}
	// A tube's bore faces the axis.
	sh := lathedTube(2, 5, 0, 20, Tolerance{})
	bores := 0
	for i := range sh.Surface {
		if f, ok := RoundFaceOf(&sh.Surface[i]); ok && f.Outward == -1 {
			bores++
			if math.Abs(f.Radius-2) > 1e-9 {
				t.Fatalf("bore: %+v", f)
			}
		}
	}
	if bores == 0 {
		t.Fatal("no bore found in a tube")
	}
	cone := roundFaceIn(t, lathedCone(4, 8, Tolerance{}))
	if !cone.IsCone() || math.Abs(cone.HalfAngle-math.Atan(0.5)) > 1e-9 ||
		cone.Point.Minus(geom.Vector{Z: 8}).Magnitude() > 1e-9 || math.Abs(cone.Axis.Z+1) > 1e-9 {
		t.Fatalf("cone: %+v", cone)
	}
	box := extrudedBox(geom.Vector{}, geom.Vector{X: 1, Y: 1, Z: 1}, Tolerance{})
	for i := range box.Surface {
		if _, ok := RoundFaceOf(&box.Surface[i]); ok {
			t.Fatal("a box face taken for a cylinder")
		}
	}
	// An extruded circle's quarters.
	var circle SBezierLoopSet
	circle.Normal = geom.Vector{Z: 1}
	circle.Outer.Curves = circleArcs(geom.Vector{X: 1, Y: 2}, geom.Vector{Z: 1}, 3)
	var ext SShell
	ext.MakeFromExtrusionOf(&circle, geom.Vector{}, geom.Vector{Z: 4}, core.RGBA{})
	f := roundFaceIn(t, &ext)
	if math.Abs(f.Radius-3) > 1e-9 || f.Point.Minus(geom.Vector{X: 1, Y: 2, Z: f.Point.Z}).Magnitude() > 1e-9 {
		t.Fatalf("extruded circle: %+v", f)
	}
}

// circleArcs is the circle of radius r about c square to n, as four
// rational quadratic quarters.
func circleArcs(c, n geom.Vector, r float64) []SBezier {
	u := n.NormalDir(0).WithMagnitude(r)
	v := n.Cross(u).WithMagnitude(r)
	w := math.Sqrt(0.5)
	pts := []geom.Vector{u, v, u.ScaledBy(-1), v.ScaledBy(-1)}
	out := make([]SBezier, 0, 4)
	for i := range 4 {
		a, b := pts[i], pts[(i+1)%4]
		arc := SBezierFrom3(c.Plus(a), c.Plus(a).Plus(b), c.Plus(b))
		arc.Weight[1] = w
		out = append(out, arc)
	}
	return out
}

// sketchLoop is the closed polygon or curve set cs in a sketch plane of
// normal n, turned to run as the sketch's loops do: clockwise seen
// along n.
func sketchLoop(cs []SBezier, n geom.Vector) *SBezierLoopSet {
	sbls := &SBezierLoopSet{Normal: n, Point: cs[0].Start()}
	// The loop's area vector by the shoelace sum over its chords.
	var area geom.Vector
	for _, b := range cs {
		for k := range 8 {
			p, q := b.PointAt(float64(k)/8), b.PointAt(float64(k+1)/8)
			area = area.Plus(p.Cross(q))
		}
	}
	if area.Dot(n) > 0 {
		rev := make([]SBezier, len(cs))
		for i, b := range cs {
			b.Reverse()
			rev[len(cs)-1-i] = b
		}
		cs = rev
	}
	sbls.Outer.Curves = cs
	return sbls
}

// rectIn is the rectangle x in [x0, x1], z in [z0, z1] in the plane y.
func rectIn(y, x0, x1, z0, z1 float64) []SBezier {
	p := []geom.Vector{{X: x0, Y: y, Z: z0}, {X: x1, Y: y, Z: z0}, {X: x1, Y: y, Z: z1}, {X: x0, Y: y, Z: z1}}
	out := make([]SBezier, 4)
	for i := range p {
		out[i] = SBezierFrom2(p[i], p[(i+1)%4])
	}
	return out
}

// wrapBand is sbls wrapped onto face from h0 to h1, in a shell of its
// own, built at tol.
func wrapBand(t *testing.T, face RoundFace, planePt, planeN geom.Vector, sbls *SBezierLoopSet, h0, h1 float64, tol Tolerance) (*SShell, *Wrap) {
	t.Helper()
	w, err := NewWrap(face, planePt, planeN, []SBezierLoopSet{*sbls})
	if err != nil {
		t.Fatal(err)
	}
	sh := SShell{Tol: tol}
	if err := sh.MakeFromWrapOf(sbls, w, h0, h1, core.RGBA{}); err != nil {
		t.Fatal(err)
	}
	return &sh, w
}

// The band of a profile of area a wrapped onto a cylinder of radius r
// between h0 and h1 has volume a ((r+h1)² - (r+h0)²) / 2r: the wrap
// keeps areas, and the band's sections scale with the radius.
func cylinderBand(a, r, h0, h1 float64) float64 {
	return a * ((r+h1)*(r+h1) - (r+h0)*(r+h0)) / (2 * r)
}

// fineMeshTol meshes finely enough, chords and segments both, that the
// difference a band makes to a body's volume is measured to 1e-3 of it.
func fineMeshTol() Tolerance { return Tol(1e-4, 2000) }

// checkAdded compares what a Boolean added to body's volume (taken
// away, if negative) against want, within rel of it.
func checkAdded(t *testing.T, name string, body, out *SShell, want, rel float64) {
	t.Helper()
	got := shellVolume(out) - shellVolume(body)
	if math.Abs(got-want) > rel*math.Abs(want) {
		t.Errorf("%s: added %v, want %v (rel err %.2g)", name, got, want, (got-want)/want)
	}
}

func TestWrapRectangleOnCylinder(t *testing.T) {
	t.Parallel()
	tol := fineMeshTol()
	body := lathedTube(0, 5, 0, 20, tol)
	face := roundFaceIn(t, body)
	for _, n := range []geom.Vector{{Y: -1}, {Y: 1}} {
		sbls := sketchLoop(rectIn(-20, -3, 3, 5, 15), n)
		band, w := wrapBand(t, face, geom.Vector{Y: -20}, n, sbls, -0.25, 1, tol)
		if !meshIsSound(band) {
			t.Fatalf("normal %v: the band is not sound", n)
		}
		checkVolume(t, "band", shellVolume(band), cylinderBand(60, 5, -0.25, 1))
		// The lines across the axis are exact arcs, those along it lines.
		for _, l := range sbls.loops() {
			for _, b := range l.Curves {
				pieces, err := w.MapCurve(&b)
				if err != nil || len(pieces) != 1 || pieces[0].Degree > 2 {
					t.Fatalf("an edge of the rectangle wrapped into %d pieces (%v)", len(pieces), err)
				}
			}
		}
		// The facing line is where the sketch's x = 0 is, on the -y side.
		if p := w.onFace(geom.Vector{Y: -20, Z: 7}); p.Minus(geom.Vector{Y: -5, Z: 7}).Magnitude() > 1e-12 {
			t.Fatalf("the sketch's origin line went to %v", p)
		}
		out := SShell{Tol: tol}
		out.MakeFromUnionOf(body, band)
		if out.BooleanFailed || !meshIsSound(&out) {
			t.Fatalf("normal %v: the union is not sound", n)
		}
		checkAdded(t, "embossed", body, &out, cylinderBand(60, 5, 0, 1), 1e-3)
	}
}

func TestWrapCircleEngravedInCylinder(t *testing.T) {
	t.Parallel()
	tol := fineMeshTol()
	body := lathedTube(0, 5, 0, 20, tol)
	face := roundFaceIn(t, body)
	n := geom.Vector{Y: -1}
	sbls := sketchLoop(circleArcs(geom.Vector{X: 1, Y: -20, Z: 10}, n, 2.5), n)
	band, w := wrapBand(t, face, geom.Vector{Y: -20}, n, sbls, -1, 0.25, tol)
	if !meshIsSound(band) {
		t.Fatal("the band is not sound")
	}
	// Fitted pieces lie on the face.
	for _, l := range sbls.loops() {
		for _, b := range l.Curves {
			pieces, err := w.MapCurve(&b)
			if err != nil {
				t.Fatal(err)
			}
			for _, p := range pieces {
				for k := range 41 {
					if d := face.offset(p.PointAt(float64(k) / 40)); math.Abs(d) > wrapFitTol {
						t.Fatalf("a wrapped point is %g off the face", d)
					}
				}
			}
		}
	}
	a := math.Pi * 2.5 * 2.5
	checkVolume(t, "band", shellVolume(band), cylinderBand(a, 5, -1, 0.25))
	out := SShell{Tol: tol}
	out.MakeFromDifferenceOf(body, band)
	if out.BooleanFailed || !meshIsSound(&out) {
		t.Fatal("the engraving is not sound")
	}
	checkAdded(t, "engraved", body, &out, -cylinderBand(a, 5, -1, 0), 1e-3)
}

// coneBand is the volume of the band a rectangle of the sketch, s0 to
// s1 across and z0 to z1 along the axis from the apex, makes on w's cone
// between the heights h0 and h1 along its normal. A sheet of the band
// at height h is the face's area grown by 1 + h/R, R = rho tan(alpha)
// the face's distance from the axis along its normal, so the volume is
// A (h1-h0) + (h1²-h0²)/2 times the integral of 1/R over the face, here
// by the midpoint rule over the development.
func coneBand(w *Wrap, s0, s1, z0, z1, h0, h1 float64) float64 {
	const n = 400
	ds, dz := (s1-s0)/n, (z1-z0)/n
	tan := w.sinA / w.cosA
	sum := 0.0
	for i := range n {
		for j := range n {
			s := s0 + (float64(i)+0.5)*ds
			z := z0 + (float64(j)+0.5)*dz
			rho := math.Hypot(s, w.rhoC+z-w.rhoC*w.cosA)
			sum += 1 / (rho * tan)
		}
	}
	return (s1-s0)*(z1-z0)*(h1-h0) + (h1*h1-h0*h0)/2*sum*ds*dz
}

func TestWrapRectangleOnCone(t *testing.T) {
	t.Parallel()
	tol := fineMeshTol()
	// Apex up at z = 16, base radius 8 on z = 0.
	body := lathedCone(8, 16, tol)
	face := roundFaceIn(t, body)
	n := geom.Vector{Y: -1}
	sbls := sketchLoop(rectIn(-20, -2, 2, 3, 7), n)
	band, w := wrapBand(t, face, geom.Vector{Y: -20}, n, sbls, -0.25, 1, tol)
	if !meshIsSound(band) {
		t.Fatal("the band is not sound")
	}
	// From the apex the rectangle runs 9 to 13 along the axis. The band's
	// mesh comes to its volume slowly: 7e-3 short at a chord of 1e-4,
	// 3e-4 at 3e-5, where meshing it takes 20 s.
	want := coneBand(w, -2, 2, 9, 13, -0.25, 1)
	if v := shellVolume(band); math.Abs(v-want) > 1e-2*want {
		t.Fatalf("band: volume %g, want %g", v, want)
	}
	if testing.Short() {
		t.Skip("the union with the cone, meshed finely, takes 15 s")
	}
	out := SShell{Tol: tol}
	out.MakeFromUnionOf(body, band)
	if out.BooleanFailed || !meshIsSound(&out) {
		t.Fatal("the union is not sound")
	}
	checkAdded(t, "embossed", body, &out, coneBand(w, -2, 2, 9, 13, 0, 1), 1e-2)
}

// A loop round 275° of a cylinder has caps of three patches, and of a
// cone too; the seams between them cross it, and the hole in it.
func TestWrapRoundMostOfTheWay(t *testing.T) {
	t.Parallel()
	tol := fineMeshTol()
	n := geom.Vector{Y: -1}
	body := lathedTube(0, 5, 0, 20, tol)
	face := roundFaceIn(t, body)
	sbls := sketchLoop(rectIn(-20, -12, 12, 5, 15), n)
	hole := sketchLoop(rectIn(-20, -10, 10, 8, 12), n)
	hole.Outer.Reverse()
	sbls.Holes = []SBezierLoop{hole.Outer}
	band, w := wrapBand(t, face, geom.Vector{Y: -20}, n, sbls, -0.25, 1, tol)
	caps, seams, err := w.capPatches(sbls, -0.25, 1)
	if err != nil || len(caps) != 3 || len(seams) != 2 {
		t.Fatalf("%d caps, %d seams, %v", len(caps), len(seams), err)
	}
	if !meshIsSound(band) {
		t.Fatal("the band is not sound")
	}
	checkVolume(t, "band", shellVolume(band), cylinderBand(240-80, 5, -0.25, 1))
	out := SShell{Tol: tol}
	out.MakeFromUnionOf(body, band)
	if out.BooleanFailed || !meshIsSound(&out) {
		t.Fatal("the union is not sound")
	}
	checkAdded(t, "embossed", body, &out, cylinderBand(160, 5, 0, 1), 1e-3)

	cone := lathedCone(8, 16, tol)
	cface := roundFaceIn(t, cone)
	csbls := sketchLoop(rectIn(-20, -12, 12, 3, 7), n)
	cband, cw := wrapBand(t, cface, geom.Vector{Y: -20}, n, csbls, -0.25, 1, tol)
	if _, seams, _ := cw.capPatches(csbls, -0.25, 1); len(seams) == 0 {
		t.Fatal("the cone's band has caps of one patch")
	}
	if !meshIsSound(cband) {
		t.Fatal("the cone's band is not sound")
	}
	if v, want := shellVolume(cband), coneBand(cw, -12, 12, 9, 13, -0.25, 1); math.Abs(v-want) > 1e-2*want {
		t.Fatalf("the cone's band: volume %g, want %g", v, want)
	}
	if testing.Short() {
		t.Skip("the union with the cone, meshed finely, takes 15 s")
	}
	cout := SShell{Tol: tol}
	cout.MakeFromUnionOf(cone, cband)
	if cout.BooleanFailed || !meshIsSound(&cout) {
		t.Fatal("the union with the cone is not sound")
	}
	checkAdded(t, "embossed on the cone", cone, &cout, coneBand(cw, -12, 12, 9, 13, 0, 1), 1e-2)
}

func TestWrapRefusals(t *testing.T) {
	t.Parallel()
	face := roundFaceIn(t, lathedTube(0, 5, 0, 20, Tolerance{}))
	sbls := sketchLoop(rectIn(-20, -3, 3, 5, 15), geom.Vector{Y: -1})
	if _, err := NewWrap(face, geom.Vector{}, geom.Vector{Y: -1, Z: 1}.WithMagnitude(1), nil); !errors.Is(err, ErrWrapPlane) {
		t.Fatalf("a tilted plane: %v", err)
	}
	w, err := NewWrap(face, geom.Vector{Y: -20}, geom.Vector{Y: -1}, []SBezierLoopSet{*sbls})
	if err != nil {
		t.Fatal(err)
	}
	var sh SShell
	if err := sh.MakeFromWrapOf(sbls, w, -6, 0, core.RGBA{}); err == nil {
		t.Fatal("an engraving through the axis was taken")
	}
	long := sketchLoop(rectIn(-20, -16, 16, 5, 15), geom.Vector{Y: -1})
	if err := sh.MakeFromWrapOf(long, w, 0, 1, core.RGBA{}); err == nil {
		t.Fatal("a loop round 367° was taken")
	}
	cone := roundFaceIn(t, lathedCone(8, 16, Tolerance{}))
	high := sketchLoop(rectIn(-20, -2, 2, 12, 17), geom.Vector{Y: -1})
	wc, err := NewWrap(cone, geom.Vector{Y: -20}, geom.Vector{Y: -1}, []SBezierLoopSet{*high})
	if err != nil {
		t.Fatal(err)
	}
	if err := sh.MakeFromWrapOf(high, wc, 0, 1, core.RGBA{}); err == nil {
		t.Fatal("a sketch past the apex was taken")
	}
}
