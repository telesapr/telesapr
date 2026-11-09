package sketch

import (
	"errors"
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/solver"
)

// A mechanism's freedom is counted part by part (Kramer's degrees of
// freedom analysis): how many ways each part can turn and slide, and
// what stays put while it does, its invariant: a point it turns about,
// an axis, a plane it moves in, a direction it slides along. Kramer
// writes a table of what each kind of mate leaves of a part's freedom;
// here the count is read off the mechanism's own equations instead,
// which knows every mate there is and any constraint a user adds: the
// null space of the block's Jacobian (solver.System.NullSpace) is
// every way its unknowns can move together, and each null vector, read
// through a part's move and turn, is a motion of the part, a twist
// (a turn ω and the speed v of its origin). The twists' span is the
// part's freedom. Its rank in ω is the turns, the rest the slides, and
// the invariant is what every twist of the span leaves still.
//
// A null space is first order: it sees only the motion at this
// position. The coupler of a four-bar turns, at each position, about a
// point, but that point moves as the linkage does, and the coupler has
// no fixed axis. So an invariant found is checked at a second position,
// a small step along each of the mechanism's freedoms, solved: one that
// moved is not an invariant, and the part is said to move with the
// mechanism. Where the mechanism stands at a singular position, such as
// a chain at full stretch, the count is what the rank there gives, as
// the group's DOF is.

// ErrNotPart is the error of PartFreedoms for a group that is not a
// link group, a part.
var ErrNotPart = errors.New("not a linked part")

// ErrNotSolved is the error of PartFreedoms for a part whose mechanism
// did not solve.
var ErrNotSolved = errors.New("the mechanism is not solved")

// Invariant is what a part's freedom leaves still (PartFreedom).
type Invariant int

const (
	// InvariantNone is a part with nothing fixed: free, fixed whole, or
	// moving with its mechanism.
	InvariantNone Invariant = iota
	// InvariantPoint is a part that turns about a point and does not
	// slide: a ball joint.
	InvariantPoint
	// InvariantAxis is a part that turns about an axis, and may slide
	// along it or screw along it: a hinge, a cylinder in a bore, a screw.
	InvariantAxis
	// InvariantPlane is a part that moves in a plane: it slides two ways
	// in it, and may turn about its normal.
	InvariantPlane
	// InvariantDirection is a part that slides one way and does not turn.
	InvariantDirection
)

func (iv Invariant) String() string {
	switch iv {
	case InvariantPoint:
		return "point"
	case InvariantAxis:
		return "axis"
	case InvariantPlane:
		return "plane"
	case InvariantDirection:
		return "direction"
	}
	return "none"
}

// PartFreedom is how a part of a mechanism can move: how many ways it
// turns and slides, and what stays put while it does.
type PartFreedom struct {
	// Group is the part's link group.
	Group core.HGroup
	// Turns is the ways the part can turn, 0 to 3, and Slides the ways
	// it can slide without turning, 0 to 3. Both 0 is a part fixed.
	Turns, Slides int
	// Invariant is what stays put: the point it turns about (Point), the
	// axis it turns about (through Point, along Dir), the plane it moves
	// in (through Point, normal Dir) or the direction it slides along
	// (Dir).
	Invariant  Invariant
	Point, Dir geom.Vector
	// Lead is, for a part that screws along its axis, how far it goes
	// along Dir in a turn; 0 for a plain turn.
	Lead float64
	// Moving is that the part turns about a point or an axis at this
	// position, but that point or axis moves as the mechanism does, as
	// a four-bar's coupler's does: no invariant.
	Moving bool
	// DrivenBy are the parameters of the dimensions with expressions
	// that drive the mechanism and hold this part: a driven hinge's
	// angle. The counts are the mates' freedom, with these and the
	// bounds let go.
	DrivenBy []string
}

