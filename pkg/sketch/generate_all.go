package sketch

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/solver"
	"github.com/spacesolve/spacesolve/pkg/srf"
)

// GenerateAll regenerates the whole model: every group's entities
// and params, then the solve, loops and shells. It is C++
// SolveSpaceUI::GenerateAll(Generate::ALL) as it runs outside export
// mode.
func (sk *Sketch) GenerateAll() {
	sk.generateAll(GenerateAll, false, false)
}

// generateAll ports C++ SolveSpaceUI::GenerateAll (generate.cpp). The
// engine has no active group or dirty tracking, so DIRTY and
// UNTIL_ACTIVE regenerate every group, like ALL.
func (sk *Sketch) generateAll(typ GenerateType, andFindFree, genForBBox bool) {
	sk.GroupOrder = sk.GroupOrder[:0]
	for i := range sk.Groups {
		sk.GroupOrder = append(sk.GroupOrder, sk.Groups[i].H)
	}
	sort.SliceStable(sk.GroupOrder, func(i, j int) bool {
		return sk.GetGroup(sk.GroupOrder[i]).Order < sk.GetGroup(sk.GroupOrder[j]).Order
	})

	first, last := 0, math.MaxInt
	if typ == GenerateRegen {
		first, last = -1, -1
	}

	// If we're generating entities for display, first we need to find
	// the bounding box to turn relative chord tolerance to absolute.
	if !genForBBox {
		sk.generateAll(typ, andFindFree, true)
		box := sk.CalculateEntityBBox(true)
		size := box.MaxP.Minus(box.MinP)
		maxSize := math.Max(size.X, math.Max(size.Y, size.Z))
		srf.SetChordTolMm(maxSize * chordTolPct / 100.0)
	}

	// Remove any requests or constraints that refer to a nonexistent
	// group; can check those immediately, since we know what the list
	// of groups should be.
	sk.PruneOrphans()

	// Don't lose our numerical guesses when we regenerate.
	prev := sk.Params
	prevByHandle := make(map[core.HParam]*core.Param, len(prev))
	for i := range prev {
		prevByHandle[prev[i].H] = &prev[i]
	}
	sk.Params = make([]core.Param, 0, len(prev))
	sk.Entities = make([]core.Entity, 0, len(sk.Entities))

	rt := &Runtime{Sketch: sk}
	for i, hg := range sk.GroupOrder {
		// The group may depend on entities or other groups, to define its
		// workplane geometry or for its operands. Those must already exist
		// in a previous group, so check them before generating.
		if sk.PruneGroups(hg) {
			sk.generateAllPruned(prev, typ, andFindFree, genForBBox)
			return
		}
		g := sk.GetGroup(hg)

		firstParam := len(sk.Params)
		groupRequestIndex := 0
		for ri := range sk.Requests {
			r := &sk.Requests[ri]
			if r.Group != hg {
				continue
			}
			r.GroupRequestIndex = groupRequestIndex
			groupRequestIndex++

			entities := core.EntityList(sk.Entities)
			params := core.ParamList(sk.Params)
			r.Generate(&entities, &params)
			sk.Entities = []core.Entity(entities)
			sk.Params = []core.Param(params)
		}
		for ci := range sk.Constraints {
			c := &sk.Constraints[ci]
			if c.Group != hg {
				continue
			}
			GenerateConstraint(c, &sk.Params)
		}
		g.Generate(sk, &sk.Entities, &sk.Params)
		// The params this group's solve may change: C++
		// WriteEqSystemForGroup collects the same set by generating the
		// group's requests, constraints and group params again.
		groupParams := make([]core.HParam, 0, len(sk.Params)-firstParam)
		for pi := firstParam; pi < len(sk.Params); pi++ {
			groupParams = append(groupParams, sk.Params[pi].H)
		}

		// The requests and constraints depend on stuff in this or the
		// previous group, so check them after generating.
		if sk.PruneRequestsAndConstraints(hg) {
			sk.generateAllPruned(prev, typ, andFindFree, genForBBox)
			return
		}

		// Use the previous values for params that we've seen before, as
		// initial guesses for the solver.
		for pi := range sk.Params {
			newp := &sk.Params[pi]
			if newp.Known {
				continue
			}
			if prevp := prevByHandle[newp.H]; prevp != nil {
				newp.Val = prevp.Val
				newp.Free = prevp.Free
			}
		}
		sk.recalculateGroupEntities(hg)

		if hg.V == HGroupReferences {
			sk.ForceReferences()
			g.Solved.How = core.SolveOkay
			g.Clean = true
		} else if i >= first && i <= last {
			// The group falls inside the range, so really solve it,
			// and then regenerate the mesh based on the solved stuff.
			if genForBBox {
				sk.SolveGroupAndReport(hg, groupParams, andFindFree)
				g.GenerateLoops(sk, rt)
			} else {
				g.GenerateShellAndMesh(sk)
				g.Clean = true
			}
		} else {
			// The group falls outside the range, so just assume that
			// it's good wherever we left it. The mesh is unchanged,
			// and the parameters must be marked as known.
			for pi := range sk.Params {
				if prevByHandle[sk.Params[pi].H] != nil {
					sk.Params[pi].Known = true
				}
			}
		}
	}

	// And update any reference dimensions with their new values
	for ci := range sk.Constraints {
		c := &sk.Constraints[ci]
		if c.Reference {
			c.ModifyToSatisfy(sk)
		}
	}
}

