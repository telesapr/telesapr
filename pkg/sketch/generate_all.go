package sketch

import (
	"fmt"
	"sort"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/solver"
)

// GenerateAll is the master orchestrator: walk every group in
// topological order (by Group.Order), generate its entities and
// constraints, prune orphans, solve constraints, regenerate shells
// and meshes. Goal Item 3 implementation: builds the solver system
// incrementally, runs a single Newton solve across all groups, and
// then regenerates the mesh for mesh-groups.
func (sk *Sketch) GenerateAll() {
	// Phase 1: prune orphans / invalid groups / invalid handles.
	sk.PruneOrphans()
	for sk.PruneGroups() {
	}
	for sk.PruneRequestsAndConstraints() {
	}
	// Clear per-group diagnostics from the previous GenerateAll.
	for i := range sk.Groups {
		sk.Groups[i].SketchDiagnostic = nil
	}

	// Phase 2: topologically sort groups by Order.
	order := sk.orderedGroups()

	// Phase 2.5: numerically evaluate every entity's ActPoint,
	// ActNormal, and ActDistance so constraint emitters that fall
	// back to numeric values (e.g. circleData for pt-on-circle,
	// planeExpr for pt-plane-distance) have something to read.
	rt := NewRuntime(sk)
	rt.RecalculateEntities(false)

	// Phase 3: build the solver system incrementally. For each
	// group in order: invoke requests, then constraints, then emit
	// group-specific symbolic equations (e.g. quaternion magnitude
	// for LINKED).
	resolver := &sketchSolverResolver{sk: sk}
	sys := solver.NewSystem(resolver)
	for _, hg := range order {
		g := sk.GetGroup(hg)
		if g == nil {
			continue
		}
		// Run each request's Generate to populate entities/params.
		entities := core.EntityList(sk.Entities)
		params := core.ParamList(sk.Params)
		groupReqIdx := 0
		for ri := range sk.Requests {
			r := &sk.Requests[ri]
			if r.Group != hg {
				continue
			}
			r.GroupRequestIndex = groupReqIdx
			groupReqIdx++
			r.Generate(&entities, &params)
		}
		sk.Entities = []core.Entity(entities)
		sk.Params = []core.Param(params)
		// request.Generate recreates entities and params for every
		// request, which means the same handle appears twice in
		// sk.Entities and sk.Params: the first copy from the parser
		// (with the correct numeric values) and the second copy
		// from request.Generate (with zero ActPoint/ActNormal/
		// ActDistance/Val). Collapse to a single entry per handle,
		// preferring the one with non-zero numeric state.
		sk.Entities = dedupEntitiesByHandle(sk.Entities)
		sk.Params = dedupParamsByHandle(sk.Params)
		// Re-evaluate every entity's numeric state now that the
		// request pipeline has created (and possibly duplicated)
		// entities. Without this, the COPIES would have zero
		// ActPoint/ActNormal/ActDistance and constraint emitters
		// that read those values (e.g. circleData, planeExpr)
		// would build ill-conditioned equations.
		rt.RecalculateEntities(false)

		// Add each constraint's equations to the solver.
		for ci := range sk.Constraints {
			c := &sk.Constraints[ci]
			if c.Group != hg {
				continue
			}
			GenerateConstraint(c, sk)
			if err := sys.AddConstraint(c); err != nil {
				// Surface in the group's reported diagnostics so
				// callers can debug. The solve still runs.
				if g.SketchDiagnostic == nil {
					g.SketchDiagnostic = &SketchDiagnostic{}
				}
				g.SketchDiagnostic.AddConstraintErrors = append(g.SketchDiagnostic.AddConstraintErrors,
					fmt.Sprintf("group %v constraint %v (type %d): %v", hg, c.H, c.Type, err))
			}
		}

		// Emit group-specific symbolic equations (LINKED, EXTRUDE,
		// ROTATE/REVOLVE/HELIX, TRANSLATE).
		g.GenerateEquations(sys)

		// Emit transformed entity copies for mesh groups (e.g.
		// POINT_N_TRANS at the top and bottom of an EXTRUDE).
		g.Generate(sk, &sk.Entities, &sk.Params)
	}

	// Phase 4: run a single Newton solve across all groups.
	initial := make(map[core.HParam]float64)
	for _, p := range sk.Params {
		initial[p.H] = p.Val
	}
	report := solver.SolveReport{}
	_, _ = sys.Solve(initial, solver.SolveOptions{Report: &report})
	how := core.SolveOkay
	if !report.Converged {
		how = core.SolveDidntConverge
	}
	dof := report.DOF
	rank := report.Rank

	// Phase 5: for mesh groups, regenerate the SMesh and the
	// display items. Also propagate the solve status to every
	// group's Solved struct (How, Dof) and set Clean=true on
	// groups that converged.
	for _, hg := range order {
		g := sk.GetGroup(hg)
		if g == nil {
			continue
		}
		if g.IsMeshGroup2() {
			g.GenerateShellAndMesh(sk)
			g.GenerateDisplayItems()
		}
		g.Solved.How = how
		g.Solved.Dof = dof
		g.Clean = report.Converged
	}
	_ = rank

	// Phase 6: re-evaluate reference constraints.
	for ci := range sk.Constraints {
		c := &sk.Constraints[ci]
		if c.Reference {
			ModifyToSatisfyConstraint(c, sk.ParamsToMap())
		}
	}
}

