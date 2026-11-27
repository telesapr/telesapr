package export

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/libm"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A face on a plane, cylinder, cone, sphere or torus is written on that
// surface (PLANE, CYLINDRICAL_SURFACE, ...), not on the rational B-spline
// patch the kernel holds it as: a receiver then sees a hole as a
// cylinder, can measure it and mate to it, and its own kernel works on
// the exact surface. Every other face is written as a B-spline. The
// face's edges stay B-spline curves, but for lines and circles
// (step_analytic_curve.go).

// analyticKind is which surface an analytic face lies on.
type analyticKind int

const (
	analyticPlane analyticKind = iota + 1
	analyticCylinder
	analyticCone
	analyticSphere
	analyticTorus
)

// analytic is a surface a face lies on exactly: its placement (origin
// and axis; a plane's normal), and its sizes.
type analytic struct {
	kind   analyticKind
	origin geom.Vector
	axis   geom.Vector
	// major is a cylinder's, sphere's or torus's (major) radius, minor
	// a torus's tube's; angle a cone's half angle, in radians, its
	// origin a point of its axis where its radius is major.
	major, minor, angle float64
}

// analyticOf finds the surface s lies on, of those STEP names; false for
// any other. The patch's points at a grid over it all lie on what is
// found, within a ten-millionth of its size.
func analyticOf(s *srf.SSurface) (analytic, bool) {
	size := patchSize(s)
	if size < geom.LengthEps {
		return analytic{}, false
	}
	tol := 1e-7*size + geom.LengthEps/10
	if a, ok := planeOf(s, tol); ok {
		return a, true
	}
	if f, ok := srf.RoundFaceOf(s); ok {
		if !f.IsCone() {
			return analytic{kind: analyticCylinder, origin: f.Point, axis: f.Axis, major: f.Radius}, true
		}
		// A cone placed where it crosses the patch's middle, its radius
		// there: some readers take a cone of radius 0 at its apex badly.
		mid := s.PointAt(0.5, 0.5)
		h := mid.Minus(f.Point).Dot(f.Axis)
		if h <= 0 {
			return analytic{}, false
		}
		return analytic{kind: analyticCone, origin: f.Point.Plus(f.Axis.ScaledBy(h)), axis: f.Axis,
			major: h * libm.Tan(f.HalfAngle), angle: f.HalfAngle}, true
	}
	if c, r, ok := srf.SphereOf(s); ok {
		return analytic{kind: analyticSphere, origin: c, axis: sphereAxis(s, c, tol), major: r}, true
	}
	if a, ok := torusOf(s, tol); ok {
		return a, true
	}
	return analytic{}, false
}

// sphereAxis is the axis a patch of a sphere about c was turned about:
// the normal of a parallel, a curve along one parameter whose circle is
// not a great one, centred off c. A patch of great circles both ways
// (none so far) is put about +z.
func sphereAxis(s *srf.SSurface, c geom.Vector, tol float64) geom.Vector {
	for _, swap := range []bool{false, true} {
		at := func(a, b float64) geom.Vector {
			if swap {
				return s.PointAt(b, a)
			}
			return s.PointAt(a, b)
		}
		pc, pn, _, ok := circleThrough(at(0, 0.3), at(0.5, 0.3), at(1, 0.3))
		if ok && pc.Minus(c).Magnitude() > tol {
			return pn
		}
	}
	return geom.Vector{Z: 1}
}

// seamOf is where a's parameter is to start for the patch s: a corner of
// s off the axis, so the surface starts where the patch does. A sheet's
// first control point may be a cone's apex or a sphere's pole, where no
// direction is defined.
func seamOf(a analytic, s *srf.SSurface) geom.Vector {
	for _, uv := range [][2]float64{{0, 0}, {0, 1}, {1, 0}, {1, 1}} {
		p := s.PointAt(uv[0], uv[1])
		rel := p.Minus(a.origin)
		if rel.Minus(a.axis.ScaledBy(rel.Dot(a.axis))).Magnitude() > geom.LengthEps {
			return p
		}
	}
	return s.PointAt(0, 0)
}

