// Package solver ports solvespace/src/system.cpp: it linearizes the
// constraint equations of one group and solves them by a modified
// Newton's method, and detects non-convergence or inconsistency.
package solver

import (
	"cmp"
	"math"
	"slices"
	"time"

	"github.com/telesapr/telesapr/pkg/core"
)

// ConvergeTolerance is how close to zero every equation must be. It
// must always be much less than LENGTH_EPS, and in practice should be
// much less (C++ System::CONVERGE_TOLERANCE).
const ConvergeTolerance = core.LengthEps / 1e2

// MaxUnknowns bounds the size of the Jacobian (C++ System::MAX_UNKNOWNS).
const MaxUnknowns = 2048

// Tags the solver puts on params and equations (C++ System enum).
const (
	varSubstituted = 10000
	varDofTest     = 10001
	eqSubstituted  = 20000
)

// Equation is one equation of the system, expr = 0.
type Equation struct {
	H   core.HEquation
	E   *core.Expr
	Tag int
}

// Param is one unknown of the system, a copy of the sketch's param.
type Param struct {
	core.Param
	Tag int
}

// Group is what System::Solve reads from the group being solved.
type Group struct {
	SuppressDofCalculation bool
	AllowRedundant         bool
	// FindToFixTimeout bounds FindWhichToRemoveToFixJacobian; Timeout
	// reports that it ran out.
	FindToFixTimeout time.Duration
	Timeout          bool
	// Constraints are the group's constraints, in handle order.
	Constraints []*core.Constraint
	// WriteEquationsExceptFor appends the equations of the group's
	// constraints except hc, of its entities and of the group itself
	// (C++ System::WriteEquationsExceptFor).
	WriteEquationsExceptFor func(hc core.HConstraint, l *[]core.Equation)
}

// System is the equations and unknowns of one group (C++ System).
type System struct {
	// Param are the unknowns, in handle order.
	Param []*Param
	// Eq are the equations, in handle order.
	Eq []Equation
	// Dragged are the params the user is dragging, which the least
	// squares step prefers to change.
	Dragged map[core.HParam]bool
	// SK looks up the sketch's params: those that aren't unknowns are
	// read as constants, and Solve writes the solution back to it.
	SK func(core.HParam) *core.Param

	paramIndex map[core.HParam]*Param

	mat struct {
		m, n  int
		param []core.HParam
		eq    []*Equation
		// A is the Jacobian: A[i][j] is d eq[i] / d param[j], nil
		// where that's zero.
		A    [][]*core.Expr
		B    []*core.Expr
		Anum [][]float64
		Bnum []float64
		X    []float64
	}
}

// AddParam adds an unknown, with the sketch param's value as its
// initial guess.
func (s *System) AddParam(p core.Param) {
	s.Param = append(s.Param, &Param{Param: p})
	slices.SortStableFunc(s.Param, func(a, b *Param) int { return cmp.Compare(a.H.V, b.H.V) })
	s.paramIndex = nil
}

func (s *System) findParam(h core.HParam) *Param {
	if s.paramIndex == nil {
		s.paramIndex = make(map[core.HParam]*Param, len(s.Param))
		for _, p := range s.Param {
			s.paramIndex[p.H] = p
		}
	}
	return s.paramIndex[h]
}

func (s *System) isDragged(h core.HParam) bool {
	return s.Dragged[h]
}

// setEquations replaces the equations, sorted by handle as the C++
// IdList keeps them.
func (s *System) setEquations(l []core.Equation) {
	s.Eq = s.Eq[:0]
	for _, e := range l {
		s.Eq = append(s.Eq, Equation{H: e.H, E: e.Expr})
	}
	slices.SortStableFunc(s.Eq, func(a, b Equation) int { return cmp.Compare(a.H.V, b.H.V) })
}

func (s *System) clearParamTags() {
	for _, p := range s.Param {
		p.Tag = 0
	}
}

func (s *System) clearEqTags() {
	for i := range s.Eq {
		s.Eq[i].Tag = 0
	}
}