// generateAllPruned is GenerateAll's "pruned:" exit: restore the
// numerical guesses and try again.
func (sk *Sketch) generateAllPruned(prev []core.Param, typ GenerateType, andFindFree, genForBBox bool) {
	sk.Params = prev
	sk.generateAll(typ, andFindFree, genForBBox)
}

// HGroupReferences is the handle value of the references group.
const HGroupReferences = core.HGroupReferences

// ForceReferences forces the values of the params that define the
// three reference coordinate systems (C++ SolveSpaceUI::ForceReferences).
func (sk *Sketch) ForceReferences() {
	quat := []struct {
		hr core.HRequest
		q  geom.Quaternion
	}{
		{HRequestReferenceXY, geom.Quaternion{W: 1, X: 0, Y: 0, Z: 0}},
		{HRequestReferenceYZ, geom.Quaternion{W: 0.5, X: 0.5, Y: 0.5, Z: 0.5}},
		{HRequestReferenceZX, geom.Quaternion{W: 0.5, X: -0.5, Y: -0.5, Z: -0.5}},
	}
	for _, rq := range quat {
		wrkpl := sk.MustGetEntity(rq.hr.Entity(0))
		// The origin for our coordinate system, always zero
		origin := sk.MustGetEntity(wrkpl.Point[0])
		origin.PointForceTo(sk, geom.Vector{})
		origin.Construction = true
		for k := 0; k < 3; k++ {
			sk.GetParam(origin.Param[k]).Known = true
		}
		// The quaternion that defines the rotation, from the table.
		normal := sk.MustGetEntity(wrkpl.Normal)
		normal.NormalForceTo(sk, rq.q)
		for k := 0; k < 4; k++ {
			sk.GetParam(normal.Param[k]).Known = true
		}
	}
}

// groupExists reports whether hg is a group (C++ GroupExists).
func (sk *Sketch) groupExists(hg core.HGroup) bool {
	return sk.GetGroup(hg) != nil
}

// entityExists reports whether he is an entity or no entity at all
// (C++ EntityExists).
func (sk *Sketch) entityExists(he core.HEntity) bool {
	if he == core.NoEntity {
		return true
	}
	return sk.GetEntity(he) != nil
}

// groupsInOrder reports whether before comes before after (C++
// GroupsInOrder).
func (sk *Sketch) groupsInOrder(before, after core.HGroup) bool {
	if before.V == 0 || after.V == 0 {
		return true
	}
	if !sk.groupExists(before) || !sk.groupExists(after) {
		return false
	}
	return sk.GetGroup(before).Order < sk.GetGroup(after).Order
}

// PruneOrphans removes requests and constraints whose group doesn't
// exist (C++ SolveSpaceUI::PruneOrphans).
func (sk *Sketch) PruneOrphans() bool {
	requests, constraints := len(sk.Requests), len(sk.Constraints)

	keptR := sk.Requests[:0]
	for _, r := range sk.Requests {
		if sk.groupExists(r.Group) {
			keptR = append(keptR, r)
		}
	}
	sk.Requests = keptR

	keptC := sk.Constraints[:0]
	for _, c := range sk.Constraints {
		if sk.groupExists(c.Group) {
			keptC = append(keptC, c)
		}
	}
	sk.Constraints = keptC

	return requests > len(sk.Requests) || constraints > len(sk.Constraints)
}