// patchSize is the diagonal of the patch's control points' box.
func patchSize(s *srf.SSurface) float64 {
	lo, hi := s.Ctrl[0][0], s.Ctrl[0][0]
	for i := 0; i <= s.DegM; i++ {
		for j := 0; j <= s.DegN; j++ {
			p := s.Ctrl[i][j]
			lo = geom.Vector{X: math.Min(lo.X, p.X), Y: math.Min(lo.Y, p.Y), Z: math.Min(lo.Z, p.Z)}
			hi = geom.Vector{X: math.Max(hi.X, p.X), Y: math.Max(hi.Y, p.Y), Z: math.Max(hi.Z, p.Z)}
		}
	}
	return hi.Minus(lo).Magnitude()
}

// planeOf finds the plane every control point of s lies in, its normal
// the face's.
func planeOf(s *srf.SSurface, tol float64) (analytic, bool) {
	n := s.NormalAt(0.5, 0.5)
	if n.Magnitude() == 0 {
		return analytic{}, false
	}
	n = n.WithMagnitude(1)
	o := s.PointAt(0.5, 0.5)
	for i := 0; i <= s.DegM; i++ {
		for j := 0; j <= s.DegN; j++ {
			if math.Abs(s.Ctrl[i][j].Minus(o).Dot(n)) > tol {
				return analytic{}, false
			}
		}
	}
	return analytic{kind: analyticPlane, origin: o, axis: n}, true
}

// torusOf finds the torus s lies on: its parallels, the curves along one
// parameter, circles about one axis, and its meridians, along the
// other, circles of one radius whose centres are as far from the axis.
// A torus whose tube is as wide as its hole or wider, which crosses
// itself, is left a B-spline.
func torusOf(s *srf.SSurface, tol float64) (analytic, bool) {
	if s.DegM < 2 || s.DegN < 2 {
		return analytic{}, false
	}
	for _, swap := range []bool{false, true} {
		at := func(a, b float64) geom.Vector {
			if swap {
				return s.PointAt(b, a)
			}
			return s.PointAt(a, b)
		}
		// A parallel at b = 0.5, a meridian at a = 0.5.
		pc, pn, _, ok := circleThrough(at(0, 0.5), at(0.5, 0.5), at(1, 0.5))
		if !ok {
			continue
		}
		mc, _, mr, ok := circleThrough(at(0.5, 0), at(0.5, 0.5), at(0.5, 1))
		if !ok {
			continue
		}
		axis := pn
		centre := pc.Plus(axis.ScaledBy(mc.Minus(pc).Dot(axis)))
		major := mc.Minus(centre).Magnitude()
		if major <= mr+tol {
			continue
		}
		a := analytic{kind: analyticTorus, origin: centre, axis: axis, major: major, minor: mr}
		if a.holds(s, tol) {
			return a, true
		}
	}
	return analytic{}, false
}

// holds reports whether every point of a 5 by 5 grid over s lies on a.
func (a analytic) holds(s *srf.SSurface, tol float64) bool {
	for i := range 5 {
		for j := range 5 {
			if math.Abs(a.offset(s.PointAt(float64(i)/4, float64(j)/4))) > tol {
				return false
			}
		}
	}
	return true
}

// holdsAll reports whether every patch lies on a, as all the patches of
// a face written as one surface must: a swept pipe's one smooth face is
// a cylinder along the path's line and a torus round its arc.
func (a analytic) holdsAll(patches []*srf.SSurface) bool {
	for _, p := range patches {
		if !a.holds(p, 1e-7*patchSize(p)+geom.LengthEps/10) {
			return false
		}
	}
	return true
}

// offset is how far p is from the surface, along its normal.
func (a analytic) offset(p geom.Vector) float64 {
	rel := p.Minus(a.origin)
	z := rel.Dot(a.axis)
	rho := rel.Minus(a.axis.ScaledBy(z)).Magnitude()
	switch a.kind {
	case analyticPlane:
		return z
	case analyticCylinder:
		return rho - a.major
	case analyticCone:
		// From the cone's line in the half plane through p; at the
		// origin the radius is major.
		return (rho-a.major)*libm.Cos(a.angle) - z*libm.Sin(a.angle)
	case analyticSphere:
		return rel.Magnitude() - a.major
	case analyticTorus:
		return libm.Hypot(rho-a.major, z) - a.minor
	}
	return math.Inf(1)
}