// deepCopyWithParamsAsPointers copies e, binding each param to the
// system's copy if it is an unknown, else to the sketch's; a known
// param becomes a constant. Constant subtrees are folded (C++
// Expr::DeepCopyWithParamsAsPointers with foldConstants).
func (s *System) deepCopyWithParamsAsPointers(e *core.Expr) *core.Expr {
	if e.Op == core.OpParam {
		var p *core.Param
		if sp := s.findParam(e.Param); sp != nil {
			p = &sp.Param
		} else {
			p = s.SK(e.Param)
			if p == nil {
				panic("param not found")
			}
		}
		if p.Known {
			return core.NewConstant(p.Val)
		}
		return core.NewParamPtr(p)
	}
	n := *e
	hasConstants := false
	if e.Left != nil {
		n.Left = s.deepCopyWithParamsAsPointers(e.Left)
		hasConstants = n.Left.Op == core.OpConstant
		if e.Right != nil {
			n.Right = s.deepCopyWithParamsAsPointers(e.Right)
			hasConstants = hasConstants || n.Right.Op == core.OpConstant
		}
	}
	if hasConstants {
		// The children are folded already; this folds just this node.
		return n.FoldNode()
	}
	return &n
}

// paramsUsed lists the params e refers to (C++ Expr::ParamsUsedList).
func paramsUsed(e *core.Expr, set map[core.HParam]bool, order *[]core.HParam) {
	if h, ok := e.ParamHandle(); ok {
		if !set[h] {
			set[h] = true
			*order = append(*order, h)
		}
		return
	}
	if e.Left != nil {
		paramsUsed(e.Left, set, order)
	}
	if e.Right != nil {
		paramsUsed(e.Right, set, order)
	}
}

// WriteJacobian writes the equations and params tagged tag into the
// matrix, symbolically (C++ System::WriteJacobian).
func (s *System) WriteJacobian(tag int) bool {
	s.mat.param = s.mat.param[:0]
	s.mat.eq = s.mat.eq[:0]
	s.mat.A = nil
	s.mat.B = nil

	for i := range s.Eq {
		if s.Eq[i].Tag != tag {
			continue
		}
		s.mat.eq = append(s.mat.eq, &s.Eq[i])
	}
	if len(s.mat.eq) >= MaxUnknowns {
		return false
	}
	s.mat.m = len(s.mat.eq)

	paramToIndex := make(map[core.HParam]int)
	for _, p := range s.Param {
		if p.Tag != tag {
			continue
		}
		paramToIndex[p.H] = len(s.mat.param)
		s.mat.param = append(s.mat.param, p.H)
	}
	s.mat.n = len(s.mat.param)

	s.mat.A = make([][]*core.Expr, s.mat.m)
	for i, e := range s.mat.eq {
		// Deep-copy and simplify (fold) the current equation.
		f := s.deepCopyWithParamsAsPointers(e.E)

		var used []core.HParam
		paramsUsed(f, map[core.HParam]bool{}, &used)

		s.mat.A[i] = make([]*core.Expr, s.mat.n)
		for _, p := range used {
			j, ok := paramToIndex[p]
			if !ok {
				continue
			}
			// compute partial derivative of f
			pd := f.Derivative(p).FoldConstants()
			if pd.IsZeroConst() {
				continue
			}
			s.mat.A[i][j] = pd
		}
		s.mat.B = append(s.mat.B, f)
	}
	return true
}

// EvalJacobian evaluates the symbolic Jacobian (C++ System::EvalJacobian).
func (s *System) EvalJacobian() {
	s.mat.Anum = make([][]float64, s.mat.m)
	for i := range s.mat.m {
		s.mat.Anum[i] = make([]float64, s.mat.n)
		for j, pd := range s.mat.A[i] {
			if pd != nil {
				s.mat.Anum[i][j] = pd.Eval(nil, nil)
			}
		}
	}
}

// substitute renames the params of e that subMap maps (C++
// Expr::Substitute).
func substitute(e *core.Expr, subMap map[core.HParam]*Param) {
	if e.Op == core.OpParam {
		if by, ok := subMap[e.Param]; ok {
			e.Param = by.H
		}
		return
	}
	if e.Left != nil {
		substitute(e.Left, subMap)
		if e.Right != nil {
			substitute(e.Right, subMap)
		}
	}
}

