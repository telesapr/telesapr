package step

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// Model is the boundary representation of a STEP file, in millimetres:
// the shapes it holds and where the assembly places each.
type Model struct {
	Shapes []*Shape
	// Placements put the shapes into the file's top-level space; a part
	// used twice in an assembly has two.
	Placements []Placement
	// Unit is the length unit the file is written in.
	Unit string
	// unsupported counts what kept faces or shapes out, by reason.
	unsupported map[string]int
	// errDetail is the first error of each reason, for the report.
	errDetail map[string]string
}

// Placement is one use of a shape.
type Placement struct {
	Shape int
	T     Transform
}

// Shape is a solid or a shell: a list of faces.
type Shape struct {
	ID    int
	Kind  string
	Faces []*Face
}

// Face is one face of a shape: a surface trimmed by its bounds. Sense
// tells whether the face's normal is the surface's.
type Face struct {
	ID     int
	Surf   Surface
	Sense  bool
	Bounds []*Bound
	Color  core.RgbaColor
	// HasColor is set when the file gave the face, or its shape, a
	// colour.
	HasColor bool
}

// Bound is a loop of edges, or the corners of a polygon.
type Bound struct {
	Edges []OrientedEdge
	// Poly holds a POLY_LOOP's corners, in order.
	Poly []geom.Vector
}

// OrientedEdge is an edge as a loop runs along it.
type OrientedEdge struct {
	E       *Edge
	Forward bool
}

// Edge is an edge of a shape, shared by the faces it bounds, so both
// meet along the same points.
type Edge struct {
	ID         int
	Start, End geom.Vector
	// Closed is set when the edge starts and ends at one vertex.
	Closed bool
	Curve  Curve
	// Sense tells whether the edge runs along the curve's parameter.
	Sense bool

	once sync.Once
	poly []geom.Vector
	err  error
}

// Transform maps p to M p + T.
type Transform struct {
	M [3][3]float64
	T geom.Vector
}

// Identity is the transform that changes nothing.
func Identity() Transform {
	return Transform{M: [3][3]float64{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}}
}

// Apply transforms a point.
func (t Transform) Apply(p geom.Vector) geom.Vector {
	return t.ApplyDir(p).Plus(t.T)
}

// ApplyDir transforms a direction, without the translation.
func (t Transform) ApplyDir(p geom.Vector) geom.Vector {
	m := &t.M
	return geom.Vector{
		X: m[0][0]*p.X + m[0][1]*p.Y + m[0][2]*p.Z,
		Y: m[1][0]*p.X + m[1][1]*p.Y + m[1][2]*p.Z,
		Z: m[2][0]*p.X + m[2][1]*p.Y + m[2][2]*p.Z,
	}
}

// Then is t followed by u.
func (t Transform) Then(u Transform) Transform {
	var r Transform
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			for k := 0; k < 3; k++ {
				r.M[i][j] += u.M[i][k] * t.M[k][j]
			}
		}
	}
	r.T = u.Apply(t.T)
	return r
}

// Det is the determinant of the linear part; negative for a mirror.
func (t Transform) Det() float64 {
	m := &t.M
	return m[0][0]*(m[1][1]*m[2][2]-m[1][2]*m[2][1]) -
		m[0][1]*(m[1][0]*m[2][2]-m[1][2]*m[2][0]) +
		m[0][2]*(m[1][0]*m[2][1]-m[1][1]*m[2][0])
}

// Inverse is the transform that undoes t, which must not be singular.
func (t Transform) Inverse() Transform {
	m := &t.M
	d := t.Det()
	var r Transform
	r.M[0][0] = (m[1][1]*m[2][2] - m[1][2]*m[2][1]) / d
	r.M[0][1] = (m[0][2]*m[2][1] - m[0][1]*m[2][2]) / d
	r.M[0][2] = (m[0][1]*m[1][2] - m[0][2]*m[1][1]) / d
	r.M[1][0] = (m[1][2]*m[2][0] - m[1][0]*m[2][2]) / d
	r.M[1][1] = (m[0][0]*m[2][2] - m[0][2]*m[2][0]) / d
	r.M[1][2] = (m[0][2]*m[1][0] - m[0][0]*m[1][2]) / d
	r.M[2][0] = (m[1][0]*m[2][1] - m[1][1]*m[2][0]) / d
	r.M[2][1] = (m[0][1]*m[2][0] - m[0][0]*m[2][1]) / d
	r.M[2][2] = (m[0][0]*m[1][1] - m[0][1]*m[1][0]) / d
	r.T = r.ApplyDir(t.T).ScaledBy(-1)
	return r
}

// frameTransform maps a frame's local coordinates to its space.
func frameTransform(f frame) Transform {
	return Transform{
		M: [3][3]float64{
			{f.x.X, f.y.X, f.z.X},
			{f.x.Y, f.y.Y, f.z.Y},
			{f.x.Z, f.y.Z, f.z.Z},
		},
		T: f.o,
	}
}

// entityError is what kept an entity from being read: one this reader
// does not support, or one written wrong.
type entityError struct {
	id          int
	name        string
	unsupported bool
	msg         string
}

func (e *entityError) Error() string {
	if e.unsupported {
		return fmt.Sprintf("%s (#%d) is not supported", e.name, e.id)
	}
	return fmt.Sprintf("%s (#%d): %s", e.name, e.id, e.msg)
}

// reason is how the error is counted in a report: by entity name, so
// many faces failing on one kind of entity make one line.
func (e *entityError) reason() string {
	if e.unsupported {
		return "unsupported " + e.name
	}
	return "bad " + e.name
}

// maxRefDepth bounds how deep references are followed, so a file whose
// references loop can't recurse without end.
const maxRefDepth = 48

// reader maps the entities of a file onto the model, in one length unit.
type reader struct {
	f *File
	// scale is the factor from the file's length unit to millimetres,
	// angle from its plane angle unit to radians.
	scale, angle float64

	edges   map[int]*Edge
	curves  map[int]Curve
	surfs   map[int]Surface
	points  map[int]geom.Vector
	depth   int
	colours map[int]core.RgbaColor
}

func newReader(f *File, scale, ang float64, colours map[int]core.RgbaColor) *reader {
	return &reader{
		f: f, scale: scale, angle: ang,
		edges: map[int]*Edge{}, curves: map[int]Curve{}, surfs: map[int]Surface{},
		points: map[int]geom.Vector{}, colours: colours,
	}
}

func (r *reader) bad(in *Instance, format string, args ...any) error {
	return &entityError{id: in.ID, name: in.Name(), msg: fmt.Sprintf(format, args...)}
}

func (r *reader) unsupported(in *Instance) error {
	return &entityError{id: in.ID, name: in.Name(), unsupported: true}
}

// params is the parameters of the instance's record name; for a simple
// instance, those of its one record whatever its name.
func params(in *Instance, name string) []Value {
	if rec := in.Record(name); rec != nil {
		return rec.Params
	}
	if len(in.Records) == 1 {
		return in.Records[0].Params
	}
	return nil
}

// ref resolves a reference parameter.
func (r *reader) ref(in *Instance, ps []Value, i int) (*Instance, error) {
	if i >= len(ps) || ps[i].Kind != KindRef {
		return nil, r.bad(in, "parameter %d is not a reference", i+1)
	}
	t := r.f.Get(ps[i].Ref)
	if t == nil {
		return nil, r.bad(in, "#%d is not defined", ps[i].Ref)
	}
	return t, nil
}

// num reads a number parameter, plain or typed (LENGTH_MEASURE(2.)).
func num(v Value) (float64, bool) {
	switch v.Kind {
	case KindInteger, KindReal:
		return v.Num, !math.IsNaN(v.Num) && !math.IsInf(v.Num, 0)
	case KindTyped:
		if len(v.List) == 1 {
			return num(v.List[0])
		}
	}
	return 0, false
}

