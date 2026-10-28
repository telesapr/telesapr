package topo

import (
	"cmp"
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// Chamfer corners. Where every edge at a vertex is chamfered, three or
// more straight edges between flat faces, the strips meet one another at
// creases, not tangentially, so the corner takes neither a sphere nor
// the setback's tangent patches. On each face the two strips' lines,
// each at its distance from its edge, cross; a strip ends on a straight
// line across it between two such points, and the hole the strips leave
// is filled by a face with straight sides: a plane where its corners lie
// in one, else a bilinear patch, else a fan of triangles about its
// middle.
//
// At a corner of three, as OpenCASCADE's makeChamfer does, one strip,
// the pivot, ends across between the points where its lines cross the
// others', and the other two are cut square to their edges where their
// lines cross the pivot's; the face between them gets the line from one
// cut to the other. With every distance one at a square corner each cut
// passes through where the two lines cross on the third face, and the
// hole is the triangle of the three points; with distances of three
// sizes it is a quadrilateral. The pivot is the strip of the largest
// distance whose lines cross the others' furthest along them, so that
// no cut lies short of where the strip's lines cross. At a corner of
// more, the strips end between the points where their lines cross.

// chamferCornerTakes reports whether the corner at vertex v is one
// chamferCornerAt makes: every edge there chamfered, straight, three of
// them of one distance, between as many flat faces, all convex or all
// concave.
func (t *Topology) chamferCornerTakes(v int, edges []filletEdge, byEdge map[int]int) bool {
	xs := t.Vertices[v].Edges
	if len(xs) != 3 || len(t.facesAt(v)) != len(xs) {
		return false
	}
	for _, x := range xs {
		j, ok := byEdge[x]
		if !ok || edges[j].circ || edges[j].concave != edges[byEdge[xs[0]]].concave {
			return false
		}
		if !edges[j].even() || math.Abs(edges[j].ss[0]-edges[byEdge[xs[0]]].ss[0]) > geom.LengthEps {
			return false
		}
	}
	for _, f := range t.facesAt(v) {
		if _, ok := t.planarNormal(f); !ok {
			return false
		}
	}
	return true
}

// chamferStrip is a chamfered edge at a corner, in the order the strips
// go round it: the face it shares with the strip before (in) and with
// the one after (out).
type chamferStrip struct {
	fe      *filletEdge
	in, out int
}

// chamferCornerAt works out the corner at vertex v, which
// chamferCornerTakes takes.
func (t *Topology) chamferCornerAt(v int, edges []filletEdge, byEdge map[int]int) (*filletCorner, error) {
	strips, err := t.chamferStrips(v, edges, byEdge)
	if err != nil {
		return nil, err
	}
	g := &chamferGeom{p: t.Vertices[v].P, v: v, strips: strips}
	if err := g.crossings(); err != nil {
		return nil, err
	}
	ends := g.ends()
	n := len(strips)
	c := &filletCorner{kind: setbackCorner, at: map[[2]int]geom.Vector{}, long: -1,
		arcs: map[int]srf.SBezier{}, faceCurves: map[int]srf.SBezier{}}
	var hole []srf.SBezier
	for i, s := range strips {
		a, b := ends[i][0], ends[i][1]
		if a.Equals(b) {
			return nil, filletErr(g.p, "a chamfer at the corner ends in a point")
		}
		c.at[[2]int{s.fe.e, s.in}], c.at[[2]int{s.fe.e, s.out}] = a, b
		c.arcs[s.fe.e] = srf.SBezierFrom2(a, b)
		hole = append(hole, c.arcs[s.fe.e])
		if q := ends[(i+1)%n][0]; !b.Equals(q) {
			c.faceCurves[s.out] = srf.SBezierFrom2(b, q)
			hole = append(hole, c.faceCurves[s.out])
		}
	}
	if err := g.outside(hole); err != nil {
		return nil, err
	}
	plates, err := chamferPlates(hole)
	if err != nil {
		return nil, filletErr(g.p, "the corner can't be filled: %v", err)
	}
	c.plates = plates
	return c, nil
}

// chamferGeom is a corner of chamfers being worked out: its vertex, at
// p, the strips round it, and where the two strips' lines on each face
// cross.
type chamferGeom struct {
	p      geom.Vector
	v      int
	strips []chamferStrip
	cross  map[int]geom.Vector
}

// away is the direction along fe's edge away from the corner.
func (g *chamferGeom) away(fe *filletEdge) geom.Vector {
	if fe.v[1] == g.v {
		return fe.dir.ScaledBy(-1)
	}
	return fe.dir
}

// base is the point at the corner of strip s's line on face f: the line
// runs from it along the edge, at the strip's distance on that face.
func (g *chamferGeom) base(s chamferStrip, f int) geom.Vector {
	k := 0
	if s.fe.f[1] == f {
		k = 1
	}
	return g.p.Plus(s.fe.u[k].ScaledBy(s.fe.ss[k]))
}

// along is how far along strip s's edge the lines on face f cross.
func (g *chamferGeom) along(s chamferStrip, f int) float64 {
	return g.cross[f].Minus(g.p).Dot(g.away(s.fe))
}

// crossings finds where the lines of the two strips on each face cross,
// which must be past the corner along both.
func (g *chamferGeom) crossings() error {
	g.cross = map[int]geom.Vector{}
	n := len(g.strips)
	for i, s := range g.strips {
		nx := g.strips[(i+1)%n]
		x, ok := lineMeet(g.base(s, s.out), g.away(s.fe), g.base(nx, s.out), g.away(nx.fe))
		if !ok {
			return filletErr(g.p, "two chamfers at the corner run along one another")
		}
		if x.Minus(g.p).Dot(g.away(s.fe)) < 10*geom.LengthEps || x.Minus(g.p).Dot(g.away(nx.fe)) < 10*geom.LengthEps {
			return filletErr(g.p, "two chamfers' lines on a face at the corner cross behind it")
		}
		g.cross[s.out] = x
	}
	return nil
}

// ends are each strip's ends on its in and out faces: where the lines
// cross, but at a corner of three with a pivot, the far ends of the
// pivot's neighbours, cut square to their edges where their lines cross
// the pivot's.
func (g *chamferGeom) ends() [][2]geom.Vector {
	n := len(g.strips)
	ends := make([][2]geom.Vector, n)
	for i, s := range g.strips {
		ends[i] = [2]geom.Vector{g.cross[s.in], g.cross[s.out]}
	}
	if n != 3 {
		return ends
	}
	piv := chamferPivot(g.strips, g.along)
	if piv < 0 {
		return ends
	}
	// The strip cut at near, its far end on face far.
	cut := func(s chamferStrip, near, far int) geom.Vector {
		d := g.along(s, near)
		if math.Abs(d-g.along(s, far)) <= geom.LengthEps {
			return g.cross[far]
		}
		return g.base(s, far).Plus(g.away(s.fe).ScaledBy(d))
	}
	nx, pv := (piv+1)%3, (piv+2)%3
	ends[nx][1] = cut(g.strips[nx], g.strips[nx].in, g.strips[nx].out)
	ends[pv][0] = cut(g.strips[pv], g.strips[pv].out, g.strips[pv].in)
	return ends
}

// outside checks that every corner of the hole lies on each strip's
// plane or past it, away from the vertex: else the face that fills it
// would fold into a strip.
func (g *chamferGeom) outside(hole []srf.SBezier) error {
	for _, s := range g.strips {
		o := g.base(s, s.in)
		m := s.fe.dir.Cross(g.base(s, s.out).Minus(o))
		if m.Magnitude() < 1e-12 {
			return filletErr(g.p, "a chamfer at the corner has no width")
		}
		m = m.WithMagnitude(1)
		side := g.p.Minus(o).Dot(m)
		for _, sb := range hole {
			if d := sb.Start().Minus(o).Dot(m); d*side > 0 && math.Abs(d) > 10*geom.LengthEps {
				return filletErr(g.p, "the chamfers' distances at the corner are too unlike for a face to close it")
			}
		}
	}
	return nil
}

// chamferStrips are the chamfered edges at vertex v in the order they go
// round it, each with the faces it shares with its neighbours.
func (t *Topology) chamferStrips(v int, edges []filletEdge, byEdge map[int]int) ([]chamferStrip, error) {
	p := t.Vertices[v].P
	var round []*filletEdge
	for _, x := range t.Vertices[v].Edges {
		round = append(round, &edges[byEdge[x]])
	}
	other := func(fe *filletEdge, f int) *filletEdge {
		for _, g := range round {
			if g != fe && (g.f[0] == f || g.f[1] == f) {
				return g
			}
		}
		return nil
	}
	fe, f := round[0], round[0].f[0]
	var out []chamferStrip
	for range round {
		g := fe.f[0]
		if g == f {
			g = fe.f[1]
		}
		out = append(out, chamferStrip{fe: fe, in: f, out: g})
		next := other(fe, g)
		if next == nil {
			return nil, filletErr(p, "the corner's edges don't go round it one after another")
		}
		fe, f = next, g
	}
	if fe != round[0] {
		return nil, filletErr(p, "the corner's edges don't go round it one after another")
	}
	return out, nil
}

// chamferPivot is the strip of three that ends between the points where
// its lines cross the others', or -1 when none can: the strip of the
// largest distance, and of those the first, whose neighbours' lines
// cross its own no nearer the vertex than they cross each other's.
// along is how far along a strip's edge the lines on a face cross.
func chamferPivot(strips []chamferStrip, along func(chamferStrip, int) float64) int {
	order := []int{0, 1, 2}
	most := func(s chamferStrip) float64 { return math.Max(s.fe.ss[0], s.fe.ss[1]) }
	slices.SortStableFunc(order, func(a, b int) int { return cmp.Compare(most(strips[b]), most(strips[a])) })
	for _, i := range order {
		nx, pv := strips[(i+1)%3], strips[(i+2)%3]
		if along(nx, nx.in) >= along(nx, nx.out)-geom.LengthEps && along(pv, pv.out) >= along(pv, pv.in)-geom.LengthEps {
			return i
		}
	}
	return -1
}

// cornerPlate is a face that fills a chamfer's corner: its surface and
// its loop.
type cornerPlate struct {
	s    srf.SSurface
	loop []srf.SBezier
}

// chamferPlates fill the hole bounded by the lines of hole, each from
// the end of the one before: a plane where its corners lie in one, a
// bilinear patch where there are four, as OpenCASCADE's filling of four
// straight sides is, else a fan of triangles about its middle.
func chamferPlates(hole []srf.SBezier) ([]cornerPlate, error) {
	pts := make([]geom.Vector, len(hole))
	for i, sb := range hole {
		pts[i] = sb.Start()
	}
	nm := newellOf(pts)
	if nm.Magnitude() < 1e-12 {
		return nil, filletErr(pts[0], "the hole has no area")
	}
	nm = nm.WithMagnitude(1)
	planar := true
	for _, q := range pts {
		planar = planar && math.Abs(q.Minus(pts[0]).Dot(nm)) <= geom.LengthEps
	}
	if planar {
		return []cornerPlate{{s: planeOver(pts, nm), loop: hole}}, nil
	}
	if len(pts) == 4 {
		bs, err := bilinearPatch(pts, nm)
		if err != nil {
			return nil, err
		}
		return []cornerPlate{{s: bs, loop: hole}}, nil
	}
	var mid geom.Vector
	for _, q := range pts {
		mid = mid.Plus(q)
	}
	mid = mid.ScaledBy(1 / float64(len(pts)))
	var out []cornerPlate
	for _, sb := range hole {
		a, b := sb.Start(), sb.Finish()
		tn := b.Minus(a).Cross(mid.Minus(a))
		if tn.Dot(nm) < 1e-12 {
			return nil, filletErr(a, "its triangles fold")
		}
		tri := []geom.Vector{a, b, mid}
		out = append(out, cornerPlate{s: planeOver(tri, tn), loop: []srf.SBezier{sb, srf.SBezierFrom2(b, mid), srf.SBezierFrom2(mid, a)}})
	}
	return out, nil
}

// bilinearPatch is the bilinear patch through the four corners pts, whose
// sides are the lines between them, as a surface of degree 2 both ways
// that covers it with a margin, its parameters turned an eighth of a
// turn to the patch's own: the kernel takes a surface of degree 1 both
// ways for a plane, and meshes a surface along its parameters' lines,
// which on the patch's own are straight and on these are parabolas. nm
// is the way the patch faces, which its normal must keep all over it.
func bilinearPatch(pts []geom.Vector, nm geom.Vector) (srf.SSurface, error) {
	bil := func(a, b float64) geom.Vector {
		p := pts[0].ScaledBy(1 - a).Plus(pts[1].ScaledBy(a))
		q := pts[3].ScaledBy(1 - a).Plus(pts[2].ScaledBy(a))
		return p.ScaledBy(1 - b).Plus(q.ScaledBy(b))
	}
	sign := 0.0
	const g = 8
	for i := 0; i <= g; i++ {
		for j := 0; j <= g; j++ {
			a, b := float64(i)/g, float64(j)/g
			du := pts[1].Minus(pts[0]).ScaledBy(1 - b).Plus(pts[2].Minus(pts[3]).ScaledBy(b))
			dv := pts[3].Minus(pts[0]).ScaledBy(1 - a).Plus(pts[2].Minus(pts[1]).ScaledBy(a))
			d := du.Cross(dv).Dot(nm)
			if math.Abs(d) < 1e-12 || sign != 0 && d*sign < 0 {
				return srf.SSurface{}, filletErr(pts[0], "its patch folds")
			}
			sign = d
		}
	}
	// (a, b) over the diamond about the patch's middle of half-diagonal
	// k, more than 1, which holds its square.
	const k = 1.25
	at := func(u, v float64) geom.Vector {
		return bil(0.5+k*(u+v-1), 0.5+k*(v-u))
	}
	// The control points of the quadratic through each row of three
	// values, at 0, 1/2 and 1, both ways.
	ctrl := func(f0, fm, f1 geom.Vector) [3]geom.Vector {
		return [3]geom.Vector{f0, fm.ScaledBy(2).Minus(f0.Plus(f1).ScaledBy(0.5)), f1}
	}
	var rows [3][3]geom.Vector
	for i := range 3 {
		u := float64(i) / 2
		rows[i] = ctrl(at(u, 0), at(u, 0.5), at(u, 1))
	}
	var s srf.SSurface
	s.DegM, s.DegN = 2, 2
	for j := range 3 {
		col := ctrl(rows[0][j], rows[1][j], rows[2][j])
		for i := range 3 {
			s.Ctrl[i][j], s.Weight[i][j] = col[i], 1
		}
	}
	return s, nil
}