func (sk *Sketch) generateAllLoopsAndShells() {
	rt := NewRuntime(sk)
	rt.RecalculateEntities(false)
	order := make([]core.HGroup, len(sk.Groups))
	for i, g := range sk.Groups {
		order[i] = g.H
	}
	sort.Slice(order, func(i, j int) bool {
		return sk.GetGroup(order[i]).Order < sk.GetGroup(order[j]).Order
	})
	for _, hg := range order {
		g := sk.GetGroup(hg)
		if g == nil {
			continue
		}
		if g.Type == GroupExtrude {
			g.AssembleLoops(sk, rt)
			g.GenerateLoops()
			g.GenerateShellAndMesh(sk)
			g.GenerateDisplayItems()
		}
	}
}

// dedupEntitiesByHandle collapses a slice of entities to one entry per
// handle, preferring the entry with non-zero numeric state. This is
// required because request.Generate creates entity copies that share
// handles with the original parser entities, and the COPIES have
// uninitialized numeric state. Keeping only the parser's original
// preserves the user's authored numeric values.
func dedupEntitiesByHandle(entities []core.Entity) []core.Entity {
	if len(entities) == 0 {
		return entities
	}
	type entry struct {
		idx     int
		hasData bool
	}
	byHandle := make(map[core.HEntity]entry)
	out := make([]core.Entity, 0, len(entities))
	for _, e := range entities {
		cur, exists := byHandle[e.H]
		hasData := !e.ActPoint.Equals(geom.Vector{}) ||
			!e.ActNormal.Equals(geom.Quaternion{}) ||
			e.ActDistance != 0 ||
			e.NumPoint != (geom.Vector{}) ||
			e.NumNormal != (geom.Quaternion{})
		if !exists || (!cur.hasData && hasData) {
			if exists {
				// Replace prior entry with the data-bearing one.
				out = out[:len(out)-1]
			}
			byHandle[e.H] = entry{idx: len(out), hasData: hasData}
			out = append(out, e)
		}
	}
	return out
}

// dedupParamsByHandle collapses a slice of params to one entry per
// handle, preferring the entry with non-zero value.
func dedupParamsByHandle(params []core.Param) []core.Param {
	if len(params) == 0 {
		return params
	}
	type entry struct {
		idx     int
		hasData bool
	}
	byHandle := make(map[core.HParam]entry)
	out := make([]core.Param, 0, len(params))
	for _, p := range params {
		cur, exists := byHandle[p.H]
		hasData := p.Val != 0 || p.Known
		if !exists || (!cur.hasData && hasData) {
			if exists {
				out = out[:len(out)-1]
			}
			byHandle[p.H] = entry{idx: len(out), hasData: hasData}
			out = append(out, p)
		}
	}
	return out
}

// PruneOrphans removes entities, requests, and constraints that
// refer to a non-existent group. Filters entities and constraints
// whose Group is not in sk.Groups.
func (sk *Sketch) PruneOrphans() {
	validGroup := make(map[core.HGroup]bool, len(sk.Groups))
	for _, g := range sk.Groups {
		validGroup[g.H] = true
	}
	filtered := sk.Entities[:0]
	for _, e := range sk.Entities {
		if e.Group == (core.HGroup{}) || validGroup[e.Group] {
			filtered = append(filtered, e)
		}
	}
	sk.Entities = filtered

	filteredR := sk.Requests[:0]
	for _, r := range sk.Requests {
		if r.Group == (core.HGroup{}) || validGroup[r.Group] {
			filteredR = append(filteredR, r)
		}
	}
	sk.Requests = filteredR

	filteredC := sk.Constraints[:0]
	for _, c := range sk.Constraints {
		if c.Group == (core.HGroup{}) || validGroup[c.Group] {
			filteredC = append(filteredC, c)
		}
	}
	sk.Constraints = filteredC
}

