package topo

import (
	"errors"
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// The local edit helpers change a shell in place, through its
// topology, and leave a shell whose topology validates when the edits
// together close it again: split an edge, split a face along a curve,
// take faces out, put a face in along the edges they left open, and
// give a face another surface. Each takes the topology of the shell as
// it is, and the shell's topology must be built again after it.
//
// The trim orientation they keep is SolveSpace's: a face's outer loop
// runs clockwise in its surface's parameters, holes the other way, and
// two faces run their common edge in opposite directions.

// ErrNotOnEdge is returned by SplitEdge for a point that is not inside
// the edge.
var ErrNotOnEdge = errors.New("topo: the point is not inside the edge")

// newCurveHandle is a handle no curve of the shell has.
func newCurveHandle(sh *srf.SShell) core.HSCurve {
	var hi uint32
	for i := range sh.Curve {
		hi = max(hi, sh.Curve[i].H.V)
	}
	return core.HSCurve{V: hi + 1}
}

func newSurfaceHandle(sh *srf.SShell) core.HSSurface {
	var hi uint32
	for i := range sh.Surface {
		hi = max(hi, sh.Surface[i].H.V)
	}
	return core.HSSurface{V: hi + 1}
}

// forget drops what a surface keeps about its trims, which edits make
// stale.
func forget(s *srf.SSurface) {
	s.Bsp = nil
	s.Edges = srf.SEdgeList{}
}

// pointIndex is the index of the curve point at p, or -1.
func pointIndex(c *srf.SCurve, p geom.Vector) int {
	for i := range c.Points {
		if c.Points[i].P.Equals(p) {
			return i
		}
	}
	return -1
}

// SplitEdge splits edge e of the shell at p, which must lie on it
// between its ends: the curve of each of its co-edges becomes two,
// and each trim along it two trims, meeting at a new vertex.
func SplitEdge(sh *srf.SShell, t *Topology, e int, p geom.Vector) error {
	edge := &t.Edges[e]
	if distanceToPolyline(p, edge.Points) > edgeTol(edge.Points) ||
		p.Equals(t.Vertices[edge.V[0]].P) || p.Equals(t.Vertices[edge.V[1]].P) {
		return ErrNotOnEdge
	}
	done := map[core.HSCurve]bool{}
	for _, ce := range edge.CoEdges {
		h := t.CoEdges[ce].Curve
		if done[h] {
			continue
		}
		done[h] = true
		if err := splitCurve(sh, h, p); err != nil {
			return err
		}
	}
	return nil
}

// splitCurve splits curve h at p and every trim along it that runs
// past p.
func splitCurve(sh *srf.SShell, h core.HSCurve, p geom.Vector) error {
	c := sh.FindCurve(h)
	if c == nil {
		return fmt.Errorf("topo: curve %d is not in the shell", h.V)
	}
	// p on the curve's own points, in the segment nearest it.
	at := pointIndex(c, p)
	if at < 0 {
		best, bestD := -1, math.Inf(1)
		for i := 1; i < len(c.Points); i++ {
			a, b := c.Points[i-1].P, c.Points[i].P
			if d := distanceToPolyline(p, []geom.Vector{a, b}); d < bestD {
				best, bestD = i, d
			}
		}
		if best < 0 {
			return ErrNotOnEdge
		}
		c.Points = append(c.Points[:best], append([]srf.SCurvePt{{Vertex: true, P: p}}, c.Points[best:]...)...)
		at = best
	} else {
		p = c.Points[at].P
	}
	if at == 0 || at == len(c.Points)-1 {
		return ErrNotOnEdge
	}
	first := *c
	second := *c
	first.Points = append([]srf.SCurvePt(nil), c.Points[:at+1]...)
	second.Points = append([]srf.SCurvePt(nil), c.Points[at:]...)
	if c.IsExact {
		tt, _ := c.Exact.ClosestPointTo(p, false)
		c.Exact.SplitAt(tt, &first.Exact, &second.Exact)
	}
	first.H = newCurveHandle(sh)
	second.H = core.HSCurve{V: first.H.V + 1}
	// Which points of the old curve each trim covers, and so which of
	// the new curves.
	for si := range sh.Surface {
		s := &sh.Surface[si]
		var trims []srf.STrimBy
		changed := false
		for _, tb := range s.Trim {
			if tb.Curve != h {
				trims = append(trims, tb)
				continue
			}
			changed = true
			// The trim's range in the curve's point order.
			lo, hi := pointIndex(c, tb.StartP), pointIndex(c, tb.FinishP)
			if tb.Backwards {
				lo, hi = hi, lo
			}
			switch {
			case hi <= at:
				tb.Curve = first.H
				trims = append(trims, tb)
			case lo >= at:
				tb.Curve = second.H
				trims = append(trims, tb)
			case !tb.Backwards:
				a, b := tb, tb
				a.Curve, a.FinishP = first.H, p
				b.Curve, b.StartP = second.H, p
				trims = append(trims, a, b)
			default:
				a, b := tb, tb
				a.Curve, a.FinishP = second.H, p
				b.Curve, b.StartP = first.H, p
				trims = append(trims, a, b)
			}
		}
		if changed {
			s.Trim = trims
			forget(s)
		}
	}
	// Replace the curve by the two.
	for i := range sh.Curve {
		if sh.Curve[i].H == h {
			sh.Curve = append(sh.Curve[:i], sh.Curve[i+1:]...)
			break
		}
	}
	sh.Curve = append(sh.Curve, first, second)
	return nil
}

// SplitFace splits face f along cut, a curve across it from one vertex
// of its outer loop to another, into two faces on two copies of its
// surface: the face keeps the part on the right of cut, the new one,
// whose surface's handle comes back, the part on its left. The face's
// holes go with the part they lie in.
func SplitFace(sh *srf.SShell, t *Topology, f int, cut srf.SBezier) (core.HSSurface, error) {
	face := &t.Faces[f]
	var outer *Loop
	for _, l := range face.Loops {
		if t.Loops[l].Outer {
			outer = &t.Loops[l]
		}
	}
	if outer == nil {
		return core.HSSurface{}, errors.New("topo: the face has no outer loop")
	}
	a, b := cut.Start(), cut.Finish()
	ia, ib := -1, -1
	for k, ce := range outer.CoEdges {
		if t.Vertices[t.Start(ce)].P.Equals(a) {
			ia = k
		}
		if t.Vertices[t.Start(ce)].P.Equals(b) {
			ib = k
		}
	}
	if ia < 0 || ib < 0 || ia == ib {
		return core.HSSurface{}, errors.New("topo: the cut does not run between two vertices of the face's outer loop")
	}
	n := len(outer.CoEdges)
	// The loop from a to b keeps the face; from b to a goes to the new
	// one.
	var move []int
	for k := ib; k != ia; k = (k + 1) % n {
		move = append(move, outer.CoEdges[k])
	}
	s := sh.FindSurface(face.Surface)
	if s == nil {
		return core.HSSurface{}, errors.New("topo: the face's surface is not in the shell")
	}
	ns := *s
	ns.H = newSurfaceHandle(sh)
	ns.Trim = nil
	forget(&ns)

	// The cut, as a curve between the two surfaces. The kept part runs
	// it from b to a, so it is the surface on the curve's left; the new
	// part runs it from a to b.
	var pts []geom.Vector
	cut.MakePwlIntoPoints(&pts, sh.Tol, 0)
	pts[0], pts[len(pts)-1] = a, b
	nc := srf.SCurve{H: newCurveHandle(sh), Exact: cut, IsExact: true, Deg: cut.Degree,
		SurfA: ns.H, SurfB: s.H}
	for _, p := range pts {
		nc.Points = append(nc.Points, srf.SCurvePt{Vertex: true, P: p})
	}

	// Holes go with the part that holds them.
	moveTrim := map[int]bool{}
	for _, ce := range move {
		moveTrim[t.CoEdges[ce].Trim] = true
	}
	newOuterUV := t.chainUV(s, move, pts, true)
	for _, l := range face.Loops {
		if t.Loops[l].Outer || len(t.Loops[l].CoEdges) == 0 {
			continue
		}
		uv := t.loopUV(s, t.Loops[l].CoEdges)
		if len(uv) > 0 && pointInPolygon(uv[0], newOuterUV) {
			for _, ce := range t.Loops[l].CoEdges {
				moveTrim[t.CoEdges[ce].Trim] = true
			}
		}
	}
	var kept []srf.STrimBy
	for ti, tb := range s.Trim {
		if moveTrim[ti] && t.trimOfFace(face.Surface, ti, f) {
			ns.Trim = append(ns.Trim, tb)
			// The curve now bounds the new surface.
			if c := sh.FindCurve(tb.Curve); c != nil {
				if c.SurfA == s.H {
					c.SurfA = ns.H
				} else if c.SurfB == s.H {
					c.SurfB = ns.H
				}
			}
			continue
		}
		kept = append(kept, tb)
	}
	kept = append(kept, srf.STrimBy{Curve: nc.H, Backwards: true, StartP: b, FinishP: a, Start: 0, Finish: 1})
	ns.Trim = append(ns.Trim, srf.STrimBy{Curve: nc.H, StartP: a, FinishP: b, Start: 0, Finish: 1})
	s.Trim = kept
	forget(s)
	ns.TrimBuilt = true
	sh.Curve = append(sh.Curve, nc)
	ns.Tol = sh.Tol
	sh.Surface = append(sh.Surface, ns)
	return ns.H, nil
}

// trimOfFace tells whether trim ti of surface h belongs to face f.
func (t *Topology) trimOfFace(h core.HSSurface, ti, f int) bool {
	for i := range t.CoEdges {
		c := &t.CoEdges[i]
		if c.Surface == h && c.Trim == ti && c.Loop >= 0 {
			return t.Loops[c.Loop].Face == f
		}
	}
	return false
}

// chainUV is the uv polygon of a chain of co-edges closed by pts, run
// forwards (or backwards).
func (t *Topology) chainUV(s *srf.SSurface, chain []int, pts []geom.Vector, forward bool) []geom.Vector {
	uv := t.loopUV(s, chain)
	if !forward {
		pts = reversed(pts)
	}
	for _, p := range pts[:len(pts)-1] {
		var u, v float64
		s.ClosestPointToUV(p, &u, &v, true)
		uv = append(uv, geom.Vector{X: u, Y: v})
	}
	return uv
}

// RemoveFaces takes faces out of the shell. Edges between two of them
// go; edges between one of them and a face that stays are left open,
// for AddFace to close.
func RemoveFaces(sh *srf.SShell, t *Topology, faces []int) {
	gone := map[int]bool{}
	for _, f := range faces {
		gone[f] = true
	}
	// The trims to drop, by surface.
	drop := map[core.HSSurface]map[int]bool{}
	for i := range t.CoEdges {
		c := &t.CoEdges[i]
		if c.Loop >= 0 && gone[t.Loops[c.Loop].Face] {
			if drop[c.Surface] == nil {
				drop[c.Surface] = map[int]bool{}
			}
			drop[c.Surface][c.Trim] = true
		}
	}
	var surfaces []srf.SSurface
	for _, s := range sh.Surface {
		if d := drop[s.H]; d != nil {
			var kept []srf.STrimBy
			for ti, tb := range s.Trim {
				if !d[ti] {
					kept = append(kept, tb)
				}
			}
			if len(kept) == 0 {
				continue
			}
			s.Trim = kept
			forget(&s)
		}
		surfaces = append(surfaces, s)
	}
	sh.Surface = surfaces
	dropUnusedCurves(sh)
}

// dropUnusedCurves takes out the curves no trim runs along, and clears
// a curve's side whose surface is gone.
func dropUnusedCurves(sh *srf.SShell) {
	used := map[core.HSCurve]bool{}
	alive := map[core.HSSurface]bool{}
	for i := range sh.Surface {
		alive[sh.Surface[i].H] = true
		for _, tb := range sh.Surface[i].Trim {
			used[tb.Curve] = true
		}
	}
	var curves []srf.SCurve
	for _, c := range sh.Curve {
		if !used[c.H] {
			continue
		}
		if !alive[c.SurfA] {
			c.SurfA = core.HSSurface{}
		}
		if !alive[c.SurfB] {
			c.SurfB = core.HSSurface{}
		}
		curves = append(curves, c)
	}
	sh.Curve = curves
}

// AddFace puts a face into the shell on surface s, bounded by loops of
// curves joined end to end, the first loop its outer boundary. A curve
// that runs along an open edge of the shell takes that edge, and the
// loop runs it the other way from the face that has it; the others
// become new edges, open until another face takes them. The surface is
// turned over, if need be, so that the loop runs clockwise in its
// parameters, as SolveSpace's trims do. The new surface's handle comes
// back.
func AddFace(sh *srf.SShell, s srf.SSurface, loops [][]srf.SBezier) (core.HSSurface, error) {
	return NewFaceAdder(sh).Add(s, loops)
}

// FaceAdder adds faces to a shell one after another, as AddFace does,
// keeping the shell's open edges as it goes rather than finding them
// again for each face: a surface of hundreds of patches goes in in time
// that grows with its patches, not their square. Nothing else may change
// the shell while it is used.
type FaceAdder struct {
	sh   *srf.SShell
	open []openEdge
	// byStart files the open edges by the cell their first point lies
	// in (openKey), for along: a fillet's thousands of patches each
	// looked through all of them.
	byStart map[vertexKey][]int
	// curveAt is where each curve lies in the shell, filed on first use
	// and kept as faces go in (findCurve): an edited shell's curves are
	// out of their handles' order, where FindCurve looks through them all.
	curveAt map[core.HSCurve]int
}

// findCurve is the shell's curve of handle h, as srf.SShell.FindCurve
// finds it; handles are unique, so the filed place is the one.
func (fa *FaceAdder) findCurve(h core.HSCurve) *srf.SCurve {
	sh := fa.sh
	if i := int(h.V) - 1; i >= 0 && i < len(sh.Curve) && sh.Curve[i].H == h {
		return &sh.Curve[i]
	}
	if fa.curveAt == nil {
		fa.curveAt = make(map[core.HSCurve]int, len(sh.Curve))
		for i := range sh.Curve {
			if _, ok := fa.curveAt[sh.Curve[i].H]; !ok {
				fa.curveAt[sh.Curve[i].H] = i
			}
		}
	}
	if i, ok := fa.curveAt[h]; ok && i < len(sh.Curve) && sh.Curve[i].H == h {
		return &sh.Curve[i]
	}
	return sh.FindCurve(h)
}

// openEdge is an edge only one face has: its curve, and its points from
// the end the face runs it from to the other.
type openEdge struct {
	curve  core.HSCurve
	points []geom.Vector
	gone   bool
}

// NewFaceAdder starts adding faces to sh, finding its open edges once.
func NewFaceAdder(sh *srf.SShell) *FaceAdder {
	fa := &FaceAdder{sh: sh, byStart: map[vertexKey][]int{}}
	t, _ := Build(sh)
	if t == nil {
		return fa
	}
	for e := range t.Edges {
		ed := &t.Edges[e]
		if len(ed.CoEdges) != 1 {
			continue
		}
		ce := ed.CoEdges[0]
		pts := ed.Points
		if !pts[0].Equals(t.Vertices[t.Start(ce)].P) {
			pts = reversed(pts)
		}
		fa.addOpen(openEdge{curve: t.CoEdges[ce].Curve, points: pts})
	}
	return fa
}

// openCell is the size of the cells FaceAdder files the open edges'
// first points in: more than the tolerance points are one within
// (geom.Vector.Equals), so a point's equals lie in its cell or the ones
// next to it.
const openCell = 1e-3

// openKey is the cell p lies in.
func openKey(p geom.Vector) vertexKey {
	q := func(x float64) int64 { return int64(math.Floor(x / openCell)) }
	return vertexKey{q(p.X), q(p.Y), q(p.Z)}
}

// addOpen adds oe to the open edges.
func (fa *FaceAdder) addOpen(oe openEdge) {
	k := openKey(oe.points[0])
	fa.byStart[k] = append(fa.byStart[k], len(fa.open))
	fa.open = append(fa.open, oe)
}

// along is the open edge that runs between the ends of sb and along it,
// or -1: the first such in the open edges' order, found among those
// starting at either end.
func (fa *FaceAdder) along(sb srf.SBezier) int {
	a, b := sb.Start(), sb.Finish()
	mid := sb.PointAt(0.5)
	best := -1
	for _, end := range [2]geom.Vector{a, b} {
		k := openKey(end)
		for dx := int64(-1); dx <= 1; dx++ {
			for dy := int64(-1); dy <= 1; dy++ {
				for dz := int64(-1); dz <= 1; dz++ {
					for _, i := range fa.byStart[vertexKey{k[0] + dx, k[1] + dy, k[2] + dz}] {
						if (best < 0 || i < best) && fa.runsAlong(i, a, b, mid) {
							best = i
						}
					}
				}
			}
		}
	}
	return best
}

// runsAlong reports whether open edge i runs between a and b, either
// way, through mid.
func (fa *FaceAdder) runsAlong(i int, a, b, mid geom.Vector) bool {
	oe := &fa.open[i]
	if oe.gone {
		return false
	}
	pa, pb := oe.points[0], oe.points[len(oe.points)-1]
	if (!pa.Equals(a) || !pb.Equals(b)) && (!pa.Equals(b) || !pb.Equals(a)) {
		return false
	}
	return distanceToPolyline(mid, oe.points) <= edgeTol(oe.points)
}

// Add puts a face into the shell as AddFace does.
func (fa *FaceAdder) Add(s srf.SSurface, loops [][]srf.SBezier) (core.HSSurface, error) {
	if len(loops) == 0 || len(loops[0]) == 0 {
		return core.HSSurface{}, errNoLoop
	}
	pf := fa.prepare(s, loops, newSurfaceHandle(fa.sh), newCurveHandle(fa.sh))
	pf.orient()
	fa.commit(pf)
	return pf.s.H, nil
}

// addPiece is a side of a face being put in: the curve it runs along, an
// open edge's or a fresh one, between a and b through points.
type addPiece struct {
	curve core.HSCurve
	// fresh is a new curve, to be added.
	fresh  *srf.SCurve
	a, b   geom.Vector
	points []geom.Vector
	// taken is the open edge the piece runs along, or -1.
	taken int
}

// addFace is a face prepared to go in (FaceAdder.prepare): its surface
// with its handle, its loops' pieces, and the handle after its fresh
// curves'.
type addFace struct {
	s     srf.SSurface
	all   [][]addPiece
	nextH core.HSCurve
}

// prepare is the first part of Add: face s with handle h, its loops run
// the way the open edges they take say, each side an open edge's or a
// fresh curve from nextH on. The open edges are kept as if the face were
// in: the fresh ones its, those it takes gone. Nothing of the shell is
// changed, so the next face may be prepared before this one goes in.
func (fa *FaceAdder) prepare(s srf.SSurface, loops [][]srf.SBezier, h core.HSSurface, nextH core.HSCurve) *addFace {
	sh := fa.sh
	s.H = h
	s.Trim = nil
	s.TrimBuilt = true
	forget(&s)

	var all [][]addPiece
	for _, loop := range loops {
		var ps []addPiece
		// Run the loop the way an open edge it takes says.
		flip := false
		for _, sb := range loop {
			if e := fa.along(sb); e >= 0 {
				// The existing face runs the edge from its start to its
				// end; this loop must run it the other way.
				flip = fa.open[e].points[0].Equals(sb.Start())
				break
			}
		}
		seq := loop
		if flip {
			seq = make([]srf.SBezier, len(loop))
			for i := range loop {
				sb := loop[len(loop)-1-i]
				sb.Reverse()
				seq[i] = sb
			}
		}
		for _, sb := range seq {
			pc := addPiece{a: sb.Start(), b: sb.Finish(), taken: -1}
			if e := fa.along(sb); e >= 0 {
				pc.curve = fa.open[e].curve
				pc.points = fa.open[e].points
				pc.taken = e
				if !pc.points[0].Equals(pc.a) {
					// The edge's points run the other way from the loop.
					pc.points = reversed(pc.points)
				}
			} else {
				var pts []geom.Vector
				sb.MakePwlIntoPoints(&pts, sh.Tol, 0)
				pts[0], pts[len(pts)-1] = pc.a, pc.b
				nc := &srf.SCurve{H: nextH, Exact: sb, IsExact: true, Deg: sb.Degree, SurfA: s.H}
				nextH.V++
				for _, p := range pts {
					nc.Points = append(nc.Points, srf.SCurvePt{Vertex: true, P: p})
				}
				pc.curve, pc.fresh, pc.points = nc.H, nc, pts
			}
			ps = append(ps, pc)
		}
		all = append(all, ps)
	}
	for _, ps := range all {
		for _, pc := range ps {
			if pc.fresh != nil {
				fa.addOpen(openEdge{curve: pc.curve, points: pc.points})
			} else if pc.taken >= 0 {
				fa.open[pc.taken].gone = true
			}
		}
	}
	return &addFace{s: s, all: all, nextH: nextH}
}

// orient turns the face's surface over unless its outer loop runs
// clockwise in its parameters. It projects on the face's own copy of
// its surface alone, so that faces may be oriented at once.
func (pf *addFace) orient() {
	// A point at a pole, a side of the patch folded to one point, has
	// any parameter along that side, which can turn the outline over (a
	// wedge of a sphere from a circle of latitude up to its pole,
	// Schenkel): it is opened along the side (openPoleUV).
	area := 0.0
	var pts []geom.Vector
	for _, pc := range pf.all[0] {
		pts = append(pts, pc.points[:len(pc.points)-1]...)
	}
	uv := surfaceLoopUV(&pf.s, pts)
	for k := range uv {
		p, q := uv[k], uv[(k+1)%len(uv)]
		area += (p.X*q.Y - q.X*p.Y) / 2
	}
	if area > 0 {
		pf.s.Reverse()
	}
}

// commit puts the prepared face in: its trims, the curves it shares told
// of it, its fresh curves and the surface added to the shell.
func (fa *FaceAdder) commit(pf *addFace) {
	sh := fa.sh
	s := pf.s
	for _, ps := range pf.all {
		for _, pc := range ps {
			c := pc.fresh
			if c == nil {
				c = fa.findCurve(pc.curve)
			}
			// Along the curve's own points or against them: the curve may
			// run past the edge's ends, as a Boolean's do.
			back := !c.Points[0].P.Equals(pc.a)
			if ia, ib := pointIndex(c, pc.a), pointIndex(c, pc.b); ia >= 0 && ib >= 0 {
				back = ib < ia
			}
			if c.SurfA.V == 0 || c.SurfA == s.H {
				c.SurfA = s.H
			} else {
				c.SurfB = s.H
			}
			s.Trim = append(s.Trim, srf.STrimBy{Curve: pc.curve, Backwards: back, StartP: pc.a, FinishP: pc.b,
				Start: 0, Finish: 1})
			if pc.fresh != nil {
				if fa.curveAt != nil {
					if _, ok := fa.curveAt[pc.fresh.H]; !ok {
						fa.curveAt[pc.fresh.H] = len(sh.Curve)
					}
				}
				sh.Curve = append(sh.Curve, *pc.fresh)
			}
		}
	}
	s.Tol = sh.Tol
	sh.Surface = append(sh.Surface, s)
}

// ReplaceSurface gives face f's surface new geometry, keeping its
// trims: s must pass through the face's edges. It is turned over if it
// faces the other way from the old one.
func ReplaceSurface(sh *srf.SShell, t *Topology, f int, s srf.SSurface) error {
	old := sh.FindSurface(t.Faces[f].Surface)
	if old == nil {
		return errors.New("topo: the face's surface is not in the shell")
	}
	// Compare the normals at a point of the face.
	var p geom.Vector
	for _, l := range t.Faces[f].Loops {
		if len(t.Loops[l].CoEdges) > 0 {
			p = t.Vertices[t.Start(t.Loops[l].CoEdges[0])].P
			break
		}
	}
	var u0, v0, u1, v1 float64
	old.ClosestPointToUV(p, &u0, &v0, true)
	s.ClosestPointToUV(p, &u1, &v1, true)
	if old.NormalAt(u0, v0).Dot(s.NormalAt(u1, v1)) < 0 {
		s.Reverse()
	}
	old.DegM, old.DegN = s.DegM, s.DegN
	old.Ctrl, old.Weight = s.Ctrl, s.Weight
	forget(old)
	return nil
}

// poleSide is the side of the patch (0 to 3: u = 0, u = 1, v = 0,
// v = 1) that (u, v) lies on and that the surface folds to one point,
// as a sphere's pole or a cone's apex, or -1.
func poleSide(s *srf.SSurface, u, v float64) int {
	const eps = 1e-6
	folds := func(at func(t float64) geom.Vector) bool {
		a := at(0)
		return a.Equals(at(0.5)) && a.Equals(at(1))
	}
	switch {
	case u < eps && folds(func(t float64) geom.Vector { return s.PointAt(0, t) }):
		return 0
	case u > 1-eps && folds(func(t float64) geom.Vector { return s.PointAt(1, t) }):
		return 1
	case v < eps && folds(func(t float64) geom.Vector { return s.PointAt(t, 0) }):
		return 2
	case v > 1-eps && folds(func(t float64) geom.Vector { return s.PointAt(t, 1) }):
		return 3
	}
	return -1
}

// surfaceLoopUV is a closed loop of points of s in its parameters, a
// run of points at a pole opened along the folded side: it becomes two
// points there, at the parameters along the side of the points before
// and after it, as the curves reach the pole, so that the outline goes
// along the side between them.
func surfaceLoopUV(s *srf.SSurface, pts []geom.Vector) []geom.Vector {
	n := len(pts)
	uv := make([]geom.Vector, n)
	side := make([]int, n)
	for k, p := range pts {
		var u, v float64
		s.ClosestPointToUV(p, &u, &v, true)
		uv[k] = geom.Vector{X: u, Y: v}
		side[k] = poleSide(s, u, v)
	}
	onSide := func(q geom.Vector, k int) geom.Vector {
		switch k {
		case 0:
			q.X = 0
		case 1:
			q.X = 1
		case 2:
			q.Y = 0
		case 3:
			q.Y = 1
		}
		return q
	}
	var out []geom.Vector
	for k := 0; k < n; k++ {
		if side[k] < 0 {
			out = append(out, uv[k])
			continue
		}
		// The points before and after the run at the pole.
		prev, next := -1, -1
		for d := 1; d < n; d++ {
			if j := (k - d + n) % n; side[j] < 0 {
				prev = j
				break
			}
		}
		end := k
		for end+1 < n && side[end+1] == side[k] {
			end++
		}
		for d := 1; d < n; d++ {
			if j := (end + d) % n; side[j] < 0 {
				next = j
				break
			}
		}
		if prev < 0 || next < 0 {
			out = append(out, uv[k])
			continue
		}
		out = append(out, onSide(uv[prev], side[k]), onSide(uv[next], side[k]))
		k = end
	}
	return out
}
