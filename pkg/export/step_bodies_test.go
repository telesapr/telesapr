package export

import (
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/persist/step"
	"github.com/telesapr/telesapr/pkg/srf"
)

// ballShell is the ball of radius r about c, a half disc turned about
// the line through c along Z.
func ballShell(t *testing.T, c geom.Vector, r float64, tol srf.Tolerance) *srf.SShell {
	t.Helper()
	x, z := geom.Vector{X: 1}, geom.Vector{Z: 1}
	var loop srf.SBezierLoop
	loop.Curves = append(loop.Curves,
		quarterArc(c, z.ScaledBy(-1), x, r), quarterArc(c, x, z, r),
		srf.SBezierFrom2(c.Plus(z.ScaledBy(r)), c.Minus(z.ScaledBy(r))))
	sbls := &srf.SBezierLoopSet{Outer: loop, Normal: geom.Vector{Y: 1}, Point: c}
	sh := &srf.SShell{Tol: tol}
	sh.MakeFromRevolutionOf(sbls, c, z, core.NewRGBA(100, 100, 100, 255), nil)
	var m srf.SMesh
	sh.TriangulateInto(&m)
	if v := m.CalculateVolume(); v < 0 {
		t.Fatalf("the ball faces in: volume %g", v)
	}
	return sh
}

// boxShell is the box between corners lo and hi.
func boxShell(lo, hi geom.Vector, tol srf.Tolerance) *srf.SShell {
	sh := &srf.SShell{Tol: tol}
	sh.MakePrism([]geom.Vector{{X: lo.X, Y: lo.Y}, {X: hi.X, Y: lo.Y}, {X: hi.X, Y: hi.Y}, {X: lo.X, Y: hi.Y}},
		geom.Vector{Z: lo.Z}, geom.Vector{Z: hi.Z}, core.NewRGBA(100, 100, 100, 255))
	return sh
}

// TestStepVoids: a block with a box and a ball taken away inside it is
// one BREP_WITH_VOIDS, the cavities its voids, each the closed shell of
// its faces turned out, used against its orientation; a cube standing
// in the box's cavity is a solid of its own. Read back, as a mesh and as
// a solid, the file holds the block less its cavities, and the cube.
func TestStepVoids(t *testing.T) {
	t.Parallel()
	tol := srf.Tol(0.1, 64)
	block := boxShell(geom.Vector{}, geom.Vector{X: 60, Y: 30, Z: 30}, tol)
	box := boxShell(geom.Vector{X: 5, Y: 10, Z: 10}, geom.Vector{X: 25, Y: 20, Z: 20}, tol)
	ball := ballShell(t, geom.Vector{X: 42, Y: 15, Z: 15}, 8, tol)
	cube := boxShell(geom.Vector{X: 8, Y: 13, Z: 13}, geom.Vector{X: 12, Y: 17, Z: 17}, tol)
	var once, hollow, all srf.SShell
	once.MakeFromDifferenceOf(block, box)
	hollow.MakeFromDifferenceOf(&once, ball)
	all.MakeFromAssemblyOf(&hollow, cube)
	var m srf.SMesh
	all.TriangulateInto(&m)
	want := m.CalculateVolume()
	if exact := 60*30*30 - 2000 - 4*math.Pi*512/3 + 64; math.Abs(want-exact) > 0.01*exact {
		t.Fatalf("the model's volume %g, want about %g", want, exact)
	}

	out := stepOf(t, all)
	count := func(entity string) int { return strings.Count(out, "="+entity+"(") }
	if count("BREP_WITH_VOIDS") != 1 || count("MANIFOLD_SOLID_BREP") != 1 ||
		count("ORIENTED_CLOSED_SHELL") != 2 || count("CLOSED_SHELL") != 4 {
		t.Errorf("%d BREP_WITH_VOIDS, %d MANIFOLD_SOLID_BREP, %d ORIENTED_CLOSED_SHELL, %d CLOSED_SHELL; want 1, 1, 2, 4",
			count("BREP_WITH_VOIDS"), count("MANIFOLD_SOLID_BREP"), count("ORIENTED_CLOSED_SHELL"), count("CLOSED_SHELL"))
	}
	if !strings.Contains(out, ",.F.);\n") || strings.Contains(out, "ORIENTED_CLOSED_SHELL('',*,#0") {
		t.Error("a void not used against its orientation")
	}

	res, err := step.Read([]byte(out), step.Options{ChordTolPct: 0.1})
	if err != nil {
		t.Fatal(err)
	}
	// No void counts as open, and the volume, area and centre the file
	// declares, the cavities taken away, are what was read.
	if p := res.Report.Problem(); p != "" {
		t.Errorf("mesh: %s", p)
	}
	if res.Report.Validated != 1 || len(res.Report.Mismatches) != 0 {
		t.Errorf("%d parts validated, mismatches %q", res.Report.Validated, res.Report.Mismatches)
	}
	r := check.Solid(nil, &res.Mesh)
	if r.NakedEdges != 0 || math.Abs(r.Volume-want) > 2e-3*want {
		t.Errorf("mesh: volume %.4f, want %.4f; %d naked edges", r.Volume, want, r.NakedEdges)
	}

	f, err := step.Parse([]byte(out))
	if err != nil {
		t.Fatal(err)
	}
	model := step.ReadModel(f)
	if len(model.Shapes) != 2 {
		t.Fatalf("%d shapes read, want 2", len(model.Shapes))
	}
	got := 0.0
	for i := range model.Shapes {
		sh, err := model.ShellOf(i, 0, tol)
		if err != nil {
			t.Fatalf("shape %d as a solid: %v", i, err)
		}
		got += solidVolume(t, sh)
	}
	if math.Abs(got-want) > 2e-3*want {
		t.Errorf("solid: volume %.4f, want %.4f", got, want)
	}
}
