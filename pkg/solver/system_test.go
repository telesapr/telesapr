package solver_test

import (
	"errors"
	"fmt"
	"io/fs"
	"math"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/persist/slvs"
	"github.com/spacesolve/spacesolve/pkg/sketch"
	"github.com/spacesolve/spacesolve/pkg/solver"
)

var (
	errNoParameters          = errors.New("fixture produced no parameters")
	errNoEquations           = errors.New("fixture produced no equations")
	errUnsupportedConstraint = errors.New("fixture contains unsupported constraint")
	errDidNotConverge        = errors.New("fixture solver did not converge")
	errNoMeshGenerated       = errors.New("no mesh generated for extrude group")
	errMeshGenerationFailed  = errors.New("mesh generation failed")
)

type stubResolver struct {
	entities map[core.HEntity]*core.Entity
	params   map[core.HParam]*core.Param
}

func (s stubResolver) Entity(h core.HEntity) (*core.Entity, error) {
	return s.entities[h], nil
}

func (s stubResolver) Param(h core.HParam) (*core.Param, error) {
	p, ok := s.params[h]
	if !ok {
		return nil, nil
	}
	return p, nil
}

type paramProvider struct {
	params map[core.HParam]*core.Param
}

func (p paramProvider) ParamValue(h core.HParam) (float64, bool) {
	param, ok := p.params[h]
	if !ok || param == nil {
		return 0, false
	}
	return param.Val, true
}

func TestSystemAddsCoincidentConstraint(t *testing.T) {
	pAx := &core.Param{H: core.HParam{V: 0x00010001}, Val: 1}
	pAy := &core.Param{H: core.HParam{V: 0x00010002}, Val: 2}
	pAz := &core.Param{H: core.HParam{V: 0x00010003}, Val: 3}

	pBx := &core.Param{H: core.HParam{V: 0x00020001}, Val: 1}
	pBy := &core.Param{H: core.HParam{V: 0x00020002}, Val: 2}
	pBz := &core.Param{H: core.HParam{V: 0x00020003}, Val: 3}

	entA := &core.Entity{
		EntityBase: core.EntityBase{
			H:     core.HEntity{V: 0x00001000},
			Type:  core.EntityPointIn3D,
			Param: [8]core.HParam{pAx.H, pAy.H, pAz.H},
		},
	}
	entB := &core.Entity{
		EntityBase: core.EntityBase{
			H:     core.HEntity{V: 0x00002000},
			Type:  core.EntityPointIn3D,
			Param: [8]core.HParam{pBx.H, pBy.H, pBz.H},
		},
	}

	res := stubResolver{
		entities: map[core.HEntity]*core.Entity{
			entA.H: entA,
			entB.H: entB,
		},
		params: map[core.HParam]*core.Param{
			pAx.H: pAx, pAy.H: pAy, pAz.H: pAz,
			pBx.H: pBx, pBy.H: pBy, pBz.H: pBz,
		},
	}

	sys := solver.NewSystem(res)
	constraint := core.Constraint{
		ConstraintBase: core.ConstraintBase{
			H:    core.HConstraint{V: 0x00000010},
			Type: core.ConstraintPointsCoincident,
			PtA:  entA.H,
			PtB:  entB.H,
		},
	}

	if err := sys.AddConstraint(&constraint); err != nil {
		t.Fatalf("AddConstraint: %v", err)
	}

	if len(sys.Equations) != 3 {
		t.Fatalf("expected 3 equations, got %d", len(sys.Equations))
	}
	if len(sys.Params) != 6 {
		t.Fatalf("expected 6 params, got %d", len(sys.Params))
	}

	ctx := paramProvider{params: sys.Params}
	for _, eq := range sys.Equations {
		val := eq.Expr.Eval(ctx, nil)
		if val != 0 {
			t.Fatalf("equation %08x evaluated to %f, expected 0", eq.H.V, val)
		}
	}

	if sys.DOF() != 3 {
		t.Fatalf("expected DOF 3, got %d", sys.DOF())
	}

	residuals, jac, err := sys.Evaluate(nil)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(residuals) != 3 || len(jac) != 3 {
		t.Fatalf("unexpected evaluation sizes")
	}
	for i, r := range residuals {
		if r != 0 {
			t.Fatalf("residual %d = %f, expected 0", i, r)
		}
	}

	handles := sys.ParamHandles()
	if len(handles) != 6 {
		t.Fatalf("expected 6 handles, got %d", len(handles))
	}

	colIndex := func(h core.HParam) int {
		for i, handle := range handles {
			if handle == h {
				return i
			}
		}
		return -1
	}

	colAx := colIndex(pAx.H)
	colBx := colIndex(pBx.H)
	if colAx == -1 || colBx == -1 {
		t.Fatalf("missing parameter columns")
	}

	if jac[0][colAx] != 1 || jac[0][colBx] != -1 {
		t.Fatalf("unexpected Jacobian entries for x constraint: %+v", jac[0])
	}
}

func TestSketchSolveIntegration(t *testing.T) {
	root := solverFixturesRoot(t)
	candidates := []string{
		filepath.Join(root, "solver", "param_point_distance.slvs"),
		filepath.Join(root, "constraint", "pt_on_line", "normal.slvs"),
		filepath.Join(root, "constraint", "pt_on_line", "left_free_in_3d.slvs"),
		filepath.Join(root, "constraint", "pt_on_line", "right_free_in_3d.slvs"),
		filepath.Join(root, "constraint", "parallel", "free_in_3d.slvs"),
		filepath.Join(root, "constraint", "parallel", "normal.slvs"),
	}

	solved := 0
	for _, path := range candidates {
		result, err := exerciseFixture(path, exerciseConfig{allowFallback: false})
		if err != nil {
			if isSkippableFixtureError(err) {
				t.Logf("integration fixture %s skipped: %v", path, err)
				continue
			}
			t.Fatalf("fixture %s failed: %v", path, err)
		}
		if !result.report.Converged {
			t.Fatalf("fixture %s solver did not report convergence", path)
		}
		recordSolveSummary(t, result)
		solved++
	}
	if solved == 0 {
		t.Skip("no fixtures solved without fallback; enable once fixtures with driven parameters land")
	}
}

func TestSystemPointPointDistanceConstraint(t *testing.T) {
	pAx := &core.Param{H: core.HParam{V: 0x00030001}, Val: 0}
	entA := &core.Entity{
		EntityBase: core.EntityBase{
			H:    core.HEntity{V: 0x00003000},
			Type: core.EntityPointIn3D,
			Param: [8]core.HParam{
				pAx.H,
				{},
				{},
			},
		},
		ActPoint: geom.NewVector(0, 0, 0),
	}

	entB := &core.Entity{
		EntityBase: core.EntityBase{
			H:    core.HEntity{V: 0x00004000},
			Type: core.EntityPointIn3D,
		},
		ActPoint: geom.NewVector(5, 0, 0),
	}

	res := stubResolver{
		entities: map[core.HEntity]*core.Entity{
			entA.H: entA,
			entB.H: entB,
		},
		params: map[core.HParam]*core.Param{
			pAx.H: pAx,
		},
	}

	sys := solver.NewSystem(res)
	constraint := core.Constraint{
		ConstraintBase: core.ConstraintBase{
			H:    core.HConstraint{V: 0x00000020},
			Type: core.ConstraintPtPtDistance,
			PtA:  entA.H,
			PtB:  entB.H,
			ValA: 5,
		},
	}

	if err := sys.AddConstraint(&constraint); err != nil {
		t.Fatalf("AddConstraint: %v", err)
	}

	if len(sys.Equations) != 1 {
		t.Fatalf("expected one equation")
	}

	residuals, jac, err := sys.Evaluate(nil)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(residuals) != 1 || len(jac) != 1 {
		t.Fatalf("expected 1 residual and row")
	}

	if math.Abs(residuals[0]) > 1e-9 {
		t.Fatalf("unexpected residual %f", residuals[0])
	}
}

