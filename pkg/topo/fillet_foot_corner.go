package topo

import (
	"maps"
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// Feet filled before. A rib whose foot was filled in an edit of its own
// has its uprights and its slope's edges end on that fillet: an upright
// runs down to where the foot's two fillets meet in a mitre below it, a
// slope's edge to where the side's fillet runs on past the slope. Rounded
// now, the edge's fillet would run on into the crease, and a ball rolled
// down it leaves the two fillets for the plate just above it: a blend of
// three surfaces, not a fillet of the crease.
//
// The foot's fillets along straight edges between flat faces are
// cylinders, the same ones the foot's edges would have had rounded with
// the edge in one edit. So the corner is filled as that edit's setback
// corner would fill it (setbackHole): at the point where the edge and the
// foot's two edges met before, the rounded edge and the two foot edges,
// rebuilt from their faces and fillets, are set back past where their
// tangent lines cross and the hole between them filled by patches. Every
// face round the hole has the stretch of its loop nearest that point
// replaced by the hole's side (faceCut): the faces either side of the
// edge by their curves, the foot's fillets by their sections, which takes
// the crease away, and the plate by its curve.

// faceCut replaces the stretch of a face's loop from a to b that comes
// nearer to near, where the faces met before, by curve.
type faceCut struct {
	a, b  geom.Vector
	near  geom.Vector
	curve srf.SBezier
}

// footSide is one side of a foot corner: the flat face along the rounded
// edge, the foot's fillet below it, the seam between them, and that
// fillet's seam on the plate.
type footSide struct {
	face, fill, seam, toe int
}

// footCornerOf reports whether vertex v is the end of a rounded edge on
// a foot filled before: the one rounded edge there, between two flat
// faces, each of which meets a curved face, the foot's fillet, along a
// straight seam that runs square to the edge's line, the fillet running
// along to a seam on another flat face, the plate, the same for both;
// and every face at v is one of those four. It returns the rounded edge,
// the two sides in the order of its faces, and the plate.
func (t *Topology) footCornerOf(v int, edges []filletEdge, byEdge map[int]int) (*filletEdge, [2]footSide, int, bool) {
	var sides [2]footSide
	var fe *filletEdge
	for _, x := range t.Vertices[v].Edges {
		if i, ok := byEdge[x]; ok {
			if fe != nil {
				return nil, sides, -1, false
			}
			fe = &edges[i]
		}
	}
	if fe == nil || fe.circ || fe.concave || fe.varies() || len(t.Vertices[v].Edges) < 3 {
		return nil, sides, -1, false
	}
	curved := func(f int) bool {
		_, ok := t.planarNormal(f)
		return f >= 0 && !ok
	}
	other := func(x, f int) int {
		fs := t.EdgeFaces(x)
		if fs[0] == f {
			return fs[1]
		}
		return fs[0]
	}
	along := func(x int) geom.Vector {
		pts := t.Edges[x].Points
		return pts[len(pts)-1].Minus(pts[0])
	}
	parallel := func(a, b geom.Vector) bool {
		return a.Cross(b).Magnitude() <= 1e-9*a.Magnitude()*b.Magnitude()
	}
	// The fillet's seam on the plate: a straight edge of fill along d,
	// to a flat face other than face.
	toeOf := func(fill, face, seam int, d geom.Vector) (int, int) {
		toe, plate := -1, -1
		for _, l := range t.Faces[fill].Loops {
			for _, ce := range t.Loops[l].CoEdges {
				x := t.CoEdges[ce].Edge
				if x == seam || !t.Straight(x) || !parallel(along(x), d) {
					continue
				}
				if g := other(x, fill); g != face && !curved(g) {
					if toe >= 0 {
						return -1, -1
					}
					toe, plate = x, g
				}
			}
		}
		return toe, plate
	}
	// The plate, from a straight seam at v onto a curved face.
	plate := -1
	for _, x := range t.Vertices[v].Edges {
		if _, ok := byEdge[x]; ok || !t.Straight(x) {
			continue
		}
		for k := range 2 {
			if f := fe.f[k]; t.EdgeFaces(x)[0] == f || t.EdgeFaces(x)[1] == f {
				if g := other(x, f); curved(g) {
					if _, p := toeOf(g, f, x, along(x)); p >= 0 {
						plate = p
					}
				}
			}
		}
	}
	if plate < 0 {
		return nil, sides, -1, false
	}
	nP, _ := t.planarNormal(plate)
	at := t.Vertices[v].P
	for k := range 2 {
		f := fe.f[k]
		n, _ := t.planarNormal(f)
		d := n.Cross(nP)
		if d.Magnitude() < 1e-9 {
			return nil, sides, -1, false
		}
		// The seam on f nearest v that runs along f's foot.
		best := math.Inf(1)
		sides[k] = footSide{face: f, fill: -1, seam: -1, toe: -1}
		for _, l := range t.Faces[f].Loops {
			for _, ce := range t.Loops[l].CoEdges {
				x := t.CoEdges[ce].Edge
				g := other(x, f)
				if !t.Straight(x) || !curved(g) || !parallel(along(x), d) {
					continue
				}
				pts := t.Edges[x].Points
				dist := at.DistanceToLine(pts[0], along(x))
				if dist > best {
					continue
				}
				toe, p := toeOf(g, f, x, d)
				if p != plate {
					continue
				}
				best = dist
				sides[k] = footSide{face: f, fill: g, seam: x, toe: toe}
			}
		}
		if sides[k].seam < 0 {
			return nil, sides, -1, false
		}
	}
	if sides[0].fill == sides[1].fill {
		return nil, sides, -1, false
	}
	for _, f := range t.facesAt(v) {
		if f != sides[0].face && f != sides[1].face && f != sides[0].fill && f != sides[1].fill {
			return nil, sides, -1, false
		}
	}
	return fe, sides, plate, true
}

// footCornerAt is the corner at vertex v, the end of a rounded edge on a
// foot filled before (footCornerOf), set back as the one edit would have
// set it back (setbackHole). ok is false where v is no such corner.
func (t *Topology) footCornerAt(v int, edges []filletEdge, byEdge map[int]int) (*filletCorner, bool, error) {
	fe, sides, plate, ok := t.footCornerOf(v, edges, byEdge)
	if !ok {
		return nil, false, nil
	}
	at := t.Vertices[v].P
	nP, _ := t.planarNormal(plate)
	normal := map[int]geom.Vector{plate: nP}
	for _, s := range sides {
		n, _ := t.planarNormal(s.face)
		normal[s.face] = n
	}
	// Where the three faces met before the foot was filled.
	level := func(f int) float64 {
		return normal[f].Dot(t.Vertices[t.Edges[t.faceEdge(f)].V[0]].P)
	}
	n0, n1 := normal[sides[0].face], normal[sides[1].face]
	o, parallel := geom.AtIntersectionOfThreePlanes(n0, level(sides[0].face), n1, level(sides[1].face), nP, level(plate))
	if parallel {
		return nil, true, filletErr(at, "the faces at the foot's corner don't meet at a point")
	}
	if o.DistanceToLine(at, fe.dir) > 1e-6 {
		return nil, true, filletErr(at, "the edge doesn't run down to where its foot's faces meet")
	}
	var foot [2]filletEdge
	for k, s := range sides {
		fk, err := t.footEdge(v, k, o, s, plate, normal)
		if err != nil {
			return nil, true, err
		}
		foot[k] = fk
	}
	round := []*filletEdge{fe, &foot[0], &foot[1]}
	c, err := setbackHole(o, v, round, []int{sides[0].face, sides[1].face, plate}, normal)
	if err != nil {
		return nil, true, err
	}
	c.cuts = map[int][]faceCut{}
	for k, s := range sides {
		x1 := c.at[[2]int{foot[k].e, s.face}]
		x2 := c.at[[2]int{foot[k].e, plate}]
		for _, on := range []struct {
			x int
			p geom.Vector
		}{{s.seam, x1}, {s.toe, x2}} {
			pts := t.Edges[on.x].Points
			if !on.p.OnLineSegment(pts[0], pts[len(pts)-1], geom.LengthEps) {
				return nil, true, filletErr(at, "the foot's fillet is too short for the corner the edge's fillet makes on it")
			}
		}
		c.cuts[s.face] = append(c.cuts[s.face], faceCut{a: c.at[[2]int{fe.e, s.face}], b: x1, near: o, curve: c.faceCurves[s.face]})
		c.cuts[s.fill] = append(c.cuts[s.fill], faceCut{a: x1, b: x2, near: o, curve: c.arcs[foot[k].e]})
	}
	c.cuts[plate] = append(c.cuts[plate], faceCut{
		a: c.at[[2]int{foot[0].e, plate}], b: c.at[[2]int{foot[1].e, plate}], near: o, curve: c.faceCurves[plate],
	})
	return c, true, nil
}

// faceEdge is an edge of face f's first loop.
func (t *Topology) faceEdge(f int) int {
	return t.CoEdges[t.Loops[t.Faces[f].Loops[0]].CoEdges[0]].Edge
}

// footEdge is the k-th foot edge at v as it was before it was filled,
// between side s's face and the plate: through o along both faces, its
// fillet the cylinder of s's fillet face, from the seam on the face to
// the toe on the plate. Its number is negative, one of v's own.
func (t *Topology) footEdge(v, k int, o geom.Vector, s footSide, plate int, normal map[int]geom.Vector) (filletEdge, error) {
	at := t.Vertices[v].P
	nS, nP := normal[s.face], normal[plate]
	dir := nS.Cross(nP).WithMagnitude(1)
	mid := func(x int) geom.Vector { return midpoint(t.Edges[x].Points) }
	if dir.Dot(mid(s.seam).Minus(o)) < 0 {
		dir = dir.ScaledBy(-1)
	}
	square := func(x int) (geom.Vector, float64) {
		w := mid(x).Minus(o)
		w = w.Minus(dir.ScaledBy(w.Dot(dir)))
		return w.WithMagnitude(1), w.Magnitude()
	}
	uS, ssS := square(s.seam)
	uP, ssP := square(s.toe)
	if math.Abs(ssS-ssP) > 1e-6*math.Max(1, ssS) {
		return filletEdge{}, filletErr(at, "the foot's fillet is not of one radius between its faces")
	}
	if !(uS.Dot(nP) > 1e-9 && uP.Dot(nS) > 1e-9) {
		return filletEdge{}, filletErr(at, "the foot's edge below the fillet was not concave")
	}
	beta := math.Acos(math.Max(-1, math.Min(1, uS.Dot(uP))))
	r := ssS * math.Tan(beta/2)
	fe := filletEdge{
		e: -2 - 2*v - k, v: [2]int{v, -1}, f: [2]int{s.face, plate}, u: [2]geom.Vector{uS, uP},
		r: r, concave: true, dir: dir, w: math.Sin(beta / 2), ss: [2]float64{ssS, ssS},
	}
	// The fillet's face is that cylinder: its axis where the arc's
	// centre lies, r from it.
	axis := o.Plus(uS.Plus(uP).WithMagnitude(r / math.Sin(beta/2)))
	sf := t.Surface(s.fill)
	for _, uv := range [][2]float64{{0.5, 0.5}, {0.1, 0.9}, {0.9, 0.1}} {
		p := sf.PointAt(uv[0], uv[1])
		if d := p.DistanceToLine(axis, dir); math.Abs(d-r) > 1e-6*math.Max(1, r) {
			return filletEdge{}, filletErr(at, "the foot's fillet is not a cylinder along its edge")
		}
	}
	return fe, nil
}

// applyCut is loops with the cut made in the loop that holds both its
// ends.
func applyCut(loops [][]srf.SBezier, c faceCut) ([][]srf.SBezier, error) {
	for li, loop := range loops {
		loop, _, ok := splitLoopAt(loop, c.a)
		if !ok {
			continue
		}
		loop, ib, ok := splitLoopAt(loop, c.b)
		if !ok {
			continue
		}
		// Splitting at b may have moved a.
		ia := slices.IndexFunc(loop, func(sb srf.SBezier) bool { return sb.Start().Equals(c.a) })
		n := len(loop)
		// How near the stretch from curve i to curve j, not taking in j,
		// comes to c.near.
		nearest := func(i, j int) float64 {
			best := math.Inf(1)
			for ; i != j; i = (i + 1) % n {
				for k := range 5 {
					best = math.Min(best, loop[i].PointAt(float64(k)/4).Minus(c.near).Magnitude())
				}
			}
			return best
		}
		curve := c.curve
		if !curve.Start().Equals(c.a) {
			curve.Reverse()
		}
		var out []srf.SBezier
		if nearest(ia, ib) < nearest(ib, ia) {
			out = append(out, curve)
			for i := ib; i != ia; i = (i + 1) % n {
				out = append(out, loop[i])
			}
		} else {
			for i := ia; i != ib; i = (i + 1) % n {
				out = append(out, loop[i])
			}
			curve.Reverse()
			out = append(out, curve)
		}
		loops[li] = out
		return loops, nil
	}
	return nil, filletErr(c.a, "the corner's cut doesn't lie on a face's loop")
}

// splitLoopAt is loop with p a point where one of its curves starts,
// the line p lies inside split there, and the index of that curve.
func splitLoopAt(loop []srf.SBezier, p geom.Vector) ([]srf.SBezier, int, bool) {
	for i, sb := range loop {
		if sb.Start().Equals(p) {
			return loop, i, true
		}
	}
	for i, sb := range loop {
		if sb.Degree != 1 || !p.OnLineSegment(sb.Start(), sb.Finish(), geom.LengthEps) {
			continue
		}
		out := make([]srf.SBezier, 0, len(loop)+1)
		out = append(out, loop[:i]...)
		out = append(out, srf.SBezierFrom2(sb.Start(), p), srf.SBezierFrom2(p, sb.Finish()))
		out = append(out, loop[i+1:]...)
		return out, i + 1, true
	}
	return loop, -1, false
}

// cornerOrder is the vertices of corners, in order.
func cornerOrder(corners map[int]*filletCorner) []int {
	vs := slices.Collect(maps.Keys(corners))
	slices.Sort(vs)
	return vs
}