// SolveBySubstitution eliminates the equations that just say two
// unknowns are equal, a - b = 0, by substituting one for the other; it
// returns what each eliminated param was substituted by (C++
// System::SolveBySubstitution).
func (s *System) SolveBySubstitution() map[core.HParam]*Param {
	// Contains pointers to last substitutions in a substitution chain
	var subVec []*Param
	// Maps a parameter to the index of its last substitution in subVec
	leaves := map[core.HParam]int{}
	var leafOrder []core.HParam
	setLeaf := func(h core.HParam, idx int) {
		if _, ok := leaves[h]; !ok {
			leafOrder = append(leafOrder, h)
		}
		leaves[h] = idx
	}
	// Tracks how many slots in subVec contain a specific last
	// substitution (slots that once contained a specific substitution
	// are updated to point to another over the run)
	slotTrack := map[core.HParam][]int{}

	for i := range s.Eq {
		teq := &s.Eq[i]
		tex := teq.E

		// If we have (a - b) = 0 where both a and b are parameters, then
		// a = b and we can substitute
		if tex.Op != core.OpMinus || tex.Left.Op != core.OpParam || tex.Right.Op != core.OpParam {
			continue
		}
		sub := s.findParam(tex.Left.Param)
		by := s.findParam(tex.Right.Param)
		if sub == nil || by == nil {
			// Don't substitute unless they're both solver params;
			// otherwise it's an equation that can be solved immediately,
			// or an error to flag later.
			continue
		}

		if sub.H == by.H {
			teq.Tag = eqSubstituted
			continue
		}

		// Take the last substitution of parameter a
		subIdx := 0
		if idx, ok := leaves[sub.H]; ok {
			subIdx = idx
			sub = subVec[idx-1]
		}

		// Take the last substitution of parameter b
		byIdx := 0
		if idx, ok := leaves[by.H]; ok {
			byIdx = idx
			by = subVec[idx-1]
		}

		// If the last substituton of a is a dragged param, keep it
		// and substitute the other param
		if s.isDragged(sub.H) {
			sub, by = by, sub
			subIdx, byIdx = byIdx, subIdx
		}

		if subIdx == 0 {
			if byIdx == 0 {
				// Neither sub nor by are in the map, so add them and
				// set the target index
				subVec = append(subVec, by)
				setLeaf(by.H, len(subVec))
				setLeaf(sub.H, len(subVec))
			} else {
				// sub isn't in the map, but by is, so just add sub to
				// the map with by as the target
				setLeaf(sub.H, byIdx)
			}
		} else {
			// sub already exists in the map, so just update any slots
			// that point to it as the last substitution to point to by
			// instead
			if slots, ok := slotTrack[sub.H]; !ok {
				// There's only this one slot, so just replace it with by
				subVec[subIdx-1] = by

				// If by was already in the map, that means we now have
				// an additional slot where it resides, so add it to the
				// slot tracker
				if byIdx != 0 {
					bySlots := slotTrack[by.H]
					if len(bySlots) == 0 {
						bySlots = append(bySlots, byIdx)
					}
					slotTrack[by.H] = append(bySlots, subIdx)
				}
			} else {
				// We have more than one slot pointing to sub, so update
				// all of the slots to point to by
				for _, k := range slots {
					subVec[k-1] = by
				}

				// No more slots are pointing to sub, so extract the slot
				// list and erase sub from the tracker
				delete(slotTrack, sub.H)

				bySlots := slotTrack[by.H]
				if len(bySlots) == 0 {
					bySlots = slots
					if byIdx != 0 {
						bySlots = append(bySlots, byIdx)
					}
				} else {
					bySlots = append(bySlots, slots...)
				}
				slotTrack[by.H] = bySlots
			}

			setLeaf(by.H, subIdx)
		}

		sub.Tag = varSubstituted
		teq.Tag = eqSubstituted
	}

	subMap := map[core.HParam]*Param{}
	for _, h := range leafOrder {
		by := subVec[leaves[h]-1]
		if h != by.H {
			subMap[h] = by
		}
	}

	// Substitute all the equations
	for i := range s.Eq {
		substitute(s.Eq[i].E, subMap)
	}

	return subMap
}

// CalculateRank returns the rank of the evaluated Jacobian (C++
// System::CalculateRank).
func (s *System) CalculateRank() int {
	if s.mat.n == 0 || s.mat.m == 0 {
		return 0
	}
	return factorSparseQR(spFromRows(s.mat.Anum, s.mat.m, s.mat.n)).rank
}

// TestRank evaluates the Jacobian and reports whether it has full row
// rank, along with the degrees of freedom and the rank (C++
// System::TestRank).
func (s *System) TestRank() (ok bool, dof, rank int) {
	s.EvalJacobian()
	jacobianRank := s.CalculateRank()
	// We are calculating dof based on real rank, not mat.m.
	// Using this approach we can calculate real dof even when redundant is allowed.
	return jacobianRank == s.mat.m, s.mat.n - jacobianRank, jacobianRank
}

