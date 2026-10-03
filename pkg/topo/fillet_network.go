package topo

import (
	"math"
	"sort"

	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/srf"
)

// Fillets of several edges at once, with one radius, as one edit: every
// face along a rounded edge, or at its end, is built again from its own
// loops with the corners there replaced, and the rounded faces are put
// in. An edge is straight, between two flat faces, and rounded by a
// cylinder; or circular, where a flat face meets a cylinder square to it
// (a boss's top or foot, a hole's rim or floor), and rounded by a torus,
// the edge's cross section turned about the cylinder's axis. A chamfer
// is the same edit with a flat cross section, which on a circular edge
// makes a cone.
//
// An edge sticks out (convex: the fillet takes material away) or goes in
// (concave: the fillet fills the corner). The faces are cut back along
// the same tangent curves either way, and the cross section is the same
// arc, whose control point is on the edge: only the centre of a corner's
// sphere lies outside the solid rather than inside.
//
// At a vertex, which must be a corner of three flat faces or a seam
// along a circular edge, the rounded edges meet in one of these ways:
//
//   - one edge: it ends on the third face, cut along the arc where the
//     rounded face meets it, as the end of a single fillet;
//   - two straight edges: their cylinders meet along an ellipse in the
//     plane that halves them, a mitre; when the faces don't lie alike
//     about them, the one whose section turns further goes on past the
//     ellipse, cut by the other's side face along a conic, to where its
//     tangent line meets the unrounded edge;
//   - three straight edges: a sphere about the point r inside all three
//     faces, or outside them for concave edges (a face's tangent point
//     is that centre moved r back to the face along its normal), which
//     each cylinder meets along the arc where it ends;
//   - two edges that meet smoothly, as two pieces of one circle at a
//     seam of the cylinder, or a rounded outline's arc and line: their
//     rounded faces meet along the cross section there.

// FilletPick picks an edge to round: a straight edge by its ends, which
// must be a whole edge's; a circular one by a point on it, and with it
// the pieces of its circle it goes on into smoothly.
type FilletPick struct {
	A, B geom.Vector
	At   geom.Vector
	Arc  bool
}

// filletEdge is one edge to round and what follows from its faces.
type filletEdge struct {
	e      int
	v      [2]int         // its vertices, from start to end
	f      [2]int         // its faces
	u      [2]geom.Vector // into each face, square to the edge (straight edges)
	dir    geom.Vector    // from v[0] to v[1] (straight edges)
	s, w   float64        // the tangent curves' distance from the edge, the arc's weight
	r      float64        // the radius, or a chamfer's distance
	origin uint32
	// concave: the edge goes in, and the fillet fills it.
	concave bool

	// A circular edge: its curve from v[0] to v[1], the cylinder's axis
	// through centre along axis (the flat face's outward normal), its
	// radius, which face is the flat one, and whether the flat face lies
	// outside the circle (a hole's rim, a boss's foot).
	circ   bool
	curve  srf.SBezier
	centre geom.Vector
	axis   geom.Vector
	radius float64
	flatK  int
	out    bool
}

// sign is 1 for a convex edge, whose fillet's centres lie inside the
// solid, and -1 for a concave one.
func (fe *filletEdge) sign() float64 {
	if fe.concave {
		return -1
	}
	return 1
}

// filletCorner is how the rounded edges meet at a vertex.
type filletCorner struct {
	kind int // the number of rounded edges there, or seam
	// at is the point an edge's end moves to in a face.
	at map[[2]int]geom.Vector
	// For one edge: the face it ends on.
	end int
	// For three: the sphere's centre.
	centre geom.Vector
	// For two: the mitre's plane, through centre.
	mitreN geom.Vector
	// For two whose faces don't lie alike: the edge whose strip turns
	// further, which goes on past the mitre; the curve the two strips
	// share, from the top's tangent point to where the other's strip
	// ends; and the conic where the longer strip meets the other's side
	// face, cutFace, from there to the third edge.
	long    int
	shared  srf.SBezier
	conic   srf.SBezier
	cutFace int
}

// seam is the kind of a corner where two pieces of a circular edge meet.
const seam = 4

// FilletNetwork rounds the edges segs (each a whole straight edge
// between two flat faces) with radius r. The rounded
// faces get origins origin, origin+1, ..., and the spheres the origins
// after them.
func FilletNetwork(sh *srf.SShell, segs [][2]geom.Vector, r float64, origin uint32) error {
	picks := make([]FilletPick, len(segs))
	for i, sg := range segs {
		picks[i] = FilletPick{A: sg[0], B: sg[1]}
	}
	return FilletPicks(sh, picks, r, origin, false)
}