// circleThrough is the circle through three points: its centre, unit
// normal (the way p0, p1, p2 turn about it) and radius; false when they
// lie on a line.
func circleThrough(p0, p1, p2 geom.Vector) (c, n geom.Vector, r float64, ok bool) {
	ab, ac := p1.Minus(p0), p2.Minus(p0)
	nn := ab.Cross(ac)
	m := nn.MagSquared()
	if m == 0 || m < 1e-18*ab.MagSquared()*ac.MagSquared() {
		return geom.Vector{}, geom.Vector{}, 0, false
	}
	v := nn.Cross(ab).ScaledBy(ac.MagSquared()).Plus(ac.Cross(nn).ScaledBy(ab.MagSquared()))
	c = p0.Plus(v.ScaledBy(1 / (2 * m)))
	return c, nn.WithMagnitude(1), p0.Minus(c).Magnitude(), true
}

// normalAt is the surface's own normal at p, as STEP orients it: a
// plane's along its axis, the others' away from their axis, centre or
// tube.
func (a analytic) normalAt(p geom.Vector) geom.Vector {
	rel := p.Minus(a.origin)
	radial := rel.Minus(a.axis.ScaledBy(rel.Dot(a.axis)))
	switch a.kind {
	case analyticPlane:
		return a.axis
	case analyticCylinder:
		return radial
	case analyticCone:
		if radial.Magnitude() == 0 {
			return geom.Vector{}
		}
		return radial.WithMagnitude(1).ScaledBy(libm.Cos(a.angle)).Minus(a.axis.ScaledBy(libm.Sin(a.angle)))
	case analyticSphere:
		return rel
	case analyticTorus:
		if radial.Magnitude() == 0 {
			return geom.Vector{}
		}
		return p.Minus(a.origin.Plus(radial.WithMagnitude(a.major)))
	}
	return geom.Vector{}
}

// sameSense tells whether the face's normal on s, at its middle, is the
// way the analytic surface's own points.
func (a analytic) sameSense(s *srf.SSurface) bool {
	return a.normalAt(s.PointAt(0.5, 0.5)).Dot(s.NormalAt(0.5, 0.5)) >= 0
}

// scaled is a placed in a file whose lengths are the model's times
// scale.
func (a analytic) scaled(scale float64) analytic {
	a.origin = a.origin.ScaledBy(scale)
	a.major *= scale
	a.minor *= scale
	return a
}

// writeAnalytic writes the surface a, its reference direction towards
// seam (where a closed surface's parameter starts, so that its seam lies
// where the face's does), and returns its id.
func (w *stepFileWriter) writeAnalytic(a analytic, seam geom.Vector) int {
	z := a.axis
	x := seam.Minus(a.origin)
	x = x.Minus(z.ScaledBy(x.Dot(z)))
	if x.Magnitude() < geom.LengthEps {
		x = z.Normal()
	}
	x = x.WithMagnitude(1)
	place := w.writePlacement(a.origin, z, x)
	id := w.id
	switch a.kind {
	case analyticPlane:
		w.printf("#%d=PLANE('',#%d);\n", id, place)
	case analyticCylinder:
		w.printf("#%d=CYLINDRICAL_SURFACE('',#%d,%s);\n", id, place, cfmtReal(a.major))
	case analyticCone:
		w.printf("#%d=CONICAL_SURFACE('',#%d,%s,%s);\n", id, place, cfmtReal(a.major), cfmtReal(a.angle))
	case analyticSphere:
		w.printf("#%d=SPHERICAL_SURFACE('',#%d,%s);\n", id, place, cfmtReal(a.major))
	case analyticTorus:
		w.printf("#%d=TOROIDAL_SURFACE('',#%d,%s,%s);\n", id, place, cfmtReal(a.major), cfmtReal(a.minor))
	}
	w.id++
	return id
}

// writePlacement writes an AXIS2_PLACEMENT_3D at o, its axis z and its
// reference direction x, and returns its id.
func (w *stepFileWriter) writePlacement(o, z, x geom.Vector) int {
	id := w.id
	w.printf(stepCartesianPointFormat, id, o.X, o.Y, o.Z)
	w.printf("#%d=DIRECTION('',(%s));\n", id+1, stepDirection(z))
	w.printf("#%d=DIRECTION('',(%s));\n", id+2, stepDirection(x))
	w.printf("#%d=AXIS2_PLACEMENT_3D('',#%d,#%d,#%d);\n", id+3, id, id+1, id+2)
	w.id = id + 4
	return id + 3
}
