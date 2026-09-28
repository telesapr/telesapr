package sketch

import (
	"math"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/libm"
	"github.com/spacesolve/spacesolve/pkg/srf"
	"github.com/spacesolve/spacesolve/pkg/ttf"
)

// GenerateEntityBeziers populates the supplied SBezierList with the
// piecewise-rational Bezier representation of this entity. Used by
// Group::AssembleLoops to convert sketch entities into trim curves
// for SShell::MakeFromExtrusionOf and friends. Mirrors C++
// Entity::GenerateBezierCurves (drawentity.cpp:389).
func GenerateEntityBeziers(e *core.Entity, beziers *srf.SBezierList, rt EntityBezierResolver) {
	if beziers == nil {
		return
	}
	first := beziers.Len()

	switch e.Type {
	case core.EntityLineSegment:
		a, errA := rt.Point(e.Point[0])
		b, errB := rt.Point(e.Point[1])
		if errA != nil || errB != nil {
			return
		}
		sb := srf.SBezierFrom2(a, b)
		sb.Entity = e.H.V
		beziers.Add(sb)

	case core.EntityCubic:
		computeInterpolatingSpline(e, beziers, rt, false)

	case core.EntityCubicPeriodic:
		computeInterpolatingSpline(e, beziers, rt, true)

	case core.EntityCircle, core.EntityArcOfCircle:
		center, errC := rt.Point(e.Point[0])
		if errC != nil {
			return
		}
		q, errN := rt.Normal(e.Normal)
		if errN != nil {
			return
		}
		r, errR := rt.Radius(e.Distance)
		if e.Type == core.EntityArcOfCircle {
			// An arc's radius is the distance from its center to its
			// start point (C++ EntityBase::CircleGetRadiusNum).
			pa, errA := rt.Point(e.Point[1])
			if errA != nil {
				return
			}
			r, errR = pa.Minus(center).Magnitude(), nil
		}
		if errR != nil {
			return
		}
		u, v := q.RotationU(), q.RotationV()

		if r < geom.LengthEps {
			// If a circle or an arc gets dragged through zero radius,
			// then we just don't generate anything.
			break
		}

		var thetaa, dtheta float64
		if e.Type == core.EntityCircle {
			thetaa = 0
			dtheta = 2 * math.Pi
		} else {
			var ok bool
			thetaa, dtheta, ok = arcGetAngles(e, rt, center, u, v)
			if !ok {
				return
			}
		}
		var n int
		switch {
		case dtheta > 3*math.Pi/2+0.01:
			n = 4
		case dtheta > math.Pi+0.01:
			n = 3
		case dtheta > math.Pi/2+0.01:
			n = 2
		default:
			n = 1
		}
		dtheta /= float64(n)

		for i := 0; i < n; i++ {
			c, s := libm.Cos(thetaa), libm.Sin(thetaa)
			// The start point of the curve, and the tangent vector at
			// that start point.
			p0 := center.Plus(u.ScaledBy(r * c)).Plus(v.ScaledBy(r * s))
			t0 := u.ScaledBy(-r * s).Plus(v.ScaledBy(r * c))

			thetaa += dtheta

			c, s = libm.Cos(thetaa), libm.Sin(thetaa)
			p2 := center.Plus(u.ScaledBy(r * c)).Plus(v.ScaledBy(r * s))
			t2 := u.ScaledBy(-r * s).Plus(v.ScaledBy(r * c))

			// The control point must lie on both tangents.
			p1 := geom.AtIntersectionOfLines(p0, p0.Plus(t0), p2, p2.Plus(t2))

			sb := srf.SBezierFrom3(p0, p1, p2)
			sb.Weight[1] = libm.Cos(dtheta / 2)
			beziers.Add(sb)
		}

	case core.EntityTTFText:
		topLeft, errT := rt.Point(e.Point[0])
		botLeft, errB := rt.Point(e.Point[1])
		q, errN := rt.Normal(e.Normal)
		if errT != nil || errB != nil || errN != nil {
			return
		}
		n := q.RotationN()
		v := topLeft.Minus(botLeft)
		u := v.Cross(n).WithMagnitude(v.Magnitude())

		// `ExtraPoints` is storing kerning boolean
		ttf.Fonts.PlotString(e.Font, e.Str, beziers, e.ExtraPoints != 0, botLeft, u, v)
	}

	// Record our style for all of the Beziers that we just created.
	for i := first; i < beziers.Len(); i++ {
		beziers.Curves[i].AuxA = int(rt.StyleForEntity(e.H).V)
	}
}