// FilletPicks rounds the picked edges with radius r, or with flat
// chamfers them at distance r along both faces; a chamfer here takes
// circular edges only.
func FilletPicks(sh *srf.SShell, picks []FilletPick, r float64, origin uint32, flat bool) error {
	what := "fillet"
	if flat {
		what = "chamfer"
	}
	if len(picks) == 0 {
		return filletErr(geom.Vector{}, "there are no edges to %s", what)
	}
	at0 := picks[0].A
	if picks[0].Arc {
		at0 = picks[0].At
	}
	if !(r > 0) || math.IsInf(r, 0) {
		if flat {
			return filletErr(at0, "a chamfer's distance must be more than zero")
		}
		return filletErr(at0, "a fillet's radius must be more than zero")
	}
	t, err := Build(sh)
	if err != nil {
		return filletErr(at0, "the solid's topology is broken: %v", err)
	}
	var edges []filletEdge
	byEdge := map[int]int{}
	add := func(fe filletEdge) {
		if _, dup := byEdge[fe.e]; dup {
			// Two entities along one edge, as an extrusion's copies are.
			return
		}
		fe.origin = origin + uint32(len(edges))
		byEdge[fe.e] = len(edges)
		edges = append(edges, fe)
	}
	for _, p := range picks {
		var e int
		if p.Arc {
			if e = t.curvedEdgeAt(p.At); e < 0 {
				return filletErr(p.At, "no edge of the solid runs there")
			}
		} else if e, err = t.wholeEdge(p.A, p.B); err != nil {
			return err
		}
		fe, err := t.filletEdgeOf(e, r, flat)
		if err != nil {
			return err
		}
		add(fe)
	}
	// A rounded edge goes on into the edges it meets smoothly: the other
	// pieces of a circle, and the lines and arcs of a rounded outline.
	for i := 0; i < len(edges); i++ {
		for _, v := range edges[i].v {
			for _, x := range t.Vertices[v].Edges {
				if _, ok := byEdge[x]; ok {
					continue
				}
				if fe, err := t.filletEdgeOf(x, r, flat); err == nil && t.smoothJoin(&edges[i], &fe, v) {
					add(fe)
				}
			}
		}
	}

	// How the edges meet at each vertex they touch.
	corners := map[int]*filletCorner{}
	for _, fe := range edges {
		for _, v := range fe.v {
			if corners[v] != nil {
				continue
			}
			c, err := t.filletCornerAt(v, edges, byEdge, r)
			if err != nil {
				return err
			}
			corners[v] = c
		}
	}

	// The faces built again: those along the edges, and those the edges
	// end on.
	redo := map[int]bool{}
	for _, fe := range edges {
		redo[fe.f[0]], redo[fe.f[1]] = true, true
	}
	for _, c := range corners {
		if c.kind == 1 {
			redo[c.end] = true
		}
	}
	faces := make([]int, 0, len(redo))
	for f := range redo {
		faces = append(faces, f)
	}
	sort.Ints(faces)

	type rebuilt struct {
		s     srf.SSurface
		loops [][]srf.SBezier
	}
	var again []rebuilt
	for _, f := range faces {
		loops, err := t.filletLoops(f, edges, byEdge, corners, flat)
		if err != nil {
			return err
		}
		again = append(again, rebuilt{*t.Surface(f), loops})
	}
	color := t.Surface(edges[0].f[0]).Color

	// The rounded faces' loops and the spheres', while t still describes
	// the solid.
	type patch struct {
		s    srf.SSurface
		loop []srf.SBezier
		at   geom.Vector
	}
	var patches []patch
	for i := range edges {
		fe := &edges[i]
		a, b := t.Vertices[fe.v[0]].P, t.Vertices[fe.v[1]].P
		ca, cb := corners[fe.v[0]], corners[fe.v[1]]
		start := [2]geom.Vector{ca.at[[2]int{fe.e, fe.f[0]}], ca.at[[2]int{fe.e, fe.f[1]}]}
		end := [2]geom.Vector{cb.at[[2]int{fe.e, fe.f[0]}], cb.at[[2]int{fe.e, fe.f[1]}]}
		mid := a.Plus(b).ScaledBy(0.5)
		if fe.circ {
			mid = fe.curve.PointAt(0.5)
		} else {
			for k := 0; k < 2; k++ {
				if end[k].Minus(start[k]).Dot(fe.dir) < 10*geom.LengthEps {
					return filletErr(mid, "the edge is too short for a %s this large", what)
				}
			}
		}
		side0 := fe.tangentCurve(0, start[0], end[0])
		side1 := fe.tangentCurve(1, start[1], end[1])
		side1.Reverse()
		loop := []srf.SBezier{side0}
		loop = append(loop, fe.connectors(t, fe.v[1], cb, end[0], end[1], flat)...)
		loop = append(loop, side1)
		loop = append(loop, fe.connectors(t, fe.v[0], ca, start[1], start[0], flat)...)
		s := fe.surface(a, b, flat)
		s.Color = color
		s.Origin = fe.origin
		patches = append(patches, patch{s, loop, mid})
	}
	next := origin + uint32(len(edges))
	var vs []int
	for v := range corners {
		vs = append(vs, v)
	}
	sort.Ints(vs)
	for _, v := range vs {
		c := corners[v]
		if c.kind != 3 {
			continue
		}
		var loop []srf.SBezier
		for _, x := range t.Vertices[v].Edges {
			fe := &edges[byEdge[x]]
			loop = append(loop, fe.connector(t, v, c, c.at[[2]int{x, fe.f[0]}], c.at[[2]int{x, fe.f[1]}], flat))
		}
		loop, err := chainCurves(loop)
		if err != nil {
			return filletErr(t.Vertices[v].P, "the corner's arcs don't close: %v", err)
		}
		sphere := spherePatch(c.centre, r, loop)
		sphere.Color = color
		sphere.Origin = next
		next++
		patches = append(patches, patch{sphere, loop, t.Vertices[v].P})
	}

	RemoveFaces(sh, t, faces)
	for _, rb := range again {
		if _, err := AddFace(sh, rb.s, rb.loops); err != nil {
			return filletErr(rb.loops[0][0].Start(), "a face can't be built again: %v", err)
		}
	}
	for _, p := range patches {
		if _, err := AddFace(sh, p.s, [][]srf.SBezier{p.loop}); err != nil {
			return filletErr(p.at, "the %s's face can't be put in: %v", what, err)
		}
	}
	if t, err = Build(sh); err != nil {
		return filletErr(at0, "%v", err)
	}
	if probs := t.Validate(); len(probs) > 0 {
		return filletErr(probs[0].At, "the %s leaves the solid open: %s", what, probs[0].Msg)
	}
	return nil
}