// Text says how the part can move, as "turns about the axis through
// (0, 0, 0) along (0, 0, 1)", its lengths written by length.
func (f PartFreedom) Text(length func(float64) string) string {
	vec := func(v geom.Vector) string {
		return "(" + length(v.X) + ", " + length(v.Y) + ", " + length(v.Z) + ")"
	}
	dir := func(v geom.Vector) string {
		return fmt.Sprintf("(%s, %s, %s)", num(v.X), num(v.Y), num(v.Z))
	}
	var s string
	switch {
	case f.Turns == 0 && f.Slides == 0:
		s = "fixed"
	case f.Turns == 3 && f.Slides == 3:
		s = "free"
	case f.Invariant == InvariantAxis && f.Slides == 1:
		s = "turns about and slides along the axis through " + vec(f.Point) + " along " + dir(f.Dir)
	case f.Invariant == InvariantAxis && f.Lead != 0:
		s = "screws along the axis through " + vec(f.Point) + " along " + dir(f.Dir) + ", " + length(f.Lead) + " a turn"
	case f.Invariant == InvariantAxis:
		s = "turns about the axis through " + vec(f.Point) + " along " + dir(f.Dir)
	case f.Invariant == InvariantPoint:
		s = "turns about the point " + vec(f.Point)
	case f.Invariant == InvariantPlane && f.Turns == 1:
		s = "moves in the plane through " + vec(f.Point) + " normal to " + dir(f.Dir)
	case f.Invariant == InvariantPlane:
		s = "slides in the plane through " + vec(f.Point) + " normal to " + dir(f.Dir)
	case f.Invariant == InvariantDirection:
		s = "slides along " + dir(f.Dir)
	case f.Moving && f.Turns == 1 && f.Slides == 0:
		s = "turns about an axis that moves with the mechanism"
	default:
		s = counted(f.Turns, "turns") + " and " + counted(f.Slides, "slides")
		if f.Turns == 0 {
			s = counted(f.Slides, "slides")
		} else if f.Slides == 0 {
			s = counted(f.Turns, "turns")
		}
		if f.Moving {
			s += ", about a point or an axis that moves with the mechanism"
		}
	}
	if len(f.DrivenBy) > 0 {
		s += ", driven by " + strings.Join(f.DrivenBy, ", ")
	}
	return s
}

// String is Text with lengths in mm.
func (f PartFreedom) String() string {
	return f.Text(num)
}

// num writes v with up to three decimals, no -0.
func num(v float64) string {
	if math.Abs(v) < 5e-4 {
		return "0"
	}
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.3f", v), "0"), ".")
}

// counted is verb and "1 way" or "n ways".
func counted(n int, verb string) string {
	if n == 1 {
		return verb + " 1 way"
	}
	return fmt.Sprintf("%s %d ways", verb, n)
}

// partFreedomCache is the freedom of one mechanism at one generation.
type partFreedomCache struct {
	generation uint64
	block      core.HGroup
	parts      []PartFreedom
}

// PartFreedoms is how each part of the mechanism group hg is a part of
// can move (PartFreedom), in the group order, counted from its mates
// at the values of the last regeneration; a link group that doesn't
// move in a mechanism is a mechanism of one, its own mates the ones
// that count. ErrNotPart for a group that is not a link, ErrNotSolved
// when the mechanism did not solve. Nothing is changed; the answer is
// kept until the next regeneration.
func (sk *Sketch) PartFreedoms(hg core.HGroup) ([]PartFreedom, error) {
	g := sk.Group(hg)
	if g == nil || g.Type != GroupLinked {
		return nil, ErrNotPart
	}
	block := sk.Mechanism(hg)
	for _, h := range block {
		if !sk.Group(h).IsSolvedOkay() {
			return nil, ErrNotSolved
		}
	}
	if c := sk.partFreedom; c != nil && c.generation == sk.Generation && c.block == block[0] {
		return slices.Clone(c.parts), nil
	}
	var parts []PartFreedom
	if err := catchPanic(func() { parts = sk.countPartFreedoms(block) }); err != nil {
		return nil, fmt.Errorf("counting the freedom of %s: %w", sk.GroupName(hg), err)
	}
	sk.partFreedom = &partFreedomCache{generation: sk.Generation, block: block[0], parts: parts}
	return slices.Clone(parts), nil
}

// PartFreedomOf is the PartFreedom of link group hg alone, of those
// PartFreedoms gives.
func (sk *Sketch) PartFreedomOf(hg core.HGroup) (PartFreedom, error) {
	parts, err := sk.PartFreedoms(hg)
	if err != nil {
		return PartFreedom{}, err
	}
	for _, p := range parts {
		if p.Group == hg {
			return p, nil
		}
	}
	return PartFreedom{}, ErrNotPart
}