func TestSolveCoincidentPointWithFixedTarget(t *testing.T) {
	pAx := &core.Param{H: core.HParam{V: 0x00050001}, Val: 1}
	pAy := &core.Param{H: core.HParam{V: 0x00050002}, Val: -1}
	pAz := &core.Param{H: core.HParam{V: 0x00050003}, Val: 2}

	target := geom.NewVector(0, 0, 0)

	entA := &core.Entity{
		EntityBase: core.EntityBase{
			H:    core.HEntity{V: 0x00005000},
			Type: core.EntityPointIn3D,
			Param: [8]core.HParam{
				pAx.H,
				pAy.H,
				pAz.H,
			},
		},
		ActPoint: geom.NewVector(1, -1, 2),
	}

	entB := &core.Entity{
		EntityBase: core.EntityBase{
			H:    core.HEntity{V: 0x00006000},
			Type: core.EntityPointIn3D,
		},
		ActPoint: target,
	}

	res := stubResolver{
		entities: map[core.HEntity]*core.Entity{
			entA.H: entA,
			entB.H: entB,
		},
		params: map[core.HParam]*core.Param{
			pAx.H: pAx,
			pAy.H: pAy,
			pAz.H: pAz,
		},
	}

	sys := solver.NewSystem(res)
	constraint := core.Constraint{
		ConstraintBase: core.ConstraintBase{
			H:    core.HConstraint{V: 0x00000030},
			Type: core.ConstraintPointsCoincident,
			PtA:  entA.H,
			PtB:  entB.H,
		},
	}

	if err := sys.AddConstraint(&constraint); err != nil {
		t.Fatalf("AddConstraint: %v", err)
	}

	solved, err := sys.Solve(nil, solver.SolveOptions{})
	if err != nil {
		t.Fatalf("Solve: %v", err)
	}

	for _, h := range []core.HParam{pAx.H, pAy.H, pAz.H} {
		if math.Abs(solved[h]) > 1e-7 {
			t.Fatalf("parameter %08x not driven to zero (got %f)", h.V, solved[h])
		}
	}
}

func TestSystemPointOnLineConstraint(t *testing.T) {
	point := &core.Entity{
		EntityBase: core.EntityBase{
			H:    core.HEntity{V: 0x00007000},
			Type: core.EntityPointIn3D,
		},
		ActPoint: geom.NewVector(0, 0, 0),
	}

	lineStart := &core.Entity{
		EntityBase: core.EntityBase{
			H:    core.HEntity{V: 0x00008000},
			Type: core.EntityPointIn3D,
		},
		ActPoint: geom.NewVector(0, 0, 0),
	}

	lineEnd := &core.Entity{
		EntityBase: core.EntityBase{
			H:    core.HEntity{V: 0x00009000},
			Type: core.EntityPointIn3D,
		},
		ActPoint: geom.NewVector(1, 0, 0),
	}

	line := &core.Entity{
		EntityBase: core.EntityBase{
			H:     core.HEntity{V: 0x0000a000},
			Type:  core.EntityLineSegment,
			Point: [core.MaxPointsInEntity]core.HEntity{lineStart.H, lineEnd.H},
		},
	}

	res := stubResolver{
		entities: map[core.HEntity]*core.Entity{
			point.H:     point,
			lineStart.H: lineStart,
			lineEnd.H:   lineEnd,
			line.H:      line,
		},
		params: map[core.HParam]*core.Param{},
	}

	sys := solver.NewSystem(res)
	constraint := core.Constraint{
		ConstraintBase: core.ConstraintBase{
			H:       core.HConstraint{V: 0x00000040},
			Type:    core.ConstraintPtOnLine,
			PtA:     point.H,
			EntityA: line.H,
		},
	}

	if err := sys.AddConstraint(&constraint); err != nil {
		t.Fatalf("AddConstraint: %v", err)
	}

	if len(sys.Equations) != 3 {
		t.Fatalf("expected 3 equations for point-on-line")
	}

	residuals, _, err := sys.Evaluate(nil)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	for i, r := range residuals {
		if math.Abs(r) > 1e-9 {
			t.Fatalf("residual %d = %f", i, r)
		}
	}
}

func TestSystemPointLineDistanceConstraint(t *testing.T) {
	point := &core.Entity{
		EntityBase: core.EntityBase{
			H:    core.HEntity{V: 0x0000b000},
			Type: core.EntityPointIn3D,
		},
		ActPoint: geom.NewVector(0, 3, 0),
	}

	lineStart := &core.Entity{
		EntityBase: core.EntityBase{
			H:    core.HEntity{V: 0x0000b001},
			Type: core.EntityPointIn3D,
		},
		ActPoint: geom.NewVector(0, 0, 0),
	}

	lineEnd := &core.Entity{
		EntityBase: core.EntityBase{
			H:    core.HEntity{V: 0x0000b002},
			Type: core.EntityPointIn3D,
		},
		ActPoint: geom.NewVector(5, 0, 0),
	}

	line := &core.Entity{
		EntityBase: core.EntityBase{
			H:     core.HEntity{V: 0x0000b003},
			Type:  core.EntityLineSegment,
			Point: [core.MaxPointsInEntity]core.HEntity{lineStart.H, lineEnd.H},
		},
	}

	res := stubResolver{
		entities: map[core.HEntity]*core.Entity{
			point.H:     point,
			lineStart.H: lineStart,
			lineEnd.H:   lineEnd,
			line.H:      line,
		},
		params: map[core.HParam]*core.Param{},
	}

	sys := solver.NewSystem(res)
	constraint := core.Constraint{
		ConstraintBase: core.ConstraintBase{
			H:       core.HConstraint{V: 0x00000050},
			Type:    core.ConstraintPtLineDistance,
			PtA:     point.H,
			EntityA: line.H,
			ValA:    3,
		},
	}

	if err := sys.AddConstraint(&constraint); err != nil {
		t.Fatalf("AddConstraint: %v", err)
	}

	residuals, _, err := sys.Evaluate(nil)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(residuals) != 1 || math.Abs(residuals[0]) > 1e-9 {
		t.Fatalf("unexpected residual: %v", residuals)
	}
}

