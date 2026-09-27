package core

import (
	"fmt"
	"math"

	"github.com/spacesolve/spacesolve/pkg/geom"
)

// Operation identifies the symbolic operation performed by an Expr.
type Operation int

const (
	OpParam Operation = iota
	OpParamPtr
	OpConstant
	OpVariable

	OpPlus
	OpMinus
	OpTimes
	OpDiv
	OpPow
	OpNegate
	OpSqrt
	OpSquare
	OpSin
	OpCos
	OpTan
	OpASin
	OpACos
	OpATan
	OpATan2
	OpHypot
	OpExp
	OpLn
	OpLog
	OpMin
	OpMax
	OpAbs
	OpFloor
	OpCeil
	OpRound
)

// Expr represents a node in the symbolic expression tree used for constraint equations.
type Expr struct {
	Op        Operation
	Left      *Expr
	Right     *Expr
	Args      []*Expr // variadic args for OpMin/OpMax
	Value     float64
	Param     HParam
	ParamPtr  *Param
	VarID     string
	constFold bool
}

// NewConstant returns an Expr representing a literal value.
func NewConstant(v float64) *Expr {
	return &Expr{Op: OpConstant, Value: v, constFold: true}
}

// NewParam returns an Expr referencing the supplied parameter handle.
func NewParam(h HParam) *Expr {
	return &Expr{Op: OpParam, Param: h}
}

// NewParamPtr returns an Expr referencing a parameter by pointer.
func NewParamPtr(p *Param) *Expr {
	return &Expr{Op: OpParamPtr, ParamPtr: p}
}

// NewVariable returns an Expr representing a symbolic variable identified by name.
func NewVariable(id string) *Expr {
	return &Expr{Op: OpVariable, VarID: id}
}

func newUnary(op Operation, child *Expr) *Expr {
	return &Expr{Op: op, Left: child}
}

func newBinary(op Operation, left, right *Expr) *Expr {
	return &Expr{Op: op, Left: left, Right: right}
}

func (e *Expr) clone() *Expr {
	if e == nil {
		return nil
	}
	out := &Expr{
		Op:        e.Op,
		Left:      e.Left,
		Right:     e.Right,
		Value:     e.Value,
		Param:     e.Param,
		ParamPtr:  e.ParamPtr,
		VarID:     e.VarID,
		constFold: e.constFold,
	}
	if len(e.Args) > 0 {
		out.Args = make([]*Expr, len(e.Args))
		copy(out.Args, e.Args)
	}
	return out
}

// DeepCopy returns a deep copy of the expression tree.
func (e *Expr) DeepCopy() *Expr {
	if e == nil {
		return nil
	}
	copy := e.clone()
	if e.Left != nil {
		copy.Left = e.Left.DeepCopy()
	}
	if e.Right != nil {
		copy.Right = e.Right.DeepCopy()
	}
	if len(e.Args) > 0 {
		copy.Args = make([]*Expr, len(e.Args))
		for i, a := range e.Args {
			if a != nil {
				copy.Args[i] = a.DeepCopy()
			}
		}
	}
	return copy
}

// EvalParamProvider supplies parameter values for expression evaluation.
type EvalParamProvider interface {
	ParamValue(HParam) (float64, bool)
}