// countPartFreedoms counts the freedom of each part of the block. The
// drives' dimensions get their values back after, and the params
// theirs.
func (sk *Sketch) countPartFreedoms(block []core.HGroup) []PartFreedom {
	fs := sk.newFreedomSetting(block)
	defer fs.restore(sk)
	hs, motions := sk.motions(fs)
	parts := sk.analyseParts(fs, hs, motions)
	sk.checkInvariants(fs, hs, motions, parts)
	return parts
}

// freedomSetting is what counting a block's freedom works from: the
// block, its unknowns, the bounds let go, and its drives, each
// parameter that is all of some dimension's expression (as a driven
// hinge's "angle + 12.5"), with the dimensions whose expressions name
// it, and the parameters' values, which a step along a drive changes.
type freedomSetting struct {
	block  []core.HGroup
	params []core.HParam
	bounds map[core.HConstraint]bool
	drives map[string][]*core.Constraint
	names  []string
	vars   map[string]float64
	// saved are the params' values, and valA the drives' dimensions',
	// as they were.
	saved []core.Param
	valA  map[*core.Constraint]float64
}

// newFreedomSetting is the freedom setting of the block as it is.
func (sk *Sketch) newFreedomSetting(block []core.HGroup) *freedomSetting {
	fs := &freedomSetting{block: block, params: sk.groupParams(block...),
		bounds: map[core.HConstraint]bool{}, drives: map[string][]*core.Constraint{}, valA: map[*core.Constraint]float64{}}
	for _, h := range fs.params {
		fs.saved = append(fs.saved, *sk.MustParam(h))
	}
	fs.vars, _ = sk.evaluateTable()
	var exprs []*core.Constraint
	for ci := range sk.Constraints {
		c := &sk.Constraints[ci]
		if !slices.Contains(block, c.Group) || c.Reference {
			continue
		}
		switch {
		case c.IsInequality():
			fs.bounds[c.H] = true
		case c.Expr != "":
			exprs = append(exprs, c)
			if _, used, err := sk.parseNamed(c.Expr); err == nil {
				if used = slices.Compact(slices.Sorted(slices.Values(used))); len(used) == 1 && sk.Parameter(used[0]) != nil {
					fs.drives[used[0]] = nil
				}
			}
		}
	}
	for _, c := range exprs {
		_, used, _ := sk.parseNamed(c.Expr)
		for _, n := range used {
			if _, ok := fs.drives[n]; ok && !slices.Contains(fs.drives[n], c) {
				fs.drives[n] = append(fs.drives[n], c)
				fs.valA[c] = c.ValA
			}
		}
	}
	fs.names = slices.Sorted(maps.Keys(fs.drives))
	return fs
}

// restore puts the params and the drives' dimensions back as they were.
func (fs *freedomSetting) restore(sk *Sketch) {
	fs.restoreParams(sk)
	for c, v := range fs.valA {
		c.ValA = v
	}
}

// restoreParams puts the params back as they were.
func (fs *freedomSetting) restoreParams(sk *Sketch) {
	for i, h := range fs.params {
		*sk.MustParam(h) = fs.saved[i]
	}
}

// setDrive gives drive name the value at, and its dimensions the values
// their expressions then have, as Sketch.EvaluateParameters would.
func (fs *freedomSetting) setDrive(sk *Sketch, name string, at float64) {
	fs.vars[name] = at
	for _, c := range fs.drives[name] {
		v, err := evalNamed(c.Expr, fs.vars)
		if err != nil {
			continue
		}
		if !signedValue(c.Type) && c.Type != core.ConstraintAxisAngle {
			v = math.Abs(v)
		}
		if IsLengthValue(c.Type) {
			v *= sk.Units.MmPer()
		}
		c.ValA = v
	}
}

// solveFreedom solves the block, its bounds let go, from the params' values
// now; false when it fails.
func (sk *Sketch) solveFreedom(fs *freedomSetting) bool {
	sys, sg := sk.freeSystem(fs.block, fs.params, fs.bounds)
	how, _, _ := sys.Solve(sg, false, false, false)
	return how == core.SolveOkay || how == core.SolveRedundantOkay
}

