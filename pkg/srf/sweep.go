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
	// Corner turns the profile where it is, about Axis through Center by
	// Angle, between two straight pieces that meet at an angle there: the
	// pieces are mitred, each cut by the plane that halves the corner
	// (roadmap 28.4).
	Corner bool
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
	if !st.Arc && !st.Corner {
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
			m.Curves = append(m.Curves, transformedCopy(&l.Curves[i], r.T, r.Q))
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
		if st.Corner {
			if st.Angle > 1e-12 {
				out = append(out, st)
			}
			continue
		}
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
func (s *SShell) MakeFromSweepOf(sbls *SBezierLoopSet, steps []SweepStep, color core.RGBA, startFace, endFace FaceID) {
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

	// The frames: where each piece starts, and where the last ends. A
	// corner is no piece: it turns the frame where the profile is.
	frames := make([]Rigid, 0, len(steps)+1)
	var pieces []SweepStep
	// mitre[k] is the plane piece k's start is cut by, after a corner;
	// mitre[k+1] its end's.
	type plane struct {
		at, n geom.Vector
		ok    bool
	}
	var mitre []plane
	f := RigidIdentity
	var pending plane
	for _, st := range steps {
		if st.Corner {
			pending = plane{at: st.Center, ok: true}
			f = f.Then(st.Motion())
			continue
		}
		frames = append(frames, f)
		mitre = append(mitre, pending)
		pending = plane{}
		pieces = append(pieces, st)
		f = f.Then(st.Motion())
	}
	frames = append(frames, f)
	mitre = append(mitre, plane{})
	for k := range mitre {
		if !mitre[k].ok {
			continue
		}
		// The plane halving the corner between the piece before and this.
		d1 := pieces[k-1].Along.WithMagnitude(1)
		d2 := pieces[k].Along.WithMagnitude(1)
		mitre[k].n = d1.Plus(d2).WithMagnitude(1)
	}
	steps = pieces
	end := frames[len(steps)]
	// section is curve c at joint j: moved by the frame there, or where
	// a mitre cuts there, the curve as the piece before carries it
	// projected along that piece onto the mitre's plane.
	section := func(c *SBezier, j int) SBezier {
		if !mitre[j].ok {
			return transformedCopy(c, frames[j].T, frames[j].Q)
		}
		before := frames[j-1].Then(steps[j-1].Motion())
		b := transformedCopy(c, before.T, before.Q)
		return projectedAlong(b, steps[j-1].Along.WithMagnitude(1), mitre[j].at, mitre[j].n)
	}
	// The point p's place at joint j, as section moves it.
	pointAt := func(p geom.Vector, j int) geom.Vector {
		if !mitre[j].ok {
			return frames[j].Apply(p)
		}
		before := frames[j-1].Then(steps[j-1].Motion())
		return projectPoint(before.Apply(p), steps[j-1].Along.WithMagnitude(1), mitre[j].at, mitre[j].n)
	}

	// The caps, in the csys of the sketch as an extrusion's are.
	n := sbls.Normal.ScaledBy(-1)
	u := n.NormalDir(0)
	v := n.NormalDir(1)
	orig := sbls.Point
	umin, umax := math.Inf(1), math.Inf(-1)
	sbls.BoundingProjd(u, orig, &umin, &umax)
	vmin, vmax := math.Inf(1), math.Inf(-1)
	sbls.BoundingProjd(v, orig, &vmin, &vmax)
	orig = orig.Plus(u.ScaledBy(umin)).Plus(v.ScaledBy(vmin))
	u = u.ScaledBy(umax - umin)
	v = v.ScaledBy(vmax - vmin)

	s0 := FromPlane(orig, u, v)
	s0.Color, s0.TrimBuilt, s0.Face = color, true, startFace
	s1 := FromPlane(end.Apply(orig.Plus(u)), end.Q.Rotate(u.ScaledBy(-1)), end.Q.Rotate(v))
	s1.Color, s1.TrimBuilt, s1.Face = color, true, endFace
	hs0 := s.addAndAssignSurfaceID(s0)
	hs1 := s.addAndAssignSurfaceID(s1)

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
				b := transformedCopy(&loop.Curves[i], frames[k].T, frames[k].Q)
				var ss SSurface
				mitred := mitre[k].ok || mitre[k+1].ok
				switch {
				case st.Arc:
					ss = FromRevolutionOf(&b, st.Center, st.Axis, 0, st.Angle, 0, 0)

				case mitred:
					// Past each mitre, whose section leans out of the piece:
					// the surface reaches over the whole of it.
					ext := 1.0
					if mitre[k].ok {
						ext = math.Max(ext, mitreReach(&b, st.Along, mitre[k].at, mitre[k].n))
					}
					if mitre[k+1].ok {
						e := translatedCopy(b, st.Along)
						ext = math.Max(ext, mitreReach(&e, st.Along, mitre[k+1].at, mitre[k+1].n))
					}
					d := st.Along.WithMagnitude(ext)
					ss = FromExtrusionOf(&b, d.ScaledBy(-1), st.Along.Plus(d))

				default:
					ss = FromExtrusionOf(&b, geom.Vector{}, st.Along)
				}
				ss.Color, ss.TrimBuilt = color, true
				// A mitre trims the side across its rulings, along a
				// curve in the mitre's plane.
				ss.AvoidFlatEars = mitred
				srfs[k][i] = s.addAndAssignSurfaceID(ss)
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
				sc := SCurve{IsExact: true, Exact: section(&loop.Curves[i], j)}
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
					p, q := pointAt(loop.Curves[i].Start(), k), pointAt(loop.Curves[i].Start(), k+1)
					if !mitre[k+1].ok {
						q = frames[k].Apply(loop.Curves[i].Start()).Plus(st.Along)
					}
					sc = SCurve{IsExact: true, Exact: SBezierFrom2(p, q)}
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

// projectPoint is p moved along d onto the plane through at of normal
// n.
func projectPoint(p, d, at, n geom.Vector) geom.Vector {
	t := at.Minus(p).Dot(n) / d.Dot(n)
	return p.Plus(d.ScaledBy(t))
}

// projectedAlong is b moved along d onto the plane through at of normal
// n, point by point: an affine map, so its control points with the
// same weights.
func projectedAlong(b SBezier, d, at, n geom.Vector) SBezier {
	for i := 0; i <= b.Degree; i++ {
		b.Ctrl[i] = projectPoint(b.Ctrl[i], d, at, n)
	}
	return b
}

// mitreReach is how far along a straight piece's direction the
// control points of b, a section square to it, move onto the plane
// through at of normal n, and a millimetre more: how far past that end
// the piece's surface must reach to hold the mitred section.
func mitreReach(b *SBezier, along, at, n geom.Vector) float64 {
	d := along.WithMagnitude(1)
	reach := 0.0
	for i := 0; i <= b.Degree; i++ {
		t := at.Minus(b.Ctrl[i]).Dot(n) / d.Dot(n)
		reach = math.Max(reach, math.Abs(t))
	}
	return reach + 1
}

// translatedCopy is b moved by by.
func translatedCopy(b SBezier, by geom.Vector) SBezier {
	for i := 0; i <= b.Degree; i++ {
		b.Ctrl[i] = b.Ctrl[i].Plus(by)
	}
	return b
}