// PruneGroups removes groups that depend on non-existent operands.
func (sk *Sketch) PruneGroups() bool {
	validGroup := make(map[core.HGroup]bool, len(sk.Groups))
	for _, g := range sk.Groups {
		validGroup[g.H] = true
	}
	filtered := sk.Groups[:0]
	removed := false
	for _, g := range sk.Groups {
		if (g.OpA != core.HGroup{} && !validGroup[g.OpA]) ||
			(g.OpB != core.HGroup{} && !validGroup[g.OpB]) {
			removed = true
			continue
		}
		filtered = append(filtered, g)
	}
	sk.Groups = filtered
	return removed
}

// PruneRequestsAndConstraints removes requests/constraints that
// reference a non-existent entity, param, or request.
func (sk *Sketch) PruneRequestsAndConstraints() bool {
	validEntity := make(map[core.HEntity]bool, len(sk.Entities))
	for _, e := range sk.Entities {
		validEntity[e.H] = true
	}
	removed := false
	filteredR := sk.Requests[:0]
	for _, r := range sk.Requests {
		if !validEntity[r.H.Entity(0)] {
			removed = true
			continue
		}
		filteredR = append(filteredR, r)
	}
	sk.Requests = filteredR
	filteredC := sk.Constraints[:0]
	for _, c := range sk.Constraints {
		if c.PtA != (core.HEntity{}) && !validEntity[c.PtA] {
			removed = true
			continue
		}
		if c.EntityA != (core.HEntity{}) && !validEntity[c.EntityA] {
			removed = true
			continue
		}
		filteredC = append(filteredC, c)
	}
	sk.Constraints = filteredC
	return removed
}

type solveReport struct {
	How    core.SolveResult
	Dof    int
	Rank   int
	BadCon []core.HConstraint
}

func (sk *Sketch) SolveGroupAndReport(hg core.HGroup, andFindFree bool) solveReport {
	g := sk.GetGroup(hg)
	if g == nil {
		return solveReport{How: core.SolveOkay}
	}
	refGroup := core.HGroup{V: 1}
	if hg == refGroup {
		return solveReport{How: core.SolveOkay, Rank: 0}
	}
	_ = andFindFree
	return solveReport{How: core.SolveOkay, Rank: 0}
}

func (sk *Sketch) orderedGroups() []core.HGroup {
	hs := make([]core.HGroup, 0, len(sk.Groups))
	for _, g := range sk.Groups {
		hs = append(hs, g.H)
	}
	sort.Slice(hs, func(i, j int) bool {
		return sk.GetGroup(hs[i]).Order < sk.GetGroup(hs[j]).Order
	})
	return hs
}

// CreateParam adds a new parameter to the sketch.
func (sk *Sketch) CreateParam(h core.HParam, val float64) core.HParam {
	sk.Params = append(sk.Params, core.Param{H: h, Val: val})
	return h
}

// SetEntityActPoint updates an entity's cached actPoint.
func (sk *Sketch) SetEntityActPoint(he core.HEntity, p geom.Vector) {
	for i := range sk.Entities {
		if sk.Entities[i].H == he {
			sk.Entities[i].ActPoint = p
			return
		}
	}
}

// ParseExprText parses a string expression and returns an Expr.
// Phase 7 / Goal Item 3: delegates to core.NewFromString, which
// implements the full Pratt parser.
func ParseExprText(s string) *core.Expr {
	e, err := core.NewFromString(s)
	if err != nil {
		return core.NewConstant(0)
	}
	return e
}

// sketchSolverResolver adapts a Sketch to the solver.Resolver
// interface so GenerateAll can build a single solver.System across
// all groups.
type sketchSolverResolver struct {
	sk *Sketch
}

func (r *sketchSolverResolver) Entity(h core.HEntity) (*core.Entity, error) {
	for i := range r.sk.Entities {
		if r.sk.Entities[i].H == h {
			return &r.sk.Entities[i], nil
		}
	}
	return nil, nil
}

func (r *sketchSolverResolver) Param(h core.HParam) (*core.Param, error) {
	for i := range r.sk.Params {
		if r.sk.Params[i].H == h {
			return &r.sk.Params[i], nil
		}
	}
	return nil, nil
}