// blockMotion is a way the block's unknowns can move together, a change of
// each of hs: a null vector of its equations, or, for a drive, what a
// unit of the drive moves them by.
type blockMotion struct {
	by    []float64
	drive string
}

// driveDelta is half the change of a drive the motion it makes is read
// from, by a central difference: a thousandth of a degree or of a
// millimetre, small enough that the chord is the tangent to round-off.
const driveDelta = 1e-3

// motions are the ways the block can move at the params' values now,
// its bounds let go: its null space, then what each drive moves.
func (sk *Sketch) motions(fs *freedomSetting) ([]core.HParam, []blockMotion) {
	hs, null := sk.blockNullSpace(fs.block, fs.params, fs.bounds)
	var out []blockMotion
	for _, n := range null {
		out = append(out, blockMotion{by: n})
	}
	if hs == nil {
		return hs, out
	}
	at := make([]core.Param, len(fs.params))
	for i, h := range fs.params {
		at[i] = *sk.MustParam(h)
	}
	back := func() {
		for i, h := range fs.params {
			*sk.MustParam(h) = at[i]
		}
	}
	for _, name := range fs.names {
		base := fs.vars[name]
		var ends [2][]float64
		ok := true
		for k, s := range []float64{-1, 1} {
			fs.setDrive(sk, name, base+s*driveDelta)
			if !sk.solveFreedom(fs) {
				ok = false
			}
			ends[k] = make([]float64, len(hs))
			for j, h := range hs {
				ends[k][j] = sk.MustParam(h).Val
			}
			back()
		}
		fs.setDrive(sk, name, base)
		if !ok {
			continue
		}
		by := make([]float64, len(hs))
		for j := range hs {
			by[j] = (ends[1][j] - ends[0][j]) / (2 * driveDelta)
		}
		out = append(out, blockMotion{by: by, drive: name})
	}
	return hs, out
}

// analyseParts is the freedom of each part of the block that the
// motions give: its span of twists, and the drives that add to what the
// null space leaves it.
func (sk *Sketch) analyseParts(fs *freedomSetting, hs []core.HParam, motions []blockMotion) []PartFreedom {
	parts := make([]PartFreedom, len(fs.block))
	for i, hg := range fs.block {
		var all, cur []twist
		var drivenBy []string
		for _, m := range motions {
			tw := sk.partTwists(hg, hs, [][]float64{m.by})[0]
			all = append(all, tw)
			if m.drive == "" {
				cur = append(cur, tw)
			}
		}
		for k, m := range motions {
			if m.drive == "" {
				continue
			}
			if more := append(slices.Clone(cur), all[k]); len(spanOf(more)) > len(spanOf(cur)) {
				cur = more
				drivenBy = append(drivenBy, m.drive)
			}
		}
		parts[i] = analyseTwists(hg, sk.partOrigin(hg), all)
		parts[i].DrivenBy = drivenBy
	}
	return parts
}

// blockNullSpace is the null space of the block's equations, but those
// of the constraints left out, at the params' values now: a system of
// its own, so that the kept one is left as it is.
func (sk *Sketch) blockNullSpace(block []core.HGroup, params []core.HParam, leftOut map[core.HConstraint]bool) ([]core.HParam, [][]float64) {
	sys, sg := sk.freeSystem(block, params, leftOut)
	return sys.NullSpace(sg)
}

// freeSystem is a new system of the block's equations over params,
// without those of the constraints left out.
func (sk *Sketch) freeSystem(block []core.HGroup, params []core.HParam, leftOut map[core.HConstraint]bool) (*solver.System, *solver.Group) {
	sys := &solver.System{SK: sk.Param}
	for _, h := range params {
		p := *sk.Param(h)
		p.Known = false
		sys.AddParam(p)
	}
	sg := &solver.Group{
		FindToFixTimeout: sk.findToFixTimeout(),
		AllowRedundant:   true,
		WriteEquationsExceptFor: func(hc core.HConstraint, l *[]core.Equation) {
			var all []core.Equation
			for _, hg := range block {
				sk.WriteEquationsExceptFor(hg, hc, &all)
			}
			for _, e := range all {
				if e.H.IsFromConstraint() && leftOut[e.H.Constraint()] {
					continue
				}
				*l = append(*l, e)
			}
		},
	}
	return sys, sg
}