// PruneGroups removes group hg if its operand or predef entities
// don't exist (yet), and reports whether it did (C++
// SolveSpaceUI::PruneGroups).
func (sk *Sketch) PruneGroups(hg core.HGroup) bool {
	g := sk.GetGroup(hg)
	if sk.groupsInOrder(g.OpA, hg) &&
		sk.entityExists(g.Predef.Origin) &&
		sk.entityExists(g.Predef.EntityB) &&
		sk.entityExists(g.Predef.EntityC) {
		return false
	}
	for i := range sk.Groups {
		if sk.Groups[i].H == hg {
			sk.Groups = append(sk.Groups[:i], sk.Groups[i+1:]...)
			break
		}
	}
	return true
}

// PruneRequestsAndConstraints removes group hg's requests and
// constraints that refer to entities that don't exist (C++
// SolveSpaceUI::PruneRequestsAndConstraints).
func (sk *Sketch) PruneRequestsAndConstraints(hg core.HGroup) bool {
	entityRequestExists := func(he core.HEntity, checkEntity bool) bool {
		if he == core.NoEntity {
			return true
		}
		if he.IsFromRequest() {
			if sk.GetRequest(he.Request()) != nil {
				return true
			}
		} else if !checkEntity || sk.GetEntity(he) != nil {
			return true
		}
		return false
	}

	requests, constraints := len(sk.Requests), len(sk.Constraints)

	keptR := sk.Requests[:0]
	for _, r := range sk.Requests {
		if r.Group != hg || entityRequestExists(r.Workplane, false) {
			keptR = append(keptR, r)
		}
	}
	sk.Requests = keptR

	keptC := sk.Constraints[:0]
	for _, c := range sk.Constraints {
		if c.Group != hg ||
			(entityRequestExists(c.Workplane, true) &&
				entityRequestExists(c.PtA, true) &&
				entityRequestExists(c.PtB, true) &&
				entityRequestExists(c.EntityA, true) &&
				entityRequestExists(c.EntityB, true) &&
				entityRequestExists(c.EntityC, true) &&
				entityRequestExists(c.EntityD, true)) {
			keptC = append(keptC, c)
		}
	}
	sk.Constraints = keptC

	return requests > len(sk.Requests) || constraints > len(sk.Constraints)
}

// timeoutRedundantConstr bounds the search for the constraints that
// make a group inconsistent (C++ SS.timeoutRedundantConstr's default).
const timeoutRedundantConstr = 1000 * time.Millisecond

// WriteEquationsExceptFor appends the equations of group hg: those of
// its constraints except hc, of its entities, and of the group itself
// (C++ System::WriteEquationsExceptFor).
func (sk *Sketch) WriteEquationsExceptFor(hg core.HGroup, hc core.HConstraint, l *[]core.Equation) {
	g := sk.GetGroup(hg)
	// Generate all the equations from constraints in this group
	for ci := range sk.Constraints {
		c := &sk.Constraints[ci]
		if c.Group != hg {
			continue
		}
		if c.H == hc {
			continue
		}

		if c.HasLabel() && c.Type != core.ConstraintComment && g.AllDimsReference {
			// When all dimensions are reference, we adjust them to display
			// the correct value, and then don't generate any equations.
			c.ModifyToSatisfy(sk)
			continue
		}
		if g.RelaxConstraints && c.Type != core.ConstraintPointsCoincident {
			// When the constraints are relaxed, we keep only the point-
			// coincident constraints, and the constraints generated by
			// the entities and groups.
			continue
		}

		c.GenerateEquations(sk, l, false)
	}
	// And the equations from entities
	for ei := range sk.Entities {
		e := &sk.Entities[ei]
		if e.Group != hg {
			continue
		}
		e.GenerateEquations(sk, sk.Constraints, l)
	}
	// And from the groups themselves
	g.GenerateEquations(sk, l)
}