func (r *reader) num(in *Instance, ps []Value, i int) (float64, error) {
	if i < len(ps) {
		if x, ok := num(ps[i]); ok {
			return x, nil
		}
	}
	return 0, r.bad(in, "parameter %d is not a number", i+1)
}

func (r *reader) length(in *Instance, ps []Value, i int) (float64, error) {
	x, err := r.num(in, ps, i)
	return x * r.scale, err
}

func (r *reader) boolean(in *Instance, ps []Value, i int) (bool, error) {
	if i < len(ps) && ps[i].Kind == KindEnum {
		switch ps[i].Str {
		case "T":
			return true, nil
		case "F":
			return false, nil
		}
	}
	return false, r.bad(in, "parameter %d is not .T. or .F.", i+1)
}

func (r *reader) list(in *Instance, ps []Value, i int) ([]Value, error) {
	if i < len(ps) && ps[i].Kind == KindList {
		return ps[i].List, nil
	}
	return nil, r.bad(in, "parameter %d is not a list", i+1)
}

func (r *reader) enter() error {
	r.depth++
	if r.depth > maxRefDepth {
		return fmt.Errorf("references nest too deeply")
	}
	return nil
}

func (r *reader) leave() { r.depth-- }

// point reads a CARTESIAN_POINT, in millimetres.
func (r *reader) point(in *Instance) (geom.Vector, error) {
	if p, ok := r.points[in.ID]; ok {
		return p, nil
	}
	if !in.Is("CARTESIAN_POINT") {
		return geom.Vector{}, r.bad(in, "is not a CARTESIAN_POINT")
	}
	ps := params(in, "CARTESIAN_POINT")
	cs, err := r.list(in, ps, 1)
	if err != nil {
		return geom.Vector{}, err
	}
	if len(cs) < 1 || len(cs) > 3 {
		return geom.Vector{}, r.bad(in, "has %d coordinates", len(cs))
	}
	var c [3]float64
	for i := range cs {
		x, ok := num(cs[i])
		if !ok {
			return geom.Vector{}, r.bad(in, "coordinate %d is not a number", i+1)
		}
		c[i] = x * r.scale
	}
	p := geom.Vector{X: c[0], Y: c[1], Z: c[2]}
	r.points[in.ID] = p
	return p, nil
}

func (r *reader) pointRef(in *Instance, ps []Value, i int) (geom.Vector, error) {
	t, err := r.ref(in, ps, i)
	if err != nil {
		return geom.Vector{}, err
	}
	return r.point(t)
}

// direction reads a DIRECTION, normalised.
func (r *reader) direction(in *Instance) (geom.Vector, error) {
	if !in.Is("DIRECTION") {
		return geom.Vector{}, r.bad(in, "is not a DIRECTION")
	}
	ps := params(in, "DIRECTION")
	cs, err := r.list(in, ps, 1)
	if err != nil {
		return geom.Vector{}, err
	}
	if len(cs) < 1 || len(cs) > 3 {
		return geom.Vector{}, r.bad(in, "has %d components", len(cs))
	}
	var c [3]float64
	for i := range cs {
		x, ok := num(cs[i])
		if !ok {
			return geom.Vector{}, r.bad(in, "component %d is not a number", i+1)
		}
		c[i] = x
	}
	d := geom.Vector{X: c[0], Y: c[1], Z: c[2]}
	m := d.Magnitude()
	if !(m > 0) || math.IsInf(m, 0) {
		return geom.Vector{}, r.bad(in, "has no length")
	}
	return d.ScaledBy(1 / m), nil
}

// optDirection reads an optional DIRECTION parameter; ok is false when
// it is omitted.
func (r *reader) optDirection(in *Instance, ps []Value, i int) (d geom.Vector, ok bool, err error) {
	if i >= len(ps) || ps[i].Kind == KindOmitted {
		return geom.Vector{}, false, nil
	}
	t, err := r.ref(in, ps, i)
	if err != nil {
		return geom.Vector{}, false, err
	}
	d, err = r.direction(t)
	return d, err == nil, err
}

// placement reads an AXIS2_PLACEMENT_3D, or a 2D one in the xy plane,
// with the axes ISO 10303-42 builds when some are omitted.
func (r *reader) placement(in *Instance) (frame, error) {
	switch {
	case in.Is("AXIS2_PLACEMENT_3D"):
		ps := params(in, "AXIS2_PLACEMENT_3D")
		o, err := r.pointRef(in, ps, 1)
		if err != nil {
			return frame{}, err
		}
		z, ok, err := r.optDirection(in, ps, 2)
		if err != nil {
			return frame{}, err
		}
		if !ok {
			z = geom.Vector{Z: 1}
		}
		x, okx, err := r.optDirection(in, ps, 3)
		if err != nil {
			return frame{}, err
		}
		return buildAxes(o, z, x, okx), nil
	case in.Is("AXIS2_PLACEMENT_2D"):
		ps := params(in, "AXIS2_PLACEMENT_2D")
		o, err := r.pointRef(in, ps, 1)
		if err != nil {
			return frame{}, err
		}
		x, okx, err := r.optDirection(in, ps, 2)
		if err != nil {
			return frame{}, err
		}
		return buildAxes(o, geom.Vector{Z: 1}, x, okx), nil
	}
	return frame{}, r.unsupported(in)
}

func (r *reader) placementRef(in *Instance, ps []Value, i int) (frame, error) {
	t, err := r.ref(in, ps, i)
	if err != nil {
		return frame{}, err
	}
	return r.placement(t)
}

// buildAxes makes an orthonormal frame from an axis and an optional
// reference direction (ISO 10303-42 build_axes).
func buildAxes(o, z, x geom.Vector, hasX bool) frame {
	z = z.WithMagnitude(1)
	if !hasX || x.Cross(z).Magnitude() < 1e-12 {
		if math.Abs(z.X) < 0.99 {
			x = geom.Vector{X: 1}
		} else {
			x = geom.Vector{Z: 1}
		}
	}
	x = x.Minus(z.ScaledBy(x.Dot(z))).WithMagnitude(1)
	return frame{o: o, x: x, y: z.Cross(x), z: z}
}

// ---- curves ----

func (r *reader) curve(in *Instance) (Curve, error) {
	if c, ok := r.curves[in.ID]; ok {
		return c, nil
	}
	if err := r.enter(); err != nil {
		return nil, r.bad(in, "%v", err)
	}
	defer r.leave()
	c, err := r.readCurve(in)
	if err != nil {
		return nil, err
	}
	r.curves[in.ID] = c
	return c, nil
}

