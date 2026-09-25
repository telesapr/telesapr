package sketch

import (
	"math"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/srf"
)

// GenerateEntityBeziers populates the supplied SBezierList with the
// piecewise-rational Bezier representation of this entity. Used by
// Group::AssembleLoops to convert sketch entities into trim curves
// for SShell::MakeFromExtrusionOf and friends.
func GenerateEntityBeziers(e *core.Entity, beziers *srf.SBezierList, rt EntityBezierResolver) {
	if beziers == nil {
		return
	}
	switch e.Type {
	case core.EntityLineSegment:
		a, errA := rt.Point(e.Point[0])
		b, errB := rt.Point(e.Point[1])
		if errA != nil || errB != nil {
			return
		}
		sb := srf.SBezierFrom2(a, b)
		sb.AuxA = int(e.H.V)
		beziers.Add(sb)
	case core.EntityCubic:
		var pts [4]geom.Vector
		ok := true
		for i := 0; i < 4; i++ {
			p, err := rt.Point(e.Point[i+1])
			if err != nil {
				ok = false
				break
			}
			pts[i] = p
		}
		if !ok {
			return
		}
		sb := srf.SBezierFrom4(pts[0], pts[1], pts[2], pts[3])
		sb.AuxA = int(e.H.V)
		beziers.Add(sb)
	case core.EntityCircle, core.EntityArcOfCircle:
		center, errC := rt.Point(e.Point[0])
		if errC != nil {
			return
		}
		normalQ, errN := rt.Normal(e.Normal)
		if errN != nil {
			return
		}
		r, errR := rt.Radius(e.Distance)
		if errR != nil {
			return
		}
		if r < 1e-9 {
			return
		}
		u := normalQ.RotationU()
		v := normalQ.RotationV()
		thetaa, thetaf := 0.0, 2*math.Pi
		sweep := thetaf - thetaa
		nArcs := int(sweep/(3*math.Pi/2)+0.5) + 1
		if nArcs < 1 {
			nArcs = 1
		}
		if nArcs > 4 {
			nArcs = 4
		}
		dtheta := sweep / float64(nArcs)
		for k := 0; k < nArcs; k++ {
			t0 := thetaa + float64(k)*dtheta
			t1 := t0 + dtheta
			c0, s0 := math.Cos(t0), math.Sin(t0)
			c1, s1 := math.Cos(t1), math.Sin(t1)
			p0 := center.Plus(u.ScaledBy(r*c0)).Plus(v.ScaledBy(r*s0))
			p2 := center.Plus(u.ScaledBy(r*c1)).Plus(v.ScaledBy(r*s1))
			tangent0 := u.ScaledBy(-r*s0).Plus(v.ScaledBy(r*c0))
			tangent2 := u.ScaledBy(-r*s1).Plus(v.ScaledBy(r*c1))
			p1 := geom.AtIntersectionOfLines(p0, p0.Plus(tangent0), p2, p2.Plus(tangent2))
			sb := srf.SBezierFrom3(p0, p1, p2)
			sb.Weight[1] = math.Cos(dtheta / 2)
			sb.AuxA = int(e.H.V)
			beziers.Add(sb)
		}
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
}