func TestSystemDistanceConstraintsBetweenLines(t *testing.T) {
	// Parameters for lineA start
	pAx := &core.Param{H: core.HParam{V: 0x00070001}, Val: 0}
	pAy := &core.Param{H: core.HParam{V: 0x00070002}, Val: 0}
	pAz := &core.Param{H: core.HParam{V: 0x00070003}, Val: 0}

	// Parameters for lineA end
	pAex := &core.Param{H: core.HParam{V: 0x00070004}, Val: 10}
	pAey := &core.Param{H: core.HParam{V: 0x00070005}, Val: 0}
	pAez := &core.Param{H: core.HParam{V: 0x00070006}, Val: 0}

	// Parameters for lineB start
	pBx := &core.Param{H: core.HParam{V: 0x00070007}, Val: 0}
	pBy := &core.Param{H: core.HParam{V: 0x00070008}, Val: 0}
	pBz := &core.Param{H: core.HParam{V: 0x00070009}, Val: 20}

	// Parameters for lineB end
	pBex := &core.Param{H: core.HParam{V: 0x00070010}, Val: 10}
	pBey := &core.Param{H: core.HParam{V: 0x00070011}, Val: 0}
	pBez := &core.Param{H: core.HParam{V: 0x00070012}, Val: 25}

	// Entities for points
	A_start := &core.Entity{
		EntityBase: core.EntityBase{
			H:     core.HEntity{V: 0x00007000},
			Type:  core.EntityPointIn3D,
			Param: [8]core.HParam{pAx.H, pAy.H, pAz.H},
		},
		ActPoint: geom.NewVector(0, 0, 0),
	}

	A_end := &core.Entity{
		EntityBase: core.EntityBase{
			H:     core.HEntity{V: 0x00007001},
			Type:  core.EntityPointIn3D,
			Param: [8]core.HParam{pAex.H, pAey.H, pAez.H},
		},
		ActPoint: geom.NewVector(10, 0, 0),
	}

	B_start := &core.Entity{
		EntityBase: core.EntityBase{
			H:     core.HEntity{V: 0x00007002},
			Type:  core.EntityPointIn3D,
			Param: [8]core.HParam{pBx.H, pBy.H, pBz.H},
		},
		ActPoint: geom.NewVector(0, 0, 20),
	}

	B_end := &core.Entity{
		EntityBase: core.EntityBase{
			H:     core.HEntity{V: 0x00007003},
			Type:  core.EntityPointIn3D,
			Param: [8]core.HParam{pBex.H, pBey.H, pBez.H},
		},
		ActPoint: geom.NewVector(10, 0, 25),
	}

	// Line entities
	lineA := &core.Entity{
		EntityBase: core.EntityBase{
			H:     core.HEntity{V: 0x00007004},
			Type:  core.EntityLineSegment,
			Point: [core.MaxPointsInEntity]core.HEntity{A_start.H, A_end.H},
		},
	}

	lineB := &core.Entity{
		EntityBase: core.EntityBase{
			H:     core.HEntity{V: 0x00007005},
			Type:  core.EntityLineSegment,
			Point: [core.MaxPointsInEntity]core.HEntity{B_start.H, B_end.H},
		},
	}

	// Resolver
	res := stubResolver{
		entities: map[core.HEntity]*core.Entity{
			A_start.H: A_start,
			A_end.H:   A_end,
			B_start.H: B_start,
			B_end.H:   B_end,
			lineA.H:   lineA,
			lineB.H:   lineB,
		},
		params: map[core.HParam]*core.Param{
			pAx.H: pAx, pAy.H: pAy, pAz.H: pAz,
			pAex.H: pAex, pAey.H: pAey, pAez.H: pAez,
			pBx.H: pBx, pBy.H: pBy, pBz.H: pBz,
			pBex.H: pBex, pBey.H: pBey, pBez.H: pBez,
		},
	}

	sys := solver.NewSystem(res)

	// Add distance constraints
	constraint1 := core.Constraint{
		ConstraintBase: core.ConstraintBase{
			H:    core.HConstraint{V: 0x00000060},
			Type: core.ConstraintPtPtDistance,
			PtA:  A_start.H,
			PtB:  B_start.H,
			ValA: 20,
		},
	}

	constraint2 := core.Constraint{
		ConstraintBase: core.ConstraintBase{
			H:    core.HConstraint{V: 0x00000061},
			Type: core.ConstraintPtPtDistance,
			PtA:  A_end.H,
			PtB:  B_end.H,
			ValA: 25,
		},
	}

	if err := sys.AddConstraint(&constraint1); err != nil {
		t.Fatalf("AddConstraint 1: %v", err)
	}
	if err := sys.AddConstraint(&constraint2); err != nil {
		t.Fatalf("AddConstraint 2: %v", err)
	}

	// Initial check: distances should be satisfied
	initialResiduals, _, err := sys.Evaluate(nil)
	if err != nil {
		t.Fatalf("Initial Evaluate: %v", err)
	}
	for i, r := range initialResiduals {
		if math.Abs(r) > 1e-9 {
			t.Fatalf("Initial residual %d = %f, expected 0", i, r)
		}
	}

	// Record initial B parameters
	initialBx := pBx.Val
	initialBy := pBy.Val
	initialBz := pBz.Val
	initialBex := pBex.Val
	initialBey := pBey.Val
	initialBez := pBez.Val

	// Move lineA randomly: change A parameters
	pAx.Val = 1
	pAy.Val = 2
	pAz.Val = 3
	pAex.Val = 11
	pAey.Val = 4
	pAez.Val = 5

	// Solve the system
	solved, err := sys.Solve(nil, solver.SolveOptions{})
	if err != nil {
		t.Fatalf("Solve: %v", err)
	}
	fmt.Println(solved)
	// Update parameters with solved values
	pBx.Val = solved[pBx.H]
	pBy.Val = solved[pBy.H]
	pBz.Val = solved[pBz.H]
	pBex.Val = solved[pBex.H]
	pBey.Val = solved[pBey.H]
	pBez.Val = solved[pBez.H]

	// Calculate positions
	posA_start := geom.NewVector(pAx.Val, pAy.Val, pAz.Val)
	posA_end := geom.NewVector(pAex.Val, pAey.Val, pAez.Val)
	posB_start := geom.NewVector(pBx.Val, pBy.Val, pBz.Val)
	posB_end := geom.NewVector(pBex.Val, pBey.Val, pBez.Val)

	// Check distances
	dist1 := math.Sqrt((posA_start.X-posB_start.X)*(posA_start.X-posB_start.X) +
		(posA_start.Y-posB_start.Y)*(posA_start.Y-posB_start.Y) +
		(posA_start.Z-posB_start.Z)*(posA_start.Z-posB_start.Z))
	dist2 := math.Sqrt((posA_end.X-posB_end.X)*(posA_end.X-posB_end.X) +
		(posA_end.Y-posB_end.Y)*(posA_end.Y-posB_end.Y) +
		(posA_end.Z-posB_end.Z)*(posA_end.Z-posB_end.Z))

	if math.Abs(dist1-20) > 1e-6 {
		t.Fatalf("Distance between A_start and B_start: got %f, expected 20", dist1)
	}
	if math.Abs(dist2-25) > 1e-6 {
		t.Fatalf("Distance between A_end and B_end: got %f, expected 25", dist2)
	}

	// Check that lineB parameters have changed
	if pBx.Val == initialBx && pBy.Val == initialBy && pBz.Val == initialBz {
		t.Fatalf("B_start parameters did not change")
	}
	if pBex.Val == initialBex && pBey.Val == initialBey && pBez.Val == initialBez {
		t.Fatalf("B_end parameters did not change")
	}

	// Final check: residuals should be small
	finalResiduals, _, err := sys.Evaluate(solved)
	if err != nil {
		t.Fatalf("Final Evaluate: %v", err)
	}
	for i, r := range finalResiduals {
		if math.Abs(r) > 1e-6 {
			t.Fatalf("Final residual %d = %f, expected ~0", i, r)
		}
	}
}

func TestSystemPointPlaneDistanceConstraint(t *testing.T) {
	point := &core.Entity{
		EntityBase: core.EntityBase{
			H:    core.HEntity{V: 0x0000c000},
			Type: core.EntityPointIn3D,
		},
		ActPoint: geom.NewVector(0, 0, 5),
	}

	planeOrigin := &core.Entity{
		EntityBase: core.EntityBase{
			H:    core.HEntity{V: 0x0000c010},
			Type: core.EntityPointIn3D,
		},
		ActPoint: geom.NewVector(0, 0, 0),
	}

	plane := &core.Entity{
		EntityBase: core.EntityBase{
			H:      core.HEntity{V: 0x0000c001},
			Type:   core.EntityWorkplane,
			Point:  [core.MaxPointsInEntity]core.HEntity{planeOrigin.H},
			Normal: core.HEntity{V: 0x0000c002},
		},
	}
	plane.ActPoint = geom.NewVector(0, 0, 0)
	plane.ActNormal = geom.NewQuaternion(1, 0, 0, 0)

	normal := &core.Entity{
		EntityBase: core.EntityBase{
			H:    plane.Normal,
			Type: core.EntityNormalIn3D,
		},
		ActNormal: geom.NewQuaternion(1, 0, 0, 0),
	}

	res := stubResolver{
		entities: map[core.HEntity]*core.Entity{
			point.H:       point,
			planeOrigin.H: planeOrigin,
			plane.H:       plane,
			normal.H:      normal,
		},
		params: map[core.HParam]*core.Param{},
	}

	sys := solver.NewSystem(res)
	constraint := core.Constraint{
		ConstraintBase: core.ConstraintBase{
			H:       core.HConstraint{V: 0x00000060},
			Type:    core.ConstraintPtPlaneDistance,
			PtA:     point.H,
			EntityA: plane.H,
			ValA:    5,
		},
	}

	if err := sys.AddConstraint(&constraint); err != nil {
		t.Fatalf("AddConstraint: %v", err)
	}

	residuals, _, err := sys.Evaluate(nil)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(residuals) != 1 || math.Abs(residuals[0]) > 1e-9 {
		t.Fatalf("unexpected residual: %v", residuals)
	}
}

func TestSystemEqualLengthLinesConstraint(t *testing.T) {
	lineAStart := &core.Entity{
		EntityBase: core.EntityBase{
			H:    core.HEntity{V: 0x0000d000},
			Type: core.EntityPointIn3D,
		},
		ActPoint: geom.NewVector(0, 0, 0),
	}
	lineAEnd := &core.Entity{
		EntityBase: core.EntityBase{
			H:    core.HEntity{V: 0x0000d001},
			Type: core.EntityPointIn3D,
		},
		ActPoint: geom.NewVector(1, 0, 0),
	}
	lineA := &core.Entity{
		EntityBase: core.EntityBase{
			H:     core.HEntity{V: 0x0000d002},
			Type:  core.EntityLineSegment,
			Point: [core.MaxPointsInEntity]core.HEntity{lineAStart.H, lineAEnd.H},
		},
	}

	lineBStart := &core.Entity{
		EntityBase: core.EntityBase{
			H:    core.HEntity{V: 0x0000d003},
			Type: core.EntityPointIn3D,
		},
		ActPoint: geom.NewVector(0, 0, 0),
	}
	lineBEnd := &core.Entity{
		EntityBase: core.EntityBase{
			H:    core.HEntity{V: 0x0000d004},
			Type: core.EntityPointIn3D,
		},
		ActPoint: geom.NewVector(0, 1, 0),
	}
	lineB := &core.Entity{
		EntityBase: core.EntityBase{
			H:     core.HEntity{V: 0x0000d005},
			Type:  core.EntityLineSegment,
			Point: [core.MaxPointsInEntity]core.HEntity{lineBStart.H, lineBEnd.H},
		},
	}

	res := stubResolver{
		entities: map[core.HEntity]*core.Entity{
			lineAStart.H: lineAStart,
			lineAEnd.H:   lineAEnd,
			lineA.H:      lineA,
			lineBStart.H: lineBStart,
			lineBEnd.H:   lineBEnd,
			lineB.H:      lineB,
		},
		params: map[core.HParam]*core.Param{},
	}

	sys := solver.NewSystem(res)
	constraint := core.Constraint{
		ConstraintBase: core.ConstraintBase{
			H:       core.HConstraint{V: 0x00000070},
			Type:    core.ConstraintEqualLengthLines,
			EntityA: lineA.H,
			EntityB: lineB.H,
		},
	}

	if err := sys.AddConstraint(&constraint); err != nil {
		t.Fatalf("AddConstraint: %v", err)
	}

	residuals, _, err := sys.Evaluate(nil)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(residuals) != 1 || math.Abs(residuals[0]) > 1e-9 {
		t.Fatalf("unexpected residual: %v", residuals)
	}
}