// twist is a motion of a part: its turn w, radians per unit, and the
// speed v of its origin.
type twist struct{ w, v geom.Vector }

// partOrigin is where part hg's origin is: its move.
func (sk *Sketch) partOrigin(hg core.HGroup) geom.Vector {
	val := func(i int) float64 { return sk.MustParam(hg.Param(i)).Val }
	return geom.Vector{X: val(0), Y: val(1), Z: val(2)}
}

// partTwists are the motions of part hg along each of the null vectors
// null, whose components are for hs: the change of its move, and the
// turn the change of its quaternion makes, read off the change of its
// rotation (R' Rᵀ, the cross product by the turn).
func (sk *Sketch) partTwists(hg core.HGroup, hs []core.HParam, null [][]float64) []twist {
	col := make([]int, 7)
	for i := range 7 {
		col[i] = slices.Index(hs, hg.Param(i))
	}
	val := func(i int) float64 { return sk.MustParam(hg.Param(i)).Val }
	q := geom.Quaternion{W: val(3), X: val(4), Y: val(5), Z: val(6)}
	out := make([]twist, 0, len(null))
	for _, n := range null {
		at := func(i int) float64 {
			if col[i] < 0 {
				return 0
			}
			return n[col[i]]
		}
		dq := geom.Quaternion{W: at(3), X: at(4), Y: at(5), Z: at(6)}
		out = append(out, twist{w: turnOf(q, dq), v: geom.Vector{X: at(0), Y: at(1), Z: at(2)}})
	}
	return out
}

// turnOf is the turn a change dq of unit quaternion q makes: the
// rotation's columns are quadratic in q, so a central difference of
// them is their derivative exactly, and R'·Rᵀ is the cross product by
// the turn.
func turnOf(q, dq geom.Quaternion) geom.Vector {
	plus := geom.Quaternion{W: q.W + dq.W, X: q.X + dq.X, Y: q.Y + dq.Y, Z: q.Z + dq.Z}
	minus := geom.Quaternion{W: q.W - dq.W, X: q.X - dq.X, Y: q.Y - dq.Y, Z: q.Z - dq.Z}
	cols := func(r geom.Quaternion) [3]geom.Vector {
		return [3]geom.Vector{r.RotationU(), r.RotationV(), r.RotationN()}
	}
	rp, rm, r := cols(plus), cols(minus), cols(q)
	var om [3][3]float64
	for k := range 3 {
		d := rp[k].Minus(rm[k]).ScaledBy(0.5)
		dc, rc := [3]float64{d.X, d.Y, d.Z}, [3]float64{r[k].X, r[k].Y, r[k].Z}
		for i := range 3 {
			for j := range 3 {
				om[i][j] += dc[i] * rc[j]
			}
		}
	}
	return geom.Vector{
		X: (om[2][1] - om[1][2]) / 2,
		Y: (om[0][2] - om[2][0]) / 2,
		Z: (om[1][0] - om[0][1]) / 2,
	}
}

// twistScale is the length a turn is weighed by against a speed when
// the twists' rank is taken, so that both are speeds of points of a
// part about this size, in mm.
const twistScale = 100

// turnTol is the size, weighed by twistScale, under which the turn of
// a unit twist of a part's span is none.
const turnTol = 1e-7

// spanTol is the relative size under which a twist adds nothing to the
// span of the others.
const spanTol = 1e-8

// spanOf is an orthonormal basis of the twists' span, each a vector of
// six, the turn weighed by twistScale.
func spanOf(ts []twist) [][]float64 {
	vs := make([][]float64, len(ts))
	for i, t := range ts {
		w := t.w.ScaledBy(twistScale)
		vs[i] = []float64{w.X, w.Y, w.Z, t.v.X, t.v.Y, t.v.Z}
	}
	return gramSchmidt(vs, spanTol)
}

// gramSchmidt is an orthonormal basis of the span of vs, dropping a
// vector the ones before span to within tol of the largest's size.
func gramSchmidt(vs [][]float64, tol float64) [][]float64 {
	largest := 0.0
	for _, v := range vs {
		largest = math.Max(largest, norm(v))
	}
	var out [][]float64
	for _, v0 := range vs {
		v := slices.Clone(v0)
		for range 2 { // twice, for round-off
			for _, u := range out {
				d := dotOf(u, v)
				for k := range v {
					v[k] -= d * u[k]
				}
			}
		}
		n := norm(v)
		if n <= tol*largest || n == 0 {
			continue
		}
		for k := range v {
			v[k] /= n
		}
		out = append(out, v)
	}
	return out
}