// curvedEdgeAt is the edge through p, a point on the curve the edge's
// points are chords of: within a twentieth of its longest chord, more
// than the sag of a chord that turns by less than a fifth of a turn.
func (t *Topology) curvedEdgeAt(p geom.Vector) int {
	best, bestD := -1, math.Inf(1)
	for e := range t.Edges {
		pts := t.Edges[e].Points
		longest := 0.0
		for i := 1; i < len(pts); i++ {
			longest = math.Max(longest, pts[i].Minus(pts[i-1]).Magnitude())
		}
		d := distanceToPolyline(p, pts)
		if d <= math.Max(1e-3, 0.05*longest) && d < bestD {
			best, bestD = e, d
		}
	}
	return best
}

// wholeEdge is the edge from a to b, which must be a whole edge.
func (t *Topology) wholeEdge(a, b geom.Vector) (int, error) {
	m := a.Plus(b).ScaledBy(0.5)
	e := t.edgeAtPoint(m)
	if e < 0 {
		return -1, filletErr(m, "no edge of the solid runs there")
	}
	va, vb := t.vertexNear(a), t.vertexNear(b)
	ed := &t.Edges[e]
	whole := ed.V[0] == va && ed.V[1] == vb || ed.V[0] == vb && ed.V[1] == va
	if va < 0 || vb < 0 || !whole {
		return -1, filletErr(m, "the segment is not a whole edge of the solid")
	}
	return e, nil
}

// filletEdgeOf checks edge e and works out its fillet, or with flat its
// chamfer.
func (t *Topology) filletEdgeOf(e int, r float64, flat bool) (filletEdge, error) {
	var fe filletEdge
	ed := &t.Edges[e]
	pa, pb := t.Vertices[ed.V[0]].P, t.Vertices[ed.V[1]].P
	m := midpoint(ed.Points)
	fs := t.EdgeFaces(e)
	if fs[0] < 0 || fs[1] < 0 || len(ed.CoEdges) != 2 {
		return fe, filletErr(m, "the edge is not between two faces")
	}
	if !t.Straight(e) {
		return t.circularEdgeOf(e, r, flat)
	}
	if flat {
		return fe, filletErr(m, "a chamfer here cuts circular edges; straight ones are cut by a Boolean")
	}
	n1, ok1 := t.planarNormal(fs[0])
	n2, ok2 := t.planarNormal(fs[1])
	if !ok1 || !ok2 {
		return fe, filletErr(m, "a face at the edge is curved: a fillet here rounds edges between flat faces")
	}
	u1 := t.intoFace(fs[0], pa, pb, n1)
	u2 := t.intoFace(fs[1], pa, pb, n2)
	// Each face runs away from the other's outside for a convex edge,
	// towards it for a concave one.
	c1, c2 := u1.Dot(n2), u2.Dot(n1)
	if math.Abs(c1) < 1e-9 || math.Abs(c2) < 1e-9 || (c1 > 0) != (c2 > 0) {
		return fe, filletErr(m, "the edge is flat: a fillet rounds edges whose faces meet at an angle")
	}
	// The angle between the faces on the side the arc lies, outside the
	// solid for a concave edge.
	beta := math.Acos(math.Max(-1, math.Min(1, u1.Dot(u2))))
	fe = filletEdge{
		e: e, v: ed.V, f: fs, u: [2]geom.Vector{u1, u2}, r: r, concave: c1 > 0,
		dir: pb.Minus(pa).WithMagnitude(1), s: r / math.Tan(beta/2), w: math.Sin(beta / 2),
	}
	return fe, nil
}

// circularEdgeOf checks a curved edge: an arc where a flat face meets a
// cylinder square to it, the solid in the corner between them (convex)
// or round it (concave).
func (t *Topology) circularEdgeOf(e int, r float64, flat bool) (filletEdge, error) {
	var fe filletEdge
	ed := &t.Edges[e]
	m := midpoint(ed.Points)
	fs := t.EdgeFaces(e)
	what := "a fillet"
	if flat {
		what = "a chamfer"
	}
	flatK := -1
	var np geom.Vector
	for k, f := range fs {
		if n, ok := t.planarNormal(f); ok {
			flatK, np = k, n
		}
	}
	if flatK < 0 {
		return fe, filletErr(m, "the edge is curved and neither face at it is flat: %s rounds edges of flat faces and cylinders", what)
	}
	var axis, centre, start, finish geom.Vector
	var rad float64
	cyl := t.Surface(fs[1-flatK])
	if !cyl.IsCylinder(&axis, &centre, &rad, &start, &finish) || axis.WithMagnitude(1).Cross(np).Magnitude() > 1e-9 {
		return fe, filletErr(m, "the edge is curved, and the face beside the flat one is not a cylinder square to it")
	}
	pa, pb := t.Vertices[ed.V[0]].P, t.Vertices[ed.V[1]].P
	c := t.shell.FindCurve(ed.Curve)
	if c == nil || !c.IsExact {
		return fe, filletErr(m, "the edge has no exact curve")
	}
	sb := c.Exact
	switch {
	case sb.Start().Equals(pa) && sb.Finish().Equals(pb):
	case sb.Start().Equals(pb) && sb.Finish().Equals(pa):
		sb.Reverse()
	default:
		return fe, filletErr(m, "the edge is part of a longer curve")
	}
	var cc geom.Vector
	var cr float64
	if !sb.IsCircle(np, &cc, &cr) || math.Abs(cr-rad) > geom.LengthEps {
		return fe, filletErr(m, "the edge is not a circle of the cylinder")
	}
	fe = filletEdge{
		e: e, v: ed.V, f: fs, s: r, r: r, w: math.Sqrt2 / 2,
		circ: true, curve: sb, centre: cc, axis: np, radius: rad, flatK: flatK,
	}
	// The cylinder lies under the flat face for a convex edge (a boss's
	// top, a hole's rim), over it for a concave one (a boss's foot, a
	// hole's floor); the flat face on one side of the circle.
	below, above := false, false
	for _, x := range t.FaceEdges(fs[1-flatK]) {
		for _, p := range t.Edges[x].Points {
			h := p.Minus(m).Dot(np)
			below = below || h < -10*geom.LengthEps
			above = above || h > 10*geom.LengthEps
		}
	}
	step := 1e-3 * math.Min(rad, 1)
	radial := fe.radial(m)
	inside := t.inFace(fs[flatK], m.Minus(radial.ScaledBy(step)), np)
	outside := t.inFace(fs[flatK], m.Plus(radial.ScaledBy(step)), np)
	if below == above || inside == outside {
		return fe, filletErr(m, "the flat face and the cylinder don't meet as a corner at the edge")
	}
	if inside && r >= rad-geom.LengthEps {
		return fe, filletErr(m, "a radius this large is wider than the cylinder")
	}
	fe.out, fe.concave = outside, above
	return fe, nil
}