// Eval computes the numeric value of the expression using the provided context.
func (e *Expr) Eval(ctx EvalParamProvider, vars map[string]float64) float64 {
	if e == nil {
		return 0
	}

	switch e.Op {
	case OpConstant:
		return e.Value
	case OpParam:
		if ctx == nil {
			panic("expr: ParamValue required for evaluation")
		}
		val, ok := ctx.ParamValue(e.Param)
		if !ok {
			panic(fmt.Sprintf("expr: missing parameter %08x", e.Param.V))
		}
		return val
	case OpParamPtr:
		if e.ParamPtr == nil {
			panic("expr: nil ParamPtr")
		}
		return e.ParamPtr.Val
	case OpVariable:
		if vars == nil {
			panic(fmt.Sprintf("expr: variable %q missing", e.VarID))
		}
		val, ok := vars[e.VarID]
		if !ok {
			panic(fmt.Sprintf("expr: variable %q missing", e.VarID))
		}
		return val
	case OpPlus:
		return e.Left.Eval(ctx, vars) + e.Right.Eval(ctx, vars)
	case OpMinus:
		return e.Left.Eval(ctx, vars) - e.Right.Eval(ctx, vars)
	case OpTimes:
		return e.Left.Eval(ctx, vars) * e.Right.Eval(ctx, vars)
	case OpDiv:
		r := e.Right.Eval(ctx, vars)
		return e.Left.Eval(ctx, vars) / r
	case OpNegate:
		return -e.Left.Eval(ctx, vars)
	case OpSqrt:
		return math.Sqrt(e.Left.Eval(ctx, vars))
	case OpSquare:
		v := e.Left.Eval(ctx, vars)
		return v * v
	case OpSin:
		return math.Sin(e.Left.Eval(ctx, vars))
	case OpCos:
		return math.Cos(e.Left.Eval(ctx, vars))
	case OpASin:
		return math.Asin(e.Left.Eval(ctx, vars))
	case OpACos:
		return math.Acos(e.Left.Eval(ctx, vars))
	case OpPow:
		return math.Pow(e.Left.Eval(ctx, vars), e.Right.Eval(ctx, vars))
	case OpTan:
		return math.Tan(e.Left.Eval(ctx, vars))
	case OpATan:
		return math.Atan(e.Left.Eval(ctx, vars))
	case OpATan2:
		return math.Atan2(e.Left.Eval(ctx, vars), e.Right.Eval(ctx, vars))
	case OpHypot:
		return math.Hypot(e.Left.Eval(ctx, vars), e.Right.Eval(ctx, vars))
	case OpExp:
		return math.Exp(e.Left.Eval(ctx, vars))
	case OpLn:
		return math.Log(e.Left.Eval(ctx, vars))
	case OpLog:
		return math.Log10(e.Left.Eval(ctx, vars))
	case OpAbs:
		return math.Abs(e.Left.Eval(ctx, vars))
	case OpFloor:
		return math.Floor(e.Left.Eval(ctx, vars))
	case OpCeil:
		return math.Ceil(e.Left.Eval(ctx, vars))
	case OpRound:
		return math.Round(e.Left.Eval(ctx, vars))
	case OpMin:
		if len(e.Args) == 0 {
			return 0
		}
		v := math.Inf(1)
		for _, a := range e.Args {
			x := a.Eval(ctx, vars)
			if x < v {
				v = x
			}
		}
		return v
	case OpMax:
		if len(e.Args) == 0 {
			return 0
		}
		v := math.Inf(-1)
		for _, a := range e.Args {
			x := a.Eval(ctx, vars)
			if x > v {
				v = x
			}
		}
		return v
	default:
		panic(fmt.Sprintf("expr: unsupported op %d", e.Op))
	}
}

// Plus returns an expression representing a + b.
func (e *Expr) Plus(b *Expr) *Expr {
	return newBinary(OpPlus, e, b)
}

// Minus returns an expression representing a - b.
func (e *Expr) Minus(b *Expr) *Expr {
	return newBinary(OpMinus, e, b)
}

// Times returns an expression representing a * b.
func (e *Expr) Times(b *Expr) *Expr {
	return newBinary(OpTimes, e, b)
}

// Div returns an expression representing a / b.
func (e *Expr) Div(b *Expr) *Expr {
	return newBinary(OpDiv, e, b)
}

// Negate returns the negated expression.
func (e *Expr) Negate() *Expr {
	return newUnary(OpNegate, e)
}

// Sqrt returns sqrt of the expression.
func (e *Expr) Sqrt() *Expr {
	return newUnary(OpSqrt, e)
}

// Square returns square of expression.
func (e *Expr) Square() *Expr {
	return newUnary(OpSquare, e)
}

// Pow returns the expression raised to the power of another expression.
func (e *Expr) Pow(b *Expr) *Expr {
	return newBinary(OpPow, e, b)
}

// Sin returns sin(expression).
func (e *Expr) Sin() *Expr {
	return newUnary(OpSin, e)
}

