package topo

import (
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// rbPole is a pole of a face along the edge, where the seams of its
// patches meet (a cone's apex, a sphere's top), that lies in the strip a
// fillet takes: the ball's point crosses seams from it, and its other
// seams run from it to the edge.
type rbPole struct {
	// v is the pole's vertex, and to the vertices on the edge its other
	// seams run to.
	v  int
	to []int
	// swallowed are the patches those seams bound: those the ball's point
	// never reaches go whole round the pole.
	swallowed []int
}

// seamPole is the pole the seam crossed by c at q starts from, when the
// seam doesn't reach the edge: a vertex of it each of whose other edges
// is a seam between faces of c's side that the ball's point crosses too
// (one of cross), or that runs to the edge (reaches), not along it. Of
// two such seams, the one nearest q.
func (rb *rollingBall) seamPole(c rbCrossing, cross []rbCrossing, q geom.Vector, isChain map[int]bool, reaches func(int) bool) (rbPole, bool) {
	t := rb.t
	pl := poleLook{
		onSide:  func(f int) bool { return slices.Contains(rb.side[c.side], f) },
		isChain: isChain,
		reaches: reaches,
		crossed: func(g [2]int) bool {
			return slices.ContainsFunc(cross, func(o rbCrossing) bool { return o.side == c.side && sameFaces(g, o.faces) })
		},
	}
	var best rbPole
	near := math.Inf(1)
	for y := range t.Edges {
		if isChain[y] || !sameFaces(t.EdgeFaces(y), c.faces) {
			continue
		}
		d := distanceToPolyline(q, t.Edges[y].Points)
		if d >= near {
			continue
		}
		for _, a := range t.Edges[y].V {
			if p, ok := pl.at(t, a, y); ok {
				best, near = p, d
			}
		}
	}
	return best, !math.IsInf(near, 1)
}

// atSeamEnd reports whether q lies within d of an end of a seam between
// faces fs: the ball's point crossing there passes through the vertex
// where the seams meet, crossing no one seam.
func (rb *rollingBall) atSeamEnd(fs [2]int, q geom.Vector, d float64) bool {
	t := rb.t
	for y := range t.Edges {
		if !sameFaces(t.EdgeFaces(y), fs) {
			continue
		}
		for _, a := range t.Edges[y].V {
			if t.Vertices[a].P.Minus(q).Magnitude() < d {
				return true
			}
		}
	}
	return false
}

// sameFaces reports whether g and fs are the same two faces.
func sameFaces(g, fs [2]int) bool { return g == fs || g == [2]int{fs[1], fs[0]} }

// poleLook is what seamPole asks of a pole's edges.
type poleLook struct {
	onSide  func(f int) bool
	isChain map[int]bool
	reaches func(v int) bool
	crossed func(faces [2]int) bool
}

// at is vertex a as a pole whose seam y is crossed: every other edge at a
// a seam between the side's faces, crossed too or running to the edge.
func (pl poleLook) at(t *Topology, a, y int) (rbPole, bool) {
	if pl.reaches(a) {
		return rbPole{}, false
	}
	p := rbPole{v: a}
	for _, e := range t.Vertices[a].Edges {
		if e == y {
			continue
		}
		ed := &t.Edges[e]
		g := t.EdgeFaces(e)
		if pl.isChain[e] || !pl.onSide(g[0]) || !pl.onSide(g[1]) {
			return rbPole{}, false
		}
		w := ed.V[0]
		if w == a {
			w = ed.V[1]
		}
		if w == a {
			return rbPole{}, false
		}
		if !pl.reaches(w) {
			if !pl.crossed(g) {
				return rbPole{}, false
			}
			// Cut at its own crossing.
			continue
		}
		p.to = append(p.to, w)
		for _, f := range g {
			if !slices.Contains(p.swallowed, f) {
				p.swallowed = append(p.swallowed, f)
			}
		}
	}
	return p, len(p.to) > 0
}

// poleReach is how near the chain, in radii, a pole of a face along it
// lies for the patches round it to be taken among its side's faces.
const poleReach = 4

// addPoleFaces takes among each side's faces the patches round a pole of
// one of them near the chain (a sphere's top, a cone's apex): the ball's
// point, passing over the pole, goes on to a patch the edge never
// touches.
func (rb *rollingBall) addPoleFaces() {
	t := rb.t
	onChain := map[int]bool{}
	for _, v := range rb.verts {
		onChain[v] = true
	}
	for k := range 2 {
		for v := range t.Vertices {
			if onChain[v] || len(t.Vertices[v].Edges) < 3 {
				continue
			}
			p := t.Vertices[v].P
			var round []int
			pole := false
			for _, e := range t.Vertices[v].Edges {
				for _, f := range t.EdgeFaces(e) {
					if !slices.Contains(round, f) {
						round = append(round, f)
					}
					if slices.Contains(rb.side[k], f) && collapsedAt(t.Surface(f), p) {
						pole = true
					}
				}
			}
			if !pole || len(round) < 3 || !rb.nearChain(p, poleReach*rb.r) {
				continue
			}
			for _, f := range round {
				// Only the patches of the pole's own sheet, each collapsed
				// there too, and none on the other side.
				if !collapsedAt(t.Surface(f), p) || slices.Contains(rb.side[1-k], f) {
					continue
				}
				if !slices.Contains(rb.side[k], f) {
					rb.side[k] = append(rb.side[k], f)
					if rb.poleOf == nil {
						rb.poleOf = map[int]geom.Vector{}
					}
					rb.poleOf[f] = p
				}
			}
		}
	}
}

// poleForwards are, for the faces of side k taken in round a pole (which
// have no trim along the edge), forward: that of a face of the side
// along the edge whose patch is collapsed at the same pole, the patches
// of one sheet all facing the same way. Taken before any face is cut
// back, which drops the trims forward reads.
func (rb *rollingBall) poleForwards(sh *srf.SShell, k int, chainCurve map[core.HSCurve]bool) map[int]bool {
	var out map[int]bool
	for f, p := range rb.poleOf {
		if !slices.Contains(rb.side[k], f) {
			continue
		}
		for _, g := range rb.side[k] {
			if _, taken := rb.poleOf[g]; taken {
				continue
			}
			s := sh.FindSurface(rb.t.Faces[g].Surface)
			if s == nil || !collapsedAt(s, p) {
				continue
			}
			if fwd, found := rb.forward(sh, s, chainCurve); found {
				if out == nil {
					out = map[int]bool{}
				}
				out[f] = fwd
				break
			}
		}
	}
	return out
}

// nearChain reports whether p lies within d of the chain's edges.
func (rb *rollingBall) nearChain(p geom.Vector, d float64) bool {
	for _, pc := range rb.pieces {
		if distanceToPolyline(p, rb.t.Edges[pc.Edge].Points) <= d {
			return true
		}
	}
	return false
}

// collapsedAt reports whether a side of patch s is the one point p.
func collapsedAt(s *srf.SSurface, p geom.Vector) bool {
	if s == nil {
		return false
	}
	at := func(i, j int) bool { return s.Ctrl[i][j].Equals(p) }
	row := func(i int) bool {
		for j := 0; j <= s.DegN; j++ {
			if !at(i, j) {
				return false
			}
		}
		return true
	}
	col := func(j int) bool {
		for i := 0; i <= s.DegM; i++ {
			if !at(i, j) {
				return false
			}
		}
		return true
	}
	return row(0) || row(s.DegM) || col(0) || col(s.DegN)
}

// trimsClose reports whether trims make closed loops: each one's finish
// some other's start.
func trimsClose(trims []srf.STrimBy) bool {
	used := make([]bool, len(trims))
	for _, a := range trims {
		found := false
		for j, b := range trims {
			if !used[j] && a.FinishP.Equals(b.StartP) {
				used[j], found = true, true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
