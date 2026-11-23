package export

import (
	"bytes"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// quarterArc is a quarter of the circle of radius r about c in the plane
// of u and v, from c+r·u to c+r·v, as the kernel holds an arc.
func quarterArc(c, u, v geom.Vector, r float64) srf.SBezier {
	a := c.Plus(u.ScaledBy(r))
	m := c.Plus(u.ScaledBy(r)).Plus(v.ScaledBy(r))
	b := c.Plus(v.ScaledBy(r))
	return srf.SBezierFromV4(geom.Vector4FromWeighted(1, a), geom.Vector4FromWeighted(math.Sqrt2/2, m),
		geom.Vector4FromWeighted(1, b))
}

// TestAnalyticSurfaces: patches the kernel builds on a plane, a
// cylinder, a cone, a sphere and a torus are found as such, with their
// sizes, and the face's sense against the surface's own normal; a patch
// on no such surface is not.
func TestAnalyticSurfaces(t *testing.T) {
	t.Parallel()
	x, y, z := geom.Vector{X: 1}, geom.Vector{Y: 1}, geom.Vector{Z: 1}
	o := geom.Vector{}
	arc := quarterArc(o, x, y, 4)
	cubic := srf.SBezierFrom4(o, geom.Vector{X: 1, Y: 3}, geom.Vector{X: 3, Y: -2}, geom.Vector{X: 5})
	cone := srf.SBezierFrom2(geom.Vector{X: 5}, geom.Vector{X: 2, Z: 6})
	ball := quarterArc(o, x, z, 7)
	tube := quarterArc(geom.Vector{X: 10}, x, z, 3)
	plane := srf.FromPlane(geom.Vector{Z: 2}, x, y)
	for _, c := range []struct {
		name         string
		s            srf.SSurface
		kind         analyticKind
		major, minor float64
	}{
		{"plane", plane, analyticPlane, 0, 0},
		{"cylinder", srf.FromExtrusionOf(&arc, o, geom.Vector{Z: 9}), analyticCylinder, 4, 0},
		{"cone", srf.FromRevolutionOf(&cone, o, z, 0, math.Pi/2, 0, 0), analyticCone, 0, 0},
		{"sphere", srf.FromRevolutionOf(&ball, o, z, 0, math.Pi/2, 0, 0), analyticSphere, 7, 0},
		{"torus", srf.FromRevolutionOf(&tube, o, z, 0, math.Pi/2, 0, 0), analyticTorus, 10, 3},
		{"extruded cubic", srf.FromExtrusionOf(&cubic, o, geom.Vector{Z: 9}), 0, 0, 0},
	} {
		a, ok := analyticOf(&c.s)
		switch {
		case c.kind == 0 && ok:
			t.Errorf("%s: found as kind %d", c.name, a.kind)
		case c.kind == 0:
		case !ok || a.kind != c.kind:
			t.Errorf("%s: found %v as kind %d, want %d", c.name, ok, a.kind, c.kind)
		case c.major != 0 && math.Abs(a.major-c.major) > 1e-9, math.Abs(a.minor-c.minor) > 1e-9:
			t.Errorf("%s: radii %g, %g, want %g, %g", c.name, a.major, a.minor, c.major, c.minor)
		default:
			// The face on the patch's own side, then turned over.
			for i := range 9 {
				p := c.s.PointAt(float64(i%3)/2, float64(i/3)/2)
				if d := math.Abs(a.offset(p)); d > 1e-9 {
					t.Errorf("%s: a point of the patch %g off the surface", c.name, d)
				}
			}
			flipped := c.s
			flipped.Reverse()
			if a.sameSense(&c.s) == a.sameSense(&flipped) {
				t.Errorf("%s: the same sense both ways round", c.name)
			}
		}
	}
	// A cone is placed where its patch's middle is, at the radius
	// there: 3.5 halfway up from 5 to 2.
	s := srf.FromRevolutionOf(&cone, o, z, 0, math.Pi/2, 0, 0)
	if a, _ := analyticOf(&s); math.Abs(a.major-3.5) > 1e-9 || math.Abs(a.angle-math.Atan2(3, 6)) > 1e-9 {
		t.Errorf("cone at radius %g, half angle %g, want 3.5, %g", a.major, a.angle, math.Atan2(3, 6))
	}
}

// stepOf writes shell as the group's solid, named and dated.
func stepOf(t *testing.T, sh srf.SShell) string {
	t.Helper()
	sk := sketch.NewSketch()
	sk.NewFile()
	sk.GenerateAll()
	g := &sk.Groups[len(sk.Groups)-1]
	g.RunningShell = sh
	o := options(nil)
	o.StepName = "two blocks"
	o.StepTime = time.Date(2026, 11, 23, 12, 30, 0, 0, time.UTC)
	var buf bytes.Buffer
	if err := WriteStep(&buf, sk, g, o); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// TestStepAP242: a STEP file says it is AP242 edition 4, and gives its
// name and time; a block's faces are planes and its edges lines; two
// blocks apart are two solids, each of its own shell; and the part's
// volume, area and centre are written as validation properties.
func TestStepAP242(t *testing.T) {
	t.Parallel()
	grey := core.NewRGBA(100, 100, 100, 255)
	var a, b, both srf.SShell
	a.Tol, b.Tol = srf.Tol(0.1, 64), srf.Tol(0.1, 64)
	a.MakePrism([]geom.Vector{{}, {X: 10}, {X: 10, Y: 10}, {Y: 10}}, geom.Vector{}, geom.Vector{Z: 5}, grey)
	b.MakePrism([]geom.Vector{{X: 20}, {X: 30}, {X: 30, Y: 10}, {X: 20, Y: 10}}, geom.Vector{}, geom.Vector{Z: 5}, grey)
	both.MakeFromAssemblyOf(&a, &b)
	out := stepOf(t, both)
	count := func(entity string) int { return strings.Count(out, "="+entity+"(") }
	for _, want := range []string{
		"'AP242_MANAGED_MODEL_BASED_3D_ENGINEERING_MIM_LF { 1 0 10303 442 7 1 4 }'",
		"'two blocks.step'", "'2026-11-23T12:30:00+00:00'", "PRODUCT('two blocks','two blocks'",
		"APPLICATION_PROTOCOL_DEFINITION('international standard','ap242_managed_model_based_3d_engineering',2025",
		"PRODUCT_RELATED_PRODUCT_CATEGORY('part',$,(#179))",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s", want)
		}
	}
	if n := count("PLANE"); n != 12 {
		t.Errorf("%d planes, want 12", n)
	}
	if n := count("LINE"); n != 24 {
		t.Errorf("%d lines, want 24", n)
	}
	if n := strings.Count(out, "B_SPLINE"); n != 0 {
		t.Errorf("%d B-splines, want none", n)
	}
	if count("CLOSED_SHELL") != 2 || count("MANIFOLD_SOLID_BREP") != 2 {
		t.Errorf("%d shells and %d solids, want 2 of each", count("CLOSED_SHELL"), count("MANIFOLD_SOLID_BREP"))
	}
	measure := func(name string) float64 {
		m := regexp.MustCompile(name + `\(([-0-9.]+)\)`).FindStringSubmatch(out)
		if m == nil {
			t.Fatalf("no %s", name)
		}
		v, _ := strconv.ParseFloat(m[1], 64)
		return v
	}
	if v := measure("VOLUME_MEASURE"); math.Abs(v-1000) > 1e-6 {
		t.Errorf("volume %g, want 1000", v)
	}
	if v := measure("AREA_MEASURE"); math.Abs(v-800) > 1e-6 {
		t.Errorf("area %g, want 800", v)
	}
	if !strings.Contains(out, "CARTESIAN_POINT('centre point',(15.,5.,2.5))") {
		t.Error("no centre point at (15, 5, 2.5)")
	}
}

// TestStepCircleEdges: the arcs of a cylinder's rims are written as
// circles, about the axis and of its radius.
func TestStepCircleEdges(t *testing.T) {
	t.Parallel()
	x, y := geom.Vector{X: 1}, geom.Vector{Y: 1}
	var w stepFileWriter
	w.f = &bytes.Buffer{}
	w.id = 200
	arc := quarterArc(geom.Vector{Z: 3}, x, y, 4)
	id := w.ExportCurve(&arc)
	out := w.f.String()
	if !strings.Contains(out, "#"+strconv.Itoa(id)+"=CIRCLE('',") || !strings.Contains(out, ",4.);") {
		t.Errorf("arc not written as a circle of radius 4:\n%s", out)
	}
	if !strings.Contains(out, "DIRECTION('',(0.,0.,1.))") {
		t.Errorf("circle's axis not +z, the way the arc turns:\n%s", out)
	}
	// A wireframe's curves stay B-splines: a CIRCLE is a whole circle.
	free := stepFileWriter{f: &bytes.Buffer{}, id: 200, freeCurves: true}
	free.ExportCurve(&arc)
	if s := free.f.String(); strings.Contains(s, "CIRCLE") || !strings.Contains(s, "B_SPLINE_CURVE") {
		t.Errorf("wireframe arc:\n%s", s)
	}
}