// arcGetAngles returns the arc's start angle and sweep in the (u, v)
// basis of its normal. Mirrors C++ EntityBase::ArcGetAngles
// (entity.cpp:147).
func arcGetAngles(e *core.Entity, rt EntityBezierResolver, c, u, v geom.Vector) (thetaa, dtheta float64, ok bool) {
	pa, errA := rt.Point(e.Point[1])
	pb, errB := rt.Point(e.Point[2])
	if errA != nil || errB != nil {
		return 0, 0, false
	}
	c2 := c.Project2d(u, v)
	pa2 := pa.Project2d(u, v).Minus(c2)
	pb2 := pb.Project2d(u, v).Minus(c2)

	thetaa = libm.Atan2(pa2.V, pa2.U)
	thetab := libm.Atan2(pb2.V, pb2.U)
	dtheta = thetab - thetaa
	// If the endpoints are coincident, call it a full arc, not a zero arc;
	// useful concept to have when splitting
	for dtheta < 1e-6 {
		dtheta += 2 * math.Pi
	}
	for dtheta > 2*math.Pi {
		dtheta -= 2 * math.Pi
	}
	return thetaa, dtheta, true
}

// splineTerm is a value expressed as a constant part plus multiples
// of X[i-1], X[i] and X[i+1]. Mirrors the Vector4 (w; x, y, z) use in
// C++ Entity::ComputeInterpolatingSpline.
type splineTerm struct{ w, x, y, z float64 }

func (a splineTerm) plus(b splineTerm) splineTerm {
	return splineTerm{a.w + b.w, a.x + b.x, a.y + b.y, a.z + b.z}
}

func (a splineTerm) minus(b splineTerm) splineTerm {
	return splineTerm{a.w - b.w, a.x - b.x, a.y - b.y, a.z - b.z}
}

func (a splineTerm) scaledBy(s float64) splineTerm {
	return splineTerm{a.w * s, a.x * s, a.y * s, a.z * s}
}

func wrapInt(v, n int) int {
	for v >= n {
		v -= n
	}
	for v < 0 {
		v += n
	}
	return v
}

