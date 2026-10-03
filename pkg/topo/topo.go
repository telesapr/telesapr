// Package topo derives the topology of a shell: its faces, the loops
// that bound them, the co-edges of each loop, the edges where two faces
// meet and the vertices where edges end. srf.SShell keeps none of
// these: a shell is a list of trimmed surfaces and the curves that trim
// them, each curve naming its two surfaces. Build walks that list once
// and answers adjacency questions from the result.
//
// The topology is derived, not kept: it is rebuilt from the shell
// whenever the shell changes, as a regeneration does (ROADMAP-JUMP.md,
// step 9, "Topology as output of the kernel" was not taken).
package topo

import (
	"fmt"
	"math"
	"sort"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// Topology is the derived topology of a shell. Indices into its slices
// name its parts; they are stable for one shell, and are not names that
// survive a regeneration (see Name for those).
type Topology struct {
	Vertices []Vertex
	Edges    []Edge
	CoEdges  []CoEdge
	Loops    []Loop
	Faces    []Face
	// Sheets are the faces as a person sees them, each one or more
	// patches (Faces) joined along seams (see sheet.go).
	Sheets []Sheet

	shell *srf.SShell
	// surfFace maps a surface handle to the faces on it.
	surfFace map[core.HSSurface][]int
	// byPair lists the edges between each pair of identities in the
	// order names number them.
	byPair map[[2]srf.FaceID][]int
}

// Vertex is a point where edges end.
type Vertex struct {
	P geom.Vector
	// Edges are the edges that end here.
	Edges []int
}

// Edge is where two faces meet: a piece of a trim curve of the shell,
// between two vertices, used by two co-edges running opposite ways.
type Edge struct {
	Curve core.HSCurve
	// V are the vertices at the edge's start and finish, along the
	// curve's points.
	V [2]int
	// CoEdges are the uses of the edge by loops; two in a sound shell.
	CoEdges []int
	// Points are the curve's points from V[0] to V[1].
	Points []geom.Vector
	// Seam is set when the edge joins two patches of one sheet.
	Seam bool
}

// CoEdge is an edge as a loop runs along it.
type CoEdge struct {
	Edge int
	Loop int
	// Reversed is set when the loop runs from the edge's V[1] to V[0].
	Reversed bool
	// Surface is the surface whose trim this is, and Trim its index.
	Surface core.HSSurface
	Trim    int
	// Curve is the trim's curve. The two co-edges of an edge may trim
	// by two curves, which lie on each other: a Boolean keeps the curve
	// each side had.
	Curve core.HSCurve
}

// Loop is a closed chain of co-edges bounding a face.
type Loop struct {
	Face    int
	CoEdges []int
	// Outer is set for a face's outer boundary, clear for a hole.
	Outer bool
}

// Face is a connected region of one surface: an outer loop and its
// holes. It is a patch of a sheet, which may have others.
type Face struct {
	Surface core.HSSurface
	// ID is the face's identity as the shell gives it: the entity of the
	// sketch that made it (srf.SSurface.Face), or for a surface no
	// entity names, the group that made it and its place among that
	// group's surfaces (srf.SSurface.Origin).
	ID    srf.FaceID
	Loops []int
	// Sheet is the sheet the face is a patch of.
	Sheet int
}

// faceID is a surface's identity for naming.
func faceID(s *srf.SSurface) srf.FaceID {
	if s.Face != 0 {
		return s.Face
	}
	return srf.FaceID(s.Origin)
}

// Start is the vertex the loop leaves the co-edge from.
func (t *Topology) Start(ce int) int {
	c := &t.CoEdges[ce]
	if c.Reversed {
		return t.Edges[c.Edge].V[1]
	}
	return t.Edges[c.Edge].V[0]
}

// End is the vertex the loop reaches at the end of the co-edge.
func (t *Topology) End(ce int) int {
	c := &t.CoEdges[ce]
	if c.Reversed {
		return t.Edges[c.Edge].V[0]
	}
	return t.Edges[c.Edge].V[1]
}

// Shell is the shell the topology was built from.
func (t *Topology) Shell() *srf.SShell { return t.shell }

// Surface is the surface of a face.
func (t *Topology) Surface(face int) *srf.SSurface {
	return t.shell.FindSurface(t.Faces[face].Surface)
}

// vertexKey quantises a point for finding the vertex at it.
type vertexKey [3]int64

const vertexEps = geom.LengthEps

// vertexAt is the vertex at p, made if there is none within
// LengthEps.
func (t *Topology) vertexAt(p geom.Vector, index map[vertexKey][]int) int {
	q := func(x float64) int64 { return int64(math.Floor(x / (4 * vertexEps))) }
	k := vertexKey{q(p.X), q(p.Y), q(p.Z)}
	for dx := int64(-1); dx <= 1; dx++ {
		for dy := int64(-1); dy <= 1; dy++ {
			for dz := int64(-1); dz <= 1; dz++ {
				for _, v := range index[vertexKey{k[0] + dx, k[1] + dy, k[2] + dz}] {
					if t.Vertices[v].P.EqualsTol(p, vertexEps) {
						return v
					}
				}
			}
		}
	}
	t.Vertices = append(t.Vertices, Vertex{P: p})
	index[k] = append(index[k], len(t.Vertices)-1)
	return len(t.Vertices) - 1
}

// edgeKey names the edges between two vertices, the lower first.
type edgeKey struct{ a, b int }

// edgeFor is the edge between vertices a and b along pts (from a to
// b), made if no edge between them lies along the same points: two
// edges can join the same two vertices, as the halves of a circle do.
func (t *Topology) edgeFor(edges map[edgeKey][]int, a, b int, pts []geom.Vector, curve core.HSCurve) int {
	k := edgeKey{min(a, b), max(a, b)}
	mid := midpoint(pts)
	for _, e := range edges[k] {
		if distanceToPolyline(mid, t.Edges[e].Points) <= edgeTol(pts) {
			return e
		}
	}
	e := len(t.Edges)
	edges[k] = append(edges[k], e)
	t.Edges = append(t.Edges, Edge{Curve: curve, V: [2]int{a, b}, Points: pts})
	t.Vertices[a].Edges = append(t.Vertices[a].Edges, e)
	if b != a {
		t.Vertices[b].Edges = append(t.Vertices[b].Edges, e)
	}
	return e
}

// edgeTol is how far apart two polylines of one edge may be: their
// curves are sampled each to the chord tolerance.
func edgeTol(pts []geom.Vector) float64 {
	l := 0.0
	for i := 1; i < len(pts); i++ {
		l += pts[i].Minus(pts[i-1]).Magnitude()
	}
	return 1e-2*l + 10*geom.LengthEps
}

// midpoint is the point halfway along a polyline.
func midpoint(pts []geom.Vector) geom.Vector {
	l := 0.0
	for i := 1; i < len(pts); i++ {
		l += pts[i].Minus(pts[i-1]).Magnitude()
	}
	half := l / 2
	for i := 1; i < len(pts); i++ {
		d := pts[i].Minus(pts[i-1]).Magnitude()
		if d >= half && d > 0 {
			return pts[i-1].Plus(pts[i].Minus(pts[i-1]).ScaledBy(half / d))
		}
		half -= d
	}
	return pts[len(pts)-1]
}

// distanceToPolyline is how far p is from the nearest segment.
func distanceToPolyline(p geom.Vector, pts []geom.Vector) float64 {
	best := math.Inf(1)
	for i := 1; i < len(pts); i++ {
		a, b := pts[i-1], pts[i]
		ab := b.Minus(a)
		tt := 0.0
		if m := ab.MagSquared(); m > 0 {
			tt = math.Max(0, math.Min(1, p.Minus(a).Dot(ab)/m))
		}
		best = math.Min(best, p.Minus(a.Plus(ab.ScaledBy(tt))).Magnitude())
	}
	if len(pts) == 1 {
		best = p.Minus(pts[0]).Magnitude()
	}
	return best
}

// Build derives the topology of a shell. A shell whose trims don't
// chain into loops gives an error, with what topology could be made.
func Build(sh *srf.SShell) (*Topology, error) {
	t := &Topology{shell: sh, surfFace: map[core.HSSurface][]int{}}
	index := map[vertexKey][]int{}
	edges := map[edgeKey][]int{}
	var problems []string

	// First the trims, with the vertices at their ends.
	type piece struct {
		surf  int
		trim  int
		curve core.HSCurve
		pts   []geom.Vector
	}
	var pieces []piece
	for si := range sh.Surface {
		s := &sh.Surface[si]
		for ti := range s.Trim {
			stb := &s.Trim[ti]
			sc := sh.FindCurve(stb.Curve)
			if sc == nil {
				problems = append(problems, fmt.Sprintf("surface %d trims by a curve that isn't there", s.H.V))
				continue
			}
			pts := curvePoints(sc, stb)
			a := t.vertexAt(stb.StartP, index)
			b := t.vertexAt(stb.FinishP, index)
			if len(pts) < 2 || a == b && len(pts) < 3 {
				// A trim that starts where it finishes: a closed curve,
				// which SolveSpace splits, or a sliver; no edge.
				continue
			}
			pts[0], pts[len(pts)-1] = t.Vertices[a].P, t.Vertices[b].P
			pieces = append(pieces, piece{surf: si, trim: ti, curve: stb.Curve, pts: pts})
		}
	}
	// A Boolean can leave a trim running past a vertex where two faces
	// meet on its other side: split it there, so each piece of it
	// meets one face.
	cuts := t.vertexCuts()
	var ces [][]int
	for range sh.Surface {
		ces = append(ces, nil)
	}
	for _, pc := range pieces {
		for _, sub := range cuts(pc.pts) {
			a := t.vertexAt(sub[0], index)
			b := t.vertexAt(sub[len(sub)-1], index)
			e := t.edgeFor(edges, a, b, sub, pc.curve)
			ce := len(t.CoEdges)
			t.CoEdges = append(t.CoEdges, CoEdge{Edge: e, Loop: -1, Reversed: t.Edges[e].V[0] != a,
				Surface: sh.Surface[pc.surf].H, Trim: pc.trim, Curve: pc.curve})
			t.Edges[e].CoEdges = append(t.Edges[e].CoEdges, ce)
			ces[pc.surf] = append(ces[pc.surf], ce)
		}
	}
	for si := range sh.Surface {
		s := &sh.Surface[si]
		loops, err := t.chain(ces[si])
		if err != nil {
			problems = append(problems, fmt.Sprintf("surface %d: %v", s.H.V, err))
		}
		t.makeFaces(s, loops)
	}
	t.findSheets()
	t.nameEdges()
	if len(problems) > 0 {
		return t, fmt.Errorf("topo: %s", problems[0])
	}
	return t, nil
}

// vertexCuts is a function that splits a polyline at the vertices that
// lie inside it, giving the pieces from end to end.
func (t *Topology) vertexCuts() func(pts []geom.Vector) [][]geom.Vector {
	// The vertices by cell, to find those near a polyline.
	const cell = 1.0
	key := func(p geom.Vector) vertexKey {
		return vertexKey{int64(math.Floor(p.X / cell)), int64(math.Floor(p.Y / cell)), int64(math.Floor(p.Z / cell))}
	}
	cells := map[vertexKey][]int{}
	for v := range t.Vertices {
		k := key(t.Vertices[v].P)
		cells[k] = append(cells[k], v)
	}
	const tol = 10 * vertexEps
	return func(pts []geom.Vector) [][]geom.Vector {
		type cut struct {
			seg int
			at  float64
			p   geom.Vector
		}
		var cs []cut
		seen := map[int]bool{}
		for i := 1; i < len(pts); i++ {
			a, b := pts[i-1], pts[i]
			lo := key(geom.Vector{X: math.Min(a.X, b.X) - tol, Y: math.Min(a.Y, b.Y) - tol, Z: math.Min(a.Z, b.Z) - tol})
			hi := key(geom.Vector{X: math.Max(a.X, b.X) + tol, Y: math.Max(a.Y, b.Y) + tol, Z: math.Max(a.Z, b.Z) + tol})
			if (hi[0]-lo[0]+1)*(hi[1]-lo[1]+1)*(hi[2]-lo[2]+1) > 4096 {
				// A long segment: look at every vertex.
				lo, hi = vertexKey{math.MinInt64 / 4}, vertexKey{}
			}
			try := func(v int) {
				p := t.Vertices[v].P
				if seen[v] || p.EqualsTol(pts[0], vertexEps) || p.EqualsTol(pts[len(pts)-1], vertexEps) {
					return
				}
				ab := b.Minus(a)
				l := ab.MagSquared()
				if l == 0 {
					return
				}
				u := p.Minus(a).Dot(ab) / l
				if u <= 0 || u >= 1 {
					if !p.EqualsTol(b, vertexEps) || i == len(pts)-1 {
						return
					}
					u = 1
				}
				if p.Minus(a.Plus(ab.ScaledBy(u))).Magnitude() > tol {
					return
				}
				seen[v] = true
				cs = append(cs, cut{seg: i, at: u, p: p})
			}
			if lo[0] == math.MinInt64/4 {
				for v := range t.Vertices {
					try(v)
				}
				continue
			}
			for x := lo[0]; x <= hi[0]; x++ {
				for y := lo[1]; y <= hi[1]; y++ {
					for z := lo[2]; z <= hi[2]; z++ {
						for _, v := range cells[vertexKey{x, y, z}] {
							try(v)
						}
					}
				}
			}
		}
		if len(cs) == 0 {
			return [][]geom.Vector{pts}
		}
		sort.Slice(cs, func(i, j int) bool {
			if cs[i].seg != cs[j].seg {
				return cs[i].seg < cs[j].seg
			}
			return cs[i].at < cs[j].at
		})
		var out [][]geom.Vector
		cur := []geom.Vector{pts[0]}
		k := 0
		for i := 1; i < len(pts); i++ {
			for k < len(cs) && cs[k].seg == i {
				cur = append(cur, cs[k].p)
				out = append(out, cur)
				cur = []geom.Vector{cs[k].p}
				k++
			}
			if !pts[i].EqualsTol(cur[len(cur)-1], vertexEps) {
				cur = append(cur, pts[i])
			}
		}
		if len(cur) >= 2 {
			out = append(out, cur)
		} else if len(out) > 0 {
			// The last cut was at the end itself.
			last := out[len(out)-1]
			last[len(last)-1] = pts[len(pts)-1]
		}
		return out
	}
}

// curvePoints are the points of a trim, from its start to its finish
// as the trim runs.
func curvePoints(sc *srf.SCurve, stb *srf.STrimBy) []geom.Vector {
	var out []geom.Vector
	n := len(sc.Points)
	in := false
	for k := 0; k < n; k++ {
		i := k
		if stb.Backwards {
			i = n - 1 - k
		}
		p := sc.Points[i].P
		if !in && p.Equals(stb.StartP) {
			in = true
		}
		if in {
			out = append(out, p)
			if p.Equals(stb.FinishP) && len(out) > 1 {
				break
			}
		}
	}
	return out
}

func reversed(pts []geom.Vector) []geom.Vector {
	out := make([]geom.Vector, len(pts))
	for i := range pts {
		out[i] = pts[len(pts)-1-i]
	}
	return out
}

// chain joins a surface's co-edges into loops, each co-edge followed by
// one that starts where it ends. Where several do, as at a vertex the
// loop passes twice, the one that turns most sharply left is taken,
// which keeps loops from crossing.
func (t *Topology) chain(ces []int) ([][]int, error) {
	byStart := map[int][]int{}
	for _, ce := range ces {
		byStart[t.Start(ce)] = append(byStart[t.Start(ce)], ce)
	}
	used := map[int]bool{}
	var loops [][]int
	var err error
	for _, first := range ces {
		if used[first] {
			continue
		}
		loop := []int{first}
		used[first] = true
		cur := first
		for {
			end := t.End(cur)
			if end == t.Start(first) && len(byStartUnused(byStart[end], used)) == 0 {
				break
			}
			cands := byStartUnused(byStart[end], used)
			if len(cands) == 0 {
				if end != t.Start(first) {
					err = fmt.Errorf("a loop doesn't close at %v", t.Vertices[end].P)
				}
				break
			}
			next := cands[0]
			if len(cands) > 1 {
				next = t.sharpestLeft(cur, cands)
			}
			used[next] = true
			loop = append(loop, next)
			cur = next
			if t.End(cur) == t.Start(first) {
				break
			}
		}
		loops = append(loops, loop)
	}
	return loops, err
}

func byStartUnused(cands []int, used map[int]bool) []int {
	var out []int
	for _, c := range cands {
		if !used[c] {
			out = append(out, c)
		}
	}
	return out
}

// direction is the way a co-edge leaves its start (out) or arrives at
// its end.
func (t *Topology) direction(ce int, out bool) geom.Vector {
	pts := t.Edges[t.CoEdges[ce].Edge].Points
	if t.CoEdges[ce].Reversed {
		pts = reversed(pts)
	}
	if len(pts) < 2 {
		return geom.Vector{}
	}
	if out {
		return pts[1].Minus(pts[0])
	}
	return pts[len(pts)-1].Minus(pts[len(pts)-2])
}

// sharpestLeft picks, of the co-edges leaving the vertex where cur
// ends, the one that turns most to the left about the surface's normal
// there.
func (t *Topology) sharpestLeft(cur int, cands []int) int {
	s := t.shell.FindSurface(t.CoEdges[cur].Surface)
	p := t.Vertices[t.End(cur)].P
	var n geom.Vector
	if s != nil {
		var u, v float64
		s.ClosestPointToUV(p, &u, &v, true)
		n = s.NormalAt(u, v)
	}
	in := t.direction(cur, false)
	best, bestA := cands[0], math.Inf(1)
	for _, c := range cands {
		out := t.direction(c, true)
		// The angle turned from in to out, left positive, in (-pi, pi].
		a := math.Atan2(in.Cross(out).Dot(n.WithMagnitude(1)), in.Dot(out))
		// Most to the left is the largest turn; take the smallest
		// complement.
		if r := math.Pi - a; r < bestA {
			best, bestA = c, r
		}
	}
	return best
}

// makeFaces sorts a surface's loops into faces: each outer loop, and
// the holes that lie inside it. A loop is a hole when it runs clockwise
// about the surface's normal, as SolveSpace orients its trims (its
// outer loops run clockwise in the surface's parameters).
func (t *Topology) makeFaces(s *srf.SSurface, loops [][]int) {
	type info struct {
		ces   []int
		area  float64
		uv    []geom.Vector
		outer bool
	}
	infos := make([]info, len(loops))
	for i, l := range loops {
		uv := t.loopUV(s, l)
		a := 0.0
		for k := range uv {
			p, q := uv[k], uv[(k+1)%len(uv)]
			a += (p.X*q.Y - q.X*p.Y) / 2
		}
		infos[i] = info{ces: l, area: -a, uv: uv, outer: a < 0}
	}
	// Outer loops, largest first, each taking the holes inside it.
	var outers []int
	for i := range infos {
		if infos[i].outer {
			outers = append(outers, i)
		}
	}
	sort.SliceStable(outers, func(a, b int) bool { return infos[outers[a]].area > infos[outers[b]].area })
	owner := make([]int, len(infos))
	for i := range infos {
		owner[i] = -1
		if infos[i].outer {
			continue
		}
		// The smallest outer loop that holds a point of the hole.
		for k := len(outers) - 1; k >= 0; k-- {
			o := outers[k]
			if len(infos[i].uv) > 0 && pointInPolygon(infos[i].uv[0], infos[o].uv) {
				owner[i] = o
				break
			}
		}
		if owner[i] < 0 && len(outers) > 0 {
			owner[i] = outers[0]
		}
	}
	faceOf := map[int]int{}
	for _, o := range outers {
		f := len(t.Faces)
		faceOf[o] = f
		t.Faces = append(t.Faces, Face{Surface: s.H, ID: faceID(s)})
		t.surfFace[s.H] = append(t.surfFace[s.H], f)
	}
	if len(outers) == 0 && len(infos) > 0 {
		// Only holes: a surface whose loops all run the other way.
		f := len(t.Faces)
		t.Faces = append(t.Faces, Face{Surface: s.H, ID: faceID(s)})
		t.surfFace[s.H] = append(t.surfFace[s.H], f)
		for i := range infos {
			owner[i] = -2
		}
		faceOf[-2] = f
	}
	for i := range infos {
		key := i
		if !infos[i].outer {
			key = owner[i]
		}
		f, ok := faceOf[key]
		if !ok {
			continue
		}
		l := len(t.Loops)
		t.Loops = append(t.Loops, Loop{Face: f, CoEdges: infos[i].ces, Outer: infos[i].outer})
		t.Faces[f].Loops = append(t.Faces[f].Loops, l)
		for _, ce := range infos[i].ces {
			t.CoEdges[ce].Loop = l
		}
	}
	// The outer loop first in each face.
	for _, o := range outers {
		f := &t.Faces[faceOf[o]]
		sort.SliceStable(f.Loops, func(a, b int) bool { return t.Loops[f.Loops[a]].Outer && !t.Loops[f.Loops[b]].Outer })
	}
}

// loopUV is a loop's points in the surface's parameters.
func (t *Topology) loopUV(s *srf.SSurface, loop []int) []geom.Vector {
	var out []geom.Vector
	for _, ce := range loop {
		pts := t.Edges[t.CoEdges[ce].Edge].Points
		if t.CoEdges[ce].Reversed {
			pts = reversed(pts)
		}
		for k := 0; k+1 < len(pts); k++ {
			var u, v float64
			s.ClosestPointToUV(pts[k], &u, &v, true)
			out = append(out, geom.Vector{X: u, Y: v})
		}
	}
	return out
}

func pointInPolygon(p geom.Vector, poly []geom.Vector) bool {
	in := false
	for i := range poly {
		a, b := poly[i], poly[(i+1)%len(poly)]
		if (a.Y > p.Y) != (b.Y > p.Y) {
			x := a.X + (p.Y-a.Y)*(b.X-a.X)/(b.Y-a.Y)
			if x > p.X {
				in = !in
			}
		}
	}
	return in
}