// Cos returns cos(expression).
func (e *Expr) Cos() *Expr {
	return newUnary(OpCos, e)
}

// Tan returns tan(expression).
func (e *Expr) Tan() *Expr {
	return newUnary(OpTan, e)
}

// ASin returns arcsin(expression).
func (e *Expr) ASin() *Expr {
	return newUnary(OpASin, e)
}

// ACos returns arccos(expression).
func (e *Expr) ACos() *Expr {
	return newUnary(OpACos, e)
}

// ATan returns arctan(expression).
func (e *Expr) ATan() *Expr {
	return newUnary(OpATan, e)
}

// ATan2 returns arctan2(y, x).
func (e *Expr) ATan2(x *Expr) *Expr {
	return newBinary(OpATan2, e, x)
}

// Hypot returns sqrt(x*x + y*y) without intermediate overflow.
func (e *Expr) Hypot(x *Expr) *Expr {
	return newBinary(OpHypot, e, x)
}

// Exp returns e^expression.
func (e *Expr) Exp() *Expr {
	return newUnary(OpExp, e)
}

// Ln returns natural log of expression.
func (e *Expr) Ln() *Expr {
	return newUnary(OpLn, e)
}

// Log returns base-10 log of expression.
func (e *Expr) Log() *Expr {
	return newUnary(OpLog, e)
}

// Abs returns |expression|.
func (e *Expr) Abs() *Expr {
	return newUnary(OpAbs, e)
}

// Floor returns floor(expression).
func (e *Expr) Floor() *Expr {
	return newUnary(OpFloor, e)
}

// Ceil returns ceil(expression).
func (e *Expr) Ceil() *Expr {
	return newUnary(OpCeil, e)
}

// Round returns the nearest integer.
func (e *Expr) Round() *Expr {
	return newUnary(OpRound, e)
}

// Min returns an OpMin variadic expr.
func MinExpr(args ...*Expr) *Expr {
	return &Expr{Op: OpMin, Args: args}
}

// Max returns an OpMax variadic expr.
func MaxExpr(args ...*Expr) *Expr {
	return &Expr{Op: OpMax, Args: args}
}

// IsZeroConst reports whether the expression structurally equals zero.
func (e *Expr) IsZeroConst() bool {
	return e != nil && e.Op == OpConstant && math.Abs(e.Value) <= LengthEps
}

// DependsOn reports whether the expression references the provided parameter.
func (e *Expr) DependsOn(h HParam) bool {
	if e == nil {
		return false
	}
	switch e.Op {
	case OpParam:
		return e.Param == h
	case OpParamPtr:
		if e.ParamPtr == nil {
			return false
		}
		return e.ParamPtr.H == h
	}
	for _, a := range e.Args {
		if a.DependsOn(h) {
			return true
		}
	}
	return e.Left.DependsOn(h) || e.Right.DependsOn(h)
}

// ReferencedParams collects handles referenced by the expression.
func (e *Expr) ReferencedParams(set map[HParam]struct{}) {
	if e == nil {
		return
	}
	switch e.Op {
	case OpParam:
		set[e.Param] = struct{}{}
	case OpParamPtr:
		if e.ParamPtr != nil {
			set[e.ParamPtr.H] = struct{}{}
		}
	}
	for _, a := range e.Args {
		a.ReferencedParams(set)
	}
	if e.Left != nil {
		e.Left.ReferencedParams(set)
	}
	if e.Right != nil {
		e.Right.ReferencedParams(set)
	}
}