func dotOf(a, b []float64) float64 {
	var s float64
	for k := range a {
		s += a[k] * b[k]
	}
	return s
}

func norm(a []float64) float64 { return math.Sqrt(dotOf(a, a)) }

// vecAt is the part of six of a basis vector from k.
func vecAt(v []float64, k int) geom.Vector { return geom.Vector{X: v[k], Y: v[k+1], Z: v[k+2]} }

// invariantTol is how near, in mm at a part's size, what a twist moves
// is to still: the pitch of a plain turn, the speed of a fixed point.
const invariantTol = 1e-6

// analyseTwists is the freedom of part hg, its origin at o, that the
// twists ts span.
func analyseTwists(hg core.HGroup, o geom.Vector, ts []twist) PartFreedom {
	f := PartFreedom{Group: hg}
	basis := spanOf(ts)
	if len(basis) == 0 {
		return f
	}
	// The basis back in turns and speeds.
	bt := make([]twist, len(basis))
	for i, b := range basis {
		bt[i] = twist{w: vecAt(b, 0).ScaledBy(1.0 / twistScale), v: vecAt(b, 3)}
	}
	// The slides: the twists of the span with no turn, the null space
	// of the turns' map from the basis.
	ws := make([][]float64, 3)
	for r := range 3 {
		ws[r] = make([]float64, len(bt))
		for j, t := range bt {
			ws[r][j] = [3]float64{t.w.X, t.w.Y, t.w.Z}[r] * twistScale
		}
	}
	var slides []geom.Vector
	// The basis is orthonormal, so a turn is measured against 1: what a
	// drive's difference leaves of round-off, some 1e-9, is no turn.
	for _, c := range nullOfSmall(ws, len(bt), turnTol) {
		var v geom.Vector
		for j, t := range bt {
			v = v.Plus(t.v.ScaledBy(c[j]))
		}
		slides = append(slides, v)
	}
	slides = orthonormalVectors(slides)
	f.Slides = len(slides)
	f.Turns = len(basis) - f.Slides
	// A turn's speed of the origin with the slides taken out.
	offSlides := func(v geom.Vector) geom.Vector {
		for _, d := range slides {
			v = v.Minus(d.ScaledBy(v.Dot(d)))
		}
		return v
	}
	switch f.Turns {
	case 0:
		switch f.Slides {
		case 1:
			f.Invariant, f.Dir = InvariantDirection, slides[0]
		case 2:
			f.Invariant, f.Point, f.Dir = InvariantPlane, o, unitNormal(slides[0].Cross(slides[1]))
		}
	case 1:
		// The turn: the basis twist that turns most, its slides taken out.
		best := bt[0]
		for _, t := range bt {
			if t.w.Magnitude() > best.w.Magnitude() {
				best = t
			}
		}
		w, v := best.w, offSlides(best.v)
		w2 := w.Dot(w)
		axis := unitNormal(w)
		p := o.Plus(w.Cross(v).ScaledBy(1 / w2))
		pitch := w.Dot(v) / w2
		parallel := func(d geom.Vector) bool { return d.Cross(axis).Magnitude() < invariantTol }
		switch {
		case f.Slides == 0:
			f.Invariant, f.Point, f.Dir = InvariantAxis, p, axis
			if math.Abs(pitch) > invariantTol {
				f.Lead = 2 * math.Pi * pitch
			}
		case f.Slides == 1 && parallel(slides[0]):
			f.Invariant, f.Point, f.Dir = InvariantAxis, p, axis
		case f.Slides == 2 && parallel(slides[0].Cross(slides[1])):
			f.Invariant, f.Point, f.Dir = InvariantPlane, o, axis
		}
	case 2, 3:
		if f.Slides > 0 {
			break
		}
		if d, ok := fixedPoint(bt); ok {
			f.Invariant, f.Point = InvariantPoint, o.Plus(d)
		}
	}
	if f.Invariant == InvariantAxis || f.Invariant == InvariantPlane || f.Invariant == InvariantDirection {
		f.Dir = canonicalDir(f.Dir)
	}
	f.Point, f.Dir = roundVec(f.Point), roundVec(f.Dir)
	return f
}

