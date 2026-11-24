package export

import (
	"bytes"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/persist/step"
	"github.com/telesapr/telesapr/pkg/persist/tlspr"
	"github.com/telesapr/telesapr/pkg/srf"
)

// An edge of a face that crosses many patches, as a swept or wrapped
// face's does, is written as one EDGE_CURVE, not one a piece: fewer
// edges than the loops have pieces, both faces along an edge using the
// same one, and the file read back is the same closed solid.
func TestStepJoinsEdgePieces(t *testing.T) {
	t.Parallel()
	for _, model := range []string{"solid/sweep-twisted-arcs.tlspr", "solid/wrap-cylinder-text-sunk.tlspr"} {
		for _, one := range []bool{false, true} {
			doc, err := tlspr.LoadFile(filepath.Join("..", "..", "testdata", filepath.FromSlash(model)))
			if err != nil {
				t.Fatal(err)
			}
			sk := doc.Sketch
			sk.GenerateAll()
			g := sk.Group(sk.ActiveGroup())
			sh := &g.RunningShell
			want := solidVolume(t, sh)

			o := options(nil)
			o.StepOneSurfacePerFace = one
			// The pieces the loops have: written one edge each, every edge
			// is two of them, one each face.
			w := &stepFileWriter{o: o, f: &bytes.Buffer{}}
			pieces := 0
			for _, job := range w.shellJobs(sh, 1) {
				for _, loops := range job.faces {
					for _, l := range loops {
						pieces += len(l.Curves)
					}
				}
			}

			var out bytes.Buffer
			if err := WriteStep(&out, sk, g, o); err != nil {
				t.Fatal(err)
			}
			text := out.String()
			edges := strings.Count(text, "=EDGE_CURVE(")
			// Written as the patches, an edge's pieces end where its
			// faces' patches meet a third.
			if edges == 0 || (one && 2*edges >= pieces) || 2*edges > pieces {
				t.Errorf("%s, one surface per face %v: %d edges of %d pieces", model, one, edges, pieces)
			}
			// Every edge is bounded by two vertices of the file.
			if strings.Contains(text, "=VERTEX_POINT('',#-1)") {
				t.Errorf("%s: a vertex of no point", model)
			}

			f, err := step.Parse(out.Bytes())
			if err != nil {
				t.Fatal(err)
			}
			m := step.ReadModel(f)
			if len(m.Shapes) != 1 {
				t.Fatalf("%s: %d shapes read, want one", model, len(m.Shapes))
			}
			read, err := m.ShellOf(0, 0, sh.Tol)
			if err != nil {
				t.Fatalf("%s: read back: %v", model, err)
			}
			if got := solidVolume(t, read); math.Abs(got-want) > 1e-3*want {
				t.Errorf("%s, one surface per face %v: volume read %.4f, ours %.4f", model, one, got, want)
			}
		}
	}
}

// solidVolume is the volume of a sound solid.
func solidVolume(t *testing.T, sh *srf.SShell) float64 {
	t.Helper()
	var m srf.SMesh
	sh.TriangulateInto(&m)
	r := check.Solid(sh, &m)
	if !r.Sound() {
		t.Errorf("unsound: %s", r.String())
	}
	return r.Volume
}

// A run of Beziers of mixed degree and weights, joined, is the same
// curve: each span of the B-spline, its knots of multiplicity the
// degree, is its piece's Bezier raised and reweighted, which passes
// through the piece's points, and the joints are shared.
func TestJoinedCurveExact(t *testing.T) {
	t.Parallel()
	line := srf.SBezier{Degree: 1}
	line.Ctrl[0], line.Ctrl[1] = geom.Vector{}, geom.Vector{X: 1}
	line.Weight[0], line.Weight[1] = 1, 1
	arc := srf.SBezier{Degree: 2}
	arc.Ctrl[0], arc.Ctrl[1], arc.Ctrl[2] = geom.Vector{X: 1}, geom.Vector{X: 2}, geom.Vector{X: 2, Y: 1}
	arc.Weight[0], arc.Weight[1], arc.Weight[2] = 1, math.Sqrt2/2, 1
	var bef, aft srf.SBezier
	arc.SplitAt(0.3, &bef, &aft) // rational pieces whose end weights are not 1
	cubic := srf.SBezier{Degree: 3}
	cubic.Ctrl[0], cubic.Ctrl[1], cubic.Ctrl[2], cubic.Ctrl[3] = aft.Finish(), geom.Vector{X: 2, Y: 2, Z: 1}, geom.Vector{X: 1, Y: 3}, geom.Vector{Y: 3}
	for i := range 4 {
		cubic.Weight[i] = 1
	}
	run := []srf.SBezier{line, bef, aft, cubic}

	deg, ctrl, weight, mults, knots := joinedCurve(run)
	if deg != 3 || len(ctrl) != 3*len(run)+1 || len(weight) != len(ctrl) {
		t.Fatalf("degree %d, %d points, %d weights", deg, len(ctrl), len(weight))
	}
	if len(knots) != len(run)+1 || mults[0] != 4 || mults[len(mults)-1] != 4 {
		t.Fatalf("knots %v, multiplicities %v", knots, mults)
	}
	for k := 1; k < len(knots); k++ {
		if knots[k] <= knots[k-1] || (k < len(knots)-1 && mults[k] != 3) {
			t.Fatalf("knots %v, multiplicities %v", knots, mults)
		}
	}
	for k := range run {
		span := srf.SBezier{Degree: deg}
		for i := 0; i <= deg; i++ {
			span.Ctrl[i], span.Weight[i] = ctrl[k*deg+i], weight[k*deg+i]
		}
		if !span.Start().EqualsTol(run[k].Start(), 1e-12) || !span.Finish().EqualsTol(run[k].Finish(), 1e-9) {
			t.Errorf("span %d runs %v to %v, its piece %v to %v", k, span.Start(), span.Finish(), run[k].Start(), run[k].Finish())
		}
		for _, s := range []float64{0.1, 0.25, 0.5, 0.75, 0.9} {
			p := span.PointAt(s)
			var d float64
			if run[k].IsRational() {
				// The arc's circle, about (1, 1).
				d = math.Abs(p.Minus(geom.Vector{X: 1, Y: 1}).Magnitude() - 1)
			} else {
				u, _ := run[k].ClosestPointTo(p, true)
				d = run[k].PointAt(u).Minus(p).Magnitude()
			}
			if d > 1e-12 {
				t.Errorf("span %d at %g is %g off its piece", k, s, d)
			}
		}
	}
}