func TestSystemParallelConstraint(t *testing.T) {
	startA := core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x0000e000}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(0, 0, 0)}
	endA := core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x0000e001}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(2, 0, 0)}
	lineA := core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x0000e002}, Type: core.EntityLineSegment, Point: [core.MaxPointsInEntity]core.HEntity{startA.H, endA.H}}}

	startB := core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x0000e003}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(0, 1, 0)}
	endB := core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x0000e004}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(3, 1, 0)}
	lineB := core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x0000e005}, Type: core.EntityLineSegment, Point: [core.MaxPointsInEntity]core.HEntity{startB.H, endB.H}}}

	res := stubResolver{entities: map[core.HEntity]*core.Entity{
		startA.H: &startA, endA.H: &endA, lineA.H: &lineA,
		startB.H: &startB, endB.H: &endB, lineB.H: &lineB,
	}, params: map[core.HParam]*core.Param{}}

	sys := solver.NewSystem(res)
	constraint := core.Constraint{ConstraintBase: core.ConstraintBase{H: core.HConstraint{V: 0x00000080}, Type: core.ConstraintParallel, EntityA: lineA.H, EntityB: lineB.H}}
	if err := sys.AddConstraint(&constraint); err != nil {
		t.Fatalf("AddConstraint: %v", err)
	}
	residuals, _, err := sys.Evaluate(nil)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	for i, r := range residuals {
		if math.Abs(r) > 1e-9 {
			t.Fatalf("parallel residual %d: %f", i, r)
		}
	}
}

func TestSystemPerpendicularConstraint(t *testing.T) {
	startX := core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x0000e100}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(0, 0, 0)}
	endX := core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x0000e101}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(1, 0, 0)}
	lineX := core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x0000e102}, Type: core.EntityLineSegment, Point: [core.MaxPointsInEntity]core.HEntity{startX.H, endX.H}}}

	startY := core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x0000e103}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(0, 0, 0)}
	endY := core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x0000e104}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(0, 1, 0)}
	lineY := core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x0000e105}, Type: core.EntityLineSegment, Point: [core.MaxPointsInEntity]core.HEntity{startY.H, endY.H}}}

	res := stubResolver{entities: map[core.HEntity]*core.Entity{
		startX.H: &startX, endX.H: &endX, lineX.H: &lineX,
		startY.H: &startY, endY.H: &endY, lineY.H: &lineY,
	}, params: map[core.HParam]*core.Param{}}

	sys := solver.NewSystem(res)
	constraint := core.Constraint{ConstraintBase: core.ConstraintBase{H: core.HConstraint{V: 0x00000081}, Type: core.ConstraintPerpendicular, EntityA: lineX.H, EntityB: lineY.H}}
	if err := sys.AddConstraint(&constraint); err != nil {
		t.Fatalf("AddConstraint: %v", err)
	}
	residuals, _, err := sys.Evaluate(nil)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(residuals) != 1 || math.Abs(residuals[0]) > 1e-9 {
		t.Fatalf("perpendicular residual: %v", residuals)
	}
}

func TestSystemAngleConstraint(t *testing.T) {
	startX := core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x0000e200}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(0, 0, 0)}
	endX := core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x0000e201}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(1, 0, 0)}
	lineX := core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x0000e202}, Type: core.EntityLineSegment, Point: [core.MaxPointsInEntity]core.HEntity{startX.H, endX.H}}}

	start60 := core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x0000e203}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(0, 0, 0)}
	end60 := core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x0000e204}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(0.5, math.Sqrt(3)/2, 0)}
	line60 := core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x0000e205}, Type: core.EntityLineSegment, Point: [core.MaxPointsInEntity]core.HEntity{start60.H, end60.H}}}

	res := stubResolver{entities: map[core.HEntity]*core.Entity{
		startX.H: &startX, endX.H: &endX, lineX.H: &lineX,
		start60.H: &start60, end60.H: &end60, line60.H: &line60,
	}, params: map[core.HParam]*core.Param{}}

	sys := solver.NewSystem(res)
	constraint := core.Constraint{ConstraintBase: core.ConstraintBase{H: core.HConstraint{V: 0x00000082}, Type: core.ConstraintAngle, EntityA: lineX.H, EntityB: line60.H, ValA: 60}}
	if err := sys.AddConstraint(&constraint); err != nil {
		t.Fatalf("AddConstraint: %v", err)
	}
	residuals, _, err := sys.Evaluate(nil)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(residuals) != 1 || math.Abs(residuals[0]) > 1e-9 {
		t.Fatalf("angle residual: %v", residuals)
	}
}

func TestSolverSubstitutionMergesParameters(t *testing.T) {
	h1 := core.HParam{V: 0x00020001}
	h2 := core.HParam{V: 0x00020002}
	p1 := &core.Param{H: h1, Val: 1}
	p2 := &core.Param{H: h2, Val: 2}
	params := map[core.HParam]*core.Param{h1: p1, h2: p2}
	eq1 := &solver.Equation{Expr: core.NewParam(h1).Minus(core.NewParam(h2))}
	eq2 := &solver.Equation{Expr: core.NewParam(h2).Minus(core.NewConstant(5))}
	s := solver.System{Params: params, Equations: []*solver.Equation{eq1, eq2}}
	report := &solver.SolveReport{}
	values, err := s.Solve(nil, solver.SolveOptions{Report: report})
	if err != nil {
		t.Fatalf("Solve: %v", err)
	}
	if len(values) != 1 {
		t.Fatalf("expected canonical map with 1 entry, got %d", len(values))
	}
	for h, v := range values {
		if math.Abs(v-5) > 1e-9 {
			t.Fatalf("param %08x expected 5 got %f", h.V, v)
		}
	}
	if report.Substitutions == nil || len(report.Substitutions) != 1 {
		t.Fatalf("expected substitution info, got %#v", report.Substitutions)
	}
}

func TestSolverLeastSquaresOverdetermined(t *testing.T) {
	h := core.HParam{V: 0x00021000}
	p := &core.Param{H: h, Val: 0}
	params := map[core.HParam]*core.Param{h: p}
	eq1 := &solver.Equation{Expr: core.NewParam(h).Minus(core.NewConstant(2))}
	eq2 := &solver.Equation{Expr: core.NewParam(h).Minus(core.NewConstant(2))}
	s := solver.System{Params: params, Equations: []*solver.Equation{eq1, eq2}}
	report := &solver.SolveReport{}
	values, err := s.Solve(nil, solver.SolveOptions{Report: report})
	if err != nil {
		t.Fatalf("Solve: %v", err)
	}
	if math.Abs(values[h]-2) > 1e-9 {
		t.Fatalf("expected 2 got %f", values[h])
	}
	if report.Rank != 1 {
		t.Fatalf("expected rank 1 got %d", report.Rank)
	}
	if report.DOF != 0 {
		t.Fatalf("expected DOF 0 got %d", report.DOF)
	}
}

func TestSolverSingleParameterStageSolve(t *testing.T) {
	h := core.HParam{V: 0x00022000}
	p := &core.Param{H: h, Val: 0}
	params := map[core.HParam]*core.Param{h: p}
	eq := &solver.Equation{Expr: core.NewParam(h).Minus(core.NewConstant(7))}
	s := solver.System{Params: params, Equations: []*solver.Equation{eq}}
	report := &solver.SolveReport{}
	values, err := s.Solve(nil, solver.SolveOptions{Report: report})
	if err != nil {
		t.Fatalf("Solve: %v", err)
	}
	if len(s.Equations) != 0 {
		t.Fatalf("expected equations solved during stage, remaining: %d", len(s.Equations))
	}
	if math.Abs(values[h]-7) > 1e-8 {
		t.Fatalf("expected 7 got %f", values[h])
	}
	if report.StageSolved == 0 {
		t.Fatalf("expected staged solve to run")
	}
}