func (r *reader) readCurve(in *Instance) (Curve, error) {
	switch {
	case in.Is("LINE"):
		ps := params(in, "LINE")
		p, err := r.pointRef(in, ps, 1)
		if err != nil {
			return nil, err
		}
		vec, err := r.ref(in, ps, 2)
		if err != nil {
			return nil, err
		}
		d, err := r.vector(vec)
		if err != nil {
			return nil, err
		}
		return &line{p: p, d: d}, nil
	case in.Is("CIRCLE"):
		ps := params(in, "CIRCLE")
		f, err := r.placementRef(in, ps, 1)
		if err != nil {
			return nil, err
		}
		rad, err := r.length(in, ps, 2)
		if err != nil {
			return nil, err
		}
		if !(rad > 0) {
			return nil, r.bad(in, "the radius is not positive")
		}
		return &ellipse{f: f, a: rad, b: rad}, nil
	case in.Is("ELLIPSE"):
		ps := params(in, "ELLIPSE")
		f, err := r.placementRef(in, ps, 1)
		if err != nil {
			return nil, err
		}
		a, err := r.length(in, ps, 2)
		if err != nil {
			return nil, err
		}
		b, err := r.length(in, ps, 3)
		if err != nil {
			return nil, err
		}
		if !(a > 0 && b > 0) {
			return nil, r.bad(in, "a semi-axis is not positive")
		}
		return &ellipse{f: f, a: a, b: b}, nil
	case in.Is("PARABOLA"), in.Is("HYPERBOLA"):
		hyp := in.Is("HYPERBOLA")
		ps := params(in, in.Records[0].Name)
		f, err := r.placementRef(in, ps, 1)
		if err != nil {
			return nil, err
		}
		a, err := r.length(in, ps, 2)
		if err != nil {
			return nil, err
		}
		b := a
		if hyp {
			if b, err = r.length(in, ps, 3); err != nil {
				return nil, err
			}
		}
		if a == 0 || b == 0 {
			return nil, r.bad(in, "a parameter is zero")
		}
		return &conic{f: f, hyperbola: hyp, a: a, b: b}, nil
	case in.Is("SURFACE_CURVE"), in.Is("SEAM_CURVE"), in.Is("INTERSECTION_CURVE"),
		in.Is("BOUNDED_SURFACE_CURVE"):
		ps := params(in, in.Records[0].Name)
		if rec := in.Record("SURFACE_CURVE"); rec != nil {
			ps = rec.Params
		}
		t, err := r.ref(in, ps, 1)
		if err != nil {
			return nil, err
		}
		return r.curve(t)
	case in.Is("TRIMMED_CURVE"):
		// The edge's vertices trim it.
		ps := params(in, "TRIMMED_CURVE")
		t, err := r.ref(in, ps, 1)
		if err != nil {
			return nil, err
		}
		return r.curve(t)
	case in.Is("POLYLINE"):
		ps := params(in, "POLYLINE")
		l, err := r.list(in, ps, 1)
		if err != nil {
			return nil, err
		}
		pts, err := r.pointList(in, l)
		if err != nil {
			return nil, err
		}
		if len(pts) < 2 {
			return nil, r.bad(in, "has fewer than two points")
		}
		c, err := polylineCurve(pts)
		if err != nil {
			return nil, r.bad(in, "%v", err)
		}
		return c, nil
	case in.Is("B_SPLINE_CURVE_WITH_KNOTS"), in.Is("BEZIER_CURVE"), in.Is("UNIFORM_CURVE"),
		in.Is("QUASI_UNIFORM_CURVE"):
		return r.bsplineCurve(in)
	}
	return nil, r.unsupported(in)
}

// vector reads a VECTOR: its direction times its magnitude, a length.
func (r *reader) vector(in *Instance) (geom.Vector, error) {
	if !in.Is("VECTOR") {
		if in.Is("DIRECTION") {
			return r.direction(in)
		}
		return geom.Vector{}, r.bad(in, "is not a VECTOR")
	}
	ps := params(in, "VECTOR")
	t, err := r.ref(in, ps, 1)
	if err != nil {
		return geom.Vector{}, err
	}
	d, err := r.direction(t)
	if err != nil {
		return geom.Vector{}, err
	}
	m, err := r.length(in, ps, 2)
	if err != nil {
		return geom.Vector{}, err
	}
	if m == 0 {
		return geom.Vector{}, r.bad(in, "has no length")
	}
	return d.ScaledBy(m), nil
}

func (r *reader) pointList(in *Instance, l []Value) ([]geom.Vector, error) {
	pts := make([]geom.Vector, len(l))
	for i, v := range l {
		if v.Kind != KindRef || r.f.Get(v.Ref) == nil {
			return nil, r.bad(in, "a control point is not a reference")
		}
		p, err := r.point(r.f.Get(v.Ref))
		if err != nil {
			return nil, err
		}
		pts[i] = p
	}
	return pts, nil
}

func ints(l []Value) ([]int, bool) {
	out := make([]int, len(l))
	for i, v := range l {
		if v.Kind != KindInteger || v.Num < 0 || v.Num > 1e6 {
			return nil, false
		}
		out[i] = int(v.Num)
	}
	return out, true
}

func reals(l []Value) ([]float64, bool) {
	out := make([]float64, len(l))
	for i, v := range l {
		x, ok := num(v)
		if !ok {
			return nil, false
		}
		out[i] = x
	}
	return out, true
}

// splineParts splits a B-spline's parameters into those of the base
// entity (from the degree on), of the knots (from the multiplicities
// on), and the weights, for a simple or a complex instance. knotsAt is
// where the knot parameters start in a simple instance's list.
func splineParts(in *Instance, base, withKnots, rational string, knotsAt int) (b, k []Value, w []Value, hasKnots bool) {
	if rec := in.Record(base); rec != nil && len(in.Records) > 1 {
		b = rec.Params
		if kr := in.Record(withKnots); kr != nil {
			k, hasKnots = kr.Params, true
		}
		if wr := in.Record(rational); wr != nil && len(wr.Params) > 0 {
			w = wr.Params
		}
		return
	}
	ps := in.Records[0].Params
	if len(ps) > 0 {
		b = ps[1:]
	}
	if in.Records[0].Name == withKnots && len(ps) > knotsAt {
		k, hasKnots = ps[knotsAt:], true
	}
	return
}

func (r *reader) bsplineCurve(in *Instance) (Curve, error) {
	b, k, w, hasKnots := splineParts(in, "B_SPLINE_CURVE", "B_SPLINE_CURVE_WITH_KNOTS", "RATIONAL_B_SPLINE_CURVE", 6)
	if len(b) < 2 {
		return nil, r.bad(in, "too few parameters")
	}
	deg, ok := num(b[0])
	if !ok || deg != math.Trunc(deg) {
		return nil, r.bad(in, "the degree is not an integer")
	}
	if b[1].Kind != KindList {
		return nil, r.bad(in, "the control points are not a list")
	}
	pts, err := r.pointList(in, b[1].List)
	if err != nil {
		return nil, err
	}
	var knots []float64
	switch {
	case hasKnots:
		if len(k) < 2 || k[0].Kind != KindList || k[1].Kind != KindList {
			return nil, r.bad(in, "the knots are not lists")
		}
		mults, ok1 := ints(k[0].List)
		ks, ok2 := reals(k[1].List)
		if !ok1 || !ok2 {
			return nil, r.bad(in, "the knots are not numbers")
		}
		if knots, err = expandKnots(mults, ks); err != nil {
			return nil, r.bad(in, "%v", err)
		}
	case in.Is("UNIFORM_CURVE"):
		knots = uniformKnots(int(deg), len(pts), false)
	default: // Bezier and quasi uniform
		knots = uniformKnots(int(deg), len(pts), true)
	}
	var weights []float64
	if w != nil {
		if w[0].Kind != KindList {
			return nil, r.bad(in, "the weights are not a list")
		}
		if weights, ok = reals(w[0].List); !ok {
			return nil, r.bad(in, "the weights are not numbers")
		}
	}
	c, err := newBsplineCurve(int(deg), knots, pts, weights)
	if err != nil {
		return nil, r.bad(in, "%v", err)
	}
	return c, nil
}

// ---- surfaces ----

func (r *reader) surface(in *Instance) (Surface, error) {
	if s, ok := r.surfs[in.ID]; ok {
		return s, nil
	}
	if err := r.enter(); err != nil {
		return nil, r.bad(in, "%v", err)
	}
	defer r.leave()
	s, err := r.readSurface(in)
	if err != nil {
		return nil, err
	}
	r.surfs[in.ID] = s
	return s, nil
}