// smoothJoin reports whether rounded edges a and b meet smoothly at
// vertex v: they share a face, their other faces meet tangentially at
// the third edge there, and their cross sections at v are one, so that
// their rounded faces go on into each other with no corner between, as
// at a seam of a cylinder or where a rounded outline's arc meets its
// line.
func (t *Topology) smoothJoin(a, b *filletEdge, v int) bool {
	if a.e == b.e || a.concave != b.concave || math.Abs(a.s-b.s) > geom.LengthEps || math.Abs(a.w-b.w) > 1e-9 {
		return false
	}
	if (a.v[0] != v && a.v[1] != v) || (b.v[0] != v && b.v[1] != v) {
		return false
	}
	p := t.Vertices[v].P
	shared := 0
	for ka := 0; ka < 2; ka++ {
		for kb := 0; kb < 2; kb++ {
			if a.f[ka] == b.f[kb] {
				shared++
				// The tangent points on the shared face, and on the others.
				qa, qb := p.Plus(a.into(ka, p).ScaledBy(a.s)), p.Plus(b.into(kb, p).ScaledBy(b.s))
				ra, rb := p.Plus(a.into(1-ka, p).ScaledBy(a.s)), p.Plus(b.into(1-kb, p).ScaledBy(b.s))
				if !qa.Equals(qb) || !ra.Equals(rb) {
					return false
				}
			}
		}
	}
	return shared == 1
}

// radial is the direction from a circular edge's axis to p, square to
// the axis.
func (fe *filletEdge) radial(p geom.Vector) geom.Vector {
	d := p.Minus(fe.centre)
	return d.Minus(fe.axis.ScaledBy(d.Dot(fe.axis))).WithMagnitude(1)
}

// into is the direction at p, on the edge, square to it in face k and
// into that face: for a circular edge's flat face towards the axis, or
// away from it when the face lies outside the circle, and down its
// cylinder, or up it for a concave edge.
func (fe *filletEdge) into(k int, p geom.Vector) geom.Vector {
	switch {
	case !fe.circ:
		return fe.u[k]
	case k != fe.flatK:
		return fe.axis.ScaledBy(-fe.sign())
	case fe.out:
		return fe.radial(p)
	}
	return fe.radial(p).ScaledBy(-1)
}

// tangentCurve is the curve along which the rounded face meets face k,
// from p (at the edge's start) to q (at its end): a line, or for a
// circular edge its arc, turned to a new radius on the flat face or
// moved along the cylinder.
func (fe *filletEdge) tangentCurve(k int, p, q geom.Vector) srf.SBezier {
	if !fe.circ {
		return srf.SBezierFrom2(p, q)
	}
	sb := fe.curve
	if k == fe.flatK {
		d := p.Minus(fe.centre)
		scale := d.Minus(fe.axis.ScaledBy(d.Dot(fe.axis))).Magnitude() / fe.radius
		for i := 0; i <= sb.Degree; i++ {
			sb.Ctrl[i] = fe.centre.Plus(sb.Ctrl[i].Minus(fe.centre).ScaledBy(scale))
		}
	} else {
		for i := 0; i <= sb.Degree; i++ {
			sb.Ctrl[i] = sb.Ctrl[i].Plus(fe.into(k, p).ScaledBy(fe.s))
		}
	}
	sb.Ctrl[0], sb.Ctrl[sb.Degree] = p, q
	return sb
}

// section is the cross section of the rounded face at p on the edge,
// from its point on face 0 to its point on face 1: an arc whose control
// point is p, or with flat a line.
func (fe *filletEdge) section(p geom.Vector, flat bool) srf.SBezier {
	a := p.Plus(fe.into(0, p).ScaledBy(fe.s))
	b := p.Plus(fe.into(1, p).ScaledBy(fe.s))
	if flat {
		return srf.SBezierFrom2(a, b)
	}
	return arc(a, p, b, fe.w)
}

