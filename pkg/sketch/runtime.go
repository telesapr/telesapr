package sketch

import (
	"errors"
	"fmt"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
)

// Runtime provides quick handle-based lookup for a Sketch.
type Runtime struct {
	Sketch *Sketch

	groups      map[core.HGroup]*Group
	requests    map[core.HRequest]*Request
	entities    map[core.HEntity]*core.Entity
	params      map[core.HParam]*core.Param
	constraints map[core.HConstraint]*core.Constraint
	styles      map[core.HStyle]*core.Style
}

// NewRuntime creates a runtime lookup wrapper for the provided sketch.
func NewRuntime(sk *Sketch) *Runtime {
	rt := &Runtime{
		Sketch: sk,

		groups:      make(map[core.HGroup]*Group, len(sk.Groups)),
		requests:    make(map[core.HRequest]*Request, len(sk.Requests)),
		entities:    make(map[core.HEntity]*core.Entity, len(sk.Entities)),
		params:      make(map[core.HParam]*core.Param, len(sk.Params)),
		constraints: make(map[core.HConstraint]*core.Constraint, len(sk.Constraints)),
		styles:      make(map[core.HStyle]*core.Style, len(sk.Styles)),
	}
	rt.refresh()
	return rt
}

// Rebuild refreshes all handle indexes after the sketch mutates.
func (rt *Runtime) Rebuild() {
	rt.refresh()
}

func (rt *Runtime) refresh() {
	rt.groups = make(map[core.HGroup]*Group, len(rt.Sketch.Groups))
	for i := range rt.Sketch.Groups {
		g := &rt.Sketch.Groups[i]
		rt.groups[g.H] = g
	}

	rt.requests = make(map[core.HRequest]*Request, len(rt.Sketch.Requests))
	for i := range rt.Sketch.Requests {
		r := &rt.Sketch.Requests[i]
		rt.requests[r.H] = r
	}

	rt.entities = make(map[core.HEntity]*core.Entity, len(rt.Sketch.Entities))
	for i := range rt.Sketch.Entities {
		e := &rt.Sketch.Entities[i]
		rt.entities[e.H] = e
	}

	rt.params = make(map[core.HParam]*core.Param, len(rt.Sketch.Params))
	for i := range rt.Sketch.Params {
		p := &rt.Sketch.Params[i]
		rt.params[p.H] = p
	}

	rt.constraints = make(map[core.HConstraint]*core.Constraint, len(rt.Sketch.Constraints))
	for i := range rt.Sketch.Constraints {
		c := &rt.Sketch.Constraints[i]
		rt.constraints[c.H] = c
	}

	rt.styles = make(map[core.HStyle]*core.Style, len(rt.Sketch.Styles))
	for i := range rt.Sketch.Styles {
		s := &rt.Sketch.Styles[i]
		rt.styles[s.H] = s
	}
}

// Group retrieves a group by handle.
func (rt *Runtime) Group(h core.HGroup) (*Group, error) {
	if h == (core.HGroup{}) {
		return nil, nil
	}
	g, ok := rt.groups[h]
	if !ok {
		return nil, fmt.Errorf("group %08x not found", h.V)
	}
	return g, nil
}

// Request retrieves a request by handle.
func (rt *Runtime) Request(h core.HRequest) (*Request, error) {
	if h == (core.HRequest{}) {
		return nil, nil
	}
	r, ok := rt.requests[h]
	if !ok {
		return nil, fmt.Errorf("request %08x not found", h.V)
	}
	return r, nil
}

// Entity retrieves an entity by handle. It reads the sketch live, so
// it sees entities GenerateAll has regenerated since NewRuntime.
func (rt *Runtime) Entity(h core.HEntity) (*core.Entity, error) {
	if h == (core.HEntity{}) {
		return nil, nil
	}
	e := rt.Sketch.GetEntity(h)
	if e == nil {
		return nil, fmt.Errorf("entity %08x not found", h.V)
	}
	return e, nil
}

// Param retrieves a parameter by handle, reading the sketch live.
func (rt *Runtime) Param(h core.HParam) (*core.Param, error) {
	if h == (core.HParam{}) {
		return nil, nil
	}
	p := rt.Sketch.GetParam(h)
	if p == nil {
		return nil, fmt.Errorf("param %08x not found", h.V)
	}
	return p, nil
}

// Constraint retrieves a constraint by handle.
func (rt *Runtime) Constraint(h core.HConstraint) (*core.Constraint, error) {
	if h == (core.HConstraint{}) {
		return nil, nil
	}
	c, ok := rt.constraints[h]
	if !ok {
		return nil, fmt.Errorf("constraint %08x not found", h.V)
	}
	return c, nil
}

// Style retrieves a style by handle.
func (rt *Runtime) Style(h core.HStyle) (*core.Style, error) {
	if h == (core.HStyle{}) {
		return nil, nil
	}
	s, ok := rt.styles[h]
	if !ok {
		return nil, fmt.Errorf("style %08x not found", h.V)
	}
	return s, nil
}

