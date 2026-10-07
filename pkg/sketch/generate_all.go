package sketch

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/solver"
	"github.com/telesapr/telesapr/pkg/srf"
)

// GenerateAll regenerates the whole model: every group's entities
// and params, then the solve, loops and shells. It is C++
// SolveSpaceUI::GenerateAll(Generate::ALL) as it runs outside export
// mode.
func (sk *Sketch) GenerateAll() {
	sk.generateAll(GenerateAll, false, false)
}

// generateAll ports C++ SolveSpaceUI::GenerateAll (generate.cpp).
func (sk *Sketch) generateAll(typ GenerateType, andFindFree, genForBBox bool) {
	defer func() { sk.Generation++ }()
	sk.GroupOrder = sk.GroupOrder[:0]
	for i := range sk.Groups {
		sk.GroupOrder = append(sk.GroupOrder, sk.Groups[i].H)
	}
	slices.SortStableFunc(sk.GroupOrder, func(a, b core.HGroup) int { return cmp.Compare(sk.Group(a).Order, sk.Group(b).Order) })

	// Parameters and stored expressions first, since they set the
	// values the groups are solved for and which groups are dirty.
	if !genForBBox {
		sk.EvaluateParameters()
	}

	var first, last int
	switch typ {
	case GenerateDirty:
		first, last = math.MaxInt, 0

		// Start from the first dirty group, and solve until the active group,
		// since all groups after the active group are hidden.
		active := sk.ActiveGroup()
		for i, hg := range sk.GroupOrder {
			g := sk.Group(hg)
			if !g.Clean || !g.IsSolvedOkay() {
				first = min(first, i)
			}
			if g.H == active {
				last = i
			}
		}
		if first == math.MaxInt || last == 0 {
			// All clean; so just regenerate the entities, and don't solve anything.
			first, last = -1, -1
		}

	case GenerateAll:
		first, last = 0, math.MaxInt

	case GenerateRegen:
		first, last = -1, -1

	case GenerateUntilActive:
		i := 0
		for ; i < len(sk.GroupOrder); i++ {
			if sk.GroupOrder[i] == sk.ActiveGroup() {
				break
			}
		}
		first, last = 0, i
	}

	// If we're generating entities for display, first we need to find
	// the bounding box to turn relative chord tolerance to absolute.
	if !srf.ExportMode() && !genForBBox {
		sk.generateAll(typ, andFindFree, true)
		box := sk.CalculateEntityBBox(true)
		size := box.MaxP.Minus(box.MinP)
		maxSize := geom.Max(size.X, geom.Max(size.Y, size.Z))
		srf.SetChordTolMm(maxSize * sk.chordTolPct() / 100.0)
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

	sk.markSuppressed()

	rt := &Runtime{Sketch: sk}
	for i, hg := range sk.GroupOrder {
		if sk.Cancel != nil && sk.Cancel() {
			return
		}
		// A group suppressed whole, or using one that is, is left out
		// before anything could prune what it refers to.
		if g := sk.Group(hg); g.SuppressedBy.V != 0 {
			sk.skipGroup(g, prev, i >= first && i <= last, genForBBox)
			continue
		}
		// The group may depend on entities or other groups, to define its
		// workplane geometry or for its operands. Those must already exist
		// in a previous group, so check them before generating.
		if sk.PruneGroups(hg) {
			sk.generateAllPruned(prev, typ, andFindFree, genForBBox)
			return
		}
		g := sk.Group(hg)

		prevGroupParams := make(map[core.HParam]bool, len(sk.Params))
		for _, p := range sk.Params {
			prevGroupParams[p.H] = true
		}
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
			r.Generate(sk, &entities, &params)
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
		// C++ keeps its entity and param lists sorted by handle; the
		// group iterates over them in that order.
		sk.sortByHandle()
		g.Generate(sk, &sk.Entities, &sk.Params)
		sk.sortByHandle()
		// The params this group's solve may change: C++
		// WriteEqSystemForGroup collects the same set by generating the
		// group's requests, constraints and group params again.
		groupParams := make([]core.HParam, 0, len(sk.Params)-len(prevGroupParams))
		for _, p := range sk.Params {
			if !prevGroupParams[p.H] {
				groupParams = append(groupParams, p.H)
			}
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

		switch {
		case hg.V == HGroupReferences:
			sk.ForceReferences()
			g.Solved.How = core.SolveOkay
			g.Clean = true

		case i >= first && i <= last:
			// The group falls inside the range, so really solve it,
			// and then regenerate the mesh based on the solved stuff.
			switch {
			case genForBBox:
				sk.SolveGroupAndReport(hg, groupParams, andFindFree)
				g.GenerateLoops(sk, rt)

			case sk.DeferShells:
				// The solid as it was, to be made off the thread.
				g.ShellsStale = true
				g.Clean = true

			default:
				g.GenerateShellAndMesh(sk)
				g.ShellsStale = false
				g.Clean = true
			}

		default:
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
		if c.Reference && sk.Group(c.Group).SuppressedBy.V == 0 {
			c.ModifyToSatisfy(sk)
		}
	}

	sk.publishBroken()
	sk.rememberNames()

	// The user interface's part: the traced point, the selection, and
	// what a deletion invalidates.
	if sk.OnGenerated != nil {
		sk.OnGenerated(sk.Deleted.Requests, sk.Deleted.Constraints, sk.Deleted.Groups)
	}

	if sk.Deleted.Requests > 0 || sk.Deleted.Constraints > 0 || sk.Deleted.Groups > 0 {
		d := sk.Deleted
		sk.Deleted.Requests, sk.Deleted.Constraints = 0, 0
		sk.Deleted.NonTrivialConstraints, sk.Deleted.Groups = 0, 0

		// People get annoyed if I complain whenever they delete any request,
		// and I otherwise will, since those always come with pt-coincident
		// constraints.
		if (d.Requests > 0 || d.NonTrivialConstraints > 0 || d.Groups > 0) && sk.Message != nil {
			plural := func(n int) string {
				if n == 1 {
					return ""
				}
				return "s"
			}
			undo := ""
			if sk.Undo.Cnt > 0 {
				undo = "\n\nChoose Edit -> Undo to undelete all elements."
			}
			sk.Message(fmt.Sprintf("Additional sketch elements were deleted, because they "+
				"depend on the element that was just deleted explicitly. "+
				"These include: \n"+
				"     %d request%s\n"+
				"     %d constraint%s\n"+
				"     %d group%s"+
				"%s",
				d.Requests, plural(d.Requests),
				d.Constraints, plural(d.Constraints),
				d.Groups, plural(d.Groups), undo))
		}
	}
}

// sortByHandle sorts the entities and params by handle, the order C++
// IDList keeps them in.
func (sk *Sketch) sortByHandle() {
	slices.SortStableFunc(sk.Entities, func(a, b core.Entity) int { return cmp.Compare(a.H.V, b.H.V) })
	slices.SortStableFunc(sk.Params, func(a, b core.Param) int { return cmp.Compare(a.H.V, b.H.V) })
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
		wrkpl := sk.MustEntity(rq.hr.Entity(0))
		// The origin for our coordinate system, always zero
		origin := sk.MustEntity(wrkpl.Point[0])
		origin.PointForceTo(sk, geom.Vector{})
		origin.Construction = true
		for k := range 3 {
			sk.Param(origin.Param[k]).Known = true
		}
		// The quaternion that defines the rotation, from the table.
		normal := sk.MustEntity(wrkpl.Normal)
		normal.NormalForceTo(sk, rq.q)
		for k := range 4 {
			sk.Param(normal.Param[k]).Known = true
		}
	}
}

// groupExists reports whether hg is a group (C++ GroupExists).
func (sk *Sketch) groupExists(hg core.HGroup) bool {
	return sk.Group(hg) != nil
}

// entityExists reports whether he is an entity or no entity at all
// (C++ EntityExists).
func (sk *Sketch) entityExists(he core.HEntity) bool {
	if he == core.NoEntity {
		return true
	}
	return sk.Entity(he) != nil
}

// groupsInOrder reports whether before comes before after (C++
// GroupsInOrder).
func (sk *Sketch) GroupsInOrder(before, after core.HGroup) bool {
	if before.V == 0 || after.V == 0 {
		return true
	}
	if !sk.groupExists(before) || !sk.groupExists(after) {
		return false
	}
	return sk.Group(before).Order < sk.Group(after).Order
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
	sk.Deleted.Requests += requests - len(sk.Requests)

	keptC := sk.Constraints[:0]
	for _, c := range sk.Constraints {
		if sk.groupExists(c.Group) {
			keptC = append(keptC, c)
		}
	}
	sk.Constraints = keptC
	sk.Deleted.Constraints += constraints - len(sk.Constraints)
	sk.Deleted.NonTrivialConstraints += constraints - len(sk.Constraints)

	return requests > len(sk.Requests) || constraints > len(sk.Constraints)
}

// PruneGroups removes group hg if its operand or predef entities
// don't exist (yet), and reports whether it did (C++
// SolveSpaceUI::PruneGroups).
func (sk *Sketch) PruneGroups(hg core.HGroup) bool {
	g := sk.Group(hg)
	// An edge of a chamfer or a sweep path that is gone is dropped
	// and listed; the group says what it lacks.
	if len(g.Edges) > 0 {
		kept := g.Edges[:0]
		// An edge's own radius follows it to its new place.
		at := make([]int, len(g.Edges))
		for i, he := range g.Edges {
			at[i] = -1
			if sk.entityExists(he) && he != core.NoEntity {
				at[i] = len(kept)
				kept = append(kept, he)
			} else {
				sk.broke(hg, "edge of "+g.DescriptionString(), sk.entityName(he))
			}
		}
		if dropped := len(g.Edges) - len(kept); dropped > 0 && len(g.EdgeRadii) > 0 {
			radii := g.EdgeRadii[:0]
			for _, er := range g.EdgeRadii {
				switch {
				case er.Edge >= len(at):
					er.Edge -= dropped
				case at[er.Edge] < 0:
					continue
				default:
					er.Edge = at[er.Edge]
				}
				radii = append(radii, er)
			}
			g.EdgeRadii = radii
		}
		g.Edges = kept
	}
	var missing string
	switch {
	case !sk.GroupsInOrder(g.OpA, hg):
		missing = sk.anyGroupName(g.OpA)
	case !sk.entityExists(g.Predef.Origin):
		missing = sk.entityName(g.Predef.Origin)
	case !sk.entityExists(g.Predef.EntityB):
		missing = sk.entityName(g.Predef.EntityB)
	case !sk.entityExists(g.Predef.EntityC):
		missing = sk.entityName(g.Predef.EntityC)
	default:
		return false
	}
	sk.broke(hg, g.DescriptionString(), missing)
	sk.Deleted.Groups++
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
			if sk.Request(he.Request()) != nil {
				return true
			}
		} else if !checkEntity || sk.Entity(he) != nil {
			return true
		}
		return false
	}

	requests, constraints := len(sk.Requests), len(sk.Constraints)

	keptR := sk.Requests[:0]
	for _, r := range sk.Requests {
		if r.Group != hg || entityRequestExists(r.Workplane, false) {
			keptR = append(keptR, r)
		} else {
			sk.broke(hg, r.DescriptionString(), sk.entityName(r.Workplane))
		}
	}
	sk.Requests = keptR
	sk.Deleted.Requests += requests - len(sk.Requests)

	keptC := sk.Constraints[:0]
	for _, c := range sk.Constraints {
		if c.Group != hg {
			keptC = append(keptC, c)
			continue
		}
		missing := core.NoEntity
		for _, he := range [...]core.HEntity{c.Workplane, c.PtA, c.PtB,
			c.EntityA, c.EntityB, c.EntityC, c.EntityD} {
			if !entityRequestExists(he, true) {
				missing = he
				break
			}
		}
		if missing == core.NoEntity {
			keptC = append(keptC, c)
			continue
		}
		sk.broke(hg, c.DescriptionString(), sk.entityName(missing))

		if c.Type != core.ConstraintPointsCoincident &&
			c.Type != core.ConstraintHorizontal &&
			c.Type != core.ConstraintVertical {
			sk.Deleted.NonTrivialConstraints++
		}
	}
	sk.Constraints = keptC
	sk.Deleted.Constraints += constraints - len(sk.Constraints)

	return requests > len(sk.Requests) || constraints > len(sk.Constraints)
}