// fixedPoint is the point, from the origin, that every twist of ts
// leaves still: d with w × d + v = 0 for each, by least squares, and
// whether it is still for all of them.
func fixedPoint(ts []twist) (geom.Vector, bool) {
	// Σ (|w|² I - w wᵀ) d = Σ w × v.
	var a [3][3]float64
	var b geom.Vector
	for _, t := range ts {
		w := [3]float64{t.w.X, t.w.Y, t.w.Z}
		w2 := t.w.Dot(t.w)
		for i := range 3 {
			for j := range 3 {
				a[i][j] -= w[i] * w[j]
			}
			a[i][i] += w2
		}
		b = b.Plus(t.w.Cross(t.v))
	}
	d, ok := solve3(a, b)
	if !ok {
		return geom.Vector{}, false
	}
	for _, t := range ts {
		if t.w.Cross(d).Plus(t.v).Magnitude() > invariantTol*math.Max(1, t.v.Magnitude()) {
			return geom.Vector{}, false
		}
	}
	return d, true
}

// solve3 solves a·x = b by Cramer's rule; false when a is singular.
func solve3(a [3][3]float64, b geom.Vector) (geom.Vector, bool) {
	col := func(i int) geom.Vector { return geom.Vector{X: a[0][i], Y: a[1][i], Z: a[2][i]} }
	c0, c1, c2 := col(0), col(1), col(2)
	det := c0.Dot(c1.Cross(c2))
	scale := c0.Magnitude() * c1.Magnitude() * c2.Magnitude()
	if scale == 0 || math.Abs(det) < 1e-12*scale {
		return geom.Vector{}, false
	}
	return geom.Vector{
		X: b.Dot(c1.Cross(c2)) / det,
		Y: c0.Dot(b.Cross(c2)) / det,
		Z: c0.Dot(c1.Cross(b)) / det,
	}, true
}

// nullOfSmall is an orthonormal basis of the null space of a, rows of
// n, by elimination with partial pivoting, a pivot under tol taken for
// none.
func nullOfSmall(a [][]float64, n int, tol float64) [][]float64 {
	m := make([][]float64, len(a))
	for i, r := range a {
		m[i] = slices.Clone(r)
	}
	pivotCol := make([]int, 0, len(m))
	row := 0
	for c := 0; c < n && row < len(m); c++ {
		p := row
		for i := row + 1; i < len(m); i++ {
			if math.Abs(m[i][c]) > math.Abs(m[p][c]) {
				p = i
			}
		}
		if math.Abs(m[p][c]) <= tol {
			continue
		}
		m[row], m[p] = m[p], m[row]
		for i := range m {
			if i == row {
				continue
			}
			f := m[i][c] / m[row][c]
			for k := c; k < n; k++ {
				m[i][k] -= f * m[row][k]
			}
		}
		pivotCol = append(pivotCol, c)
		row++
	}
	var out [][]float64
	for f := range n {
		if slices.Contains(pivotCol, f) {
			continue
		}
		v := make([]float64, n)
		v[f] = 1
		for r, c := range pivotCol {
			v[c] = -m[r][f] / m[r][c]
		}
		out = append(out, v)
	}
	return gramSchmidt(out, spanTol)
}

// orthonormalVectors is an orthonormal basis of the span of vs.
func orthonormalVectors(vs []geom.Vector) []geom.Vector {
	in := make([][]float64, len(vs))
	for i, v := range vs {
		in[i] = []float64{v.X, v.Y, v.Z}
	}
	var out []geom.Vector
	for _, b := range gramSchmidt(in, spanTol) {
		out = append(out, vecAt(b, 0))
	}
	return out
}

// unitNormal is v at length 1.
func unitNormal(v geom.Vector) geom.Vector { return v.ScaledBy(1 / v.Magnitude()) }

// canonicalDir is d or -d, whichever has its largest component
// positive, so that an axis is named the same whichever way a null
// vector ran.
func canonicalDir(d geom.Vector) geom.Vector {
	c := d.X
	if math.Abs(d.Y) > math.Abs(c) {
		c = d.Y
	}
	if math.Abs(d.Z) > math.Abs(c) {
		c = d.Z
	}
	if c < 0 {
		return d.ScaledBy(-1)
	}
	return d
}