func (r *reader) readSurface(in *Instance) (Surface, error) {
	switch {
	case in.Is("PLANE"):
		f, err := r.placementRef(in, params(in, "PLANE"), 1)
		if err != nil {
			return nil, err
		}
		return &plane{f: f}, nil
	case in.Is("CYLINDRICAL_SURFACE"):
		ps := params(in, "CYLINDRICAL_SURFACE")
		f, err := r.placementRef(in, ps, 1)
		if err != nil {
			return nil, err
		}
		rad, err := r.length(in, ps, 2)
		if err != nil {
			return nil, err
		}
		if !(rad > 0) {
			return nil, r.bad(in, "the radius is not positive")
		}
		return &cone{f: f, r: rad}, nil
	case in.Is("CONICAL_SURFACE"):
		ps := params(in, "CONICAL_SURFACE")
		f, err := r.placementRef(in, ps, 1)
		if err != nil {
			return nil, err
		}
		rad, err := r.length(in, ps, 2)
		if err != nil {
			return nil, err
		}
		a, err := r.num(in, ps, 3)
		if err != nil {
			return nil, err
		}
		a *= r.angle
		if !(rad >= 0) || !(a > 0 && a < math.Pi/2) {
			return nil, r.bad(in, "the radius or the angle is out of range")
		}
		return &cone{f: f, r: rad, tan: math.Tan(a)}, nil
	case in.Is("SPHERICAL_SURFACE"):
		ps := params(in, "SPHERICAL_SURFACE")
		f, err := r.placementRef(in, ps, 1)
		if err != nil {
			return nil, err
		}
		rad, err := r.length(in, ps, 2)
		if err != nil {
			return nil, err
		}
		if !(rad > 0) {
			return nil, r.bad(in, "the radius is not positive")
		}
		return &sphere{f: f, r: rad}, nil
	case in.Is("TOROIDAL_SURFACE"), in.Is("DEGENERATE_TOROIDAL_SURFACE"):
		ps := params(in, in.Records[0].Name)
		if rec := in.Record("TOROIDAL_SURFACE"); rec != nil && len(in.Records) > 1 {
			ps = rec.Params
		}
		f, err := r.placementRef(in, ps, 1)
		if err != nil {
			return nil, err
		}
		major, err := r.length(in, ps, 2)
		if err != nil {
			return nil, err
		}
		minor, err := r.length(in, ps, 3)
		if err != nil {
			return nil, err
		}
		if !(minor > 0) || !(major >= 0) {
			return nil, r.bad(in, "a radius is out of range")
		}
		return &torus{f: f, major: major, r: minor}, nil
	case in.Is("B_SPLINE_SURFACE_WITH_KNOTS"), in.Is("BEZIER_SURFACE"), in.Is("UNIFORM_SURFACE"),
		in.Is("QUASI_UNIFORM_SURFACE"):
		return r.bsplineSurface(in)
	case in.Is("SURFACE_OF_LINEAR_EXTRUSION"):
		ps := params(in, "SURFACE_OF_LINEAR_EXTRUSION")
		ct, err := r.ref(in, ps, 1)
		if err != nil {
			return nil, err
		}
		c, err := r.curve(ct)
		if err != nil {
			return nil, err
		}
		vt, err := r.ref(in, ps, 2)
		if err != nil {
			return nil, err
		}
		d, err := r.vector(vt)
		if err != nil {
			return nil, err
		}
		return &extrusion{c: c, d: d}, nil
	case in.Is("SURFACE_OF_REVOLUTION"):
		ps := params(in, "SURFACE_OF_REVOLUTION")
		ct, err := r.ref(in, ps, 1)
		if err != nil {
			return nil, err
		}
		c, err := r.curve(ct)
		if err != nil {
			return nil, err
		}
		at, err := r.ref(in, ps, 2)
		if err != nil {
			return nil, err
		}
		if !at.Is("AXIS1_PLACEMENT") {
			return nil, r.bad(in, "the axis is not an AXIS1_PLACEMENT")
		}
		aps := params(at, "AXIS1_PLACEMENT")
		o, err := r.pointRef(at, aps, 1)
		if err != nil {
			return nil, err
		}
		a, ok, err := r.optDirection(at, aps, 2)
		if err != nil {
			return nil, err
		}
		if !ok {
			a = geom.Vector{Z: 1}
		}
		return &revolution{c: c, o: o, a: a}, nil
	case in.Is("OFFSET_SURFACE"):
		ps := params(in, "OFFSET_SURFACE")
		bt, err := r.ref(in, ps, 1)
		if err != nil {
			return nil, err
		}
		base, err := r.surface(bt)
		if err != nil {
			return nil, err
		}
		d, err := r.length(in, ps, 2)
		if err != nil {
			return nil, err
		}
		return &offset{base: base, d: d}, nil
	case in.Is("RECTANGULAR_TRIMMED_SURFACE"):
		// The face's bounds trim it.
		bt, err := r.ref(in, params(in, "RECTANGULAR_TRIMMED_SURFACE"), 1)
		if err != nil {
			return nil, err
		}
		return r.surface(bt)
	}
	return nil, r.unsupported(in)
}

func (r *reader) bsplineSurface(in *Instance) (Surface, error) {
	b, k, w, hasKnots := splineParts(in, "B_SPLINE_SURFACE", "B_SPLINE_SURFACE_WITH_KNOTS", "RATIONAL_B_SPLINE_SURFACE", 8)
	if len(b) < 3 {
		return nil, r.bad(in, "too few parameters")
	}
	du, ok1 := num(b[0])
	dv, ok2 := num(b[1])
	if !ok1 || !ok2 || du != math.Trunc(du) || dv != math.Trunc(dv) {
		return nil, r.bad(in, "a degree is not an integer")
	}
	if b[2].Kind != KindList {
		return nil, r.bad(in, "the control points are not a list")
	}
	pts := make([][]geom.Vector, len(b[2].List))
	for i, row := range b[2].List {
		if row.Kind != KindList {
			return nil, r.bad(in, "a row of control points is not a list")
		}
		var err error
		if pts[i], err = r.pointList(in, row.List); err != nil {
			return nil, err
		}
	}
	if len(pts) == 0 || len(pts[0]) == 0 {
		return nil, r.bad(in, "has no control points")
	}
	var ku, kv []float64
	switch {
	case hasKnots:
		if len(k) < 4 {
			return nil, r.bad(in, "too few knot parameters")
		}
		for _, x := range k[:4] {
			if x.Kind != KindList {
				return nil, r.bad(in, "the knots are not lists")
			}
		}
		mu, ok1 := ints(k[0].List)
		mv, ok2 := ints(k[1].List)
		ksu, ok3 := reals(k[2].List)
		ksv, ok4 := reals(k[3].List)
		if !ok1 || !ok2 || !ok3 || !ok4 {
			return nil, r.bad(in, "the knots are not numbers")
		}
		var err error
		if ku, err = expandKnots(mu, ksu); err != nil {
			return nil, r.bad(in, "along u: %v", err)
		}
		if kv, err = expandKnots(mv, ksv); err != nil {
			return nil, r.bad(in, "along v: %v", err)
		}
	default:
		clamped := !in.Is("UNIFORM_SURFACE")
		ku = uniformKnots(int(du), len(pts), clamped)
		kv = uniformKnots(int(dv), len(pts[0]), clamped)
	}
	var weights [][]float64
	if w != nil {
		if w[0].Kind != KindList {
			return nil, r.bad(in, "the weights are not a list")
		}
		weights = make([][]float64, len(w[0].List))
		for i, row := range w[0].List {
			var ok bool
			if row.Kind != KindList {
				return nil, r.bad(in, "a row of weights is not a list")
			}
			if weights[i], ok = reals(row.List); !ok {
				return nil, r.bad(in, "the weights are not numbers")
			}
		}
	}
	s, err := newBsplineSurface(int(du), int(dv), ku, kv, pts, weights)
	if err != nil {
		return nil, r.bad(in, "%v", err)
	}
	return s, nil
}

// ---- topology ----

func (r *reader) vertex(in *Instance) (geom.Vector, error) {
	if !in.Is("VERTEX_POINT") {
		return geom.Vector{}, r.bad(in, "is not a VERTEX_POINT")
	}
	return r.pointRef(in, params(in, "VERTEX_POINT"), 1)
}