// SolveLeastSquares computes the minimum-norm Newton step X with
// A X = B (C++ System::SolveLeastSquares).
func (s *System) SolveLeastSquares() bool {
	m, n := s.mat.m, s.mat.n
	// Scale the columns; this scale weights the parameters for the least
	// squares solve, so that we can encourage the solver to make bigger
	// changes in some parameters, and smaller in others.
	scale := make([]float64, n)
	for c := range n {
		scale[c] = 1
		if s.isDragged(s.mat.param[c]) {
			// It's least squares, so this parameter doesn't need to be all
			// that big to get a large effect.
			scale[c] = 1 / 20.0
		}
	}
	a := spFromRows(s.mat.Anum, m, n)
	for c := range n {
		for k := range a.val[c] {
			a.val[c][k] *= scale[c]
		}
	}

	s.mat.X = make([]float64, n)
	if m == 0 || n == 0 {
		return true
	}

	s.mat.X = leastSquares(a, s.mat.Bnum)
	for c := range n {
		s.mat.X[c] *= scale[c]
	}
	return true
}

// leastSquares returns the minimum-norm solution X of A X = B, as C++
// System::SolveLeastSquares finds it once A is scaled.
func leastSquares(a *spMat, b []float64) []float64 {
	m, n := a.rows, a.cols
	var z []float64
	if m <= n {
		// We want the minimum norm solution of the underdetermined system
		// A*x = B, which is x = A'*(A*A')^-1*B. Factor A' rather than
		// forming A*A', which would square the condition number: as
		// A'*P = Q*R, A*A' = P*R'*R*P', so
		//     z = (A*A')^-1*B = P*R^-1*R'^-1*P'*B,
		// two triangular solves, with the rank decision taken on R.
		f := factorSparseQR(a.transpose())
		v := make([]float64, m)
		if f.rank > 0 {
			for k := range f.rank {
				v[k] = b[f.perm[k]]
			}
			f.solveUpperTransposed(v)
			f.solveUpper(v)
		}
		z = make([]float64, m)
		for k := range m {
			z[f.perm[k]] = v[k]
		}
	} else {
		// More equations than unknowns; A' is wide, which Eigen's sparse
		// QR does not handle, so fall back to the normal equations. A
		// system like that is redundant anyway, and gets reported as
		// such.
		z = factorSparseQR(a.mulSelfTransposed()).solve(b)
	}

	return a.mulTransposed(z)
}

// isReasonable is C++ IsReasonable, which despite its name is true for
// values that are clearly not.
func isReasonable(x float64) bool {
	return math.IsNaN(x) || x > 1e11 || x < -1e11
}

// NewtonSolve iterates Newton steps until every equation is within
// ConvergeTolerance of zero (C++ System::NewtonSolve).
func (s *System) NewtonSolve() bool {
	iter := 0
	converged := false

	// Evaluate the functions at our operating point.
	s.mat.Bnum = make([]float64, s.mat.m)
	for i := range s.mat.m {
		s.mat.Bnum[i] = s.mat.B[i].Eval(nil, nil)
	}
	for {
		// And evaluate the Jacobian at our initial operating point.
		s.EvalJacobian()
		if !s.SolveLeastSquares() {
			break
		}

		// Take the Newton step;
		//      J(x_n) (x_{n+1} - x_n) = 0 - F(x_n)
		for i := range s.mat.n {
			p := s.findParam(s.mat.param[i])
			p.Val -= s.mat.X[i]
			if isReasonable(p.Val) {
				// Very bad, and clearly not convergent
				return false
			}
		}

		// Re-evalute the functions, since the params have just changed.
		for i := range s.mat.m {
			s.mat.Bnum[i] = s.mat.B[i].Eval(nil, nil)
			if isReasonable(s.mat.Bnum[i]) {
				// Very bad, and clearly not convergent
				return false
			}
		}

		// Check for convergence
		converged = true
		for i := range s.mat.m {
			if math.Abs(s.mat.Bnum[i]) > ConvergeTolerance {
				converged = false
				break
			}
		}
		if iter >= 50 || converged {
			break
		}
		iter++
	}

	return converged
}