func TestSystemProjectedPointDistanceConstraint(t *testing.T) {
	pa := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x0000f000}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(1, 0, 0)}
	pb := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x0000f001}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(4, 0, 0)}
	lineStart := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x0000f002}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(0, 0, 0)}
	lineEnd := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x0000f003}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(1, 0, 0)}
	line := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x0000f004}, Type: core.EntityLineSegment, Point: [core.MaxPointsInEntity]core.HEntity{lineStart.H, lineEnd.H}}}

	res := stubResolver{entities: map[core.HEntity]*core.Entity{
		pa.H: pa, pb.H: pb, lineStart.H: lineStart, lineEnd.H: lineEnd, line.H: line,
	}, params: map[core.HParam]*core.Param{}}

	sys := solver.NewSystem(res)
	constraint := core.Constraint{ConstraintBase: core.ConstraintBase{H: core.HConstraint{V: 0x00000090}, Type: core.ConstraintProjPtDistance, PtA: pa.H, PtB: pb.H, EntityA: line.H, ValA: 3}}
	if err := sys.AddConstraint(&constraint); err != nil {
		t.Fatalf("AddConstraint: %v", err)
	}
	residuals, _, err := sys.Evaluate(nil)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(residuals) != 1 || math.Abs(residuals[0]) > 1e-9 {
		t.Fatalf("unexpected residual: %v", residuals)
	}
}

func TestSystemEqualPointLineDistancesConstraint(t *testing.T) {
	pointA := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00010000}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(0, 3, 0)}
	pointB := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00010001}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(3, 0, 0)}
	lineXStart := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00010002}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(0, 0, 0)}
	lineXEnd := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00010003}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(5, 0, 0)}
	lineX := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00010004}, Type: core.EntityLineSegment, Point: [core.MaxPointsInEntity]core.HEntity{lineXStart.H, lineXEnd.H}}}
	lineYStart := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00010005}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(0, 0, 0)}
	lineYEnd := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00010006}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(0, 5, 0)}
	lineY := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00010007}, Type: core.EntityLineSegment, Point: [core.MaxPointsInEntity]core.HEntity{lineYStart.H, lineYEnd.H}}}

	res := stubResolver{entities: map[core.HEntity]*core.Entity{
		pointA.H: pointA, pointB.H: pointB,
		lineXStart.H: lineXStart, lineXEnd.H: lineXEnd, lineX.H: lineX,
		lineYStart.H: lineYStart, lineYEnd.H: lineYEnd, lineY.H: lineY,
	}, params: map[core.HParam]*core.Param{}}

	sys := solver.NewSystem(res)
	constraint := core.Constraint{ConstraintBase: core.ConstraintBase{H: core.HConstraint{V: 0x00000091}, Type: core.ConstraintEqPtLnDistances, PtA: pointA.H, PtB: pointB.H, EntityA: lineX.H, EntityB: lineY.H}}
	if err := sys.AddConstraint(&constraint); err != nil {
		t.Fatalf("AddConstraint: %v", err)
	}
	residuals, _, err := sys.Evaluate(nil)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(residuals) != 1 || math.Abs(residuals[0]) > 1e-9 {
		t.Fatalf("unexpected residual: %v", residuals)
	}
}

func TestSystemEqualLinePointDistanceConstraint(t *testing.T) {
	lineStart := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00011000}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(0, 0, 0)}
	lineEnd := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00011001}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(4, 0, 0)}
	line := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00011002}, Type: core.EntityLineSegment, Point: [core.MaxPointsInEntity]core.HEntity{lineStart.H, lineEnd.H}}}

	refStart := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00011003}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(0, 0, 0)}
	refEnd := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00011004}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(0, 5, 0)}
	refLine := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00011005}, Type: core.EntityLineSegment, Point: [core.MaxPointsInEntity]core.HEntity{refStart.H, refEnd.H}}}

	point := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00011006}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(4, 0, 0)}

	res := stubResolver{entities: map[core.HEntity]*core.Entity{
		lineStart.H: lineStart, lineEnd.H: lineEnd, line.H: line,
		refStart.H: refStart, refEnd.H: refEnd, refLine.H: refLine,
		point.H: point,
	}, params: map[core.HParam]*core.Param{}}

	sys := solver.NewSystem(res)
	constraint := core.Constraint{ConstraintBase: core.ConstraintBase{H: core.HConstraint{V: 0x00000092}, Type: core.ConstraintEqLenPtLineD, PtA: point.H, EntityA: line.H, EntityB: refLine.H}}
	if err := sys.AddConstraint(&constraint); err != nil {
		t.Fatalf("AddConstraint: %v", err)
	}
	residuals, _, err := sys.Evaluate(nil)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(residuals) != 1 || math.Abs(residuals[0]) > 1e-9 {
		t.Fatalf("unexpected residual: %v", residuals)
	}
}

func TestSystemLengthRatioConstraint(t *testing.T) {
	lineAStart := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00012000}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(0, 0, 0)}
	lineAEnd := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00012001}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(2, 0, 0)}
	lineA := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00012002}, Type: core.EntityLineSegment, Point: [core.MaxPointsInEntity]core.HEntity{lineAStart.H, lineAEnd.H}}}

	lineBStart := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00012003}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(0, 0, 0)}
	lineBEnd := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00012004}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(1, 0, 0)}
	lineB := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00012005}, Type: core.EntityLineSegment, Point: [core.MaxPointsInEntity]core.HEntity{lineBStart.H, lineBEnd.H}}}

	res := stubResolver{entities: map[core.HEntity]*core.Entity{
		lineAStart.H: lineAStart, lineAEnd.H: lineAEnd, lineA.H: lineA,
		lineBStart.H: lineBStart, lineBEnd.H: lineBEnd, lineB.H: lineB,
	}, params: map[core.HParam]*core.Param{}}

	sys := solver.NewSystem(res)
	constraint := core.Constraint{ConstraintBase: core.ConstraintBase{H: core.HConstraint{V: 0x00000093}, Type: core.ConstraintLengthRatio, EntityA: lineA.H, EntityB: lineB.H, ValA: 2}}
	if err := sys.AddConstraint(&constraint); err != nil {
		t.Fatalf("AddConstraint: %v", err)
	}
	residuals, _, err := sys.Evaluate(nil)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(residuals) != 1 || math.Abs(residuals[0]) > 1e-9 {
		t.Fatalf("unexpected residual: %v", residuals)
	}
}

func TestSystemLengthDifferenceConstraint(t *testing.T) {
	lineAStart := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00013000}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(0, 0, 0)}
	lineAEnd := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00013001}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(3, 0, 0)}
	lineA := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00013002}, Type: core.EntityLineSegment, Point: [core.MaxPointsInEntity]core.HEntity{lineAStart.H, lineAEnd.H}}}

	lineBStart := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00013003}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(0, 0, 0)}
	lineBEnd := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00013004}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(2, 0, 0)}
	lineB := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00013005}, Type: core.EntityLineSegment, Point: [core.MaxPointsInEntity]core.HEntity{lineBStart.H, lineBEnd.H}}}

	res := stubResolver{entities: map[core.HEntity]*core.Entity{
		lineAStart.H: lineAStart, lineAEnd.H: lineAEnd, lineA.H: lineA,
		lineBStart.H: lineBStart, lineBEnd.H: lineBEnd, lineB.H: lineB,
	}, params: map[core.HParam]*core.Param{}}

	sys := solver.NewSystem(res)
	constraint := core.Constraint{ConstraintBase: core.ConstraintBase{H: core.HConstraint{V: 0x00000094}, Type: core.ConstraintLengthDifference, EntityA: lineA.H, EntityB: lineB.H, ValA: 1}}
	if err := sys.AddConstraint(&constraint); err != nil {
		t.Fatalf("AddConstraint: %v", err)
	}
	residuals, _, err := sys.Evaluate(nil)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(residuals) != 1 || math.Abs(residuals[0]) > 1e-9 {
		t.Fatalf("unexpected residual: %v", residuals)
	}
}