func (r *reader) edge(in *Instance) (*Edge, error) {
	if e, ok := r.edges[in.ID]; ok {
		return e, nil
	}
	if !in.Is("EDGE_CURVE") {
		return nil, r.unsupported(in)
	}
	ps := params(in, "EDGE_CURVE")
	st, err := r.ref(in, ps, 1)
	if err != nil {
		return nil, err
	}
	et, err := r.ref(in, ps, 2)
	if err != nil {
		return nil, err
	}
	start, err := r.vertex(st)
	if err != nil {
		return nil, err
	}
	end, err := r.vertex(et)
	if err != nil {
		return nil, err
	}
	ct, err := r.ref(in, ps, 3)
	if err != nil {
		return nil, err
	}
	c, err := r.curve(ct)
	if err != nil {
		return nil, err
	}
	sense, err := r.boolean(in, ps, 4)
	if err != nil {
		return nil, err
	}
	e := &Edge{ID: in.ID, Start: start, End: end, Curve: c, Sense: sense,
		Closed: st.ID == et.ID || start.Minus(end).Magnitude() < 1e-9*(1+start.Magnitude())}
	r.edges[in.ID] = e
	return e, nil
}

// orientedEdge reads an ORIENTED_EDGE, which may itself name an
// oriented edge.
func (r *reader) orientedEdge(in *Instance) (OrientedEdge, error) {
	if err := r.enter(); err != nil {
		return OrientedEdge{}, r.bad(in, "%v", err)
	}
	defer r.leave()
	if !in.Is("ORIENTED_EDGE") {
		e, err := r.edge(in)
		return OrientedEdge{E: e, Forward: true}, err
	}
	ps := params(in, "ORIENTED_EDGE")
	et, err := r.ref(in, ps, 3)
	if err != nil {
		return OrientedEdge{}, err
	}
	fwd, err := r.boolean(in, ps, 4)
	if err != nil {
		return OrientedEdge{}, err
	}
	oe, err := r.orientedEdge(et)
	if err != nil {
		return OrientedEdge{}, err
	}
	oe.Forward = oe.Forward == fwd
	return oe, nil
}

// bound reads a FACE_BOUND or FACE_OUTER_BOUND.
func (r *reader) bound(in *Instance) (*Bound, error) {
	if !in.Is("FACE_BOUND") && !in.Is("FACE_OUTER_BOUND") {
		return nil, r.bad(in, "is not a face bound")
	}
	ps := params(in, in.Records[0].Name)
	lt, err := r.ref(in, ps, 1)
	if err != nil {
		return nil, err
	}
	orient, err := r.boolean(in, ps, 2)
	if err != nil {
		return nil, err
	}
	b := &Bound{}
	switch {
	case lt.Is("EDGE_LOOP"):
		l, err := r.list(lt, params(lt, "EDGE_LOOP"), 1)
		if err != nil {
			return nil, err
		}
		for _, v := range l {
			if v.Kind != KindRef || r.f.Get(v.Ref) == nil {
				return nil, r.bad(lt, "an edge is not a reference")
			}
			oe, err := r.orientedEdge(r.f.Get(v.Ref))
			if err != nil {
				return nil, err
			}
			b.Edges = append(b.Edges, oe)
		}
		if !orient {
			for i, j := 0, len(b.Edges)-1; i < j; i, j = i+1, j-1 {
				b.Edges[i], b.Edges[j] = b.Edges[j], b.Edges[i]
			}
			for i := range b.Edges {
				b.Edges[i].Forward = !b.Edges[i].Forward
			}
		}
	case lt.Is("POLY_LOOP"):
		l, err := r.list(lt, params(lt, "POLY_LOOP"), 1)
		if err != nil {
			return nil, err
		}
		if b.Poly, err = r.pointList(lt, l); err != nil {
			return nil, err
		}
		if !orient {
			for i, j := 0, len(b.Poly)-1; i < j; i, j = i+1, j-1 {
				b.Poly[i], b.Poly[j] = b.Poly[j], b.Poly[i]
			}
		}
	case lt.Is("VERTEX_LOOP"):
		// A loop shrunk to a point, such as at a cone's apex, bounds
		// nothing.
		return nil, nil
	default:
		return nil, r.unsupported(lt)
	}
	return b, nil
}

// face reads an ADVANCED_FACE or a FACE_SURFACE.
func (r *reader) face(in *Instance) (*Face, error) {
	name := ""
	switch {
	case in.Is("ADVANCED_FACE"):
		name = "ADVANCED_FACE"
	case in.Is("FACE_SURFACE"):
		name = "FACE_SURFACE"
	default:
		return nil, r.unsupported(in)
	}
	ps := params(in, name)
	bl, err := r.list(in, ps, 1)
	if err != nil {
		return nil, err
	}
	st, err := r.ref(in, ps, 2)
	if err != nil {
		return nil, err
	}
	s, err := r.surface(st)
	if err != nil {
		return nil, err
	}
	sense, err := r.boolean(in, ps, 3)
	if err != nil {
		return nil, err
	}
	f := &Face{ID: in.ID, Surf: s, Sense: sense}
	for _, v := range bl {
		if v.Kind != KindRef || r.f.Get(v.Ref) == nil {
			return nil, r.bad(in, "a bound is not a reference")
		}
		b, err := r.bound(r.f.Get(v.Ref))
		if err != nil {
			return nil, err
		}
		if b != nil {
			f.Bounds = append(f.Bounds, b)
		}
	}
	if c, ok := r.colours[in.ID]; ok {
		f.Color, f.HasColor = c, true
	}
	return f, nil
}

// shellFaces reads the faces of a shell, flipped when it is used
// against its orientation. Faces that can't be read are counted in
// skip, by reason.
func (r *reader) shellFaces(in *Instance, flip bool, skip map[string]int, errs map[string]string) ([]*Face, error) {
	if err := r.enter(); err != nil {
		return nil, r.bad(in, "%v", err)
	}
	defer r.leave()
	switch {
	case in.Is("ORIENTED_CLOSED_SHELL"), in.Is("ORIENTED_OPEN_SHELL"):
		ps := params(in, in.Records[0].Name)
		t, err := r.ref(in, ps, 2)
		if err != nil {
			return nil, err
		}
		o, err := r.boolean(in, ps, 3)
		if err != nil {
			return nil, err
		}
		return r.shellFaces(t, flip != !o, skip, errs)
	case in.Is("CLOSED_SHELL"), in.Is("OPEN_SHELL"), in.Is("CONNECTED_FACE_SET"):
	default:
		return nil, r.unsupported(in)
	}
	l, err := r.list(in, params(in, in.Records[0].Name), 1)
	if err != nil {
		return nil, err
	}
	var faces []*Face
	for _, v := range l {
		if v.Kind != KindRef || r.f.Get(v.Ref) == nil {
			skip["bad "+in.Name()]++
			continue
		}
		f, err := r.face(r.f.Get(v.Ref))
		if err != nil {
			reason := "bad face"
			if ee, ok := err.(*entityError); ok {
				reason = ee.reason()
			}
			skip[reason]++
			if _, ok := errs[reason]; !ok {
				errs[reason] = err.Error()
			}
			continue
		}
		if flip {
			g := *f
			g.Sense = !g.Sense
			f = &g
		}
		faces = append(faces, f)
	}
	return faces, nil
}

// shapeRoots are the entities that hold a shape this reader maps.
var shapeRoots = []string{
	"MANIFOLD_SOLID_BREP", "BREP_WITH_VOIDS", "FACETED_BREP", "SHELL_BASED_SURFACE_MODEL",
}

// otherShapes are shape items that this reader names as unsupported
// when a file holds no boundary representation.
var otherShapes = []string{
	"CSG_SOLID", "EXTRUDED_AREA_SOLID", "REVOLVED_AREA_SOLID", "SWEPT_DISK_SOLID",
	"BOOLEAN_RESULT", "BLOCK", "RIGHT_CIRCULAR_CYLINDER", "RIGHT_CIRCULAR_CONE", "SPHERE",
	"TORUS", "RIGHT_ANGULAR_WEDGE", "HALF_SPACE_SOLID", "TESSELLATED_SOLID",
	"TESSELLATED_SHELL", "TRIANGULATED_FACE", "COMPLEX_TRIANGULATED_SURFACE_SET",
	"TRIANGULATED_SURFACE_SET", "GEOMETRIC_CURVE_SET", "GEOMETRIC_SET",
}