// surface is the rounded face's surface: for a straight edge from a to
// b, the cross section swept along it, with a margin; for a circular
// one, the cross section at its start turned about the axis to its end.
func (fe *filletEdge) surface(a, b geom.Vector, flat bool) srf.SSurface {
	if !fe.circ {
		margin := fe.s + 2*fe.r
		l := b.Minus(a).Magnitude()
		p0 := a.Minus(fe.dir.ScaledBy(margin))
		section := fe.section(p0, flat)
		return srf.FromExtrusionOf(&section, geom.Vector{}, fe.dir.ScaledBy(l+2*margin))
	}
	section := fe.section(fe.curve.Start(), flat)
	// The angle from the arc's start to its end about the axis; an arc
	// of degree 2 turns by less than half a turn.
	ra, rb := fe.radial(fe.curve.Start()), fe.radial(fe.curve.Finish())
	theta := math.Atan2(ra.Cross(rb).Dot(fe.axis), ra.Dot(rb))
	// RotatedAroundAxis by a positive angle turns ra towards axis x ra,
	// or the other way.
	if ra.RotatedAroundAxis(fe.axis, 0.1).Dot(fe.axis.Cross(ra)) < 0 {
		theta = -theta
	}
	return srf.FromRevolutionOf(&section, fe.centre, fe.axis, 0, theta, 0, 0)
}

// facesAt are the faces round vertex v, each once.
func (t *Topology) facesAt(v int) []int {
	var out []int
	seen := map[int]bool{}
	for _, x := range t.Vertices[v].Edges {
		for _, f := range t.EdgeFaces(x) {
			if f >= 0 && !seen[f] {
				seen[f] = true
				out = append(out, f)
			}
		}
	}
	return out
}

// filletCornerAt works out how the rounded edges meet at vertex v.
func (t *Topology) filletCornerAt(v int, edges []filletEdge, byEdge map[int]int, r float64) (*filletCorner, error) {
	p := t.Vertices[v].P
	if len(t.Vertices[v].Edges) != 3 || len(t.facesAt(v)) != 3 {
		return nil, filletErr(p, "the edge ends where %d edges meet; a fillet's end needs a corner of three faces", len(t.Vertices[v].Edges))
	}
	var round, plain []int
	circular := 0
	for _, x := range t.Vertices[v].Edges {
		if i, ok := byEdge[x]; ok {
			round = append(round, x)
			if edges[i].circ {
				circular++
			}
		} else {
			plain = append(plain, x)
		}
	}
	c := &filletCorner{kind: len(round), at: map[[2]int]geom.Vector{}, long: -1}
	if c.kind == 2 && t.smoothJoin(&edges[byEdge[round[0]]], &edges[byEdge[round[1]]], v) {
		// A smooth join: both share the cross section, and the third edge,
		// between their other faces, is cut back to its point on them.
		a, b := &edges[byEdge[round[0]]], &edges[byEdge[round[1]]]
		c.kind = seam
		for _, fe := range []*filletEdge{a, b} {
			for k := 0; k < 2; k++ {
				c.at[[2]int{fe.e, fe.f[k]}] = p.Plus(fe.into(k, p).ScaledBy(fe.s))
			}
		}
		k := 0
		if a.f[0] == b.f[0] || a.f[0] == b.f[1] {
			k = 1
		}
		down := p.Plus(a.into(k, p).ScaledBy(a.s))
		for _, f := range t.EdgeFaces(plain[0]) {
			c.at[[2]int{plain[0], f}] = down
		}
		return c, nil
	}
	if circular > 0 && c.kind == 2 {
		return nil, filletErr(p, "a circular edge's fillet meets another's at an angle; they must meet smoothly")
	}
	if circular > 0 && c.kind != 1 {
		return nil, filletErr(p, "a circular edge's fillet meets other fillets here")
	}
	sg := 1.0
	for i, x := range round {
		fe := &edges[byEdge[x]]
		if i > 0 && fe.sign() != sg {
			return nil, filletErr(p, "a convex edge's fillet meets a concave one's here")
		}
		sg = fe.sign()
	}
	if circular == 0 {
		for _, f := range t.facesAt(v) {
			if _, ok := t.planarNormal(f); !ok {
				return nil, filletErr(p, "a face at the edge's end is curved")
			}
		}
	}
	switch c.kind {
	case 1:
		fe := &edges[byEdge[round[0]]]
		along, end, err := t.endEdges(fe.e, v, fe.f)
		if err != nil {
			return nil, err
		}
		ne, ok := t.planarNormal(end)
		if !ok {
			return nil, filletErr(p, "the face at the edge's end is curved")
		}
		if fe.circ && (math.Abs(ne.Dot(fe.axis)) > 1e-9 || math.Abs(ne.Dot(fe.radial(p))) > 1e-9) {
			return nil, filletErr(p, "a circular edge's fillet ends on a face that is not square to it")
		}
		c.end = end
		for k := 0; k < 2; k++ {
			q, err := t.pointAlong(along[k], v, fe.into(k, p), fe.s, fe.concave)
			if err != nil {
				return nil, err
			}
			c.at[[2]int{fe.e, fe.f[k]}] = q
			c.at[[2]int{along[k], fe.f[k]}] = q
			c.at[[2]int{along[k], end}] = q
		}
	case 3:
		fs := t.facesAt(v)
		var n [3]geom.Vector
		var d [3]float64
		for i, f := range fs {
			n[i], _ = t.planarNormal(f)
			d[i] = n[i].Dot(p) - sg*r
		}
		if math.Abs(n[0].Dot(n[1].Cross(n[2]))) < 1e-9 {
			return nil, filletErr(p, "the faces at the corner don't meet in a point")
		}
		c.centre = meet3(n[0], d[0], n[1], d[1], n[2], d[2])
		for i, f := range fs {
			tp := c.centre.Plus(n[i].ScaledBy(sg * r))
			for _, x := range t.Vertices[v].Edges {
				efs := t.EdgeFaces(x)
				if efs[0] == f || efs[1] == f {
					c.at[[2]int{x, f}] = tp
				}
			}
		}
	case 2:
		a, b := &edges[byEdge[round[0]]], &edges[byEdge[round[1]]]
		z := plain[0]
		top := -1
		for _, f := range a.f {
			if f == b.f[0] || f == b.f[1] {
				top = f
			}
		}
		if top < 0 {
			return nil, filletErr(p, "two rounded edges meet here without a face between them")
		}
		// The tangent line of each edge on its other face reaches z.
		var q [2]geom.Vector
		var side [2]int
		for i, fe := range []*filletEdge{a, b} {
			k := 0
			if fe.f[0] == top {
				k = 1
			}
			side[i] = fe.f[k]
			var err error
			if q[i], err = t.pointAlong(z, v, fe.u[k], fe.s, false); err != nil {
				return nil, err
			}
		}
		nt, _ := t.planarNormal(top)
		n1, _ := t.planarNormal(side[0])
		n2, _ := t.planarNormal(side[1])
		c.centre = meet3(nt, nt.Dot(p)-sg*r, n1, n1.Dot(p)-sg*r, n2, n2.Dot(p)-sg*r)
		tt := c.centre.Plus(nt.ScaledBy(sg * r))
		if !q[0].Equals(q[1]) {
			if err := t.unevenMitre(c, v, [2]*filletEdge{a, b}, top, side, q, sg*r); err != nil {
				return nil, err
			}
			return c, nil
		}
		// The plane through the centre that halves the two cylinders and
		// holds the mitre's ends.
		m := q[0].Minus(c.centre).Cross(tt.Minus(c.centre))
		if m.Magnitude() < 1e-12 {
			return nil, filletErr(p, "the mitre between the two fillets is degenerate")
		}
		c.mitreN = m.WithMagnitude(1)
		c.at[[2]int{a.e, top}], c.at[[2]int{b.e, top}] = tt, tt
		c.at[[2]int{a.e, side[0]}], c.at[[2]int{z, side[0]}] = q[0], q[0]
		c.at[[2]int{b.e, side[1]}], c.at[[2]int{z, side[1]}] = q[1], q[1]
	}
	return c, nil
}