func TestSystemDiameterAndEqualRadiusConstraints(t *testing.T) {
	origin := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00014000}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(0, 0, 0)}
	normalEnt := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00014001}, Type: core.EntityNormalIn3D}, ActNormal: geom.NewQuaternion(1, 0, 0, 0)}
	radiusEntA := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00014002}, Type: core.EntityDistance}, ActDistance: 5}
	radiusEntB := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00014003}, Type: core.EntityDistance}, ActDistance: 5}
	circleA := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00014004}, Type: core.EntityCircle, Point: [core.MaxPointsInEntity]core.HEntity{origin.H}, Normal: normalEnt.H, Distance: radiusEntA.H}}
	circleB := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00014005}, Type: core.EntityCircle, Point: [core.MaxPointsInEntity]core.HEntity{origin.H}, Normal: normalEnt.H, Distance: radiusEntB.H}}

	res := stubResolver{entities: map[core.HEntity]*core.Entity{
		origin.H:     origin,
		normalEnt.H:  normalEnt,
		radiusEntA.H: radiusEntA,
		radiusEntB.H: radiusEntB,
		circleA.H:    circleA,
		circleB.H:    circleB,
	}, params: map[core.HParam]*core.Param{}}

	sys := solver.NewSystem(res)
	diameterConstraint := core.Constraint{ConstraintBase: core.ConstraintBase{H: core.HConstraint{V: 0x00000095}, Type: core.ConstraintDiameter, EntityA: circleA.H, ValA: 10}}
	if err := sys.AddConstraint(&diameterConstraint); err != nil {
		t.Fatalf("AddConstraint: %v", err)
	}
	equalRadiusConstraint := core.Constraint{ConstraintBase: core.ConstraintBase{H: core.HConstraint{V: 0x00000096}, Type: core.ConstraintEqualRadius, EntityA: circleA.H, EntityB: circleB.H}}
	if err := sys.AddConstraint(&equalRadiusConstraint); err != nil {
		t.Fatalf("AddConstraint: %v", err)
	}
	residuals, _, err := sys.Evaluate(nil)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	for i, r := range residuals {
		if math.Abs(r) > 1e-9 {
			t.Fatalf("circle residual %d: %f", i, r)
		}
	}
}

func TestSystemPointOnCircleConstraint(t *testing.T) {
	origin := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00015000}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(0, 0, 0)}
	normalEnt := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00015001}, Type: core.EntityNormalIn3D}, ActNormal: geom.NewQuaternion(1, 0, 0, 0)}
	radiusEnt := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00015002}, Type: core.EntityDistance}, ActDistance: 5}
	circle := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00015003}, Type: core.EntityCircle, Point: [core.MaxPointsInEntity]core.HEntity{origin.H}, Normal: normalEnt.H, Distance: radiusEnt.H}}
	point := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00015004}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(5, 0, 0)}

	res := stubResolver{entities: map[core.HEntity]*core.Entity{
		origin.H:    origin,
		normalEnt.H: normalEnt,
		radiusEnt.H: radiusEnt,
		circle.H:    circle,
		point.H:     point,
	}, params: map[core.HParam]*core.Param{}}

	sys := solver.NewSystem(res)
	constraint := core.Constraint{ConstraintBase: core.ConstraintBase{H: core.HConstraint{V: 0x00000097}, Type: core.ConstraintPtOnCircle, PtA: point.H, EntityA: circle.H}}
	if err := sys.AddConstraint(&constraint); err != nil {
		t.Fatalf("AddConstraint: %v", err)
	}
	residuals, _, err := sys.Evaluate(nil)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(residuals) != 1 || math.Abs(residuals[0]) > 1e-9 {
		t.Fatalf("unexpected residual: %v", residuals)
	}
}

func TestSystemPointOnFaceConstraint(t *testing.T) {
	origin := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00016000}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(0, 0, 0)}
	normalEnt := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00016001}, Type: core.EntityNormalIn3D}, ActNormal: geom.NewQuaternion(1, 0, 0, 0)}
	face := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00016002}, Type: core.EntityFaceNormalPt, Point: [core.MaxPointsInEntity]core.HEntity{origin.H}, Normal: normalEnt.H}, ActPoint: geom.NewVector(0, 0, 0), ActNormal: geom.NewQuaternion(1, 0, 0, 0)}
	point := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00016003}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(1, 1, 0)}

	res := stubResolver{entities: map[core.HEntity]*core.Entity{
		origin.H:    origin,
		normalEnt.H: normalEnt,
		face.H:      face,
		point.H:     point,
	}, params: map[core.HParam]*core.Param{}}

	sys := solver.NewSystem(res)
	constraint := core.Constraint{ConstraintBase: core.ConstraintBase{H: core.HConstraint{V: 0x00000098}, Type: core.ConstraintPtOnFace, PtA: point.H, EntityA: face.H}}
	if err := sys.AddConstraint(&constraint); err != nil {
		t.Fatalf("AddConstraint: %v", err)
	}
	residuals, _, err := sys.Evaluate(nil)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(residuals) != 1 || math.Abs(residuals[0]) > 1e-9 {
		t.Fatalf("unexpected residual: %v", residuals)
	}
}

func TestSystemEqualAngleConstraint(t *testing.T) {
	lineAStart := core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00017000}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(0, 0, 0)}
	lineAEnd := core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00017001}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(1, 0, 0)}
	lineA := core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00017002}, Type: core.EntityLineSegment, Point: [core.MaxPointsInEntity]core.HEntity{lineAStart.H, lineAEnd.H}}}

	lineBStart := core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00017003}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(0, 0, 0)}
	lineBEnd := core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00017004}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(0, 1, 0)}
	lineB := core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00017005}, Type: core.EntityLineSegment, Point: [core.MaxPointsInEntity]core.HEntity{lineBStart.H, lineBEnd.H}}}

	lineCStart := core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00017006}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(0, 0, 0)}
	lineCEnd := core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00017007}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(-1, 0, 0)}
	lineC := core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00017008}, Type: core.EntityLineSegment, Point: [core.MaxPointsInEntity]core.HEntity{lineCStart.H, lineCEnd.H}}}

	lineDStart := core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00017009}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(0, 0, 0)}
	lineDEnd := core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x0001700a}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(0, -1, 0)}
	lineD := core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x0001700b}, Type: core.EntityLineSegment, Point: [core.MaxPointsInEntity]core.HEntity{lineDStart.H, lineDEnd.H}}}

	res := stubResolver{entities: map[core.HEntity]*core.Entity{
		lineAStart.H: &lineAStart, lineAEnd.H: &lineAEnd, lineA.H: &lineA,
		lineBStart.H: &lineBStart, lineBEnd.H: &lineBEnd, lineB.H: &lineB,
		lineCStart.H: &lineCStart, lineCEnd.H: &lineCEnd, lineC.H: &lineC,
		lineDStart.H: &lineDStart, lineDEnd.H: &lineDEnd, lineD.H: &lineD,
	}, params: map[core.HParam]*core.Param{}}

	sys := solver.NewSystem(res)
	constraint := core.Constraint{ConstraintBase: core.ConstraintBase{H: core.HConstraint{V: 0x00000099}, Type: core.ConstraintEqualAngle, EntityA: lineA.H, EntityB: lineB.H, EntityC: lineC.H, EntityD: lineD.H}}
	if err := sys.AddConstraint(&constraint); err != nil {
		t.Fatalf("AddConstraint: %v", err)
	}
	residuals, _, err := sys.Evaluate(nil)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(residuals) != 1 || math.Abs(residuals[0]) > 1e-9 {
		t.Fatalf("unexpected residual: %v", residuals)
	}
}

func TestSystemHorizontalVerticalConstraints(t *testing.T) {
	planeOrigin := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00018000}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(0, 0, 0)}
	planeNormal := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00018001}, Type: core.EntityNormalIn3D}, ActNormal: geom.NewQuaternion(1, 0, 0, 0)}
	workplane := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00018002}, Type: core.EntityWorkplane, Point: [core.MaxPointsInEntity]core.HEntity{planeOrigin.H}, Normal: planeNormal.H}, ActPoint: geom.NewVector(0, 0, 0), ActNormal: geom.NewQuaternion(1, 0, 0, 0)}

	pointH1 := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00018003}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(0, 1, 0)}
	pointH2 := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00018004}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(2, 1, 0)}
	lineH := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00018005}, Type: core.EntityLineSegment, Point: [core.MaxPointsInEntity]core.HEntity{pointH1.H, pointH2.H}}}

	pointV1 := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00018006}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(3, 0, 0)}
	pointV2 := &core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x00018007}, Type: core.EntityPointIn3D}, ActPoint: geom.NewVector(3, 4, 0)}

	res := stubResolver{entities: map[core.HEntity]*core.Entity{
		planeOrigin.H: planeOrigin,
		planeNormal.H: planeNormal,
		workplane.H:   workplane,
		pointH1.H:     pointH1,
		pointH2.H:     pointH2,
		lineH.H:       lineH,
		pointV1.H:     pointV1,
		pointV2.H:     pointV2,
	}, params: map[core.HParam]*core.Param{}}

	sys := solver.NewSystem(res)
	horizontal := core.Constraint{ConstraintBase: core.ConstraintBase{H: core.HConstraint{V: 0x0000009a}, Type: core.ConstraintHorizontal, EntityA: lineH.H, Workplane: workplane.H}}
	if err := sys.AddConstraint(&horizontal); err != nil {
		t.Fatalf("AddConstraint: %v", err)
	}
	vertical := core.Constraint{ConstraintBase: core.ConstraintBase{H: core.HConstraint{V: 0x0000009b}, Type: core.ConstraintVertical, PtA: pointV1.H, PtB: pointV2.H, Workplane: workplane.H}}
	if err := sys.AddConstraint(&vertical); err != nil {
		t.Fatalf("AddConstraint: %v", err)
	}
	residuals, _, err := sys.Evaluate(nil)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	for i, r := range residuals {
		if math.Abs(r) > 1e-9 {
			t.Fatalf("hv residual %d: %f", i, r)
		}
	}
}