// PointValue returns the numeric location for the entity referenced by h.
func (rt *Runtime) PointValue(h core.HEntity) (geom.Vector, error) {
	e, err := rt.Entity(h)
	if err != nil {
		return geom.Vector{}, err
	}
	if e == nil {
		return geom.Vector{}, errors.New("nil entity")
	}
	return rt.pointValue(e)
}

// Point is an alias for PointValue that satisfies the
// EntityBezierResolver interface.
func (rt *Runtime) Point(h core.HEntity) (geom.Vector, error) {
	return rt.PointValue(h)
}

// numeric runs fn, turning a panic from a malformed entity (a missing
// entity or param, which C++ asserts on) into an error.
func numeric[T any](e *core.Entity, fn func() T) (r T, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("entity %08x: %v", e.H.V, rec)
		}
	}()
	return fn(), nil
}

func (rt *Runtime) pointValue(e *core.Entity) (geom.Vector, error) {
	// A POINT_IN_3D without params (hand-built test fixtures only; C++
	// always gives it three) is a fixed point.
	if e.Type == core.EntityPointIn3D && e.Param[0].V == 0 {
		return e.ActPoint, nil
	}
	if !e.IsPoint() {
		return geom.Vector{}, fmt.Errorf("entity %08x (type %d) is not a point", e.H.V, e.Type)
	}
	return numeric(e, func() geom.Vector { return e.PointGetNum(rt.Sketch) })
}

// NormalQuaternion returns the unit quaternion orientation for the entity referenced by h.
func (rt *Runtime) NormalQuaternion(h core.HEntity) (geom.Quaternion, error) {
	return rt.normalQuaternionHandle(h)
}

// Normal is an alias for NormalQuaternion that satisfies the
// EntityBezierResolver interface.
func (rt *Runtime) Normal(h core.HEntity) (geom.Quaternion, error) {
	return rt.normalQuaternionHandle(h)
}

// Radius returns the radius of a circle/arc entity.
func (rt *Runtime) Radius(h core.HEntity) (float64, error) {
	return rt.DistanceValue(h)
}

func (rt *Runtime) normalQuaternionHandle(h core.HEntity) (geom.Quaternion, error) {
	e, err := rt.Entity(h)
	if err != nil {
		return geom.Quaternion{}, err
	}
	if e == nil {
		return geom.Quaternion{}, fmt.Errorf("normal entity %08x not found", h.V)
	}
	return rt.normalQuaternion(e)
}

func (rt *Runtime) normalQuaternion(e *core.Entity) (geom.Quaternion, error) {
	// A NORMAL_IN_3D without params (hand-built test fixtures only) is a
	// fixed orientation.
	if e.Type == core.EntityNormalIn3D && e.Param[0].V == 0 {
		return e.ActNormal, nil
	}
	if !e.IsNormal() {
		return geom.Quaternion{}, fmt.Errorf("entity %08x (type %d) is not a normal", e.H.V, e.Type)
	}
	return numeric(e, func() geom.Quaternion { return e.NormalGetNum(rt.Sketch) })
}

func (rt *Runtime) paramValue(h core.HParam) (float64, error) {
	if h == (core.HParam{}) {
		return 0, nil
	}
	p, err := rt.Param(h)
	if err != nil {
		return 0, err
	}
	return p.Val, nil
}

// AlmostEqualVector reports whether two vectors are within tolerance.
func AlmostEqualVector(a, b geom.Vector, tol float64) bool {
	return a.DistanceTo(b) <= tol
}

// AlmostEqualQuaternion returns whether two quaternions represent the same rotation.
func AlmostEqualQuaternion(a, b geom.Quaternion, tol float64) bool {
	diff := geom.NewQuaternion(a.W-b.W, a.X-b.X, a.Y-b.Y, a.Z-b.Z)
	return diff.Magnitude() <= tol
}

// ParamValue implements core.NumericalResolver, returning the numeric value for
// the supplied parameter handle.
func (rt *Runtime) ParamValue(h core.HParam) (float64, bool) {
	if h == (core.HParam{}) {
		return 0, false
	}
	p := rt.Sketch.GetParam(h)
	if p == nil {
		return 0, false
	}
	return p.Val, true
}

// RecalculateEntities caches every entity's numeric state in its Act*
// fields (Entity::CalculateNumerical). Entities that can't be
// evaluated (missing params, as in hand-built fixtures) keep their
// cached values.
func (rt *Runtime) RecalculateEntities(forExport bool) {
	for i := range rt.Sketch.Entities {
		e := &rt.Sketch.Entities[i]
		_, _ = numeric(e, func() bool {
			e.CalculateNumerical(rt.Sketch, forExport)
			return true
		})
	}
}

// DistanceValue returns numeric distance for distance entity types.
func (rt *Runtime) DistanceValue(h core.HEntity) (float64, error) {
	e, err := rt.Entity(h)
	if err != nil {
		return 0, err
	}
	if e == nil {
		return 0, fmt.Errorf("distance entity %08x not found", h.V)
	}
	if e.Type == core.EntityDistance && e.Param[0].V == 0 {
		return e.ActDistance, nil
	}
	if !e.IsDistance() {
		return 0, fmt.Errorf("entity %08x (type %d) is not a distance", e.H.V, e.Type)
	}
	return numeric(e, func() float64 { return e.DistanceGetNum(rt.Sketch) })
}
