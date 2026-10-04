package sketch

import (
	"errors"
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// A sweep moves the loops of its profile group along a path, a chain
// of line segments, arcs and cubics that meet tangent, keeping the
// profile's attitude to the path. It starts at the end of the chain
// nearest the profile's plane, with the profile where it is drawn. Its
// entities are copies of the profile at both ends, numeric like a
// lathe's, and the faces of the two caps.
//
// Along lines and arcs alone the surfaces are exact: each piece moves
// the profile rigidly (srf.MakeFromSweepOf). Along a cubic, or with a
// twist (ValC, in degrees, over the whole path) or the frames held to a
// direction (SweepUp), the surface is fitted to the true sweep by
// rotation-minimising frames (topo.MakePathSweep, roadmap E10).

// sweepTol is how far apart two ends of the path may be and still
// join.
const sweepTol = 1e-6

// sweepTangentCos is how far from straight a joint may turn and still
// be smooth: about 0.1 degree.
const sweepTangentCos = 1 - 1.5e-6

// NewSweep starts a sweep of the active group's sketch along path, line
// segments and arcs joined end to end and meeting tangent. It is what
// the interface and scripts add with AddGroupAfterActive.
func (sk *Sketch) NewSweep(path []core.HEntity, name string) (Group, error) {
	return sk.NewSweepAlong(path, nil, name)
}

// NewSweepAlong is NewSweep along path and edges of the solid so far,
// by name, together one chain (roadmap 28.4): straight and circular
// edges, and curved ones as cubics are.
func (sk *Sketch) NewSweepAlong(path []core.HEntity, solidEdges []topo.Name, name string) (Group, error) {
	g := NewGroup()
	g.Type = GroupSweep
	g.Name = name
	if name == "" {
		g.Name = "sweep"
	}
	src := sk.GetGroup(sk.Active)
	if src == nil || src.Type != GroupDrawingWorkplane {
		return g, errors.New("the active group must be a sketch in a workplane: its loops are the profile swept")
	}
	if src.PolyError.How != int(core.PolyGood) || len(src.BezierLoops.Sets) == 0 {
		return g, errors.New("the active sketch must be closed loops to sweep")
	}
	if len(path) == 0 && len(solidEdges) == 0 {
		return g, errors.New("there is no path to sweep along")
	}
	g.OpA = src.H
	g.Edges = append([]core.HEntity(nil), path...)
	g.SolidEdges = append([]topo.Name(nil), solidEdges...)
	if _, _, err := g.sweepPath(sk); err != nil {
		return g, err
	}
	return g, nil
}

// pathPiece is one entity of a sweep's path, oriented along the
// chain.
type pathPiece struct {
	h          core.HEntity
	start, end geom.Vector
	arc        bool
	center     geom.Vector
	axis       geom.Vector // unit, turning from start to end by the right-hand rule
	angle      float64
	// curves are the piece's Beziers, in order along the chain; free
	// marks a piece that is neither a line nor an arc.
	curves []srf.SBezier
	free   bool
}

func (p pathPiece) reversed() pathPiece {
	p.start, p.end = p.end, p.start
	p.axis = p.axis.ScaledBy(-1)
	cs := make([]srf.SBezier, len(p.curves))
	for i, c := range p.curves {
		c.Reverse()
		cs[len(cs)-1-i] = c
	}
	p.curves = cs
	return p
}

// tangentAt is the piece's unit direction of travel at its start, or
// with atEnd at its end.
func (p pathPiece) tangentAt(atEnd bool) geom.Vector {
	if p.free {
		if atEnd {
			c := p.curves[len(p.curves)-1]
			return c.TangentAt(1).WithMagnitude(1)
		}
		return p.curves[0].TangentAt(0).WithMagnitude(1)
	}
	if !p.arc {
		return p.end.Minus(p.start).WithMagnitude(1)
	}
	at := p.start
	if atEnd {
		at = p.end
	}
	return p.axis.Cross(at.Minus(p.center)).WithMagnitude(1)
}

func (p pathPiece) step() srf.SweepStep {
	if !p.arc {
		return srf.SweepStep{Along: p.end.Minus(p.start)}
	}
	return srf.SweepStep{Arc: true, Center: p.center, Axis: p.axis, Angle: p.angle}
}

// pathError is a problem with the path, at a point.
type pathError struct {
	msg string
	at  *geom.Vector
}

func (e *pathError) Error() string { return e.msg }

func pathErr(at *geom.Vector, format string, args ...any) error {
	return &pathError{msg: fmt.Sprintf(format, args...), at: at}
}

// sweepPath orders the group's path into a chain starting nearest the
// profile, and returns its steps and the profile's loops.
func (g *Group) sweepPath(sk *Sketch) ([]srf.SweepStep, *Group, error) {
	src := sk.GetGroup(g.OpA)
	if src == nil {
		return nil, nil, errors.New("the profile group is gone")
	}
	sbls := &src.BezierLoops
	if len(sbls.Sets) == 0 {
		return nil, src, errors.New("the profile has no closed loops")
	}
	normal, onPlane := sbls.Sets[0].Normal, sbls.Sets[0].Point
	// Start at the end nearest the profile's plane.
	before := g.PreviousGroup(sk)
	if before == nil {
		// A sweep being made comes after the active group.
		before = sk.GetGroup(sk.Active)
	}
	solidPieces, err := sk.solidEdgePieces(before, g.SolidEdges)
	if err != nil {
		return nil, src, err
	}
	chain, _, err := sk.pathChainAny(g.Edges, solidPieces, "sweep", func(x geom.Vector) float64 {
		return math.Abs(x.Minus(onPlane).Dot(normal))
	}, true, false, true)
	if err != nil {
		return nil, src, err
	}
	cornerAt := func(k int) bool {
		return k > 0 && chain[k-1].tangentAt(true).Dot(chain[k].tangentAt(false)) < sweepTangentCos
	}
	if math.Abs(chain[0].tangentAt(false).Dot(normal)) < 1e-3 {
		return nil, src, pathErr(&chain[0].start, "the path runs along the profile's plane: it must leave it")
	}
	if g.sweepFitted(chain) {
		for k := range chain {
			if cornerAt(k) {
				x := chain[k].start
				return nil, src, pathErr(&x, "a twisted sweep, one held to a direction, or one along a curve takes no corners; "+
					"the path has one at (%g, %g, %g)", x.X, x.Y, x.Z)
			}
		}
		sp, err := sweepPathOf(chain, g)
		if err != nil {
			return nil, src, pathErr(&chain[0].start, "%v", err)
		}
		g.fittedPath = sp
		return nil, src, nil
	}
	g.fittedPath = nil

	var steps []srf.SweepStep
	for k, p := range chain {
		if cornerAt(k) {
			d1, d2 := chain[k-1].tangentAt(true), p.tangentAt(false)
			steps = append(steps, srf.SweepStep{Corner: true, Center: p.start,
				Axis: d1.Cross(d2).WithMagnitude(1), Angle: math.Acos(math.Max(-1, math.Min(1, d1.Dot(d2))))})
		}
		steps = append(steps, p.step())
	}
	// The profile must stay clear of each turn's axis, and between two
	// mitres the sections must not cross.
	frame := srf.RigidIdentity
	ctrls := func(f func(q geom.Vector) error) error {
		for si := range sbls.Sets {
			for _, l := range sbls.Sets[si].Loops() {
				for ci := range l.Curves {
					for _, q := range l.Curves[ci].Ctrl[:l.Curves[ci].Degree+1] {
						if err := f(q); err != nil {
							return err
						}
					}
				}
			}
		}
		return nil
	}
	for k, p := range chain {
		if cornerAt(k) {
			d1, d2 := chain[k-1].tangentAt(true), p.tangentAt(false)
			frame = frame.Then(srf.SweepStep{Corner: true, Center: p.start, Axis: d1.Cross(d2).WithMagnitude(1),
				Angle: math.Acos(math.Max(-1, math.Min(1, d1.Dot(d2))))}.Motion())
		}
		if !p.arc && !p.free && (cornerAt(k) || k+1 < len(chain) && cornerAt(k+1)) {
			d := p.end.Minus(p.start)
			length := d.Magnitude()
			d = d.ScaledBy(1 / length)
			at := func(c geom.Vector, q geom.Vector, prev, next geom.Vector) float64 {
				m := prev.Plus(next).WithMagnitude(1)
				return c.Minus(q).Dot(m) / d.Dot(m)
			}
			err := ctrls(func(q geom.Vector) error {
				x := frame.Apply(q)
				ts, te := 0.0, length
				if cornerAt(k) {
					ts = at(p.start, x, chain[k-1].tangentAt(true), d)
				}
				if k+1 < len(chain) && cornerAt(k+1) {
					te = length + at(p.end, x.Plus(d.ScaledBy(length)), d, chain[k+1].tangentAt(false))
				}
				if te-ts <= sweepTol {
					m := p.start.Plus(p.end).ScaledBy(0.5)
					return pathErr(&m, "line segment %s of the path is too short for the corners at its ends: "+
						"the mitres cross on it", sk.entityName(p.h))
				}
				return nil
			})
			if err != nil {
				return nil, src, err
			}
		}
		if p.arc {
			pathRadial := p.start.Minus(p.center)
			for si := range sbls.Sets {
				for _, l := range sbls.Sets[si].Loops() {
					for ci := range l.Curves {
						for _, q := range l.Curves[ci].Ctrl[:l.Curves[ci].Degree+1] {
							r := frame.Apply(q).Minus(p.center)
							r = r.Minus(p.axis.ScaledBy(r.Dot(p.axis)))
							if r.Dot(pathRadial) <= sweepTol {
								x := p.center
								return nil, src, pathErr(&x, "the profile reaches the axis of arc %s: the bend is too tight for it",
									sk.entityName(p.h))
							}
						}
					}
				}
			}
		}
		frame = frame.Then(p.step().Motion())
	}
	return steps, src, nil
}

// pathChain reads the path entities into a chain of pieces end to end,
// starting at the end where dist is least, and requires its pieces to
// meet tangent; what names the feature in messages.
func (sk *Sketch) pathChain(edges []core.HEntity, what string, dist func(geom.Vector) float64, cubics bool) ([]pathPiece, error) {
	chain, _, err := sk.pathChainOf(edges, what, dist, cubics, false)
	return chain, err
}

// pathChainOf is pathChain, and with closed a path that closes too: a
// circle, or pieces that come back to where they began, which then
// starts at the joint where dist is least, or for a circle alone at its
// point where dist is least (roadmap 28.5). It says whether the path
// closes.
func (sk *Sketch) pathChainOf(edges []core.HEntity, what string, dist func(geom.Vector) float64, cubics, closed bool) ([]pathPiece, bool, error) {
	chain, isClosed, err := sk.pathChainAny(edges, nil, what, dist, cubics, closed, false)
	if err != nil {
		return nil, false, err
	}
	return chain, isClosed, nil
}

func (sk *Sketch) pathChainAny(edges []core.HEntity, extra []pathPiece, what string, dist func(geom.Vector) float64, cubics, closed, corners bool) ([]pathPiece, bool, error) {
	pieces := append([]pathPiece(nil), extra...)
	for _, he := range edges {
		e := sk.GetEntity(he)
		if e == nil {
			return nil, false, pathErr(nil, "path entity %s doesn't exist", sk.entityName(he))
		}
		var sbl srf.SBezierList
		GenerateEntityBeziers(e, &sbl, &Runtime{Sketch: sk})
		if e.Type == core.EntityCircle && closed {
			if len(edges) > 1 {
				return nil, false, pathErr(nil, "circle %s closes on itself: a path with a circle has nothing else", sk.entityName(he))
			}
			c := sk.MustGetEntity(e.Point[0]).PointGetNum(sk)
			n := sk.MustGetEntity(e.Normal).NormalN(sk).WithMagnitude(1)
			r := e.CircleGetRadiusNum(sk)
			if r < sweepTol {
				return nil, false, pathErr(&c, "circle %s of the path has no size", sk.entityName(he))
			}
			// Start where dist is least, as near as the circle's points
			// at a degree apart find it.
			var u geom.Vector
			best := math.Inf(1)
			ref := n.NormalDir(0).WithMagnitude(1)
			for k := 0; k < 360; k++ {
				d := ref.RotatedAroundAxis(n, float64(k)*math.Pi/180)
				if x := dist(c.Plus(d.ScaledBy(r))); x < best-sweepTol {
					best, u = x, d
				}
			}
			a := c.Plus(u.ScaledBy(r))
			return []pathPiece{{h: he, start: a, end: a, arc: true, center: c, axis: n, angle: 2 * math.Pi}}, true, nil
		}
		switch e.Type {
		case core.EntityLineSegment:
			a := sk.MustGetEntity(e.Point[0]).PointGetNum(sk)
			b := sk.MustGetEntity(e.Point[1]).PointGetNum(sk)
			if b.Minus(a).Magnitude() < sweepTol {
				return nil, false, pathErr(&a, "line segment %s of the path has no length", sk.entityName(he))
			}
			pieces = append(pieces, pathPiece{h: he, start: a, end: b, curves: sbl.Curves})
		case core.EntityArcOfCircle:
			c := sk.MustGetEntity(e.Point[0]).PointGetNum(sk)
			a := sk.MustGetEntity(e.Point[1]).PointGetNum(sk)
			b := sk.MustGetEntity(e.Point[2]).PointGetNum(sk)
			_, _, dtheta := e.ArcGetAngles(sk)
			n := sk.MustGetEntity(e.Normal).NormalN(sk).WithMagnitude(1)
			pp := pathPiece{h: he, start: a, end: b, arc: true, center: c, axis: n, angle: dtheta, curves: sbl.Curves}
			if len(pp.curves) > 0 && pp.curves[0].Start().Minus(a).Magnitude() > sweepTol {
				pp.curves = pp.reversed().curves
			}
			pieces = append(pieces, pp)
		case core.EntityCubic:
			if !cubics {
				return nil, false, pathErr(nil, "%s is not a line segment or an arc: a %s's path is made of those",
					sk.entityName(he), what)
			}
			if len(sbl.Curves) == 0 {
				return nil, false, pathErr(nil, "cubic %s of the path has no curve", sk.entityName(he))
			}
			a, b := sbl.Curves[0].Start(), sbl.Curves[len(sbl.Curves)-1].Finish()
			pieces = append(pieces, pathPiece{h: he, start: a, end: b, curves: sbl.Curves, free: true})
		default:
			if cubics {
				return nil, false, pathErr(nil, "%s is not a line segment, an arc or a cubic: a path is made of those",
					sk.entityName(he))
			}
			return nil, false, pathErr(nil, "%s is not a line segment or an arc: a path is made of those", sk.entityName(he))
		}
	}
	if len(pieces) == 0 {
		return nil, false, errors.New("there is no path")
	}

	// Each end must meet at most one other.
	near := func(p, q geom.Vector) bool { return p.Minus(q).Magnitude() < sweepTol }
	degree := func(p geom.Vector) int {
		n := 0
		for _, q := range pieces {
			if near(p, q.start) {
				n++
			}
			if near(p, q.end) {
				n++
			}
		}
		return n
	}
	var ends []geom.Vector
	for _, p := range pieces {
		for _, x := range []geom.Vector{p.start, p.end} {
			switch degree(x) {
			case 1:
				ends = append(ends, x)
			case 2:
			default:
				return nil, false, pathErr(&x, "the path branches at (%g, %g, %g)", x.X, x.Y, x.Z)
			}
		}
	}
	isClosed := len(ends) == 0
	if isClosed && !closed {
		return nil, false, pathErr(&pieces[0].start, "the path is closed: a %s needs a path with two ends", what)
	}
	if isClosed {
		// Every joint is a place to start: the one where dist is least.
		for _, p := range pieces {
			ends = append(ends, p.start)
		}
	}
	start := ends[0]
	for _, x := range ends[1:] {
		if dist(x) < dist(start)-sweepTol {
			start = x
		}
	}

	var chain []pathPiece
	used := make([]bool, len(pieces))
	at := start
	for len(chain) < len(pieces) {
		found := false
		for i, p := range pieces {
			if used[i] {
				continue
			}
			switch {
			case near(at, p.start):
			case near(at, p.end):
				p = p.reversed()
			default:
				continue
			}
			used[i], found = true, true
			chain = append(chain, p)
			at = p.end
			break
		}
		if !found {
			return nil, false, pathErr(&at, "the path is not one chain: it breaks at (%g, %g, %g)", at.X, at.Y, at.Z)
		}
	}

	for k := 1; k <= len(chain); k++ {
		if k == len(chain) && !isClosed {
			break
		}
		next := chain[k%len(chain)]
		if dot := chain[k-1].tangentAt(true).Dot(next.tangentAt(false)); dot < sweepTangentCos {
			x := next.start
			if corners && !chain[k-1].arc && !chain[k-1].free && !next.arc && !next.free {
				if dot < -1+1e-6 {
					return nil, false, pathErr(&x, "the path turns back on itself at (%g, %g, %g)", x.X, x.Y, x.Z)
				}
				// A corner between two lines: mitred (roadmap 28.4).
				continue
			}
			if corners {
				return nil, false, pathErr(&x, "the path has a corner at (%g, %g, %g) next to an arc or a curve: "+
					"a %s's corners are between line segments, and its arcs and curves meet their neighbours tangent",
					x.X, x.Y, x.Z, what)
			}
			return nil, false, pathErr(&x, "the path has a corner at (%g, %g, %g): a %s needs its pieces to meet tangent",
				x.X, x.Y, x.Z, what)
		}
	}
	return chain, isClosed, nil
}

// generateSweep copies the profile's entities to both ends of the
// path, and makes the faces of the two caps.
func (g *Group) generateSweep(sk *Sketch, entity *[]core.Entity) {
	end := srf.RigidIdentity
	if steps, _, err := g.sweepPath(sk); err == nil {
		end = srf.SweepEnd(steps)
		if g.fittedPath != nil {
			end = g.fittedPath.Rigid(g.fittedPath.End())
		}
	}
	var pt core.HEntity
	for i := 0; i < len(*entity); i++ {
		e := &(*entity)[i]
		if e.Group != g.OpA {
			continue
		}
		if e.IsPoint() {
			pt = e.H
		}
		e.CalculateNumerical(sk, false)
		he := e.H
		g.copyEntityScaled(sk, entity, sk.MustGetEntity(he), 0, RemapBottom,
			core.NoParam, core.NoParam, core.NoParam,
			core.NoParam, core.NoParam, core.NoParam, core.NoParam, core.NoParam,
			CopyAsNumeric, 1)
		n := len(*entity)
		g.copyEntityScaled(sk, entity, sk.MustGetEntity(he), 0, RemapTop,
			core.NoParam, core.NoParam, core.NoParam,
			core.NoParam, core.NoParam, core.NoParam, core.NoParam, core.NoParam,
			CopyAsNumeric, 1)
		if len(*entity) > n {
			c := &(*entity)[n]
			switch {
			case c.IsPoint():
				c.NumPoint = end.Apply(c.NumPoint)
			case c.IsNormal():
				c.NumNormal = end.Q.Times(c.NumNormal)
			}
		}
	}
	if pt.V == 0 {
		return
	}
	n := g.sourceLoopNormal(sk)
	face := core.Entity{}
	face.Type = core.EntityFaceNormalPt
	face.Group = g.H
	face.NumNormal = geom.Quaternion{X: n.X, Y: n.Y, Z: n.Z}
	face.Point[0] = g.RemapEntity(pt, RemapBottom)
	face.H = g.RemapEntity(core.NoEntity, RemapBottom)
	*entity = append(*entity, face)

	ne := end.Q.Rotate(n)
	face.NumNormal = geom.Quaternion{X: ne.X, Y: ne.Y, Z: ne.Z}
	face.Point[0] = g.RemapEntity(pt, RemapTop)
	face.H = g.RemapEntity(core.NoEntity, RemapTop)
	*entity = append(*entity, face)
}

// generateSweepShell sweeps each of the profile's loop sets along the
// path.
func (g *Group) generateSweepShell(sk *Sketch) {
	src := sk.GetGroup(g.OpA)
	if src == nil || src.PolyError.How != int(core.PolyGood) {
		g.setProblem(nil, "the profile is not closed loops")
		return
	}
	steps, _, err := g.sweepPath(sk)
	if err != nil {
		var pe *pathError
		if errors.As(err, &pe) {
			g.setProblem(pe.at, "%s", pe.msg)
		} else {
			g.setProblem(nil, "%v", err)
		}
		return
	}
	start := srf.FaceID(g.RemapEntity(core.NoEntity, RemapBottom).V)
	end := srf.FaceID(g.RemapEntity(core.NoEntity, RemapTop).V)
	if g.fittedPath != nil {
		g.generateFittedSweepShell(src, uint32(start), uint32(end))
		return
	}
	for i := range src.BezierLoops.Sets {
		g.ThisShell.MakeFromSweepOf(&src.BezierLoops.Sets[i], steps, g.Color, start, end)
	}
}

// sweepFitted reports whether the sweep's surface is fitted along the
// chain rather than exact: along a cubic, or twisted, or with its frames
// held to a direction.
func (g *Group) sweepFitted(chain []pathPiece) bool {
	if g.ValC != 0 || g.SweepUp.Magnitude() > 0 {
		return true
	}
	for _, p := range chain {
		if p.free {
			return true
		}
	}
	return false
}

// sweepPathOf is the fitted sweep's path along the chain, twisted and
// held as the group says.
func sweepPathOf(chain []pathPiece, g *Group) (*topo.SweepPath, error) {
	var curves []srf.SBezier
	for _, p := range chain {
		for _, c := range p.curves {
			if n := len(curves); n > 0 {
				// The pieces' ends meet within sweepTol; make them meet.
				prev := curves[n-1].Finish()
				c.Ctrl[0] = prev
			}
			curves = append(curves, c)
		}
	}
	t0 := chain[0].tangentAt(false)
	return topo.NewSweepPath(curves, t0.NormalDir(0), topo.PathSweepOptions{
		Twist: g.ValC * math.Pi / 180,
		Up:    g.SweepUp,
	})
}

// generateFittedSweepShell sweeps each of the profile's loop sets along
// the fitted path, the caps' identities start and end, the patches'
// from the group's own.
func (g *Group) generateFittedSweepShell(src *Group, start, end uint32) {
	next := srf.OriginFor(g.H.V, 0)
	for i := range src.BezierLoops.Sets {
		set := &src.BezierLoops.Sets[i]
		loops := [][]srf.SBezier{set.Outer.Curves}
		for _, h := range set.Holes {
			loops = append(loops, h.Curves)
		}
		sh, err := topo.MakePathSweep(g.fittedPath, loops, g.Color, start, end, next)
		if err != nil {
			g.setProblem(nil, "%v", err)
			return
		}
		next += uint32(len(sh.Surface)) - 2
		if i == 0 {
			g.ThisShell = *sh
			continue
		}
		var joined srf.SShell
		joined.MakeFromUnionOf(&g.ThisShell, sh)
		g.ThisShell = joined
	}
}

// solidEdgePieces are the path's pieces that are edges of the solid at
// group at, by name: a line or an arc where the edge is one, else a
// curve.
func (sk *Sketch) solidEdgePieces(at *Group, names []topo.Name) ([]pathPiece, error) {
	if len(names) == 0 {
		return nil, nil
	}
	var solid *Group
	if at != nil {
		solid = sk.solidAt(at.H)
	}
	if solid == nil || solid.IsForcedToMesh(sk) || len(solid.RunningShell.Surface) == 0 {
		return nil, errors.New("there is no solid whose edges the path follows")
	}
	tp, err := solid.Topology()
	if err != nil {
		return nil, fmt.Errorf("the solid's topology is broken: %v", err)
	}
	var out []pathPiece
	for _, n := range names {
		e, err := tp.Resolve(n)
		if err != nil {
			return nil, pathErr(nil, "%s: %v", sk.SolidEdgeName(n), err)
		}
		ed := &tp.Edges[e]
		a, b := tp.Vertices[ed.V[0]].P, tp.Vertices[ed.V[1]].P
		if tp.Straight(e) {
			out = append(out, pathPiece{start: a, end: b, curves: []srf.SBezier{srf.SBezierFrom2(a, b)}})
			continue
		}
		sb, ok := tp.EdgeCurve(e)
		if !ok {
			m := ed.Points[len(ed.Points)/2]
			return nil, pathErr(&m, "%s has no exact curve to sweep along", sk.SolidEdgeName(n))
		}
		if a.Equals(b) {
			m := ed.Points[len(ed.Points)/2]
			return nil, pathErr(&m, "%s closes on itself: a sweep's path has two ends", sk.SolidEdgeName(n))
		}
		if sb.Degree == 2 {
			nrm := sb.Ctrl[1].Minus(sb.Ctrl[0]).Cross(sb.Ctrl[2].Minus(sb.Ctrl[0]))
			var c geom.Vector
			var r float64
			if nrm.Magnitude() > 1e-12 && sb.IsCircle(nrm.WithMagnitude(1), &c, &r) {
				axis := a.Minus(c).Cross(sb.PointAt(0.5).Minus(c)).WithMagnitude(1)
				ra, rb := a.Minus(c), b.Minus(c)
				angle := math.Atan2(ra.Cross(rb).Dot(axis), ra.Dot(rb))
				if angle < 0 {
					angle += 2 * math.Pi
				}
				out = append(out, pathPiece{start: a, end: b, arc: true, center: c, axis: axis, angle: angle,
					curves: []srf.SBezier{sb}})
				continue
			}
		}
		out = append(out, pathPiece{start: a, end: b, curves: []srf.SBezier{sb}, free: true})
	}
	return out, nil
}