// FoldConstants simplifies the expression by evaluating constant subtrees.
func (e *Expr) FoldConstants() *Expr {
	if e == nil {
		return nil
	}
	var left, right *Expr
	if e.Left != nil {
		left = e.Left.FoldConstants()
	}
	if e.Right != nil {
		right = e.Right.FoldConstants()
	}

	switch e.Op {
	case OpPlus, OpMinus, OpTimes, OpDiv:
		if left != nil && right != nil && left.Op == OpConstant && right.Op == OpConstant {
			switch e.Op {
			case OpPlus:
				return NewConstant(left.Value + right.Value)
			case OpMinus:
				return NewConstant(left.Value - right.Value)
			case OpTimes:
				return NewConstant(left.Value * right.Value)
			case OpDiv:
				return NewConstant(left.Value / right.Value)
			}
		}
	case OpNegate, OpSqrt, OpSquare, OpSin, OpCos, OpASin, OpACos, OpTan, OpATan,
		OpExp, OpLn, OpLog, OpAbs, OpFloor, OpCeil, OpRound:
		if left != nil && left.Op == OpConstant {
			switch e.Op {
			case OpNegate:
				return NewConstant(-left.Value)
			case OpSqrt:
				return NewConstant(math.Sqrt(left.Value))
			case OpSquare:
				return NewConstant(left.Value * left.Value)
			case OpSin:
				return NewConstant(math.Sin(left.Value))
			case OpCos:
				return NewConstant(math.Cos(left.Value))
			case OpTan:
				return NewConstant(math.Tan(left.Value))
			case OpASin:
				return NewConstant(math.Asin(left.Value))
			case OpACos:
				return NewConstant(math.Acos(left.Value))
			case OpATan:
				return NewConstant(math.Atan(left.Value))
			case OpExp:
				return NewConstant(math.Exp(left.Value))
			case OpLn:
				return NewConstant(math.Log(left.Value))
			case OpLog:
				return NewConstant(math.Log10(left.Value))
			case OpAbs:
				return NewConstant(math.Abs(left.Value))
			case OpFloor:
				return NewConstant(math.Floor(left.Value))
			case OpCeil:
				return NewConstant(math.Ceil(left.Value))
			case OpRound:
				return NewConstant(math.Round(left.Value))
			}
		}
	case OpPow, OpATan2, OpHypot:
		if left != nil && right != nil && left.Op == OpConstant && right.Op == OpConstant {
			lv := left.Value
			rv := right.Value
			switch e.Op {
			case OpPow:
				return NewConstant(math.Pow(lv, rv))
			case OpATan2:
				return NewConstant(math.Atan2(lv, rv))
			case OpHypot:
				return NewConstant(math.Hypot(lv, rv))
			}
		}
	case OpMin, OpMax:
		allConst := true
		for _, a := range e.Args {
			if a == nil || a.Op != OpConstant {
				allConst = false
				break
			}
		}
		if allConst && len(e.Args) > 0 {
			best := e.Args[0].Value
			for _, a := range e.Args[1:] {
				v := a.Value
				if e.Op == OpMin && v < best {
					best = v
				}
				if e.Op == OpMax && v > best {
					best = v
				}
			}
			return NewConstant(best)
		}
	}

	out := e.clone()
	out.Left = left
	out.Right = right
	return out
}

// Derivative returns the symbolic derivative with respect to param h.
func (e *Expr) Derivative(h HParam) *Expr {
	if e == nil {
		return NewConstant(0)
	}
	switch e.Op {
	case OpConstant:
		return NewConstant(0)
	case OpParam:
		if e.Param == h {
			return NewConstant(1)
		}
		return NewConstant(0)
	case OpParamPtr:
		if e.ParamPtr != nil && e.ParamPtr.H == h {
			return NewConstant(1)
		}
		return NewConstant(0)
	case OpPlus:
		return e.Left.Derivative(h).Plus(e.Right.Derivative(h))
	case OpMinus:
		return e.Left.Derivative(h).Minus(e.Right.Derivative(h))
	case OpTimes:
		return e.Left.Derivative(h).Times(e.Right).Plus(e.Left.Times(e.Right.Derivative(h)))
	case OpDiv:
		num := e.Left.Derivative(h).Times(e.Right).Minus(e.Left.Times(e.Right.Derivative(h)))
		den := e.Right.Square()
		return num.Div(den)
	case OpNegate:
		return e.Left.Derivative(h).Negate()
	case OpSqrt:
		return e.Left.Derivative(h).Div(e.Left.Sqrt().Times(NewConstant(2)))
	case OpSquare:
		return NewConstant(2).Times(e.Left).Times(e.Left.Derivative(h))
	case OpSin:
		return e.Left.Derivative(h).Times(e.Left.Cos())
	case OpCos:
		return e.Left.Derivative(h).Times(e.Left.Sin()).Negate()
	case OpASin:
		return e.Left.Derivative(h).Div(NewConstant(1).Minus(e.Left.Square()).Sqrt())
	case OpACos:
		return e.Left.Derivative(h).Div(NewConstant(1).Minus(e.Left.Square()).Sqrt()).Negate()
	default:
		panic(fmt.Sprintf("expr: derivative for op %d not implemented", e.Op))
	}
}