// unevenMitre works out the corner where two fillets meet whose side
// faces don't lie alike about them, their sections turning by different
// angles. Their cylinders, of one radius, have axes that meet at the
// centre, so they still meet along an ellipse in the plane that halves
// the edges, through the centre and square to the top; but the strip
// that turns less, A, reaches its side face's tangent line there before
// the other, B, does. B goes on past the ellipse, cut by A's side face
// along a conic, to where its own tangent line meets the third edge.
// q are where each edge's tangent line on its side face meets the third
// edge; sr is the radius, negative for concave edges.
func (t *Topology) unevenMitre(c *filletCorner, v int, fes [2]*filletEdge, top int, side [2]int, q [2]geom.Vector, sr float64) error {
	p := t.Vertices[v].P
	ia, ib := 0, 1
	if fes[0].w < fes[1].w {
		ia, ib = 1, 0
	}
	A, B := fes[ia], fes[ib]
	away := func(fe *filletEdge) geom.Vector {
		if fe.v[1] == v {
			return fe.dir.ScaledBy(-1)
		}
		return fe.dir
	}
	nt, _ := t.planarNormal(top)
	na, _ := t.planarNormal(side[ia])
	nb, _ := t.planarNormal(side[ib])
	m := away(A).Plus(away(B)).Cross(nt)
	if m.Magnitude() < 1e-9 {
		return filletErr(p, "the mitre between the two fillets is degenerate")
	}
	c.mitreN = m.WithMagnitude(1)
	tt := c.centre.Plus(nt.ScaledBy(sr))
	// A's tangent line on its side meets the mitre's plane at qa, which
	// lies further from the vertex than where the line meets the third
	// edge.
	la := c.centre.Plus(na.ScaledBy(sr))
	qa := la.Plus(A.dir.ScaledBy(c.centre.Minus(la).Dot(c.mitreN) / A.dir.Dot(c.mitreN)))
	w := q[ib]
	if qa.Minus(q[ia]).Dot(away(A)) < 10*geom.LengthEps {
		return filletErr(p, "two fillets meet at a corner whose faces lie too unlike about them for a mitre")
	}
	// The conic in A's side face on B's cylinder, from qa to w: tangent at
	// qa to both, and at w to the third edge, where B's cylinder touches
	// its own side face.
	radial := func(x geom.Vector) geom.Vector {
		d := x.Minus(c.centre)
		return d.Minus(B.dir.ScaledBy(d.Dot(B.dir)))
	}
	t0 := na.Cross(radial(qa)).WithMagnitude(1)
	t1 := na.Cross(nb).WithMagnitude(1)
	ctrl, ok := lineMeet(qa, t0, w, t1)
	if !ok {
		return filletErr(p, "the mitre between the two fillets is degenerate")
	}
	// The weight from the conic's middle, on the line from the chord's
	// middle to the control point: B's cylinder there.
	mc := qa.Plus(w).ScaledBy(0.5)
	d0, d1 := radial(mc), radial(ctrl).Minus(radial(mc))
	qa2, qb2, qc2 := d1.Dot(d1), 2*d0.Dot(d1), d0.Dot(d0)-sr*sr
	disc := qb2*qb2 - 4*qa2*qc2
	if qa2 < 1e-18 || disc < 0 {
		return filletErr(p, "the mitre between the two fillets is degenerate")
	}
	k := -1.0
	for _, x := range []float64{(-qb2 - math.Sqrt(disc)) / (2 * qa2), (-qb2 + math.Sqrt(disc)) / (2 * qa2)} {
		if x > 1e-9 && x < 1-1e-9 {
			k = x
		}
	}
	if k < 0 {
		return filletErr(p, "the mitre between the two fillets is degenerate")
	}
	c.conic = arc(qa, ctrl, w, k/(1-k))
	c.long, c.cutFace = B.e, side[ia]
	c.at[[2]int{A.e, top}], c.at[[2]int{B.e, top}] = tt, tt
	c.at[[2]int{A.e, side[ia]}] = qa
	c.at[[2]int{B.e, side[ib]}] = w
	for _, f := range t.EdgeFaces(c.plainEdge(t, v, A.e, B.e)) {
		c.at[[2]int{c.plainEdge(t, v, A.e, B.e), f}] = w
	}
	c.shared = A.connector(t, v, c, tt, qa, false)
	return nil
}