// writeEquationsExceptFor replaces the equations with the group's,
// leaving out constraint hc.
func (s *System) writeEquationsExceptFor(hc core.HConstraint, g *Group) {
	var l []core.Equation
	g.WriteEquationsExceptFor(hc, &l)
	s.setEquations(l)
}

// FindWhichToRemoveToFixJacobian lists the constraints whose removal
// would make the Jacobian full rank (C++
// System::FindWhichToRemoveToFixJacobian).
func (s *System) FindWhichToRemoveToFixJacobian(g *Group, bad *[]core.HConstraint, forceDofCheck bool) {
	start := time.Now()
	g.Timeout = false

	for a := range 2 {
		for _, c := range g.Constraints {
			if time.Since(start) > g.FindToFixTimeout {
				g.Timeout = true
				return
			}

			if (c.Type == core.ConstraintPointsCoincident && a == 0) ||
				(c.Type != core.ConstraintPointsCoincident && a == 1) {
				// Do the constraints in two passes: first everything but
				// the point-coincident constraints, then only those
				// constraints (so they appear last in the list).
				continue
			}

			s.clearParamTags()
			s.writeEquationsExceptFor(c.H, g)
			s.clearEqTags()

			// It's a major speedup to solve the easy ones by substitution here,
			// and that doesn't break anything.
			if !forceDofCheck {
				s.SolveBySubstitution()
			}

			s.WriteJacobian(0)
			s.EvalJacobian()

			rank := s.CalculateRank()
			if rank == s.mat.m {
				// We fixed it by removing this constraint
				*bad = append(*bad, c.H)
			}
		}
	}
}

// referencedParam returns the one unknown e refers to, or reports none
// or several (C++ Expr::ReferencedParams).
func (s *System) referencedParam(e *core.Expr) (h core.HParam, none, multiple bool) {
	var found core.HParam
	var walk func(e *core.Expr) bool
	walk = func(e *core.Expr) bool {
		if e.Op == core.OpParam {
			if s.findParam(e.Param) == nil {
				return true
			}
			if found.V != 0 && found != e.Param {
				return false
			}
			found = e.Param
			return true
		}
		if e.Left != nil && !walk(e.Left) {
			return false
		}
		if e.Right != nil && !walk(e.Right) {
			return false
		}
		return true
	}
	if !walk(e) {
		return core.HParam{}, false, true
	}
	return found, found.V == 0, false
}

// Solve solves the group's equations, writes the solution back to the
// sketch, and reports the result, the degrees of freedom, and (if the
// system is inconsistent or didn't converge) the constraints to blame
// (C++ System::Solve).
func (s *System) Solve(g *Group, dof *int, bad *[]core.HConstraint,
	andFindBad, andFindFree, forceDofCheck bool) core.SolveResult {
	s.writeEquationsExceptFor(core.NoConstraint, g)

	rankOk := false

	// All params and equations are assigned to group zero.
	s.clearParamTags()
	s.clearEqTags()

	var subMap map[core.HParam]*Param

	// Since we are suppressing dof calculation or allowing redundant, we
	// can't / don't want to catch result of dof checking without substitution
	if g.SuppressDofCalculation || g.AllowRedundant || !forceDofCheck {
		subMap = s.SolveBySubstitution()
	}

	// Before solving the big system, see if we can find any equations that
	// are soluble alone. This can be a huge speedup. We don't know whether
	// the system is consistent yet, but if it isn't then we'll catch that
	// later.
	alone := 1
	for i := range s.Eq {
		e := &s.Eq[i]
		if e.Tag != 0 {
			continue
		}

		hp, none, multiple := s.referencedParam(e.E)
		if none || multiple {
			continue
		}

		p := s.findParam(hp)
		if p.Tag != 0 {
			continue // let rank test catch inconsistency
		}

		e.Tag = alone
		p.Tag = alone
		s.WriteJacobian(alone)
		if !s.NewtonSolve() {
			// We don't do the rank test, so let's arbitrarily return
			// the DIDNT_CONVERGE result here.
			rankOk = true
			// Failed to converge, bail out early
			return s.didntConverge(g, bad, rankOk)
		}
		alone++
	}

	// Now write the Jacobian for what's left, and do a rank test; that
	// tells us if the system is inconsistently constrained.
	if !s.WriteJacobian(0) {
		return core.SolveTooManyUnknowns
	}
	// Clear dof value in order to have indication when dof is actually not calculated
	if dof != nil {
		*dof = -1
	}
	// We are suppressing or allowing redundant, so we no need to catch unsolveable + redundant
	rankOk = true
	if !g.SuppressDofCalculation && !g.AllowRedundant {
		rankOk = s.testRank(dof)
	}

	// And do the leftovers as one big system
	if !s.NewtonSolve() {
		return s.didntConverge(g, bad, rankOk)
	}

	// Here we are want to calculate dof even when redundant is allowed, so just handle suppressing
	rankOk = true
	if !g.SuppressDofCalculation {
		rankOk = s.testRank(dof)
	}
	if !rankOk {
		if andFindBad {
			s.FindWhichToRemoveToFixJacobian(g, bad, forceDofCheck)
		}
	} else {
		s.MarkParamsFree(andFindFree)
	}
	// System solved correctly, so write the new values back in to the
	// main parameter table.
	for _, p := range s.Param {
		val := p.Val
		if by, ok := subMap[p.H]; ok {
			val = by.Val
		}

		pp := s.SK(p.H)
		pp.Val = val
		pp.Known = true
		pp.Free = p.Free
	}
	if rankOk {
		return core.SolveOkay
	}
	return core.SolveRedundantOkay
}

