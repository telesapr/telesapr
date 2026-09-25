package solver

import (
	"fmt"
	"math"

	"github.com/spacesolve/spacesolve/pkg/core"
)

// SolveOptions configures the Newton solver.
type SolveOptions struct {
	MaxIterations        int
	Tolerance            float64
	Damping              float64
	StageIterations      int
	EnableStagedSolve    bool
	LinearSolveTolerance float64
	Report               *SolveReport
}

func (o *SolveOptions) withDefaults() SolveOptions {
	opts := *o
	if opts.MaxIterations <= 0 {
		opts.MaxIterations = 25
	}
	if opts.Tolerance <= 0 {
		opts.Tolerance = 1e-9
	}
	if opts.Damping <= 0 {
		opts.Damping = 1.0
	}
	if opts.StageIterations <= 0 {
		opts.StageIterations = 8
	}
	if opts.LinearSolveTolerance <= 0 {
		opts.LinearSolveTolerance = 1e-9
	}
	if !opts.EnableStagedSolve {
		opts.EnableStagedSolve = true
	}
	return opts
}

// SolveReport captures metadata about a solve attempt.
type SolveReport struct {
	Iterations    int
	Residual      float64
	Rank          int
	DOF           int
	Converged     bool
	Substitutions map[core.HParam]core.HParam
	StageSolved   int
}

// Solve attempts to satisfy all equations using a damped Newton iteration with
// substitution, staged solving, and a least-squares update step.
func (s *System) Solve(initial map[core.HParam]float64, options SolveOptions) (map[core.HParam]float64, error) {
	opts := options.withDefaults()
	if options.Report != nil {
		*options.Report = SolveReport{}
	}

	handles := s.ParamHandles()
	if len(handles) == 0 || len(s.Equations) == 0 {
		if options.Report != nil {
			options.Report.Converged = true
			options.Report.DOF = len(handles)
		}
		return copyValues(initial), nil
	}

	current := make(map[core.HParam]float64, len(handles))
	for _, h := range handles {
		if initial != nil {
			if v, ok := initial[h]; ok {
				current[h] = v
				continue
			}
		}
		if p, ok := s.Params[h]; ok && p != nil {
			current[h] = p.Val
		} else {
			current[h] = 0
		}
	}

	s.syncParamValues(current)

	substitutions, err := s.substituteEqualParameters(current)
	if err != nil {
		return nil, err
	}
	if options.Report != nil && len(substitutions) > 0 {
		cloned := make(map[core.HParam]core.HParam, len(substitutions))
		for k, v := range substitutions {
			cloned[k] = v
		}
		options.Report.Substitutions = cloned
	}

	handles = s.ParamHandles()
	if len(handles) == 0 || len(s.Equations) == 0 {
		s.syncParamValues(current)
		if options.Report != nil {
			options.Report.Converged = true
			options.Report.DOF = len(handles)
		}
		return current, nil
	}

	if opts.EnableStagedSolve {
		s.syncParamValues(current)
		stageSolved, err := s.solveSingleParameterEquations(current, opts)
		if err != nil {
			return nil, err
		}
		s.syncParamValues(current)
		if options.Report != nil {
			options.Report.StageSolved = stageSolved
		}
		if len(s.Equations) == 0 {
			s.syncParamValues(current)
			if options.Report != nil {
				finalHandles := s.ParamHandles()
				rank := stageSolved
				if rank > len(finalHandles) {
					rank = len(finalHandles)
				}
				options.Report.Rank = rank
				options.Report.DOF = len(finalHandles) - rank
				options.Report.Converged = true
				if options.Report.Iterations == 0 {
					options.Report.Iterations = 1
				}
				options.Report.Residual = 0
			}
			return current, nil
		}
	}

	paramOrder := s.ParamHandles()

	for iter := 0; iter < opts.MaxIterations; iter++ {
		s.syncParamValues(current)
		residuals, jacobian, err := s.Evaluate(current)
		if err != nil {
			return nil, err
		}
		maxResidual := maxAbs(residuals)
		if maxResidual <= opts.Tolerance {
			if options.Report != nil {
				options.Report.Iterations = iter + 1
				options.Report.Residual = maxResidual
				options.Report.Converged = true
				if options.Report.Rank == 0 {
					options.Report.Rank = len(paramOrder)
					options.Report.DOF = len(paramOrder) - options.Report.Rank
				}
			}
			return current, nil
		}

		delta, rank, err := solveLeastSquares(jacobian, residuals, opts.LinearSolveTolerance)
		if err != nil {
			return nil, err
		}
		if options.Report != nil {
			options.Report.Rank = rank
			options.Report.DOF = len(paramOrder) - rank
			options.Report.Iterations = iter + 1
			options.Report.Residual = maxResidual
		}

		for i, h := range paramOrder {
			step := opts.Damping * delta[i]
			if math.IsNaN(step) || math.IsInf(step, 0) {
				return nil, fmt.Errorf("solver: invalid step encountered")
			}
			current[h] += step
			if p := s.Params[h]; p != nil {
				p.Val = current[h]
			}
		}
	}

	if options.Report != nil {
		options.Report.Converged = false
	}
	return nil, fmt.Errorf("solver: did not converge within iteration limit")
}