// plainEdge is the edge at vertex v other than a and b.
func (c *filletCorner) plainEdge(t *Topology, v, a, b int) int {
	for _, x := range t.Vertices[v].Edges {
		if x != a && x != b {
			return x
		}
	}
	return -1
}

// lineMeet is where the line through a along da meets the one through b
// along db, which lie in one plane.
func lineMeet(a, da, b, db geom.Vector) (geom.Vector, bool) {
	n := da.Cross(db)
	if n.Magnitude() < 1e-12 {
		return geom.Vector{}, false
	}
	k := b.Minus(a).Cross(db).Dot(n) / n.Dot(n)
	return a.Plus(da.ScaledBy(k)), true
}

// connectors are the curves across the edge's rounded face at vertex v,
// from p to q: one, but for the strip that goes on past an uneven
// mitre, which runs along the mitre and then the conic.
func (fe *filletEdge) connectors(t *Topology, v int, c *filletCorner, p, q geom.Vector, flat bool) []srf.SBezier {
	if c.kind != 2 || c.long != fe.e || flat {
		return []srf.SBezier{fe.connector(t, v, c, p, q, flat)}
	}
	chain := []srf.SBezier{c.shared, c.conic}
	if !chain[0].Start().Equals(p) {
		chain[0].Reverse()
		chain[1].Reverse()
		chain[0], chain[1] = chain[1], chain[0]
	}
	return chain
}

// connector is the curve across the edge's rounded face at vertex v,
// from p to q: the arc it ends along, or with flat a line.
func (fe *filletEdge) connector(t *Topology, v int, c *filletCorner, p, q geom.Vector, flat bool) srf.SBezier {
	vp := t.Vertices[v].P
	if flat {
		return srf.SBezierFrom2(p, q)
	}
	// The cross section's control point is where the section's plane
	// meets the edge; across, the arc between the tangent points.
	switch c.kind {
	case 1, seam:
		// At the vertex, or projected along the edge onto the face it
		// ends on: its control point the vertex.
		return arc(p, vp, q, fe.w)
	case 3:
		x := vp.Plus(fe.dir.ScaledBy(c.centre.Minus(vp).Dot(fe.dir)))
		return arc(p, x, q, fe.w)
	}
	// A mitre: the section at the centre, projected along the edge onto
	// the mitre's plane.
	x := vp.Plus(fe.dir.ScaledBy(c.centre.Minus(vp).Dot(fe.dir)))
	onto := func(a geom.Vector) geom.Vector {
		k := c.centre.Minus(a).Dot(c.mitreN) / fe.dir.Dot(c.mitreN)
		return a.Plus(fe.dir.ScaledBy(k))
	}
	sb := arc(p, onto(x), q, fe.w)
	sb.Ctrl[0], sb.Ctrl[2] = p, q
	return sb
}

// filletLoops are face f's loops with the corners the fillets reach
// moved: each co-edge runs between the points its ends move to, a
// rounded edge along its tangent curve, and a face a fillet ends on
// takes the curve between the two points it has at that vertex.
func (t *Topology) filletLoops(f int, edges []filletEdge, byEdge map[int]int, corners map[int]*filletCorner, flat bool) ([][]srf.SBezier, error) {
	point := func(x, v int) geom.Vector {
		if c := corners[v]; c != nil {
			if q, ok := c.at[[2]int{x, f}]; ok {
				return q
			}
		}
		return t.Vertices[v].P
	}
	var loops [][]srf.SBezier
	for _, l := range t.outerFirst(f) {
		ces := t.Loops[l].CoEdges
		var loop []srf.SBezier
		for i, ce := range ces {
			x := t.CoEdges[ce].Edge
			va, vb := t.Start(ce), t.End(ce)
			a, b := point(x, va), point(x, vb)
			moved := !a.Equals(t.Vertices[va].P) || !b.Equals(t.Vertices[vb].P)
			ri, round := byEdge[x]
			switch {
			case round:
				fe := &edges[ri]
				k := 0
				if fe.f[1] == f {
					k = 1
				}
				if va == fe.v[0] {
					loop = append(loop, fe.tangentCurve(k, a, b))
				} else {
					sb := fe.tangentCurve(k, b, a)
					sb.Reverse()
					loop = append(loop, sb)
				}
			case t.Straight(x):
				loop = append(loop, srf.SBezierFrom2(a, b))
			case moved:
				return nil, filletErr(a, "a curved edge of a face at the fillet would have to be cut")
			default:
				sb, err := t.edgeCurve(ce)
				if err != nil {
					return nil, err
				}
				loop = append(loop, sb)
			}
			// At the vertex the co-edge ends at, the end of the rounded face
			// when the face is one a fillet ends on.
			nx := t.CoEdges[ces[(i+1)%len(ces)]].Edge
			if c := corners[vb]; c != nil && c.kind == 2 && c.long >= 0 && c.cutFace == f {
				// The conic where an uneven mitre's longer strip cuts this face.
				p, q := point(x, vb), point(nx, vb)
				sb := c.conic
				if !sb.Start().Equals(p) {
					sb.Reverse()
				}
				if !p.Equals(q) {
					loop = append(loop, sb)
				}
			}
			if c := corners[vb]; c != nil && c.kind == 1 && c.end == f {
				p, q := point(x, vb), point(nx, vb)
				for _, fe := range edges {
					if fe.v[0] == vb || fe.v[1] == vb {
						loop = append(loop, fe.connector(t, vb, c, p, q, flat))
						break
					}
				}
			}
		}
		loops = append(loops, loop)
	}
	return loops, nil
}