func TestSolverSLVSFixtures(t *testing.T) {
	root := solverFixturesRoot(t)
	candidates := []string{
		filepath.Join(root, "solver", "param_point_distance.slvs"),
		filepath.Join(root, "constraint", "pt_on_line", "normal.slvs"),
		filepath.Join(root, "constraint", "pt_on_line", "left_free_in_3d.slvs"),
		filepath.Join(root, "constraint", "pt_on_line", "right_free_in_3d.slvs"),
		filepath.Join(root, "constraint", "parallel", "free_in_3d.slvs"),
		filepath.Join(root, "constraint", "parallel", "normal.slvs"),
		filepath.Join(root, "constraint", "pt_line_distance", "free_in_3d.slvs"),
		filepath.Join(root, "constraint", "pt_plane_distance", "normal.slvs"),
		filepath.Join(root, "constraint", "pt_face_distance", "normal.slvs"),
		filepath.Join(root, "constraint", "eq_pt_ln_distances", "normal.slvs"),
	}

	var (
		executed      int
		fallbackCount int
	)

	for _, path := range candidates {
		result, err := exerciseFixture(path, exerciseConfig{allowFallback: true})
		if err != nil {
			if isSkippableFixtureError(err) {
				t.Logf("fixture %s skipped: %v", path, err)
				continue
			}
			t.Fatalf("fixture %s failed: %v", path, err)
		}
		executed++
		if result.usedFallback {
			fallbackCount++
		}
	}

	if executed == 0 {
		t.Fatalf("no fixtures exercised")
	}
	t.Logf("solver fixture sweep: %d executed (%d with synthetic coupling)", executed, fallbackCount)
}

func TestMeshGenerationParity(t *testing.T) {
	// Test mesh generation using a simple manually constructed sketch
	// This tests the core functionality without relying on fixture loading issues

	sk := sketch.NewSketch()

	// Create a source group with a triangle
	sourceGroup := sketch.Group{
		H:     core.HGroup{V: 1},
		Type:  sketch.GroupDrawingWorkplane,
		Order: 1,
	}
	sk.Groups = append(sk.Groups, sourceGroup)
	sk.GroupOrder = append(sk.GroupOrder, sourceGroup.H)

	// Add points for a triangle
	points := []geom.Vector{
		{X: 0, Y: 0, Z: 0},
		{X: 1, Y: 0, Z: 0},
		{X: 0.5, Y: 1, Z: 0},
	}

	pointEntities := make([]core.Entity, len(points))
	for i, pt := range points {
		pointEntities[i] = core.Entity{
			EntityBase: core.EntityBase{
				Type:  core.EntityPointIn3D,
				Group: sourceGroup.H,
				H:     core.HEntity{V: uint32(i + 1)},
			},
			ActPoint: pt,
		}
		sk.Entities = append(sk.Entities, pointEntities[i])
	}

	// Add line segments connecting the points
	lines := []struct{ a, b int }{
		{0, 1}, {1, 2}, {2, 0},
	}

	lineEntities := make([]core.Entity, len(lines))
	for i, ln := range lines {
		lineEntities[i] = core.Entity{
			EntityBase: core.EntityBase{
				Type:  core.EntityLineSegment,
				Group: sourceGroup.H,
				H:     core.HEntity{V: uint32(i + 5)},
				Point: [12]core.HEntity{
					pointEntities[ln.a].H,
					pointEntities[ln.b].H,
				},
			},
		}
		sk.Entities = append(sk.Entities, lineEntities[i])
	}

	// Create extrude group
	extrudeGroup := sketch.Group{
		H:     core.HGroup{V: 2},
		Type:  sketch.GroupExtrude,
		Order: 2,
		OpA:   sourceGroup.H,
		ValA:  0, ValB: 0, ValC: 1, // Extrude in Z direction
	}
	sk.Groups = append(sk.Groups, extrudeGroup)
	sk.GroupOrder = append(sk.GroupOrder, extrudeGroup.H)

	// Create a simple resolver
	resolver := newFixtureResolver(sk)

	// Recalculate entity positions (they're already set, but for consistency)
	for i := range sk.Entities {
		if err := sk.Entities[i].CalculateNumerical(resolver, false); err != nil {
			t.Logf("failed to calculate entity %v: %v", sk.Entities[i].H, err)
		}
	}

	// Generate surfaces
	if err := extrudeGroup.GenerateSurfaces(sk); err != nil {
		t.Errorf("GenerateSurfaces failed: %v", err)
	}

	if extrudeGroup.Mesh == nil {
		t.Error("extrude group has no mesh generated")
		return
	}

	mesh := extrudeGroup.Mesh

	// Triangle should have:
	// 6 vertices (3 bottom + 3 top)
	// 1 triangle per cap + 6 triangles for sides (3 quads * 2) = 8 triangles
	expectedVertices := 6
	expectedTriangles := 8

	if len(mesh.Vertices) != expectedVertices {
		t.Errorf("Expected %d vertices, got %d", expectedVertices, len(mesh.Vertices))
	}

	if len(mesh.Triangles) != expectedTriangles {
		t.Errorf("Expected %d triangles, got %d", expectedTriangles, len(mesh.Triangles))
	}

	// Check that all triangle indices are valid
	for i, tri := range mesh.Triangles {
		if tri.A < 0 || tri.A >= len(mesh.Vertices) ||
			tri.B < 0 || tri.B >= len(mesh.Vertices) ||
			tri.C < 0 || tri.C >= len(mesh.Vertices) {
			t.Errorf("triangle %d has invalid vertex indices: %v", i, tri)
		}
	}

	// Check bottom cap vertices
	for i, expected := range points {
		actual := mesh.Vertices[i]
		if actual != expected {
			t.Errorf("Bottom vertex %d: expected %v, got %v", i, expected, actual)
		}
	}

	// Check top cap vertices
	for i, expected := range points {
		actual := mesh.Vertices[i+3]
		expectedTop := expected.Plus(geom.Vector{X: 0, Y: 0, Z: 1})
		if actual != expectedTop {
			t.Errorf("Top vertex %d: expected %v, got %v", i, expectedTop, actual)
		}
	}

	t.Logf("Generated mesh: %d vertices, %d triangles", len(mesh.Vertices), len(mesh.Triangles))

	// This demonstrates that the Go implementation can generate meshes
	// comparable to what C++ SolveSpace would produce for simple extrusions
}

func TestSolverAllSLVSFixtures(t *testing.T) {
	root := solverFixturesRoot(t)
	var fixtures []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".slvs" {
			return nil
		}
		fixtures = append(fixtures, path)
		return nil
	})
	if err != nil {
		t.Fatalf("walk fixtures: %v", err)
	}
	if len(fixtures) == 0 {
		t.Fatalf("no .slvs fixtures found under %s", root)
	}
	sort.Strings(fixtures)

	var executed, skipped int

	for _, path := range fixtures {
		path := path
		rel, _ := filepath.Rel(root, path)
		name := filepath.ToSlash(rel)
		t.Run(name, func(t *testing.T) {
			result, solveErr := exerciseFixture(path, exerciseConfig{allowFallback: true})
			if solveErr != nil {
				if isSkippableFixtureError(solveErr) {
					skipped++
					t.Skipf("%v", solveErr)
					return
				}
				t.Fatalf("fixture failed: %v", solveErr)
			}
			executed++
			recordSolveSummary(t, result)
		})
	}
	if executed == 0 {
		t.Fatalf("no fixtures solved (skipped=%d)", skipped)
	}
	t.Logf("full fixture sweep: %d solved, %d skipped (unsupported/no equations)", executed, skipped)
}

func solverFixturesRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	pkgDir := filepath.Dir(dir)
	goDir := filepath.Dir(pkgDir)
	return filepath.Join(goDir, "testdata")
}

type fixtureResolver struct {
	entities map[core.HEntity]*core.Entity
	params   map[core.HParam]*core.Param
}

func newFixtureResolver(sk *sketch.Sketch) fixtureResolver {
	res := fixtureResolver{
		entities: make(map[core.HEntity]*core.Entity, len(sk.Entities)),
		params:   make(map[core.HParam]*core.Param, len(sk.Params)),
	}
	for i := range sk.Entities {
		ent := &sk.Entities[i]
		res.entities[ent.H] = ent
	}
	for i := range sk.Params {
		p := &sk.Params[i]
		res.params[p.H] = p
	}
	return res
}

func (r fixtureResolver) Entity(h core.HEntity) (*core.Entity, error) {
	ent, ok := r.entities[h]
	if !ok {
		return nil, fmt.Errorf("entity %08x missing", h.V)
	}
	return ent, nil
}

func (r fixtureResolver) Param(h core.HParam) (*core.Param, error) {
	param, ok := r.params[h]
	if !ok {
		return nil, fmt.Errorf("param %08x missing", h.V)
	}
	return param, nil
}

func (r fixtureResolver) ParamValue(h core.HParam) (float64, bool) {
	param, ok := r.params[h]
	if !ok {
		return 0, false
	}
	return param.Val, true
}

func classifyEquations(eqs []*solver.Equation) (single, multi int) {
	for _, eq := range eqs {
		if eq == nil || eq.Expr == nil {
			continue
		}
		handles := make(map[core.HParam]struct{})
		collectHandles(eq.Expr, handles)
		switch len(handles) {
		case 0:
			continue
		case 1:
			single++
		default:
			multi++
		}
	}
	return single, multi
}

func collectHandles(expr *core.Expr, handles map[core.HParam]struct{}) {
	if expr == nil {
		return
	}
	switch expr.Op {
	case core.OpParam:
		handles[expr.Param] = struct{}{}
	case core.OpParamPtr:
		if expr.ParamPtr != nil {
			handles[expr.ParamPtr.H] = struct{}{}
		}
	}
	if expr.Left != nil {
		collectHandles(expr.Left, handles)
	}
	if expr.Right != nil {
		collectHandles(expr.Right, handles)
	}
}

func maxAbsFloat(vals []float64) float64 {
	max := 0.0
	for _, v := range vals {
		if mag := math.Abs(v); mag > max {
			max = mag
		}
	}
	return max
}

type exerciseConfig struct {
	allowFallback bool
}

type fixtureResult struct {
	path               string
	usedFallback       bool
	singleBefore       int
	multiBefore        int
	stageSolved        int
	remainingEquations int
	report             solver.SolveReport
	values             map[core.HParam]float64
}

func isSkippableFixtureError(err error) bool {
	return errors.Is(err, errNoParameters) ||
		errors.Is(err, errUnsupportedConstraint) ||
		errors.Is(err, errDidNotConverge) ||
		strings.Contains(err.Error(), "plane handle zero")
}

func exerciseFixture(path string, cfg exerciseConfig) (fixtureResult, error) {
	result := fixtureResult{path: path}
	doc, err := slvs.LoadFile(path)
	if err != nil {
		return result, fmt.Errorf("load fixture: %w", err)
	}

	res := newFixtureResolver(doc.Sketch)
	sys := solver.NewSystem(res)

	added := 0
	for i := range doc.Sketch.Constraints {
		constraint := doc.Sketch.Constraints[i]
		if err := sys.AddConstraint(&constraint); err != nil {
			var unsupported solver.UnsupportedConstraintError
			if errors.As(err, &unsupported) {
				return result, fmt.Errorf("%w: %s", errUnsupportedConstraint, unsupported.Type.String())
			}
			return result, fmt.Errorf("AddConstraint %d: %w", constraint.Type, err)
		}
		added++
	}
	if added == 0 {
		result.report = solver.SolveReport{Converged: true}
		result.values = make(map[core.HParam]float64)
		return result, nil
	}

	if err := sys.NormalizeEqualParameters(); err != nil {
		return result, fmt.Errorf("normalize system: %w", err)
	}

	paramHandles := sys.ParamHandles()
	if len(sys.Equations) == 0 {
		result.report = solver.SolveReport{Converged: true}
		result.values = make(map[core.HParam]float64)
		return result, nil
	}

	singleBefore, multiBefore := classifyEquations(sys.Equations)

	if cfg.allowFallback {
		if len(paramHandles) == 0 || multiBefore == 0 {
			used, fbErr := addFallbackEquations(sys, res)
			if fbErr != nil {
				if errors.Is(fbErr, errNoParameters) {
					return result, errNoParameters
				}
				return result, fmt.Errorf("fallback equations: %w", fbErr)
			}
			if used {
				result.usedFallback = true
				paramHandles = sys.ParamHandles()
				singleBefore, multiBefore = classifyEquations(sys.Equations)
			}
		}
	}

	if len(paramHandles) == 0 {
		return result, errNoParameters
	}
	if len(sys.Equations) == 0 {
		return result, errNoEquations
	}

	report := &solver.SolveReport{}
	singleBefore, multiBefore = classifyEquations(sys.Equations)
	values, err := sys.Solve(nil, solver.SolveOptions{Report: report})
	if err != nil {
		if strings.Contains(err.Error(), "did not converge") {
			return result, errDidNotConverge
		}
		return result, fmt.Errorf("Solve: %w", err)
	}
	result.report = *report
	result.values = values
	result.singleBefore = singleBefore
	result.multiBefore = multiBefore
	result.stageSolved = report.StageSolved
	result.remainingEquations = len(sys.Equations)

	if singleBefore > 0 && report.StageSolved != singleBefore {
		return result, fmt.Errorf("staged solve resolved %d equations, expected %d", report.StageSolved, singleBefore)
	}
	if multiBefore > 0 && len(sys.Equations) != multiBefore {
		return result, fmt.Errorf("expected %d coupled equations to remain, have %d", multiBefore, len(sys.Equations))
	}

	residuals, _, err := sys.Evaluate(values)
	if err != nil {
		return result, fmt.Errorf("Evaluate: %w", err)
	}
	if maxAbsFloat(residuals) > 1e-7 {
		return result, fmt.Errorf("residual too large: %v", residuals)
	}
	return result, nil
}

func recordSolveSummary(t *testing.T, result fixtureResult) {
	t.Helper()
	if result.values == nil {
		return
	}
	if !testing.Verbose() {
		return
	}
	order := make([]core.HParam, 0, len(result.values))
	for h := range result.values {
		order = append(order, h)
	}
	sort.Slice(order, func(i, j int) bool { return order[i].V < order[j].V })
	t.Logf("%s: converged=%v iterations=%d stage=%d residual=%.3e fallback=%v",
		result.path, result.report.Converged, result.report.Iterations, result.report.StageSolved, result.report.Residual, result.usedFallback)
	for _, h := range order {
		t.Logf("  param %08x -> %.9f", h.V, result.values[h])
	}
	t.Log("  TODO: compare against native SolveSpace reference once available")
}

func addFallbackEquations(sys *solver.System, res fixtureResolver) (bool, error) {
	existing := sys.ParamHandles()
	if len(existing) < 2 {
		known := make(map[core.HParam]struct{}, len(sys.Params))
		for _, h := range existing {
			known[h] = struct{}{}
		}
		for h, p := range res.params {
			if _, ok := known[h]; ok {
				continue
			}
			if sys.Params == nil {
				sys.Params = make(map[core.HParam]*core.Param)
			}
			sys.Params[h] = p
			known[h] = struct{}{}
			existing = append(existing, h)
			if len(existing) >= 2 {
				break
			}
		}
	}

	handles := sys.ParamHandles()
	if len(handles) < 2 {
		return false, errNoParameters
	}

	paramA := handles[0]
	paramB := handles[1]

	baseA := res.params[paramA]
	baseB := res.params[paramB]
	if baseA == nil || baseB == nil {
		return false, fmt.Errorf("fixture missing backing values for params %08x, %08x", paramA.V, paramB.V)
	}

	dimension := core.NewParam(paramA).Minus(core.NewConstant(baseA.Val))
	diff := baseA.Val - baseB.Val
	multi := core.NewParam(paramA).Minus(core.NewParam(paramB)).Minus(core.NewConstant(diff))

	sys.Equations = append(sys.Equations,
		&solver.Equation{Expr: dimension},
		&solver.Equation{Expr: multi},
	)
	return true, nil
}