// SolveGroupAndReport solves group hg for groupParams, the params its
// requests, constraints and the group itself generate, holding every
// other param fixed (C++ SolveSpaceUI::SolveGroup, with
// WriteEqSystemForGroup).
func (sk *Sketch) SolveGroupAndReport(hg core.HGroup, groupParams []core.HParam, andFindFree bool) {
	g := sk.GetGroup(hg)
	g.SketchDiagnostic = nil

	// The solver works on copies of the group's params, with the
	// sketch's values as initial guesses.
	sys := &solver.System{SK: sk.GetParam}
	for _, h := range groupParams {
		p := *sk.GetParam(h)
		p.Known = false
		sys.AddParam(p)
	}

	var constraints []*core.Constraint
	for ci := range sk.Constraints {
		if sk.Constraints[ci].Group == hg {
			constraints = append(constraints, &sk.Constraints[ci])
		}
	}
	sort.SliceStable(constraints, func(i, j int) bool { return constraints[i].H.V < constraints[j].H.V })
	sg := &solver.Group{
		SuppressDofCalculation: g.SuppressDofCalculation,
		AllowRedundant:         g.AllowRedundant,
		FindToFixTimeout:       timeoutRedundantConstr,
		Constraints:            constraints,
		WriteEquationsExceptFor: func(hc core.HConstraint, l *[]core.Equation) {
			sk.WriteEquationsExceptFor(hg, hc, l)
		},
	}

	g.Solved.Remove = nil
	var how core.SolveResult
	err := catchPanic(func() {
		how = sys.Solve(sg, &g.Solved.Dof, &g.Solved.Remove,
			!g.AllowRedundant, andFindFree, !g.DofCheckOk)
	})
	if err != nil {
		// C++ would have asserted on the malformed sketch.
		g.SketchDiagnostic = &SketchDiagnostic{AddConstraintErrors: []string{
			fmt.Sprintf("group %v: %v", hg, err)}}
		how = core.SolveDidntConverge
	}
	g.Solved.Timeout = sg.Timeout
	if how == core.SolveOkay {
		g.DofCheckOk = true
	}
	g.Solved.How = how
	sk.recalculateGroupEntities(hg)
}

// recalculateGroupEntities refreshes the cached Act* values of group
// hg's entities, which the solver's numeric helpers and the renderer
// read; C++ evaluates those live.
func (sk *Sketch) recalculateGroupEntities(hg core.HGroup) {
	for i := range sk.Entities {
		e := &sk.Entities[i]
		if e.Group != hg {
			continue
		}
		_ = catchPanic(func() { e.CalculateNumerical(sk, false) })
	}
}

// catchPanic runs fn, returning a panic (C++ would have asserted) as
// an error.
func catchPanic(fn func()) (err error) {
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("%v", rec)
		}
	}()
	fn()
	return nil
}

// chordTolPct mirrors the default C++ "ChordTolerancePct" setting
// (solvespace.cpp:50).
const chordTolPct = 0.1

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

// GenerateType indicates which groups to regenerate.
//
// Mirrors the C++ Generate enum from solvespace.h:
//   - GenerateAll: regenerate all groups.
//   - GenerateDirty: regenerate only groups marked as needing regen.
//   - GenerateRegen: regenerate all groups (alias for GenerateAll).
//   - GenerateUntilActive: regenerate until the requested group is active.
//
// The Go engine currently regenerates everything on each GenerateAll
// call (no dirty-flag tracking). These flags are kept for parity with
// the C++ API and are passed to GenerateAllTyped; for now, all flags
// resolve to "regenerate everything".
type GenerateType int

const (
	GenerateAll GenerateType = iota
	GenerateDirty
	GenerateRegen
	GenerateUntilActive
)

// GenerateAllTyped is GenerateAll with a GenerateType. REGEN
// regenerates entities without solving or meshing; the others
// regenerate everything, since the engine has no dirty tracking or
// active group.
func (sk *Sketch) GenerateAllTyped(t GenerateType) {
	sk.generateAll(t, false, false)
}

// ScheduleGenerateAll is the parity-friendly wrapper for
// SolveSpaceUI::ScheduleGenerateAll. In the C++ codebase, this
// schedules a deferred regeneration via a UI timer; the engine
// has no UI layer, so this calls GenerateAll directly.
func (sk *Sketch) ScheduleGenerateAll() {
	sk.GenerateAll()
}