// computeInterpolatingSpline builds the C2 cubic spline through the
// entity's on-curve points. Mirrors C++
// Entity::ComputeInterpolatingSpline (drawentity.cpp:252).
func computeInterpolatingSpline(e *core.Entity, sbl *srf.SBezierList, rt EntityBezierResolver, periodic bool) {
	ep := e.ExtraPoints

	// The number of unknowns to solve for.
	n := ep
	if periodic {
		n = 3 + ep
	}
	if n >= geom.BandedMaxUnknowns {
		return
	}
	// The number of on-curve points, one more than the number of segments.
	pts := 2 + ep
	if periodic {
		pts = 4 + ep
	}

	point := func(i int) (geom.Vector, bool) {
		p, err := rt.Point(e.Point[i])
		return p, err == nil
	}

	// The starting and finishing control points that define our end tangents
	// (if the spline isn't periodic), and the on-curve points.
	var ctrlS, ctrlF geom.Vector
	pt := make([]geom.Vector, geom.BandedMaxUnknowns+4)
	var ok bool
	if periodic {
		i := 0
		for ; i < ep+3; i++ {
			if pt[i], ok = point(i); !ok {
				return
			}
		}
		if pt[i], ok = point(0); !ok {
			return
		}
	} else {
		if ctrlS, ok = point(1); !ok {
			return
		}
		if ctrlF, ok = point(ep + 2); !ok {
			return
		}
		j := 0
		if pt[j], ok = point(0); !ok {
			return
		}
		j++
		for i := 2; i <= ep+1; i++ {
			if pt[j], ok = point(i); !ok {
				return
			}
			j++
		}
		if pt[j], ok = point(ep + 3); !ok {
			return
		}
	}

	// The unknowns that we will be solving for, a set for each coordinate.
	var X [3][geom.BandedMaxUnknowns]float64
	// For a cubic Bezier section f(t) as t goes from 0 to 1,
	//    f' (0) = 3*(P1 - P0)
	//    f' (1) = 3*(P3 - P2)
	//    f''(0) = 6*(P0 - 2*P1 + P2)
	//    f''(1) = 6*(P3 - 2*P2 + P1)
	for a := 0; a < 3; a++ {
		var bm geom.BandedMatrix
		bm.N = n

		for i := 0; i < n; i++ {
			var im, it, ip int
			if periodic {
				im = wrapInt(i-1, n)
				it = i
				ip = wrapInt(i+1, n)
			} else {
				im = i
				it = i + 1
				ip = i + 2
			}
			// The on-curve interpolated point
			C := splineTerm{w: pt[it].Element(a)}
			// control point one back, C - X[i]
			B := C.plus(splineTerm{y: -1})
			// control point one forward, C + X[i]
			D := C.plus(splineTerm{y: 1})
			// control point two back
			var A, E splineTerm
			if i == 0 && !periodic {
				A = splineTerm{w: ctrlS.Element(a)}
			} else {
				// pt[im] + X[i-1]
				A = splineTerm{w: pt[im].Element(a), x: 1}
			}
			// control point two forward
			if i == n-1 && !periodic {
				E = splineTerm{w: ctrlF.Element(a)}
			} else {
				// pt[ip] - X[i+1]
				E = splineTerm{w: pt[ip].Element(a), z: -1}
			}
			// Write the second derivatives of each segment, dropping constant
			fprevPP := C.minus(B.scaledBy(2)).plus(A)
			fnextPP := C.minus(D.scaledBy(2)).plus(E)
			eq := fprevPP.minus(fnextPP)

			bm.B[i] = -eq.w
			if periodic {
				bm.A[i][wrapInt(i-2, n)] = eq.x
				bm.A[i][wrapInt(i-1, n)] = eq.y
				bm.A[i][i] = eq.z
			} else {
				// The wrapping would work, except when n = 1 and everything
				// wraps to zero...
				if i > 0 {
					bm.A[i][i-1] = eq.x
				}
				bm.A[i][i] = eq.y
				if i < n-1 {
					bm.A[i][i+1] = eq.z
				}
			}
		}
		bm.Solve()
		X[a] = bm.X
	}
	xv := func(i int) geom.Vector {
		return geom.Vector{X: X[0][i], Y: X[1][i], Z: X[2][i]}
	}

	for i := 0; i < pts-1; i++ {
		var p0, p1, p2, p3 geom.Vector
		switch {
		case periodic:
			p0 = pt[i]
			p1 = p0.Plus(xv(wrapInt(i-1, n)))
		case i == 0:
			p0 = pt[0]
			p1 = ctrlS
		default:
			p0 = pt[i]
			p1 = p0.Plus(xv(i - 1))
		}
		switch {
		case periodic:
			p3 = pt[i+1]
			p2 = p3.Minus(xv(wrapInt(i, n)))
		case i == pts-2:
			p3 = pt[pts-1]
			p2 = ctrlF
		default:
			p3 = pt[i+1]
			p2 = p3.Minus(xv(i))
		}
		sbl.Add(srf.SBezierFrom4(p0, p1, p2, p3))
	}
}

// EntityBezierResolver is the lookup interface that
// GenerateEntityBeziers needs to evaluate entity points, normals,
// and radii at solve time. The sketch.Runtime satisfies this
// interface.
type EntityBezierResolver interface {
	Point(core.HEntity) (geom.Vector, error)
	Normal(core.HEntity) (geom.Quaternion, error)
	Radius(core.HEntity) (float64, error)
	// StyleForEntity is the style the entity draws in (C++
	// Style::ForEntity).
	StyleForEntity(core.HEntity) core.HStyle
}