// edgeCurve is the exact curve of co-edge ce, run its way: the shell's
// curve, when it spans the edge.
func (t *Topology) edgeCurve(ce int) (srf.SBezier, error) {
	ed := &t.Edges[t.CoEdges[ce].Edge]
	a, b := t.Vertices[t.Start(ce)].P, t.Vertices[t.End(ce)].P
	c := t.shell.FindCurve(ed.Curve)
	if c == nil || !c.IsExact {
		return srf.SBezier{}, filletErr(a, "a curved edge of a face at the fillet has no exact curve")
	}
	sb := c.Exact
	switch {
	case sb.Start().Equals(a) && sb.Finish().Equals(b):
	case sb.Start().Equals(b) && sb.Finish().Equals(a):
		sb.Reverse()
	default:
		return srf.SBezier{}, filletErr(a, "a curved edge of a face at the fillet is part of a longer curve")
	}
	return sb, nil
}

// chainCurves orders curves into a closed loop, reversing those that
// run the other way.
func chainCurves(cs []srf.SBezier) ([]srf.SBezier, error) {
	out := []srf.SBezier{cs[0]}
	used := make([]bool, len(cs))
	used[0] = true
	for len(out) < len(cs) {
		end := out[len(out)-1].Finish()
		found := false
		for i, c := range cs {
			if used[i] {
				continue
			}
			if c.Start().Equals(end) {
				out, used[i], found = append(out, c), true, true
				break
			}
			if c.Finish().Equals(end) {
				c.Reverse()
				out, used[i], found = append(out, c), true, true
				break
			}
		}
		if !found {
			return nil, filletErr(end, "no arc goes on from here")
		}
	}
	if !out[len(out)-1].Finish().Equals(out[0].Start()) {
		return nil, filletErr(out[0].Start(), "the arcs don't close")
	}
	return out, nil
}

// spherePatch is a patch of the sphere about c of radius r over the
// spherical triangle the loop bounds: a surface of revolution about an
// axis through c square to the triangle's middle, so that neither pole
// lies on it, covering the triangle with a margin.
func spherePatch(c geom.Vector, r float64, loop []srf.SBezier) srf.SSurface {
	var pts []geom.Vector
	for _, sb := range loop {
		for k := 0; k < 32; k++ {
			pts = append(pts, sb.PointAt(float64(k)/32).Minus(c).WithMagnitude(1))
		}
	}
	var mid geom.Vector
	for _, p := range pts {
		mid = mid.Plus(p)
	}
	mid = mid.WithMagnitude(1)
	axis := mid.NormalDir(0).WithMagnitude(1)
	e0 := mid.Minus(axis.ScaledBy(mid.Dot(axis))).WithMagnitude(1)
	e1 := axis.Cross(e0)
	// The triangle's range of angle from the axis and about it.
	th0, th1 := math.Inf(1), math.Inf(-1)
	ph0, ph1 := math.Inf(1), math.Inf(-1)
	for _, p := range pts {
		th := math.Acos(math.Max(-1, math.Min(1, p.Dot(axis))))
		ph := math.Atan2(p.Dot(e1), p.Dot(e0))
		th0, th1 = math.Min(th0, th), math.Max(th1, th)
		ph0, ph1 = math.Min(ph0, ph), math.Max(ph1, ph)
	}
	const margin = 0.05
	th0, th1 = math.Max(th0-margin, 1e-3), math.Min(th1+margin, math.Pi-1e-3)
	ph0, ph1 = ph0-margin, ph1+margin
	// The meridian at angle 0 about the axis, from th0 to th1.
	at := func(th float64) geom.Vector {
		return c.Plus(axis.ScaledBy(r * math.Cos(th))).Plus(e0.ScaledBy(r * math.Sin(th)))
	}
	half := (th1 - th0) / 2
	ctrl := c.Plus(axis.ScaledBy(r * math.Cos(th0+half) / math.Cos(half))).
		Plus(e0.ScaledBy(r * math.Sin(th0+half) / math.Cos(half)))
	meridian := arc(at(th0), ctrl, at(th1), math.Cos(half))
	// RotatedAbout turns e0 towards e1 for a positive angle about axis.
	sign := 1.0
	if e0.RotatedAroundAxis(axis, 0.1).Dot(e1) < 0 {
		sign = -1
	}
	return srf.FromRevolutionOf(&meridian, c, axis, sign*ph0, sign*ph1, 0, 0)
}