// Substitute replaces occurrences of parameters according to the map.
func (e *Expr) Substitute(mapping map[HParam]*Param) {
	if e == nil {
		return
	}
	switch e.Op {
	case OpParam:
		if p, ok := mapping[e.Param]; ok {
			e.Op = OpParamPtr
			e.ParamPtr = p
		}
	case OpParamPtr:
		if e.ParamPtr != nil {
			if p, ok := mapping[e.ParamPtr.H]; ok {
				e.ParamPtr = p
			}
		}
	}
	if e.Left != nil {
		e.Left.Substitute(mapping)
	}
	if e.Right != nil {
		e.Right.Substitute(mapping)
	}
}

// Magnitude returns sqrt(x^2 + y^2 + z^2).
func (e *Expr) Magnitude() *Expr {
	return e.Square().Sqrt()
}

// ExprVector represents a 3-element vector of expressions.
type ExprVector struct {
	X, Y, Z *Expr
}

// NewExprVector constructs an ExprVector.
func NewExprVector(x, y, z *Expr) ExprVector {
	return ExprVector{X: x, Y: y, Z: z}
}

// FromVector returns an ExprVector from a numeric vector.
func FromVector(v geom.Vector) ExprVector {
	return ExprVector{
		X: NewConstant(v.X),
		Y: NewConstant(v.Y),
		Z: NewConstant(v.Z),
	}
}

// Plus returns component-wise addition.
func (v ExprVector) Plus(b ExprVector) ExprVector {
	return ExprVector{
		X: v.X.Plus(b.X),
		Y: v.Y.Plus(b.Y),
		Z: v.Z.Plus(b.Z),
	}
}

// Minus returns component-wise subtraction.
func (v ExprVector) Minus(b ExprVector) ExprVector {
	return ExprVector{
		X: v.X.Minus(b.X),
		Y: v.Y.Minus(b.Y),
		Z: v.Z.Minus(b.Z),
	}
}

// Dot returns the dot product.
func (v ExprVector) Dot(b ExprVector) *Expr {
	return v.X.Times(b.X).Plus(v.Y.Times(b.Y)).Plus(v.Z.Times(b.Z))
}

// Cross returns the cross product.
func (v ExprVector) Cross(b ExprVector) ExprVector {
	return ExprVector{
		X: v.Y.Times(b.Z).Minus(v.Z.Times(b.Y)),
		Y: v.Z.Times(b.X).Minus(v.X.Times(b.Z)),
		Z: v.X.Times(b.Y).Minus(v.Y.Times(b.X)),
	}
}

// ScaledBy scales the vector by s.
func (v ExprVector) ScaledBy(s *Expr) ExprVector {
	return ExprVector{
		X: v.X.Times(s),
		Y: v.Y.Times(s),
		Z: v.Z.Times(s),
	}
}

// WithMagnitude returns vector scaled to the provided magnitude.
func (v ExprVector) WithMagnitude(mag *Expr) ExprVector {
	return v.ScaledBy(mag.Div(v.Magnitude()))
}

// Magnitude returns sqrt of sum of squares.
func (v ExprVector) Magnitude() *Expr {
	return v.Dot(v).Sqrt()
}

// ExprQuaternion represents quaternion-valued expressions.
type ExprQuaternion struct {
	W, X, Y, Z *Expr
}

// NewExprQuaternion constructs a quaternion.
func NewExprQuaternion(w, x, y, z *Expr) ExprQuaternion {
	return ExprQuaternion{W: w, X: x, Y: y, Z: z}
}

