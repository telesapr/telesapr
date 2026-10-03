package srf

import (
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// SweepStep is one piece of a sweep's path: a translation by Along,
// or, with Arc, a turn by Angle radians about the axis through Center
// along Axis, by the right-hand rule.
type SweepStep struct {
	Arc    bool
	Along  geom.Vector
	Center geom.Vector
	Axis   geom.Vector
	Angle  float64
}

// Rigid is the rigid motion p -> Q p + T.
type Rigid struct {
	Q geom.Quaternion
	T geom.Vector
}

// RigidIdentity moves nothing.
var RigidIdentity = Rigid{Q: geom.QuaternionIdentity}

// Apply moves p.
func (r Rigid) Apply(p geom.Vector) geom.Vector { return r.Q.Rotate(p).Plus(r.T) }

// Then is r followed by s.
func (r Rigid) Then(s Rigid) Rigid {
	return Rigid{Q: s.Q.Times(r.Q), T: s.Q.Rotate(r.T).Plus(s.T)}
}

// Motion is the rigid motion of the step.
func (st SweepStep) Motion() Rigid {
	if !st.Arc {
		return Rigid{Q: geom.QuaternionIdentity, T: st.Along}
	}
	q := geom.QuaternionFromAxisAngle(st.Axis.WithMagnitude(1), st.Angle)
	return Rigid{Q: q, T: st.Center.Minus(q.Rotate(st.Center))}
}

// SweepEnd is where the steps take the profile.
func SweepEnd(steps []SweepStep) Rigid {
	r := RigidIdentity
	for _, st := range steps {
		r = r.Then(st.Motion())
	}
	return r
}

// TransformedLoopSet is a copy of sbls moved by r.
func TransformedLoopSet(sbls *SBezierLoopSet, r Rigid) *SBezierLoopSet {
	out := &SBezierLoopSet{
		Normal: r.Q.Rotate(sbls.Normal),
		Point:  r.Apply(sbls.Point),
		Area:   sbls.Area,
	}
	move := func(l *SBezierLoop) SBezierLoop {
		var m SBezierLoop
		for i := range l.Curves {
			m.Curves = append(m.Curves, transformedCopy(&l.Curves[i], r.T, r.Q, 1))
		}
		return m
	}
	out.Outer = move(&sbls.Outer)
	for i := range sbls.Holes {
		out.Holes = append(out.Holes, move(&sbls.Holes[i]))
	}
	return out
}

// sweepSections splits arcs into turns of at most a quarter, each
// turning by a positive angle, and drops steps that go nowhere.
func sweepSections(steps []SweepStep) []SweepStep {
	var out []SweepStep
	for _, st := range steps {
		if !st.Arc {
			if st.Along.Magnitude() > lengthEps {
				out = append(out, st)
			}
			continue
		}
		axis := st.Axis.WithMagnitude(1)
		angle := st.Angle
		if angle < 0 {
			axis, angle = axis.ScaledBy(-1), -angle
		}
		if angle < 1e-12 {
			continue
		}
		n := int(angle/(math.Pi/2) + 1)
		for range n {
			out = append(out, SweepStep{Arc: true, Center: st.Center, Axis: axis, Angle: angle / float64(n)})
		}
	}
	return out
}

// sweepStartDirection is the direction the steps first move the point
// p in.
func sweepStartDirection(st SweepStep, p geom.Vector) geom.Vector {
	if !st.Arc {
		return st.Along
	}
	return st.Axis.Cross(p.Minus(st.Center))
}

// MakeFromSweepOf adds the solid the planar loop set sbls sweeps
// through as it moves along steps, a chain whose pieces meet tangent:
// one side surface per curve and piece, an extrusion for a straight
// piece and a surface of revolution for each quarter of a turn, and a
// cap at each end, startFace at the profile and endFace where the
// steps take it. The joints between pieces are shared curves, so no
// Boolean is needed. The profile must cross the path, and stay off the
// axis of every turn.
func (s *SShell) MakeFromSweepOf(sbls *SBezierLoopSet, steps []SweepStep, color core.RgbaColor, startFace, endFace FaceID) {
	if sbls == nil {
		return
	}
	steps = sweepSections(steps)
	if len(steps) == 0 {
		return
	}
	// As for an extrusion, the motion has to leave the sketch against
	// its normal; else the sweep is made from the other end.
	if sweepStartDirection(steps[0], sbls.Point).Dot(sbls.Normal) > 0 {
		sbls = TransformedLoopSet(sbls, SweepEnd(steps))
		rev := make([]SweepStep, len(steps))
		for i, st := range steps {
			st.Along = st.Along.ScaledBy(-1)
			st.Axis = st.Axis.ScaledBy(-1)
			rev[len(steps)-1-i] = st
		}
		steps = rev
		startFace, endFace = endFace, startFace
	}

	// The frames: where each piece starts, and where the last ends.
	frames := make([]Rigid, len(steps)+1)
	frames[0] = RigidIdentity
	for k, st := range steps {
		frames[k+1] = frames[k].Then(st.Motion())
	}
	end := frames[len(steps)]

	// The caps, in the csys of the sketch as an extrusion's are.
	n := sbls.Normal.ScaledBy(-1)
	u := n.NormalDir(0)
	v := n.NormalDir(1)
	orig := sbls.Point
	umin, umax := math.Inf(1), math.Inf(-1)
	sbls.GetBoundingProjd(u, orig, &umin, &umax)
	vmin, vmax := math.Inf(1), math.Inf(-1)
	sbls.GetBoundingProjd(v, orig, &vmin, &vmax)
	orig = orig.Plus(u.ScaledBy(umin)).Plus(v.ScaledBy(vmin))
	u = u.ScaledBy(umax - umin)
	v = v.ScaledBy(vmax - vmin)

	s0 := FromPlane(orig, u, v)
	s0.Color, s0.TrimBuilt, s0.Face = color, true, startFace
	s1 := FromPlane(end.Apply(orig.Plus(u)), end.Q.Rotate(u.ScaledBy(-1)), end.Q.Rotate(v))
	s1.Color, s1.TrimBuilt, s1.Face = color, true, endFace
	hs0 := s.addAndAssignSurfaceId(s0)
	hs1 := s.addAndAssignSurfaceId(s1)

	type turn struct {
		center, axis geom.Vector
		from, to     int
	}
	var turns []turn

	for _, loop := range sbls.loops() {
		nc := len(loop.Curves)
		// srfs[k][i] is the surface curve i sweeps along piece k.
		srfs := make([][]core.HSSurface, len(steps))
		for k, st := range steps {
			srfs[k] = make([]core.HSSurface, nc)
			from := len(s.Surface)
			for i := range loop.Curves {
				b := transformedCopy(&loop.Curves[i], frames[k].T, frames[k].Q, 1)
				var ss SSurface
				if st.Arc {
					ss = FromRevolutionOf(&b, st.Center, st.Axis, 0, st.Angle, 0, 0)
				} else {
					ss = FromExtrusionOf(&b, geom.Vector{}, st.Along)
				}
				ss.Color, ss.TrimBuilt = color, true
				srfs[k][i] = s.addAndAssignSurfaceId(ss)
			}
			if st.Arc {
				turns = append(turns, turn{st.Center, st.Axis, from, len(s.Surface)})
			}
		}

		for i := range loop.Curves {
			// A turn's surface is triangulated on a grid; as for a
			// helix, the copies of the curve are cut at the grid's
			// values, or its triangles come out crossed at the joints.
			var tValues []float64
			for k, st := range steps {
				if st.Arc {
					tValues = append(tValues, 0)
					s.FindSurface(srfs[k][i]).MakeTriangulationGridInto(&tValues, 0, 1, true, 0)
					break
				}
			}
			// The curve where each piece starts and the last ends,
			// shared by the surfaces either side, or a cap.
			for j := 0; j <= len(steps); j++ {
				sc := SCurve{IsExact: true, Exact: transformedCopy(&loop.Curves[i], frames[j].T, frames[j].Q, 1)}
				if tValues == nil {
					sc.Points = makePwlSCurvePoints(&sc.Exact, 0)
				}
				for x, t := range tValues {
					sc.Points = append(sc.Points, SCurvePt{P: sc.Exact.PointAt(t), Vertex: x == 0 || x == len(tValues)-1})
				}
				if j < len(steps) {
					sc.SurfA = srfs[j][i]
				} else {
					sc.SurfA = hs1
				}
				if j > 0 {
					sc.SurfB = srfs[j-1][i]
				} else {
					sc.SurfB = hs0
				}
				s.addCurveTrimming(sc, true)
			}
			// And along each piece, the path of the curve's start,
			// between this curve's surface and the one before's.
			ip := (i - 1 + nc) % nc
			for k, st := range steps {
				var sc SCurve
				if st.Arc {
					ss := s.FindSurface(srfs[k][i])
					sc = SCurve{IsExact: true, Exact: SBezierFrom3(ss.Ctrl[0][0], ss.Ctrl[0][1], ss.Ctrl[0][2])}
					sc.Exact.Weight[1] = ss.Weight[0][1]
					// As a helix samples its rims.
					maxDt := 0.5
					if sc.Exact.Degree > 1 {
						maxDt = 0.125
					}
					sc.Points = makePwlSCurvePoints(&sc.Exact, maxDt)
					sc.SurfA = srfs[k][i]
					sc.SurfB = srfs[k][ip]
					s.addCurveTrimming(sc, false)
				} else {
					p := frames[k].Apply(loop.Curves[i].Start())
					sc = SCurve{IsExact: true, Exact: SBezierFrom2(p, p.Plus(st.Along))}
					sc.Points = makePwlSCurvePoints(&sc.Exact, 0)
					sc.SurfA = srfs[k][ip]
					sc.SurfB = srfs[k][i]
					s.addCurveTrimming(sc, true)
				}
			}
		}
	}

	for _, t := range turns {
		s.makeFirstOrderRevolvedRange(t.center, t.axis, t.from, t.to)
	}
}