// shape reads a shape root.
func (r *reader) shape(in *Instance, skip map[string]int, errs map[string]string) (*Shape, error) {
	sh := &Shape{ID: in.ID, Kind: in.Records[0].Name}
	add := func(t *Instance, flip bool) error {
		faces, err := r.shellFaces(t, flip, skip, errs)
		if err != nil {
			return err
		}
		sh.Faces = append(sh.Faces, faces...)
		return nil
	}
	switch {
	case in.Is("BREP_WITH_VOIDS"), in.Is("MANIFOLD_SOLID_BREP"), in.Is("FACETED_BREP"):
		ps := params(in, "MANIFOLD_SOLID_BREP")
		if ps == nil {
			ps = params(in, in.Records[0].Name)
		}
		t, err := r.ref(in, ps, 1)
		if err != nil {
			return nil, err
		}
		if err := add(t, false); err != nil {
			return nil, err
		}
		if rec := in.Record("BREP_WITH_VOIDS"); rec != nil {
			vs := rec.Params
			// A simple instance lists the voids after the outer shell; a
			// complex one has them alone.
			i := 0
			if len(in.Records) == 1 {
				i = 2
			}
			l, err := r.list(in, vs, i)
			if err != nil {
				return nil, err
			}
			for _, v := range l {
				if v.Kind != KindRef || r.f.Get(v.Ref) == nil {
					return nil, r.bad(in, "a void is not a reference")
				}
				// Each void is an oriented shell, written reversed.
				if err := add(r.f.Get(v.Ref), false); err != nil {
					return nil, err
				}
			}
		}
	case in.Is("SHELL_BASED_SURFACE_MODEL"):
		l, err := r.list(in, params(in, "SHELL_BASED_SURFACE_MODEL"), 1)
		if err != nil {
			return nil, err
		}
		for _, v := range l {
			if v.Kind != KindRef || r.f.Get(v.Ref) == nil {
				return nil, r.bad(in, "a shell is not a reference")
			}
			if err := add(r.f.Get(v.Ref), false); err != nil {
				return nil, err
			}
		}
	default:
		return nil, r.unsupported(in)
	}
	if c, ok := r.colours[in.ID]; ok {
		for i, f := range sh.Faces {
			if !f.HasColor {
				g := *f
				g.Color, g.HasColor = c, true
				sh.Faces[i] = &g
			}
		}
	}
	return sh, nil
}

// ---- units, representations and assemblies ----

// unitFactor is the factor of a unit to millimetres (length) or
// radians (plane angle); found is false for a unit of another kind.
func unitFactor(f *File, in *Instance, kind string, depth int) (factor float64, found bool) {
	if depth > 8 || !in.Is(kind) {
		return 0, false
	}
	if rec := in.Record("SI_UNIT"); rec != nil {
		// SI_UNIT(prefix, name); a simple instance has the derived
		// dimensions, *, first.
		var enums []string
		for _, v := range rec.Params {
			switch v.Kind {
			case KindEnum:
				enums = append(enums, v.Str)
			case KindOmitted:
				enums = append(enums, "")
			}
		}
		if len(enums) == 0 {
			return 0, false
		}
		name := enums[len(enums)-1]
		prefix := ""
		if len(enums) >= 2 {
			prefix = enums[len(enums)-2]
		}
		mult := 1.0
		switch prefix {
		case "KILO":
			mult = 1e3
		case "HECTO":
			mult = 1e2
		case "DECA":
			mult = 1e1
		case "DECI":
			mult = 1e-1
		case "CENTI":
			mult = 1e-2
		case "MILLI":
			mult = 1e-3
		case "MICRO":
			mult = 1e-6
		case "NANO":
			mult = 1e-9
		}
		switch name {
		case "METRE":
			return mult * 1000, true
		case "RADIAN":
			return mult, true
		}
		return 0, false
	}
	if rec := in.Record("CONVERSION_BASED_UNIT"); rec != nil && len(rec.Params) >= 2 && rec.Params[1].Kind == KindRef {
		m := f.Get(rec.Params[1].Ref)
		if m == nil || len(m.Records) == 0 {
			return 0, false
		}
		ps := m.Records[0].Params
		if len(ps) < 2 || ps[1].Kind != KindRef {
			return 0, false
		}
		x, ok := num(ps[0])
		base := f.Get(ps[1].Ref)
		if !ok || base == nil {
			return 0, false
		}
		bf, ok := unitFactor(f, base, kind, depth+1)
		if !ok || !(x > 0) {
			return 0, false
		}
		return x * bf, true
	}
	return 0, false
}

// unitName names a length unit, in the plural, by its factor.
func unitName(mm float64) string {
	near := func(x float64) bool { return math.Abs(mm-x) < 1e-9*x }
	switch {
	case near(1):
		return "millimetres"
	case near(10):
		return "centimetres"
	case near(1000):
		return "metres"
	case near(25.4):
		return "inches"
	case near(304.8):
		return "feet"
	case near(1e-3):
		return "micrometres"
	}
	return fmt.Sprintf("units of %g mm", mm)
}

// contextUnits are the length and angle factors of a representation
// context, millimetres and radians when it gives none.
func contextUnits(f *File, ctx *Instance) (length, ang float64) {
	length, ang = 1, 1
	if ctx == nil {
		return
	}
	rec := ctx.Record("GLOBAL_UNIT_ASSIGNED_CONTEXT")
	if rec == nil || len(rec.Params) == 0 || rec.Params[0].Kind != KindList {
		return
	}
	for _, v := range rec.Params[0].List {
		if v.Kind != KindRef || f.Get(v.Ref) == nil {
			continue
		}
		u := f.Get(v.Ref)
		if x, ok := unitFactor(f, u, "LENGTH_UNIT", 0); ok && x > 0 {
			length = x
		}
		if x, ok := unitFactor(f, u, "PLANE_ANGLE_UNIT", 0); ok && x > 0 {
			ang = x
		}
	}
	return
}

// isRepresentation tells a representation: an entity whose name ends
// in REPRESENTATION and whose parameters are a name, items and a
// context.
func isRepresentation(in *Instance) bool {
	for _, rec := range in.Records {
		if strings.HasSuffix(rec.Name, "REPRESENTATION") && len(rec.Params) == 3 &&
			rec.Params[1].Kind == KindList && rec.Params[2].Kind == KindRef {
			return true
		}
	}
	return false
}

func repParams(in *Instance) []Value {
	for _, rec := range in.Records {
		if strings.HasSuffix(rec.Name, "REPRESENTATION") && len(rec.Params) == 3 {
			return rec.Params
		}
	}
	return nil
}

// relationshipParams are the parameters of a representation
// relationship (name, description, rep_1, rep_2), simple or complex,
// or nil.
func relationshipParams(in *Instance) []Value {
	for _, rec := range in.Records {
		if strings.HasSuffix(rec.Name, "REPRESENTATION_RELATIONSHIP") && len(rec.Params) == 4 &&
			rec.Params[2].Kind == KindRef && rec.Params[3].Kind == KindRef {
			return rec.Params
		}
	}
	return nil
}

// assembly is the graph of representations: which are the same space,
// and which are placed in which.
type assembly struct {
	f      *File
	parent map[int]int // union-find over representations of one space
	ups    map[int][]link
}

// link places a child representation's space in a parent's.
type link struct {
	parent int
	t      Transform
}

func (a *assembly) find(x int) int {
	root := x
	for {
		p, ok := a.parent[root]
		if !ok || p == root {
			break
		}
		root = p
	}
	for x != root {
		next := a.parent[x]
		a.parent[x] = root
		x = next
	}
	return root
}

func (a *assembly) union(x, y int) {
	rx, ry := a.find(x), a.find(y)
	if rx != ry {
		a.parent[rx] = ry
	}
}