func maxAbs(v []float64) float64 {
	max := 0.0
	for _, r := range v {
		if val := math.Abs(r); val > max {
			max = val
		}
	}
	return max
}

// copyValues clones a parameter map.
func copyValues(src map[core.HParam]float64) map[core.HParam]float64 {
	if src == nil {
		return nil
	}
	dst := make(map[core.HParam]float64, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func (s *System) syncParamValues(current map[core.HParam]float64) {
	if s == nil || len(s.Params) == 0 {
		return
	}
	for h, param := range s.Params {
		if param == nil {
			continue
		}
		if current != nil {
			if val, ok := current[h]; ok {
				param.Val = val
				continue
			}
		}
		// Ensure overrides map can pick up param defaults on future iterations.
		if current != nil {
			if _, exists := current[h]; !exists {
				current[h] = param.Val
			}
		}
	}
}

// NormalizeEqualParameters collapses equations of the form (p_i - p_j) and
// rewrites the system to use canonical parameter representatives. This mirrors
// the pre-processing step performed during Solve and is exposed so callers can
// observe the effective coupled-equation count before solving.
func (s *System) NormalizeEqualParameters() error {
	_, err := s.substituteEqualParameters(nil)
	return err
}

func (s *System) substituteEqualParameters(current map[core.HParam]float64) (map[core.HParam]core.HParam, error) {
	if len(s.Equations) == 0 {
		return nil, nil
	}
	uf := newUnionFind(s.Params)
	type entry struct {
		eq   *Equation
		keep bool
	}
	entries := make([]entry, len(s.Equations))
	for i, eq := range s.Equations {
		if h1, h2, ok := isParamDifference(eq.Expr); ok {
			uf.union(h1, h2)
			entries[i] = entry{eq: eq, keep: false}
		} else {
			entries[i] = entry{eq: eq, keep: true}
		}
	}

	remap := uf.remap()
	if len(remap) == 0 {
		filtered := make([]*Equation, 0, len(s.Equations))
		for _, e := range entries {
			if e.keep {
				filtered = append(filtered, e.eq)
			}
		}
		s.Equations = filtered
		return nil, nil
	}

	groups := uf.groups()
	newParams := make(map[core.HParam]*core.Param, len(groups))
	substMap := make(map[core.HParam]*core.Param)

	for canonical, handles := range groups {
		var base *core.Param
		if p, ok := s.Params[canonical]; ok {
			base = p
		} else if len(handles) > 0 {
			if p, ok := s.Params[handles[0]]; ok {
				clone := *p
				clone.H = canonical
				base = &clone
			}
		}
		if base == nil {
			base = &core.Param{H: canonical}
		}
		newParams[canonical] = base
		var currentValueSet bool
		for _, h := range handles {
			substMap[h] = base
			if val, ok := current[h]; ok {
				if !currentValueSet {
					current[canonical] = val
					currentValueSet = true
				}
			}
			if h != canonical {
				delete(current, h)
			}
		}
		if !currentValueSet && current != nil {
			current[canonical] = base.Val
		}
	}

	filtered := make([]*Equation, 0, len(s.Equations))
	for _, e := range entries {
		if !e.keep {
			continue
		}
		e.eq.Expr.Substitute(substMap)
		e.eq.Expr = e.eq.Expr.FoldConstants()
		e.eq.derivatives = nil
		if e.eq.Expr.IsZeroConst() {
			continue
		}
		filtered = append(filtered, e.eq)
	}
	s.Equations = filtered
	s.Params = newParams

	cleaned := make(map[core.HParam]core.HParam)
	for old, parent := range remap {
		if old != parent {
			cleaned[old] = parent
		}
	}
	return cleaned, nil
}

func isParamDifference(expr *core.Expr) (core.HParam, core.HParam, bool) {
	if expr == nil || expr.Left == nil || expr.Right == nil {
		return core.HParam{}, core.HParam{}, false
	}
	if expr.Op != core.OpMinus {
		return core.HParam{}, core.HParam{}, false
	}
	if h1, ok := expr.Left.ParamHandle(); ok {
		if h2, ok := expr.Right.ParamHandle(); ok {
			if h1 != h2 {
				return h1, h2, true
			}
		}
	}
	return core.HParam{}, core.HParam{}, false
}

func (s *System) solveSingleParameterEquations(current map[core.HParam]float64, opts SolveOptions) (int, error) {
	if len(s.Equations) == 0 {
		return 0, nil
	}
	remaining := make([]*Equation, 0, len(s.Equations))
	solved := 0
	ctx := evaluationContext{overrides: current, params: s.Params}
	for _, eq := range s.Equations {
		params := collectParams(eq.Expr)
		if len(params) != 1 {
			remaining = append(remaining, eq)
			continue
		}
		var handle core.HParam
		for h := range params {
			handle = h
		}
		if err := s.solveSingleParamEquation(eq, handle, current, ctx, opts); err != nil {
			return 0, err
		}
		if math.Abs(eq.Expr.Eval(ctx, nil)) <= opts.Tolerance {
			solved++
			continue
		}
		remaining = append(remaining, eq)
	}
	s.Equations = remaining
	return solved, nil
}

func (s *System) solveSingleParamEquation(eq *Equation, h core.HParam, current map[core.HParam]float64, ctx evaluationContext, opts SolveOptions) error {
	for i := 0; i < opts.StageIterations; i++ {
		if p := s.Params[h]; p != nil {
			if current != nil {
				if val, ok := current[h]; ok {
					p.Val = val
				}
			}
		}
		val := eq.Expr.Eval(ctx, nil)
		if math.Abs(val) <= opts.Tolerance {
			return nil
		}
		deriv := eq.derivative(h).Eval(ctx, nil)
		if math.Abs(deriv) < opts.LinearSolveTolerance {
			break
		}
		step := -val / deriv
		if math.IsNaN(step) || math.IsInf(step, 0) {
			break
		}
		current[h] += step
		if p := s.Params[h]; p != nil {
			p.Val = current[h]
		}
	}
	return nil
}

func collectParams(expr *core.Expr) map[core.HParam]struct{} {
	params := make(map[core.HParam]struct{})
	var walk func(e *core.Expr)
	walk = func(e *core.Expr) {
		if e == nil {
			return
		}
		if h, ok := e.ParamHandle(); ok {
			params[h] = struct{}{}
			return
		}
		if e.Left != nil {
			walk(e.Left)
		}
		if e.Right != nil {
			walk(e.Right)
		}
	}
	walk(expr)
	return params
}

func solveLeastSquares(a [][]float64, residuals []float64, tol float64) ([]float64, int, error) {
	return gonumLeastSquares(a, residuals, tol)
}

func solveSymmetricSystem(a [][]float64, b []float64, tol float64) ([]float64, error) {
	return gonumSymmetricSolve(a, b, tol)
}

func newUnionFind(params map[core.HParam]*core.Param) *unionFind {
	uf := &unionFind{parent: make(map[core.HParam]core.HParam)}
	for h := range params {
		uf.parent[h] = h
	}
	return uf
}

type unionFind struct {
	parent map[core.HParam]core.HParam
}

func (uf *unionFind) find(h core.HParam) core.HParam {
	if p, ok := uf.parent[h]; ok {
		if p != h {
			uf.parent[h] = uf.find(p)
		}
		return uf.parent[h]
	}
	uf.parent[h] = h
	return h
}

func (uf *unionFind) union(a, b core.HParam) core.HParam {
	pa := uf.find(a)
	pb := uf.find(b)
	if pa == pb {
		return pa
	}
	if pa.V < pb.V {
		uf.parent[pb] = pa
		return pa
	}
	uf.parent[pa] = pb
	return pb
}

func (uf *unionFind) remap() map[core.HParam]core.HParam {
	remap := make(map[core.HParam]core.HParam)
	for h := range uf.parent {
		parent := uf.find(h)
		remap[h] = parent
	}
	return remap
}

func (uf *unionFind) groups() map[core.HParam][]core.HParam {
	groups := make(map[core.HParam][]core.HParam)
	for h := range uf.parent {
		parent := uf.find(h)
		groups[parent] = append(groups[parent], h)
	}
	return groups
}