func (s *System) testRank(dof *int) bool {
	ok, d, _ := s.TestRank()
	if dof != nil {
		*dof = d
	}
	return ok
}

// didntConverge lists the constraints whose equations aren't
// satisfied (the didnt_converge exit of C++ System::Solve).
func (s *System) didntConverge(g *Group, bad *[]core.HConstraint, rankOk bool) core.SolveResult {
	tagged := map[core.HConstraint]bool{}
	for i, eq := range s.mat.eq {
		b := s.mat.Bnum[i]
		if math.Abs(b) > ConvergeTolerance || isReasonable(b) {
			// This constraint is unsatisfied.
			if !eq.H.IsFromConstraint() {
				continue
			}

			hc := eq.H.Constraint()
			if !g.hasConstraint(hc) {
				continue
			}
			// Don't double-show constraints that generated multiple
			// unsatisfied equations
			if !tagged[hc] {
				*bad = append(*bad, hc)
				tagged[hc] = true
			}
		}
	}

	if rankOk {
		return core.SolveDidntConverge
	}
	return core.SolveRedundantDidntConverge
}

func (g *Group) hasConstraint(hc core.HConstraint) bool {
	for _, c := range g.Constraints {
		if c.H == hc {
			return true
		}
	}
	return false
}

// SolveRank writes the group's equations and tests the Jacobian's rank
// without solving (C++ System::SolveRank).
func (s *System) SolveRank(g *Group, rank, dof *int, bad *[]core.HConstraint,
	andFindBad, andFindFree bool) core.SolveResult {
	s.writeEquationsExceptFor(core.NoConstraint, g)

	// All params and equations are assigned to group zero.
	s.clearParamTags()
	s.clearEqTags()

	// Now write the Jacobian, and do a rank test; that
	// tells us if the system is inconsistently constrained.
	if !s.WriteJacobian(0) {
		return core.SolveTooManyUnknowns
	}

	rankOk, d, r := s.TestRank()
	if dof != nil {
		*dof = d
	}
	if rank != nil {
		*rank = r
	}
	if !rankOk {
		// When we are testing with redundant allowed, we don't want to have additional info
		// about redundants since this test is working only for single redundant constraint
		if !g.SuppressDofCalculation && !g.AllowRedundant {
			if andFindBad {
				s.FindWhichToRemoveToFixJacobian(g, bad, true)
			}
		}
	} else {
		s.MarkParamsFree(andFindFree)
	}
	if rankOk {
		return core.SolveOkay
	}
	return core.SolveRedundantOkay
}

// MarkParamsFree finds, if asked, the params that no equation pins down;
// that might be more than the number of degrees of freedom (C++
// System::MarkParamsFree).
func (s *System) MarkParamsFree(find bool) {
	for _, p := range s.Param {
		p.Free = false

		if find {
			if p.Tag == 0 {
				p.Tag = varDofTest
				s.WriteJacobian(0)
				s.EvalJacobian()
				rank := s.CalculateRank()
				if rank == s.mat.m {
					p.Free = true
				}
				p.Tag = 0
			}
		}
	}
}