// timeoutRedundantConstr bounds the search for the constraints that
// make a group inconsistent (C++ SS.timeoutRedundantConstr's default).
const timeoutRedundantConstr = 1000 * time.Millisecond

// WriteEquationsExceptFor appends the equations of group hg: those of
// its constraints except hc, of its entities, and of the group itself
// (C++ System::WriteEquationsExceptFor).
func (sk *Sketch) WriteEquationsExceptFor(hg core.HGroup, hc core.HConstraint, l *[]core.Equation) {
	g := sk.Group(hg)
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

// writeEqSystemForGroup is the system that solves group hg for
// groupParams, the params its requests, constraints and the group
// itself generate, holding every other param fixed, and marks the
// params being dragged (C++ SolveSpaceUI::WriteEqSystemForGroup).
func (sk *Sketch) writeEqSystemForGroup(hg core.HGroup, groupParams []core.HParam) (*solver.System, *solver.Group) {
	g := sk.Group(hg)

	// The solver works on copies of the group's params, with the
	// sketch's values as initial guesses.
	sys := &solver.System{SK: sk.Param}
	for _, h := range groupParams {
		p := *sk.Param(h)
		p.Known = false
		sys.AddParam(p)
	}
	if sk.DraggedParams != nil {
		for _, h := range sk.DraggedParams() {
			if sys.Dragged == nil {
				sys.Dragged = make(map[core.HParam]bool)
			}
			sys.Dragged[h] = true
		}
	}

	var constraints []*core.Constraint
	for ci := range sk.Constraints {
		if sk.Constraints[ci].Group == hg {
			constraints = append(constraints, &sk.Constraints[ci])
		}
	}
	slices.SortStableFunc(constraints, func(a, b *core.Constraint) int { return cmp.Compare(a.H.V, b.H.V) })
	sg := &solver.Group{
		SuppressDofCalculation: g.SuppressDofCalculation,
		AllowRedundant:         g.AllowRedundant,
		FindToFixTimeout:       timeoutRedundantConstr,
		Constraints:            constraints,
		WriteEquationsExceptFor: func(hc core.HConstraint, l *[]core.Equation) {
			sk.WriteEquationsExceptFor(hg, hc, l)
		},
	}
	return sys, sg
}

// groupParams lists the params group hg's requests, constraints and
// the group itself generate, in handle order: what
// WriteEqSystemForGroup generates into the system.
func (sk *Sketch) groupParams(hg core.HGroup) []core.HParam {
	var entities core.EntityList
	var params core.ParamList
	for ri := range sk.Requests {
		r := &sk.Requests[ri]
		if r.Group == hg {
			r.Generate(sk, &entities, &params)
		}
	}
	ps := []core.Param(params)
	for ci := range sk.Constraints {
		c := &sk.Constraints[ci]
		if c.Group == hg {
			GenerateConstraint(c, &ps)
		}
	}
	es := []core.Entity(entities)
	sk.Group(hg).Generate(sk, &es, &ps)

	hs := make([]core.HParam, 0, len(ps))
	for _, p := range ps {
		if sk.Param(p.H) != nil {
			hs = append(hs, p.H)
		}
	}
	slices.SortFunc(hs, func(a, b core.HParam) int { return cmp.Compare(a.V, b.V) })
	return hs
}

// TestRankForGroup reports whether group hg would be redundantly
// constrained, and its rank (C++ SolveSpaceUI::TestRankForGroup).
func (sk *Sketch) TestRankForGroup(hg core.HGroup, rank *int) core.SolveResult {
	g := sk.Group(hg)
	// If we don't calculate dof or redundant is allowed, there is
	// no point to solve rank because this result is not meaningful
	if g.SuppressDofCalculation || g.AllowRedundant {
		return core.SolveOkay
	}
	sys, sg := sk.writeEqSystemForGroup(hg, sk.groupParams(hg))
	return sys.SolveRank(sg, rank, nil, nil, false, false)
}

// SolveGroupAndReport solves group hg for groupParams, the params its
// requests, constraints and the group itself generate, holding every
// other param fixed (C++ SolveSpaceUI::SolveGroup, with
// WriteEqSystemForGroup).
func (sk *Sketch) SolveGroupAndReport(hg core.HGroup, groupParams []core.HParam, andFindFree bool) {
	g := sk.Group(hg)
	g.SketchDiagnostic = nil

	sys, sg := sk.writeEqSystemForGroup(hg, groupParams)
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

// chordTolPct is the chord tolerance as a percentage of the model's
// size: Sketch.ChordTolPct, or the default C++ "ChordTolerancePct"
// setting.
func (sk *Sketch) chordTolPct() float64 {
	if sk.ChordTolPct > 0 {
		return sk.ChordTolPct
	}
	return chordTolPct
}

// ChordTolPercent is the chord tolerance in effect, as a percentage of
// the model's size.
func (sk *Sketch) ChordTolPercent() float64 { return sk.chordTolPct() }

// chordTolPct mirrors the default C++ "ChordTolerancePct" setting
// (solvespace.cpp:50).
const chordTolPct = 0.1

func (sk *Sketch) orderedGroups() []core.HGroup {
	hs := make([]core.HGroup, 0, len(sk.Groups))
	for _, g := range sk.Groups {
		hs = append(hs, g.H)
	}
	slices.SortFunc(hs, func(a, b core.HGroup) int { return cmp.Compare(sk.Group(a).Order, sk.Group(b).Order) })
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

// GenerateType is which groups GenerateAll solves (C++
// SolveSpaceUI::Generate): all of them; from the first dirty or
// unsolved one to the active one; none, only regenerating the
// entities; or up to the active one.
type GenerateType int

const (
	GenerateAll GenerateType = iota
	GenerateDirty
	GenerateRegen
	GenerateUntilActive
)

// GenerateAllTyped is GenerateAll with a GenerateType.
func (sk *Sketch) GenerateAllTyped(t GenerateType) {
	sk.generateAll(t, false, false)
}

// GenerateAllFindFree regenerates like GenerateAllTyped and also works
// out which parameters are free, for Analyze > Show Degrees of Freedom
// (C++ GenerateAll with andFindFree).
func (sk *Sketch) GenerateAllFindFree(t GenerateType) {
	sk.generateAll(t, true, false)
}

// ScheduleGenerateAll asks for a regeneration of the dirty groups when
// Refresh next runs (C++ SolveSpaceUI::ScheduleGenerateAll, which
// Refresh serves after the pending events).
func (sk *Sketch) ScheduleGenerateAll() {
	sk.scheduledGenerateAll = true
	if sk.OnScheduleGenerateAll != nil {
		sk.OnScheduleGenerateAll()
	}
}

// Refresh does the scheduled regeneration (C++
// SolveSpaceUI::Refresh; showing the text window is the caller's).
func (sk *Sketch) Refresh() {
	if sk.scheduledGenerateAll {
		sk.scheduledGenerateAll = false
		sk.generateAll(GenerateDirty, false, false)
	}
}

// MarkGroupDirty marks group hg, and unless onlyThis every group after
// it, as needing regeneration, and schedules it (C++
// SolveSpaceUI::MarkGroupDirty).
func (sk *Sketch) MarkGroupDirty(hg core.HGroup, onlyThis bool) {
	goOn := false
	for _, gh := range sk.GroupOrder {
		g := sk.Group(gh)
		if g.H == hg {
			goOn = true
		}
		if goOn {
			g.Clean = false
			if onlyThis {
				break
			}
		}
	}
	sk.Unsaved = true
	sk.ScheduleGenerateAll()
}

// MarkGroupDirtyByEntity marks the group of entity he dirty (C++
// SolveSpaceUI::MarkGroupDirtyByEntity).
func (sk *Sketch) MarkGroupDirtyByEntity(he core.HEntity) {
	sk.MarkGroupDirty(sk.MustEntity(he).Group, false)
}