// roundVec drops round-off: components within 1e-9 of a whole number
// of 1e-9 are put there, and -0 is 0, so that the text of an invariant
// doesn't show noise.
func roundVec(v geom.Vector) geom.Vector {
	r := func(x float64) float64 {
		y := math.Round(x*1e9) / 1e9
		if y == 0 {
			return 0
		}
		return y
	}
	return geom.Vector{X: r(v.X), Y: r(v.Y), Z: r(v.Z)}
}

// invariantStep is how far, in each part's turn, the check of an
// invariant steps along a freedom: some 0.1°.
const invariantStep = 1e-3

// checkInvariants checks each part's invariant at a second position, a
// small step along each of the motions (for hs), solved; one that moved
// there is no invariant, and its part is marked Moving. The params and
// the drives get their values back after each step.
func (sk *Sketch) checkInvariants(fs *freedomSetting, hs []core.HParam, motions []blockMotion, parts []PartFreedom) {
	if !slices.ContainsFunc(parts, func(p PartFreedom) bool {
		return p.Invariant == InvariantAxis || p.Invariant == InvariantPoint || p.Invariant == InvariantPlane
	}) {
		return
	}
	turnCol := map[int]bool{}
	for _, hg := range fs.block {
		for i := 3; i < 7; i++ {
			if j := slices.Index(hs, hg.Param(i)); j >= 0 {
				turnCol[j] = true
			}
		}
	}
	for _, m := range motions {
		// A step that turns a part by some 0.1°, or moves it by 0.1 mm
		// where nothing turns.
		maxTurn, maxMove := 0.0, 0.0
		for j, x := range m.by {
			if turnCol[j] {
				maxTurn = math.Max(maxTurn, math.Abs(x))
			} else {
				maxMove = math.Max(maxMove, math.Abs(x))
			}
		}
		var eps float64
		switch {
		case maxTurn > 1e-9:
			eps = invariantStep / maxTurn
		case maxMove > 1e-9:
			eps = 0.1 / maxMove
		default:
			continue
		}
		base := 0.0
		if m.drive != "" {
			base = fs.vars[m.drive]
			fs.setDrive(sk, m.drive, base+eps)
		} else {
			for j, h := range hs {
				sk.MustParam(h).Val += eps * m.by[j]
			}
		}
		if sk.solveFreedom(fs) {
			hs2, motions2 := sk.motions(fs)
			there := sk.analyseParts(fs, hs2, motions2)
			for i := range parts {
				p := &parts[i]
				if p.Moving || p.Invariant == InvariantNone || p.Invariant == InvariantDirection {
					continue
				}
				if !sameInvariant(*p, there[i]) {
					p.Moving = true
					p.Invariant, p.Point, p.Dir, p.Lead = InvariantNone, geom.Vector{}, geom.Vector{}, 0
				}
			}
		}
		if m.drive != "" {
			fs.setDrive(sk, m.drive, base)
		}
		fs.restoreParams(sk)
	}
}

// movedTol is how far an invariant may move, in mm or in the sine of a
// turn, in a step of checkInvariants and still be one: a fixed axis
// moves by no more than the solve leaves, a four-bar coupler's by about
// the step times the linkage's size, some 0.1 mm.
const movedTol = 1e-4

// sameInvariant reports whether b's invariant is a's: of one kind, its
// direction the same and its point on a's axis, at a's point or in a's
// plane.
func sameInvariant(a, b PartFreedom) bool {
	if a.Invariant != b.Invariant || a.Turns != b.Turns || a.Slides != b.Slides {
		return false
	}
	off := b.Point.Minus(a.Point)
	switch a.Invariant {
	case InvariantPoint:
		return off.Magnitude() < movedTol
	case InvariantAxis:
		return a.Dir.Cross(b.Dir).Magnitude() < movedTol && off.Cross(a.Dir).Magnitude() < movedTol
	case InvariantPlane:
		return a.Dir.Cross(b.Dir).Magnitude() < movedTol && math.Abs(off.Dot(a.Dir)) < movedTol
	}
	return true
}