// placementOf reads an axis placement or a transformation operator as
// the transform from its local coordinates, in the units of the
// representation rep.
func (a *assembly) placementOf(ref int, rep int) (Transform, bool) {
	in := a.f.Get(ref)
	if in == nil {
		return Transform{}, false
	}
	scale, ang := 1.0, 1.0
	if r := a.f.Get(rep); r != nil {
		if ps := repParams(r); ps != nil {
			scale, ang = contextUnits(a.f, a.f.Get(ps[2].Ref))
		}
	}
	rd := newReader(a.f, scale, ang, nil)
	if in.Is("CARTESIAN_TRANSFORMATION_OPERATOR_3D") || in.Is("CARTESIAN_TRANSFORMATION_OPERATOR") {
		ps := params(in, "CARTESIAN_TRANSFORMATION_OPERATOR")
		if ps == nil {
			ps = params(in, in.Records[0].Name)
		}
		// (name, axis1, axis2, local_origin, scale[, axis3]); a complex
		// instance may drop the name.
		off := 0
		if len(ps) >= 6 || len(ps) == 5 && ps[0].Kind == KindString {
			off = 1
		}
		if len(ps) < off+4 {
			return Transform{}, false
		}
		o, err := rd.pointRef(in, ps, off+2)
		if err != nil {
			return Transform{}, false
		}
		x, okx, _ := rd.optDirection(in, ps, off)
		z := geom.Vector{Z: 1}
		if az, ok, _ := rd.optDirection(in, ps, off+4); ok {
			z = az
		}
		f := buildAxes(o, z, x, okx)
		t := frameTransform(f)
		if s, ok := num(ps[off+3]); ok && s > 0 {
			for i := 0; i < 3; i++ {
				for j := 0; j < 3; j++ {
					t.M[i][j] *= s
				}
			}
		}
		return t, true
	}
	f, err := rd.placement(in)
	if err != nil {
		return Transform{}, false
	}
	return frameTransform(f), true
}

// readAssembly builds the representation graph of a file.
func readAssembly(f *File) *assembly {
	a := &assembly{f: f, parent: map[int]int{}, ups: map[int][]link{}}
	// The representation each product definition is shown by, to tell
	// parent from child in an assembly.
	pdOfPDS := map[int]int{} // PRODUCT_DEFINITION_SHAPE -> its definition
	repsOfPD := map[int][]int{}
	nauo := map[int][2]int{} // NEXT_ASSEMBLY_USAGE_OCCURRENCE -> parent, child definitions
	type withT struct {
		id, r1, r2, op int
		pds            int
	}
	var transforms []withT
	cdsr := map[int]int{} // relationship -> PRODUCT_DEFINITION_SHAPE
	for _, id := range f.Order {
		in := f.Get(id)
		switch {
		case in.Is("PRODUCT_DEFINITION_SHAPE"):
			if ps := params(in, "PRODUCT_DEFINITION_SHAPE"); len(ps) >= 3 && ps[2].Kind == KindRef {
				pdOfPDS[id] = ps[2].Ref
			}
		case in.Is("NEXT_ASSEMBLY_USAGE_OCCURRENCE"):
			if ps := params(in, "NEXT_ASSEMBLY_USAGE_OCCURRENCE"); len(ps) >= 5 &&
				ps[3].Kind == KindRef && ps[4].Kind == KindRef {
				nauo[id] = [2]int{ps[3].Ref, ps[4].Ref}
			}
		case in.Is("CONTEXT_DEPENDENT_SHAPE_REPRESENTATION"):
			if ps := params(in, "CONTEXT_DEPENDENT_SHAPE_REPRESENTATION"); len(ps) >= 2 &&
				ps[0].Kind == KindRef && ps[1].Kind == KindRef {
				cdsr[ps[0].Ref] = ps[1].Ref
			}
		}
	}
	for _, id := range f.Order {
		in := f.Get(id)
		switch {
		case in.Is("SHAPE_DEFINITION_REPRESENTATION"):
			if ps := params(in, "SHAPE_DEFINITION_REPRESENTATION"); len(ps) >= 2 &&
				ps[0].Kind == KindRef && ps[1].Kind == KindRef {
				if pd, ok := pdOfPDS[ps[0].Ref]; ok {
					repsOfPD[pd] = append(repsOfPD[pd], ps[1].Ref)
				}
			}
		case relationshipParams(in) != nil:
			ps := relationshipParams(in)
			r1, r2 := ps[2].Ref, ps[3].Ref
			if rec := in.Record("REPRESENTATION_RELATIONSHIP_WITH_TRANSFORMATION"); rec != nil {
				op := 0
				for _, v := range rec.Params {
					if v.Kind == KindRef {
						op = v.Ref
					}
				}
				transforms = append(transforms, withT{id: id, r1: r1, r2: r2, op: op, pds: cdsr[id]})
				continue
			}
			a.union(r1, r2)
		}
	}
	// A transform links child to parent; which of rep_1 and rep_2 is
	// the child is told by the assembly usage when there is one, and is
	// rep_1 when there isn't (the CAx-IF recommended practice).
	for _, t := range transforms {
		child, parent := t.r1, t.r2
		reversed := false
		if pd, ok := pdOfPDS[t.pds]; ok {
			if use, ok := nauo[pd]; ok {
				for _, rp := range repsOfPD[use[0]] {
					if a.find(rp) == a.find(t.r1) {
						reversed = true
					}
				}
			}
		}
		op := f.Get(t.op)
		if op == nil {
			continue
		}
		var tr Transform
		switch {
		case op.Is("ITEM_DEFINED_TRANSFORMATION"):
			ps := params(op, "ITEM_DEFINED_TRANSFORMATION")
			if len(ps) < 4 || ps[2].Kind != KindRef || ps[3].Kind != KindRef {
				continue
			}
			i1, i2 := ps[2].Ref, ps[3].Ref
			if reversed {
				child, parent = t.r2, t.r1
				i1, i2 = i2, i1
			}
			m1, ok1 := a.placementOf(i1, child)
			m2, ok2 := a.placementOf(i2, parent)
			if !ok1 || !ok2 || m1.Det() == 0 {
				continue
			}
			tr = m1.Inverse().Then(m2)
		default:
			m, ok := a.placementOf(t.op, parent)
			if !ok {
				continue
			}
			if reversed {
				child, parent = t.r2, t.r1
			}
			tr = m
		}
		a.ups[child] = append(a.ups[child], link{parent: parent, t: tr})
	}
	// Mapped items place a representation inside another.
	for _, id := range f.Order {
		in := f.Get(id)
		if !isRepresentation(in) {
			continue
		}
		for _, v := range repParams(in)[1].List {
			if v.Kind != KindRef {
				continue
			}
			mi := f.Get(v.Ref)
			if mi == nil || !mi.Is("MAPPED_ITEM") {
				continue
			}
			ps := params(mi, "MAPPED_ITEM")
			if len(ps) < 3 || ps[1].Kind != KindRef || ps[2].Kind != KindRef {
				continue
			}
			rm := f.Get(ps[1].Ref)
			if rm == nil || !rm.Is("REPRESENTATION_MAP") {
				continue
			}
			mps := params(rm, "REPRESENTATION_MAP")
			if len(mps) < 2 || mps[0].Kind != KindRef || mps[1].Kind != KindRef {
				continue
			}
			child := mps[1].Ref
			origin, ok1 := a.placementOf(mps[0].Ref, child)
			target, ok2 := a.placementOf(ps[2].Ref, id)
			if !ok1 || !ok2 || origin.Det() == 0 {
				continue
			}
			a.ups[child] = append(a.ups[child], link{parent: id, t: origin.Inverse().Then(target)})
		}
	}
	return a
}

// maxPlacements bounds how many times one shape is placed.
const maxPlacements = 100000