// ParamHandle returns the parameter handle referenced by the expression, if any.
func (e *Expr) ParamHandle() (HParam, bool) {
	if e == nil {
		return HParam{}, false
	}
	switch e.Op {
	case OpParam:
		return e.Param, true
	case OpParamPtr:
		if e.ParamPtr != nil {
			return e.ParamPtr.H, true
		}
		return HParam{}, false
	default:
		return HParam{}, false
	}
}

// NewExprQuaternionConst constructs a quaternion with constant components.
func NewExprQuaternionConst(q geom.Quaternion) ExprQuaternion {
	return ExprQuaternion{
		W: NewConstant(q.W),
		X: NewConstant(q.X),
		Y: NewConstant(q.Y),
		Z: NewConstant(q.Z),
	}
}

// Rotate expresses p in the basis (RotationU, RotationV, RotationN),
// as C++ ExprQuaternion::Rotate does.
func (q ExprQuaternion) Rotate(p ExprVector) ExprVector {
	return q.RotationU().ScaledBy(p.X).Plus(
		q.RotationV().ScaledBy(p.Y)).Plus(
		q.RotationN().ScaledBy(p.Z))
}

// RotationU returns the rotated unit X axis (C++ ExprQuaternion::RotationU).
func (q ExprQuaternion) RotationU() ExprVector {
	two := NewConstant(2)
	var u ExprVector
	u.X = q.W.Square()
	u.X = u.X.Plus(q.X.Square())
	u.X = u.X.Minus(q.Y.Square())
	u.X = u.X.Minus(q.Z.Square())

	u.Y = two.Times(q.W.Times(q.Z))
	u.Y = u.Y.Plus(two.Times(q.X.Times(q.Y)))

	u.Z = two.Times(q.X.Times(q.Z))
	u.Z = u.Z.Minus(two.Times(q.W.Times(q.Y)))
	return u
}

// RotationV returns the rotated unit Y axis (C++ ExprQuaternion::RotationV).
func (q ExprQuaternion) RotationV() ExprVector {
	two := NewConstant(2)
	var v ExprVector
	v.X = two.Times(q.X.Times(q.Y))
	v.X = v.X.Minus(two.Times(q.W.Times(q.Z)))

	v.Y = q.W.Square()
	v.Y = v.Y.Minus(q.X.Square())
	v.Y = v.Y.Plus(q.Y.Square())
	v.Y = v.Y.Minus(q.Z.Square())

	v.Z = two.Times(q.W.Times(q.X))
	v.Z = v.Z.Plus(two.Times(q.Y.Times(q.Z)))
	return v
}

// RotationN returns the rotated unit Z axis (C++ ExprQuaternion::RotationN).
func (q ExprQuaternion) RotationN() ExprVector {
	two := NewConstant(2)
	var n ExprVector
	n.X = two.Times(q.W.Times(q.Y))
	n.X = n.X.Plus(two.Times(q.X.Times(q.Z)))

	n.Y = two.Times(q.Y.Times(q.Z))
	n.Y = n.Y.Minus(two.Times(q.W.Times(q.X)))

	n.Z = q.W.Square()
	n.Z = n.Z.Minus(q.X.Square())
	n.Z = n.Z.Minus(q.Y.Square())
	n.Z = n.Z.Plus(q.Z.Square())
	return n
}

// Times multiplies two quaternions (C++ ExprQuaternion::Times).
func (q ExprQuaternion) Times(b ExprQuaternion) ExprQuaternion {
	sa, sb := q.W, b.W
	va := ExprVector{X: q.X, Y: q.Y, Z: q.Z}
	vb := ExprVector{X: b.X, Y: b.Y, Z: b.Z}

	var r ExprQuaternion
	r.W = sa.Times(sb).Minus(va.Dot(vb))
	vr := vb.ScaledBy(sa).Plus(va.ScaledBy(sb).Plus(va.Cross(vb)))
	r.X = vr.X
	r.Y = vr.Y
	r.Z = vr.Z
	return r
}

// Magnitude returns sqrt(w^2 + x^2 + y^2 + z^2).
func (q ExprQuaternion) Magnitude() *Expr {
	return q.W.Square().
		Plus(q.X.Square()).
		Plus(q.Y.Square()).
		Plus(q.Z.Square()).
		Sqrt()
}
