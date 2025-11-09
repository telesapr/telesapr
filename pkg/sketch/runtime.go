package sketch

import (
	"errors"
	"fmt"
	"math"

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

// Entity retrieves an entity by handle.
func (rt *Runtime) Entity(h core.HEntity) (*core.Entity, error) {
	if h == (core.HEntity{}) {
		return nil, nil
	}
	e, ok := rt.entities[h]
	if !ok {
		return nil, fmt.Errorf("entity %08x not found", h.V)
	}
	return e, nil
}

// Param retrieves a parameter by handle.
func (rt *Runtime) Param(h core.HParam) (*core.Param, error) {
	if h == (core.HParam{}) {
		return nil, nil
	}
	p, ok := rt.params[h]
	if !ok {
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

func (rt *Runtime) pointValue(e *core.Entity) (geom.Vector, error) {
	switch e.Type {
	case core.EntityPointIn3D:
		if e.Param[0].V == 0 && e.Param[1].V == 0 && e.Param[2].V == 0 {
			return e.PointGetNum(), nil
		}
		x, err := rt.paramValue(e.Param[0])
		if err != nil {
			return geom.Vector{}, err
		}
		y, err := rt.paramValue(e.Param[1])
		if err != nil {
			return geom.Vector{}, err
		}
		z, err := rt.paramValue(e.Param[2])
		if err != nil {
			return geom.Vector{}, err
		}
		return geom.NewVector(x, y, z), nil

	case core.EntityPointIn2D:
		if e.Param[0].V == 0 && e.Param[1].V == 0 {
			return e.PointGetNum(), nil
		}
		wpEnt, err := rt.Entity(e.Workplane)
		if err != nil {
			return geom.Vector{}, err
		}
		if wpEnt == nil {
			return geom.Vector{}, fmt.Errorf("workplane entity %08x missing", e.Workplane.V)
		}
		origin, err := rt.PointValue(wpEnt.Point[0])
		if err != nil {
			return geom.Vector{}, err
		}
		normalQuat, err := rt.normalQuaternionHandle(wpEnt.Normal)
		if err != nil {
			return geom.Vector{}, err
		}
		u := normalQuat.RotationU()
		v := normalQuat.RotationV()

		uv := geom.Vector{}
		uVal, err := rt.paramValue(e.Param[0])
		if err != nil {
			return geom.Vector{}, err
		}
		vVal, err := rt.paramValue(e.Param[1])
		if err != nil {
			return geom.Vector{}, err
		}

		uv = uv.Plus(u.ScaledBy(uVal)).Plus(v.ScaledBy(vVal))
		return origin.Plus(uv), nil

	default:
		// Fallback to stored value; useful for legacy entity
		// types we do not handle yet.
		return e.PointGetNum(), nil
	}
}

// NormalQuaternion returns the unit quaternion orientation for the entity referenced by h.
func (rt *Runtime) NormalQuaternion(h core.HEntity) (geom.Quaternion, error) {
	return rt.normalQuaternionHandle(h)
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
	switch e.Type {
	case core.EntityNormalIn3D:
		if e.Param[0].V == 0 && e.Param[1].V == 0 && e.Param[2].V == 0 && e.Param[3].V == 0 {
			return e.ActNormal.Normal(), nil
		}
		w, err := rt.paramValue(e.Param[0])
		if err != nil {
			return geom.Quaternion{}, err
		}
		x, err := rt.paramValue(e.Param[1])
		if err != nil {
			return geom.Quaternion{}, err
		}
		y, err := rt.paramValue(e.Param[2])
		if err != nil {
			return geom.Quaternion{}, err
		}
		z, err := rt.paramValue(e.Param[3])
		if err != nil {
			return geom.Quaternion{}, err
		}
		q := geom.NewQuaternion(w, x, y, z).Normal()
		return q, nil

	case core.EntityNormalIn2D:
		if e.Workplane == (core.HEntity{}) {
			return geom.Quaternion{}, fmt.Errorf("normal entity missing workplane")
		}
		wpEnt, err := rt.Entity(e.Workplane)
		if err != nil {
			return geom.Quaternion{}, err
		}
		if wpEnt == nil {
			return geom.Quaternion{}, fmt.Errorf("workplane %08x not found", e.Workplane.V)
		}
		return rt.normalQuaternionHandle(wpEnt.Normal)

	default:
		// fall back to stored orientation
		return e.ActNormal.Normal(), nil
	}
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
	p, ok := rt.params[h]
	if !ok {
		return 0, false
	}
	return p.Val, true
}

// RecalculateEntities recomputes the numerical state for every entity in the
// underlying sketch. The resolver-driven implementation gracefully falls back
// to cached act values when a dependency cannot yet be evaluated.
func (rt *Runtime) RecalculateEntities(forExport bool) {
	for i := range rt.Sketch.Entities {
		_ = rt.Sketch.Entities[i].CalculateNumerical(rt, forExport)
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
	switch e.Type {
	case core.EntityDistance:
		// Some fixtures store the numeric value in a Param with the same handle as the distance entity.
		if p, err := rt.Param(core.HParam{V: e.H.V}); err == nil && p != nil {
			if p.Val != 0 {
				return p.Val, nil
			}
		}
		val, err := rt.paramValue(e.Param[0])
		if err != nil {
			return 0, err
		}
		return val, nil
	default:
		if math.Abs(e.ActDistance) > 0 {
			return e.ActDistance, nil
		}
		return 0, fmt.Errorf("unsupported distance entity type %d", e.Type)
	}
}
