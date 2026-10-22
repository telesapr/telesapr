package sketch

import (
	"cmp"
	"context"
	"fmt"
	"math"
	"runtime/trace"
	"slices"
	"sort"
	"time"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/solver"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/ttf"
)

// GenerateAll regenerates the whole model: every group's entities and
// params, then the solve, loops and shells.
func (sk *Sketch) GenerateAll() {
	sk.generateAll(GenerateAll, false, passBuild)
}

// regenPass is what one pass over the groups of generateAll does. In
// display mode the chord tolerance follows from the model's size, which
// is known only once the groups are solved.
type regenPass int

const (
	// passBuild makes the solids. Called from outside, it first runs
	// passOne, or passSize, for the tolerance; after passSize it solves
	// only a sketch on a face whose face moved.
	passBuild regenPass = iota
	// passSize solves the groups and makes their loops, to find the
	// model's size; it makes no solids.
	passSize
	// passOne solves, makes the loops and makes the solids, at the
	// tolerance of the regeneration before. Where the size it finds
	// keeps that tolerance, it is the whole regeneration: in the window,
	// where a drag or an edit seldom moves the size across a step, one
	// pass over the groups in place of two.
	passOne
)

// generateAll is the regeneration that GenerateAll and its variants
// run.
func (sk *Sketch) generateAll(typ GenerateType, andFindFree bool, pass regenPass) {
	defer func() { sk.Generation++ }()
	name := "regenerate"
	switch pass {
	case passSize:
		name = "regenerate for size"
	case passOne:
		name = "regenerate in one pass"
	}
	defer trace.StartRegion(context.Background(), name).End()
	sk.orderGroups()

	// Parameters and stored expressions first, since they set the
	// values the groups are solved for and which groups are dirty.
	if pass == passBuild {
		sk.EvaluateParameters()
		sk.reloadChangedLinks()
	}

	var first, last int
	switch typ {
	case GenerateDirty, GenerateNew:
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
		} else if typ == GenerateNew {
			// A group put in before others: they are made again too,
			// as what they refer to may have moved.
			last = math.MaxInt
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
	switch {
	case sk.Exporting.IsSet():
		sk.tol = sk.Exporting.WithMesher(sk.Mesher)
	case pass == passBuild:
		if sk.tol.IsSet() {
			sk.generateAll(typ, andFindFree, passOne)
			if sk.hooks().Cancel() {
				return
			}
			was := sk.tol
			sk.tol = sk.sizeTol()
			if sk.tol == was && !sk.newAfterOther(typ, first) {
				return
			}
		} else {
			sk.generateAll(typ, andFindFree, passSize)
			sk.tol = sk.sizeTol()
		}
		// The groups before a new one keep their solids, made at the
		// tolerance of some regeneration before; when the model's
		// size, and so the tolerance, has changed since, they are made
		// again, so that the model is meshed as when it is read.
		if sk.newAfterOther(typ, first) {
			first = 0
		}
	}

	// Whether something is dragged, asked while the entities it names
	// are there.
	dragging := len(sk.hooks().DraggedParams()) > 0

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
	prevEntities := sk.Entities
	sk.Params = make([]core.Param, 0, len(prev))
	sk.Entities = make([]core.Entity, 0, len(sk.Entities))

	sk.markSuppressed()

	// The groups before the first that is made again are clean: nothing
	// they use has changed, since a change makes its group and every
	// group after it dirty. Regenerated, they would give the entities
	// and params they have; they keep them, and the loop starts after
	// them, so that a change late in a large model costs what follows it.
	start := 0
	if (typ == GenerateDirty || typ == GenerateNew) && first > 0 && first < len(sk.GroupOrder) {
		start = first
		sk.keepGroups(sk.GroupOrder[:first], prevEntities, prev)
	}

	rt := &Runtime{Sketch: sk}
	// The pass that finds the model's size solves the groups and makes
	// their loops, placing a sketch on a face from the solids of the
	// regeneration before; where this pass finds the face elsewhere,
	// that sketch and every group after it are solved and their loops
	// made again (face_workplane.go).
	resolve := false
	// The params a solve of passOne took as known that the build pass,
	// which solves nothing, would leave as generated: unknown.
	var unknown []core.HParam
	// The keys of what the groups before are made from, and of the solid
	// so far (solid_cache.go); a pass that makes no solids leaves them.
	// While something is dragged the values change at every step, so no
	// key would come again: the groups made are given none.
	keys := pass != passSize
	prefix, prevSolid := sk.globalKey(), sk.firstSolidKey()
	if dragging {
		prefix, prevSolid = solidKey{}, solidKey{}
	}
	if start > 0 && prefix.known() {
		pg := sk.Group(sk.GroupOrder[start-1])
		prefix, prevSolid = pg.prefixKey, sk.chainKey(pg)
	}
	for i, hg := range sk.GroupOrder {
		if i < start {
			continue
		}
		if sk.hooks().Cancel() {
			return
		}
		// A group suppressed whole, or using one that is, is left out
		// before anything could prune what it refers to.
		if g := sk.Group(hg); g.SuppressedBy.V != 0 {
			if keys {
				g.prefixKey = sk.prefixKey(prefix, g, nil)
			}
			sk.skipGroup(g, prev, i >= first && i <= last, keys, sk.solidKeyOf(g.prefixKey, prevSolid))
			if keys {
				prefix, prevSolid = g.prefixKey, sk.chainKey(g)
			}
			continue
		}
		// The group may depend on entities or other groups, to define its
		// workplane geometry or for its operands. Those must already exist
		// in a previous group, so check them before generating.
		if sk.PruneGroups(hg) {
			sk.generateAllPruned(prev, typ, andFindFree, pass)
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
		// The entity and param lists are kept sorted by handle; the
		// group iterates over them in that order.
		sk.sortByHandle()
		was := g.Predef
		g.Generate(sk, &sk.Entities, &sk.Params)
		sk.sortByHandle()
		resolve = resolve || (pass == passBuild && g.facePlaneMoved(was.Q, was.At))
		// The params this group's solve may change: those its requests,
		// its constraints and the group itself generate.
		groupParams := make([]core.HParam, 0, len(sk.Params)-len(prevGroupParams))
		for _, p := range sk.Params {
			if !prevGroupParams[p.H] {
				groupParams = append(groupParams, p.H)
			}
		}

		// The requests and constraints depend on stuff in this or the
		// previous group, so check them after generating.
		if sk.PruneRequestsAndConstraints(hg) {
			sk.generateAllPruned(prev, typ, andFindFree, pass)
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
			if keys {
				g.prefixKey = sk.prefixKey(prefix, g, groupParams)
				g.solidKey = sk.solidKeyOf(g.prefixKey, prevSolid)
			}

		case i >= first && i <= last:
			// The group falls inside the range, so really solve it,
			// and then regenerate the mesh based on the solved stuff.
			if pass == passOne {
				for _, h := range groupParams {
					if !sk.Param(h).Known {
						unknown = append(unknown, h)
					}
				}
			}
			if pass != passBuild {
				sk.SolveGroupAndReport(hg, groupParams, andFindFree)
				g.GenerateLoops(sk, rt)
			}
			switch {
			case pass == passSize:
				// Its solid waits for the pass that builds.
			case sk.DeferShells:
				if resolve && pass == passBuild {
					sk.SolveGroupAndReport(hg, groupParams, andFindFree)
					g.GenerateLoops(sk, rt)
				}
				// The solid as it was, to be made off the thread; its
				// key stays that of the solid it holds.
				g.prefixKey = sk.prefixKey(prefix, g, groupParams)
				g.ShellsStale = true
				g.Clean = true

			default:
				if resolve && pass == passBuild {
					sk.SolveGroupAndReport(hg, groupParams, andFindFree)
					g.GenerateLoops(sk, rt)
				}
				g.prefixKey = sk.prefixKey(prefix, g, groupParams)
				sk.makeSolid(g, sk.solidKeyOf(g.prefixKey, prevSolid))
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
			if keys {
				g.prefixKey = sk.prefixKey(prefix, g, groupParams)
			}
		}
		if keys {
			prefix, prevSolid = g.prefixKey, sk.chainKey(g)
		}
	}

	for _, h := range unknown {
		if p := sk.Param(h); p != nil {
			p.Known = false
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
	sk.hooks().Generated(sk.Deleted.Requests, sk.Deleted.Constraints, sk.Deleted.Groups)

	if sk.Deleted.Requests > 0 || sk.Deleted.Constraints > 0 || sk.Deleted.Groups > 0 {
		d := sk.Deleted
		sk.Deleted.Requests, sk.Deleted.Constraints = 0, 0
		sk.Deleted.NonTrivialConstraints, sk.Deleted.Groups = 0, 0

		// People get annoyed if I complain whenever they delete any request,
		// and I otherwise will, since those always come with pt-coincident
		// constraints.
		if d.Requests > 0 || d.NonTrivialConstraints > 0 || d.Groups > 0 {
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
			sk.Tell(fmt.Sprintf("Additional sketch elements were deleted, because they "+
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

// sortByHandle sorts the entities and params by handle, the order the
// lists are kept in.
func (sk *Sketch) sortByHandle() {
	sortTailStable(sk.Entities, func(e *core.Entity) uint32 { return e.H.V })
	sortTailStable(sk.Params, func(p *core.Param) uint32 { return p.H.V })
}

// sortTailStable sorts s stably by key, as slices.SortStableFunc does,
// in less time where s is sorted but for a tail: generateAll sorts the
// lists after each group, which appends its own entities and params to
// lists it left sorted, and sorting the whole of a list of entities
// each time costs a large model more than the rest of a regeneration.
// The tail is sorted alone and merged into the sorted head, from where
// its first goes; what comes before that stays where it is.
func sortTailStable[E any](s []E, key func(*E) uint32) {
	head := 1
	for head < len(s) && key(&s[head-1]) <= key(&s[head]) {
		head++
	}
	if head >= len(s) {
		return
	}
	// The tail sorted by its keys and places, which is the stable
	// order, into a copy kept aside: a comparison of the elements
	// themselves passed each by value to key, which made a copy of it
	// on the heap, two for every comparison of a list of entities.
	type keyed struct {
		k uint32
		i int
	}
	order := make([]keyed, len(s)-head)
	for i := range order {
		order[i] = keyed{key(&s[head+i]), i}
	}
	slices.SortFunc(order, func(a, b keyed) int {
		if c := cmp.Compare(a.k, b.k); c != 0 {
			return c
		}
		return cmp.Compare(a.i, b.i)
	})
	tail := make([]E, len(order))
	for n, o := range order {
		tail[n] = s[head+o.i]
	}
	// The first of the head that the tail's first goes before: past
	// those of an equal key, which stay first.
	first := order[0].k
	at := sort.Search(head, func(i int) bool { return key(&s[i]) > first })
	if at == head {
		copy(s[head:], tail)
		return
	}
	// Merged from the end, the head's moving part steps right over where
	// the tail was: on an equal key the tail's goes later.
	i, j := head-1, len(tail)-1
	for k := len(s) - 1; j >= 0; k-- {
		if i >= at && key(&s[i]) > key(&tail[j]) {
			s[k] = s[i]
			i--
		} else {
			s[k] = tail[j]
			j--
		}
	}
}

// generateAllPruned is GenerateAll's "pruned:" exit: restore the
// numerical guesses and try again.
func (sk *Sketch) generateAllPruned(prev []core.Param, typ GenerateType, andFindFree bool, pass regenPass) {
	sk.Params = prev
	sk.generateAll(typ, andFindFree, pass)
}

// sizeTol is the tolerance the model's size, as its entities span it,
// gives in display mode.
func (sk *Sketch) sizeTol() srf.Tolerance {
	box := sk.CalculateEntityBBox(true)
	size := box.MaxP.Minus(box.MinP)
	maxSize := geom.Max(size.X, geom.Max(size.Y, size.Z))
	return srf.Tol(tolStep(maxSize*sk.chordTolPct()/100.0), sk.SegmentLimit()).WithMesher(sk.Mesher)
}

// newAfterOther reports whether a regeneration of typ from the group at
// first is of a group put in after groups whose solids were made at
// another tolerance than the sketch's.
func (sk *Sketch) newAfterOther(typ GenerateType, first int) bool {
	return typ == GenerateNew && first > 0 && first <= len(sk.GroupOrder) && sk.solidsMadeAtOther(first)
}

// HGroupReferences is the handle value of the references group.
const HGroupReferences = core.HGroupReferences

// ForceReferences forces the values of the params that define the three
// reference coordinate systems.
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

// groupExists reports whether hg is a group.
func (sk *Sketch) groupExists(hg core.HGroup) bool {
	return sk.Group(hg) != nil
}

// entityExists reports whether he is an entity or no entity at all.
func (sk *Sketch) entityExists(he core.HEntity) bool {
	if he == core.NoEntity {
		return true
	}
	return sk.Entity(he) != nil
}

// groupsInOrder reports whether before comes before after.
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
// exist.
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

// PruneGroups removes group hg if its operand or predef entities don't
// exist (yet), and reports whether it did.
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
// constraints that refer to entities that don't exist.
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
// make a group inconsistent.
const timeoutRedundantConstr = 1000 * time.Millisecond

// WriteEquationsExceptFor appends the equations of group hg: those of
// its constraints except hc, of its entities, and of the group itself.
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
// params being dragged.
func (sk *Sketch) writeEqSystemForGroup(hg core.HGroup, groupParams []core.HParam) (*solver.System, *solver.Group) {
	g := sk.Group(hg)

	sys := sk.keptSystem(hg, groupParams)
	for _, h := range sk.hooks().DraggedParams() {
		if sys.Dragged == nil {
			sys.Dragged = make(map[core.HParam]bool)
		}
		sys.Dragged[h] = true
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

// solveHoldingBounds solves group hg by solve, whose result how
// reads, holding its inequalities at their bounds where the sketch
// would cross them (core.Constraint.Bound). It solves with every bound
// free, then holds the one crossed furthest and solves again, until
// none is crossed or a solve fails; one at a time, since a bound held
// may keep the sketch off another it crossed. Bounds the solution
// stops at are held at last, so that the freedom counted is the
// freedom left at the solution, unless holding them makes the group
// redundant: a length dimension with a maximum of the same value is
// no conflict.
func (sk *Sketch) solveHoldingBounds(hg core.HGroup, solve func(), how func() core.SolveResult) {
	var bounds []*core.Constraint
	for ci := range sk.Constraints {
		if c := &sk.Constraints[ci]; c.Group == hg && c.IsInequality() {
			c.Bound = core.BoundFree
			if !c.Reference {
				// A point on a segment starts at its foot on it, which
				// the solve then moves it from no more than it must.
				c.SeedParam(sk)
				bounds = append(bounds, c)
			}
		}
	}
	solve()
	for range bounds {
		if r := how(); r != core.SolveOkay && r != core.SolveRedundantOkay {
			return
		}
		var worst *core.Constraint
		worstBy, worstBound := 0.0, core.BoundFree
		for _, c := range bounds {
			if c.Bound != core.BoundFree {
				continue
			}
			if by, bound := c.Crossed(sk); core.IsCrossed(by) && (worst == nil || by > worstBy) {
				worst, worstBy, worstBound = c, by, bound
			}
		}
		if worst == nil {
			break
		}
		worst.Bound = worstBound
		solve()
	}
	before := how()
	if before != core.SolveOkay && before != core.SolveRedundantOkay {
		return
	}
	var touching []*core.Constraint
	for _, c := range bounds {
		if by, bound := c.Crossed(sk); c.Bound == core.BoundFree && core.IsAtBound(by) {
			c.Bound = bound
			touching = append(touching, c)
		}
	}
	if len(touching) == 0 {
		return
	}
	solve()
	if how() != before {
		for _, c := range touching {
			c.Bound = core.BoundFree
		}
		solve()
	}
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
// constrained, and its rank; a group that allows redundancy or doesn't
// count its freedom is okay with rank 0.
func (sk *Sketch) TestRankForGroup(hg core.HGroup) (core.SolveResult, int) {
	g := sk.Group(hg)
	// If we don't calculate dof or redundant is allowed, there is
	// no point to solve rank because this result is not meaningful
	if g.SuppressDofCalculation || g.AllowRedundant {
		return core.SolveOkay, 0
	}
	sys, sg := sk.writeEqSystemForGroup(hg, sk.groupParams(hg))
	how, rank, _, _ := sys.SolveRank(sg, false, false)
	return how, rank
}

// SolveGroupAndReport solves group hg for groupParams, the params its
// requests, constraints and the group itself generate, holding every
// other param fixed.
func (sk *Sketch) SolveGroupAndReport(hg core.HGroup, groupParams []core.HParam, andFindFree bool) {
	defer trace.StartRegion(context.Background(), "solve").End()
	defer sk.timed(hg, timingSolve)()
	g := sk.Group(hg)
	g.SketchDiagnostic = nil

	g.Solved.Remove = nil
	var how core.SolveResult
	var sg *solver.Group
	solve := func() {
		var sys *solver.System
		sys, sg = sk.writeEqSystemForGroup(hg, groupParams)
		err := catchPanic(func() {
			how, g.Solved.Dof, g.Solved.Remove = sys.Solve(sg, !g.AllowRedundant, andFindFree, !g.DofCheckOk)
		})
		if err != nil {
			// A malformed sketch is reported, not a crash.
			g.SketchDiagnostic = &SketchDiagnostic{AddConstraintErrors: []string{
				fmt.Sprintf("group %v: %v", hg, err)}}
			how = core.SolveDidntConverge
		}
		sk.keepSystem(hg, sys)
	}
	sk.seedAxisAngles(hg)
	sk.solveHoldingBounds(hg, solve, func() core.SolveResult { return how })
	g.Solved.Timeout = sg.Timeout
	if how == core.SolveOkay {
		g.DofCheckOk = true
	}
	g.Solved.How = how
	sk.recalculateGroupEntities(hg)
}

// recalculateGroupEntities refreshes the cached Act* values of group
// hg's entities, which the solver's numeric helpers and the renderer
// read.
func (sk *Sketch) recalculateGroupEntities(hg core.HGroup) {
	for i := range sk.Entities {
		e := &sk.Entities[i]
		if e.Group != hg {
			continue
		}
		_ = catchPanic(func() { e.CalculateNumerical(sk, false) })
	}
}

// catchPanic runs fn, returning a panic as an error.
func catchPanic(fn func()) (err error) {
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("%v", rec)
		}
	}()
	fn()
	return nil
}

// tolStep is the chord tolerance mm put on a fixed scale, the largest
// step of it no larger: a power of two times 4/8, 5/8, 6/8 or 7/8. The
// tolerance follows the model's size, and a size that changes a little,
// as a model does that grows by a part at a time, would change it each
// time, and with it every solid's mesh, made again. On the scale it
// changes only when the size passes a step, and the mesh is never
// coarser than asked for, and at most a quarter finer. The steps are
// exact in binary, and Frexp and Ldexp are exact, so that the tolerance
// is the same on every machine.
func tolStep(mm float64) float64 {
	if mm <= 0 || math.IsInf(mm, 0) || math.IsNaN(mm) {
		return mm
	}
	frac, exp := math.Frexp(mm) // mm = frac × 2^exp, frac in [0.5, 1)
	return math.Ldexp(math.Floor(frac*8)/8, exp)
}

// chordTolPct is the chord tolerance as a percentage of the model's
// size: Sketch.ChordTolPct, or the default of the ChordTolerancePct
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

// SegmentLimit is MaxSegments, or its default.
func (sk *Sketch) SegmentLimit() int {
	if sk.MaxSegments > 0 {
		return sk.MaxSegments
	}
	return 20
}

// FontList is Fonts, or ttf.DefaultFonts.
func (sk *Sketch) FontList() *ttf.FontList {
	if sk.Fonts == nil {
		return ttf.DefaultFonts
	}
	return sk.Fonts
}

// SettingsStore is Settings, or a store that keeps nothing.
func (sk *Sketch) SettingsStore() core.Settings {
	if sk.Settings == nil {
		return core.NoSettings{}
	}
	return sk.Settings
}

// Tol is the tolerance the last regeneration followed curves and
// surfaces at: the export one while Exporting is set, otherwise one
// from the model's size and ChordTolPct.
func (sk *Sketch) Tol() srf.Tolerance { return sk.tol }

// chordTolPct is the default of the ChordTolerancePct setting.
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

// GenerateType is which groups GenerateAll solves: all of them; from
// the first dirty or unsolved one to the active one; none, only
// regenerating the entities; up to the active one; or, for a group
// just added, from the first dirty one to the active one, and all of
// them if the chord tolerance changes.
type GenerateType int

const (
	GenerateAll GenerateType = iota
	GenerateDirty
	GenerateRegen
	GenerateUntilActive
	GenerateNew
)

// GenerateAdded regenerates after group hg was added: hg and the groups
// after it, which may refer to what it makes, and the sketches it makes
// a solid of, which say what is wrong with them as a profile only once
// something is made of them. The groups before are as they were, unless
// the model's size, and so the tolerance their solids are made at, has
// changed.
func (sk *Sketch) GenerateAdded(hg core.HGroup) {
	sk.ScheduleAdded(hg)
	sk.Refresh()
}

// ScheduleAdded is GenerateAdded when Refresh next runs: what was added
// can then be changed, as a script sets the combine mode of a solid it
// has just made, before anything is made of it.
func (sk *Sketch) ScheduleAdded(hg core.HGroup) {
	sk.orderGroups()
	from := map[core.HGroup]bool{hg: true}
	if g := sk.Group(hg); g != nil {
		for _, h := range g.profiles() {
			from[h] = true
		}
	}
	dirty := false
	for _, h := range sk.GroupOrder {
		dirty = dirty || from[h]
		if dirty {
			sk.Group(h).Clean = false
		}
	}
	sk.scheduledAdded = hg
	sk.ScheduleGenerateAll()
}

// AddedPending is the group ScheduleAdded scheduled whose regeneration
// has not run yet, or zero.
func (sk *Sketch) AddedPending() core.HGroup { return sk.scheduledAdded }

// profiles are the sketches the group makes a solid of.
func (g *Group) profiles() []core.HGroup {
	switch g.Type {
	case GroupExtrude, GroupLathe, GroupRevolve, GroupHelix, GroupSweep, GroupWrap:
		return []core.HGroup{g.OpA}
	case GroupLoft:
		return g.LoftProfiles()
	}
	return nil
}

// RemakeStaleMeshes makes every group's solid again when one was made
// at another chord tolerance than the sketch's: a regeneration of the
// groups after it changed the model's size, and so the tolerance, and
// left the solid as it was. It is called before what shows or measures
// the mesh, so that it is meshed as when the file is read. The entities
// and the solve are those of the regeneration before, and are kept: a
// solve again would move the last bits of the values. Nothing is made
// while something is dragged, when making every solid at each step
// would be too slow, nor while solids are being made off the thread,
// in full. It reports whether it made them.
func (sk *Sketch) RemakeStaleMeshes() bool {
	if len(sk.hooks().DraggedParams()) > 0 || sk.AnyShellsStale() || !sk.solidsMadeAtOther(len(sk.GroupOrder)) {
		return false
	}
	prev := sk.firstSolidKey()
	for _, h := range sk.GroupOrder {
		g := sk.Group(h)
		if g.H.V == HGroupReferences {
			g.solidKey = sk.solidKeyOf(g.prefixKey, prev)
		} else {
			sk.makeSolid(g, sk.solidKeyOf(g.prefixKey, prev))
		}
		prev = sk.chainKey(g)
	}
	return true
}

// keepGroups puts in the lists the entities and params of groups keep
// as the regeneration before left them, as a regeneration of them that
// solves nothing makes them again: the params taken as known, the
// entities' tags cleared.
func (sk *Sketch) keepGroups(keep []core.HGroup, entities []core.Entity, params []core.Param) {
	in := make(map[core.HGroup]bool, len(keep))
	for _, h := range keep {
		in[h] = true
	}
	for _, e := range entities {
		if in[e.Group] {
			e.Tag = 0
			sk.Entities = append(sk.Entities, e)
		}
	}
	// Whose each param is, as paramGroup tells, from a lookup made once.
	ofRequest := make(map[core.HRequest]core.HGroup, len(sk.Requests))
	for i := range sk.Requests {
		ofRequest[sk.Requests[i].H] = sk.Requests[i].Group
	}
	ofConstraint := make(map[core.HConstraint]core.HGroup, len(sk.Constraints))
	for i := range sk.Constraints {
		ofConstraint[sk.Constraints[i].H] = sk.Constraints[i].Group
	}
	for _, p := range params {
		var owner core.HGroup
		switch {
		case p.H.V&0x80000000 != 0:
			owner = core.HGroup{V: (p.H.V >> 16) & 0x3FFF}
		case p.H.V&0x40000000 != 0:
			owner = ofConstraint[core.HConstraint{V: p.H.V &^ 0x40000000}]
		default:
			owner = ofRequest[p.H.Request()]
		}
		if in[owner] {
			sk.Params = append(sk.Params, core.Param{H: p.H, Val: p.Val, Free: p.Free, Known: true})
		}
	}
}

// solidsMadeAtOther reports whether a group before the one at first in
// GroupOrder has a solid made at another tolerance than the sketch's.
func (sk *Sketch) solidsMadeAtOther(first int) bool {
	for _, h := range sk.GroupOrder[:first] {
		// The references, and a group left out, make no solid.
		if g := sk.Group(h); g.solidTol.IsSet() && g.solidTol != sk.tol {
			return true
		}
	}
	return false
}

// orderGroups puts the groups' handles in GroupOrder in their order.
func (sk *Sketch) orderGroups() {
	sk.GroupOrder = sk.GroupOrder[:0]
	for i := range sk.Groups {
		sk.GroupOrder = append(sk.GroupOrder, sk.Groups[i].H)
	}
	slices.SortStableFunc(sk.GroupOrder, func(a, b core.HGroup) int { return cmp.Compare(sk.Group(a).Order, sk.Group(b).Order) })
}

// GenerateAllTyped is GenerateAll with a GenerateType.
func (sk *Sketch) GenerateAllTyped(t GenerateType) {
	sk.generateAll(t, false, passBuild)
}

// GenerateAllFindFree regenerates like GenerateAllTyped and also works
// out which parameters are free, for Analyze > Show Degrees of Freedom.
func (sk *Sketch) GenerateAllFindFree(t GenerateType) {
	sk.generateAll(t, true, passBuild)
}

// ScheduleGenerateAll asks for a regeneration of the dirty groups when
// Refresh next runs.
func (sk *Sketch) ScheduleGenerateAll() {
	sk.scheduledGenerateAll = true
	sk.hooks().ScheduleGenerateAll()
}

// Refresh does the scheduled regeneration.
func (sk *Sketch) Refresh() {
	if sk.scheduledGenerateAll {
		typ := GenerateDirty
		if sk.scheduledAdded.V != 0 {
			typ = GenerateNew
		}
		sk.scheduledGenerateAll, sk.scheduledAdded = false, core.HGroup{}
		sk.generateAll(typ, false, passBuild)
	}
}

// MarkGroupDirty marks group hg, and unless onlyThis every group after
// it, as needing regeneration, and schedules it.
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

// MarkGroupDirtyByEntity marks the group of entity he dirty.
func (sk *Sketch) MarkGroupDirtyByEntity(he core.HEntity) {
	sk.MarkGroupDirty(sk.MustEntity(he).Group, false)
}