// placements lists every transform that takes the space of
// representation rep to the top of the assembly.
func (a *assembly) placements(rep int) []Transform {
	// Links are kept by representation; gather them by space.
	type key = int
	ups := map[key][]link{}
	for child, ls := range a.ups {
		ups[a.find(child)] = append(ups[a.find(child)], ls...)
	}
	var out []Transform
	var walk func(space int, t Transform, depth int, seen map[int]bool)
	walk = func(space int, t Transform, depth int, seen map[int]bool) {
		if len(out) >= maxPlacements {
			return
		}
		ls := ups[space]
		if len(ls) == 0 || depth > 32 {
			out = append(out, t)
			return
		}
		for _, l := range ls {
			ps := a.find(l.parent)
			if seen[ps] {
				continue
			}
			seen[ps] = true
			walk(ps, t.Then(l.t), depth+1, seen)
			delete(seen, ps)
		}
	}
	root := a.find(rep)
	walk(root, Identity(), 0, map[int]bool{root: true})
	if len(out) == 0 {
		out = append(out, Identity())
	}
	return out
}

// readColours maps styled items (faces, shells, solids) to the surface
// colour their style gives.
func readColours(f *File) map[int]core.RgbaColor {
	out := map[int]core.RgbaColor{}
	var colourIn func(id, depth int) (core.RgbaColor, bool)
	colourIn = func(id, depth int) (core.RgbaColor, bool) {
		in := f.Get(id)
		if in == nil || depth > 12 {
			return core.RgbaColor{}, false
		}
		if rec := in.Record("COLOUR_RGB"); rec != nil {
			ps := rec.Params
			if len(ps) >= 4 {
				rv, ok1 := num(ps[1])
				gv, ok2 := num(ps[2])
				bv, ok3 := num(ps[3])
				if ok1 && ok2 && ok3 {
					c := func(x float64) uint8 { return uint8(math.Round(math.Max(0, math.Min(1, x)) * 255)) }
					return core.NewRgbaColor(c(rv), c(gv), c(bv), 255), true
				}
			}
			return core.RgbaColor{}, false
		}
		if rec := in.Record("DRAUGHTING_PRE_DEFINED_COLOUR"); rec != nil && len(rec.Params) > 0 {
			if c, ok := predefinedColours[strings.ToLower(rec.Params[0].Str)]; ok {
				return c, true
			}
			return core.RgbaColor{}, false
		}
		// Follow references depth first, in the order written.
		for _, rec := range in.Records {
			if rec.Name == "CURVE_STYLE" || rec.Name == "POINT_STYLE" || rec.Name == "TEXT_STYLE" {
				continue
			}
			var visit func(vs []Value) (core.RgbaColor, bool)
			visit = func(vs []Value) (core.RgbaColor, bool) {
				for _, v := range vs {
					switch v.Kind {
					case KindRef:
						if c, ok := colourIn(v.Ref, depth+1); ok {
							return c, true
						}
					case KindList, KindTyped:
						if c, ok := visit(v.List); ok {
							return c, true
						}
					}
				}
				return core.RgbaColor{}, false
			}
			if c, ok := visit(rec.Params); ok {
				return c, true
			}
		}
		return core.RgbaColor{}, false
	}
	for _, id := range f.Order {
		in := f.Get(id)
		var ps []Value
		switch {
		case in.Is("STYLED_ITEM"):
			ps = params(in, "STYLED_ITEM")
		case in.Is("OVER_RIDING_STYLED_ITEM"):
			ps = params(in, "OVER_RIDING_STYLED_ITEM")
		default:
			continue
		}
		if len(ps) < 3 || ps[1].Kind != KindList || ps[2].Kind != KindRef {
			continue
		}
		for _, psa := range ps[1].List {
			if psa.Kind != KindRef {
				continue
			}
			p := f.Get(psa.Ref)
			if p == nil {
				continue
			}
			// Only surface styles colour faces.
			found := false
			for _, rec := range p.Records {
				if len(rec.Params) == 0 || rec.Params[0].Kind != KindList {
					continue
				}
				for _, sv := range rec.Params[0].List {
					if sv.Kind != KindRef {
						continue
					}
					st := f.Get(sv.Ref)
					if st == nil || !st.Is("SURFACE_STYLE_USAGE") {
						continue
					}
					if c, ok := colourIn(sv.Ref, 0); ok {
						out[ps[2].Ref] = c
						found = true
						break
					}
				}
			}
			if found {
				break
			}
		}
	}
	return out
}

var predefinedColours = map[string]core.RgbaColor{
	"red":     {Red: 255, Alpha: 255},
	"green":   {Green: 255, Alpha: 255},
	"blue":    {Blue: 255, Alpha: 255},
	"yellow":  {Red: 255, Green: 255, Alpha: 255},
	"magenta": {Red: 255, Blue: 255, Alpha: 255},
	"cyan":    {Green: 255, Blue: 255, Alpha: 255},
	"black":   {Alpha: 255},
	"white":   {Red: 255, Green: 255, Blue: 255, Alpha: 255},
}

// ReadModel maps the boundary representations of a parsed file.
func ReadModel(f *File) *Model {
	m := &Model{unsupported: map[string]int{}, Unit: "millimetres"}
	colours := readColours(f)
	asm := readAssembly(f)

	// Each shape root, with the representations that hold it.
	repsOf := map[int][]int{}
	for _, id := range f.Order {
		in := f.Get(id)
		if !isRepresentation(in) {
			continue
		}
		for _, v := range repParams(in)[1].List {
			if v.Kind == KindRef {
				repsOf[v.Ref] = append(repsOf[v.Ref], id)
			}
		}
	}
	units := map[int]bool{}
	readers := map[[2]float64]*reader{}
	errs := map[string]string{}
	for _, id := range f.Order {
		in := f.Get(id)
		isRoot := false
		for _, n := range shapeRoots {
			if in.Is(n) {
				isRoot = true
			}
		}
		if !isRoot {
			continue
		}
		// A shell model inside another root is read with it.
		reps := repsOf[id]
		scale, ang := 1.0, 1.0
		if len(reps) > 0 {
			if ps := repParams(f.Get(reps[0])); ps != nil {
				scale, ang = contextUnits(f, f.Get(ps[2].Ref))
			}
		}
		key := [2]float64{scale, ang}
		rd := readers[key]
		if rd == nil {
			rd = newReader(f, scale, ang, colours)
			readers[key] = rd
		}
		skip := map[string]int{}
		sh, err := rd.shape(in, skip, errs)
		for k, n := range skip {
			m.unsupported[k] += n
		}
		if err != nil {
			reason := "bad " + in.Name()
			if ee, ok := err.(*entityError); ok {
				reason = ee.reason()
			}
			m.unsupported[reason]++
			if _, ok := errs[reason]; !ok {
				errs[reason] = err.Error()
			}
			continue
		}
		units[int(math.Round(scale*1e6))] = true
		if len(units) == 1 {
			m.Unit = unitName(scale)
		}
		idx := len(m.Shapes)
		m.Shapes = append(m.Shapes, sh)
		if len(reps) == 0 {
			m.Placements = append(m.Placements, Placement{Shape: idx, T: Identity()})
			continue
		}
		// A shape held by several representations of one space is
		// placed once.
		seen := map[int]bool{}
		for _, rep := range reps {
			if seen[asm.find(rep)] {
				continue
			}
			seen[asm.find(rep)] = true
			for _, t := range asm.placements(rep) {
				m.Placements = append(m.Placements, Placement{Shape: idx, T: t})
			}
		}
	}
	if len(units) > 1 {
		m.Unit = "mixed units"
	}
	if len(m.Shapes) == 0 {
		found := map[string]int{}
		for _, id := range f.Order {
			in := f.Get(id)
			for _, n := range otherShapes {
				if in.Is(n) {
					found[n]++
				}
			}
		}
		var names []string
		for n := range found {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			m.unsupported["unsupported "+n] += found[n]
		}
	}
	m.errDetail = errs
	return m
}
