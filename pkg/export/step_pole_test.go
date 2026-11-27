package export

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/persist/step"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// coneShell is the cone of base radius r on z = 0 and apex at height h
// on the z axis, a right triangle turned about it.
func coneShell(r, h float64, tol srf.Tolerance) *srf.SShell {
	o, apex, rim := geom.Vector{}, geom.Vector{Z: h}, geom.Vector{X: r}
	var loop srf.SBezierLoop
	loop.Curves = append(loop.Curves, srf.SBezierFrom2(o, rim), srf.SBezierFrom2(rim, apex), srf.SBezierFrom2(apex, o))
	sbls := &srf.SBezierLoopSet{Outer: loop, Normal: geom.Vector{Y: 1}, Point: o}
	sh := &srf.SShell{Tol: tol}
	sh.MakeFromRevolutionOf(sbls, o, geom.Vector{Z: 1}, core.NewRGBA(100, 100, 100, 255), nil)
	return sh
}

// halfPlaneBlock is the block from z0 to z1 on the side of the plane
// through the z axis at deg degrees from x that lies counter-clockwise
// of it.
func halfPlaneBlock(deg, z0, z1 float64, tol srf.Tolerance) *srf.SShell {
	s, c := math.Sincos(deg * math.Pi / 180)
	d, n := geom.Vector{X: 20 * c, Y: 20 * s}, geom.Vector{X: -20 * s, Y: 20 * c}
	sh := &srf.SShell{Tol: tol}
	sh.MakePrism([]geom.Vector{d.ScaledBy(-1), d, d.Plus(n), d.ScaledBy(-1).Plus(n)},
		geom.Vector{Z: z0}, geom.Vector{Z: z1}, core.NewRGBA(100, 100, 100, 255))
	return sh
}

// stepEdgeUses are, by edge of the file's one shape, the times a bound
// runs along it forwards less the times it runs back: none in a shell
// whose faces all face out.
func stepEdgeUses(t *testing.T, data []byte) map[*step.Edge]int {
	t.Helper()
	f, err := step.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	m := step.ReadModel(f)
	if len(m.Shapes) != 1 {
		t.Fatalf("%d shapes read, want one", len(m.Shapes))
	}
	uses := map[*step.Edge]int{}
	for _, fc := range m.Shapes[0].Faces {
		for _, b := range fc.Bounds {
			for _, oe := range b.Edges {
				if oe.Forward {
					uses[oe.E]++
				} else {
					uses[oe.E]--
				}
			}
		}
	}
	return uses
}

// TestStepSheetReachingPole writes solids of a lathe's sphere or cone
// cut so that a face of several patches reaches the pole or apex or
// holds both poles, one surface per face: each such face as one
// SPHERICAL_SURFACE or CONICAL_SURFACE, no B-spline. Every edge is used
// once each way round, and the file reads back as one closed solid of
// the volume read from the file written a patch at a time, and within
// the chords' sag of the volume in closed form. Until 2026-11-27 a face
// of several patches ran its bounds the wrong way round, the edges it
// shares with its neighbours the same way as they do (its trims were
// turned for the file, and turned again by the bound), which our reader
// set right from the bounds' layout but for bounds through a pole,
// where it can't tell: such a face was written as a B-spline
// (reachesPole). A face of a sphere holding a pole its bounds don't
// reach was read as the bounds' outline alone (a ball with half of its
// cap cut off read back 448 of 3736 mm³), and one bounded by holes as
// the holes.
func TestStepSheetReachingPole(t *testing.T) {
	t.Parallel()
	tol := srf.Tol(0.05, 64)
	// The ball of radius 10 less half its cap above z = 4, the cone of
	// radius 10 and height 12 less half of its tip above z = 4, and the
	// ball with a square hole through it, off its axis.
	capCut := math.Pi * 36 * (30 - 6) / 3 / 2
	tipCut := math.Pi * (10 * 8 / 12.0) * (10 * 8 / 12.0) * 8 / 3 / 2
	for _, c := range []struct {
		name, surface string
		solid, cut    func() *srf.SShell
		exact         float64
	}{
		{"half cap", "SPHERICAL_SURFACE",
			func() *srf.SShell { return ballShell(t, geom.Vector{}, 10, tol) },
			func() *srf.SShell { return halfPlaneBlock(200, 4, 20, tol) },
			4000*math.Pi/3 - capCut},
		{"half tip", "CONICAL_SURFACE",
			func() *srf.SShell { return coneShell(10, 12, tol) },
			func() *srf.SShell { return halfPlaneBlock(200, 4, 20, tol) },
			400*math.Pi - tipCut},
		{"holed ball", "SPHERICAL_SURFACE",
			func() *srf.SShell { return ballShell(t, geom.Vector{}, 10, tol) },
			func() *srf.SShell {
				sh := &srf.SShell{Tol: tol}
				sh.MakePrism([]geom.Vector{{X: 3, Y: 1}, {X: 7, Y: 1}, {X: 7, Y: 4}, {X: 3, Y: 4}},
					geom.Vector{Z: -20}, geom.Vector{Z: 20}, core.NewRGBA(100, 100, 100, 255))
				return sh
			}, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			// The turned patches one sheet, as a lathe's are.
			solid := c.solid()
			for i := range solid.Surface {
				solid.Surface[i].Sheet = 1
			}
			cut := srf.SShell{Tol: tol}
			cut.MakeFromDifferenceOf(solid, c.cut())
			solidVolume(t, &cut)
			sk := sketch.NewSketch()
			sk.NewFile()
			sk.GenerateAll()
			g := &sk.Groups[len(sk.Groups)-1]
			g.RunningShell = cut
			read := func(one bool) (string, float64) {
				var warnings []string
				o := options(&warnings)
				o.StepOneSurfacePerFace = one
				var buf bytes.Buffer
				if err := WriteStep(&buf, sk, g, o); err != nil {
					t.Fatal(err)
				}
				if len(warnings) > 0 {
					t.Errorf("one surface per face %v: warned %q", one, warnings)
				}
				for e, n := range stepEdgeUses(t, buf.Bytes()) {
					if n != 0 {
						t.Errorf("one surface per face %v: edge #%d run %+d times more forwards than back", one, e.ID, n)
					}
				}
				f, err := step.Parse(buf.Bytes())
				if err != nil {
					t.Fatal(err)
				}
				sh, err := step.ReadModel(f).ShellOf(0, 0, tol)
				if err != nil {
					t.Fatalf("one surface per face %v: read back: %v", one, err)
				}
				return buf.String(), solidVolume(t, sh)
			}
			out, got := read(true)
			_, want := read(false)
			if n := strings.Count(out, "="+c.surface+"("); n == 0 {
				t.Errorf("no %s", c.surface)
			}
			if strings.Contains(out, "B_SPLINE_SURFACE") {
				t.Error("a face written as a B-spline")
			}
			// The two files are meshed apart, each within the chord
			// tolerance, and on arm64, which fuses multiply-adds, not as
			// on amd64: the half tip's two volumes are 0.2% apart there.
			if math.Abs(got-want) > 5e-3*want {
				t.Errorf("volume read %.4f, written a patch at a time %.4f", got, want)
			}
			if c.exact != 0 && math.Abs(got-c.exact) > 1e-2*c.exact {
				t.Errorf("volume read %.4f, in closed form %.4f", got, c.exact)
			}
		})
	}
}
