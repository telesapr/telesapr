package topo

import (
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A face of the solid as a person sees it may be made of several
// patches: an extruded circle's side is four, one per quarter of the
// circle, and a B-spline surface is one per knot span. Faces in a
// Topology are patches (one surface's region each); a Sheet is the
// face they make together. Patches belong to one sheet when they carry
// one identity and meet smoothly: the edge between two of them is a
// seam, which is not shown, picked or named as an edge of the solid.
// The edges on either side of a vertex that only seams touch are one
// edge of the solid, a Chain, as the four quarters of a circle are.

// Sheet is a face made of patches joined along seams. Its ID is its
// first patch's; the others may have identities of their own, which
// name the edges they bound.
type Sheet struct {
	ID    srf.FaceID
	Faces []int
}

// seamCos is how nearly the two patches' normals must agree along an
// edge for it to be a seam: within about a tenth of a degree.
const seamCos = 1 - 1e-6

// findSheets marks the seams and groups the faces into sheets.
func (t *Topology) findSheets() {
	parent := make([]int, len(t.Faces))
	for i := range parent {
		parent[i] = i
	}
	find := func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	for e := range t.Edges {
		if t.isSeam(e) {
			t.Edges[e].Seam = true
			fs := t.EdgeFaces(e)
			parent[find(fs[0])] = find(fs[1])
		}
	}
	index := map[int]int{}
	for f := range t.Faces {
		r := find(f)
		s, ok := index[r]
		if !ok {
			s = len(t.Sheets)
			index[r] = s
			t.Sheets = append(t.Sheets, Sheet{ID: t.Faces[f].ID})
		}
		t.Faces[f].Sheet = s
		t.Sheets[s].Faces = append(t.Sheets[s].Faces, f)
	}
}

// isSeam reports whether edge e joins two patches of one face: two
// faces swept from one sketch curve (srf.SSurface.Sheet), or of one
// identity, whose normals agree along it.
func (t *Topology) isSeam(e int) bool {
	ed := &t.Edges[e]
	if len(ed.CoEdges) != 2 {
		return false
	}
	fs := t.EdgeFaces(e)
	if fs[0] < 0 || fs[1] < 0 || fs[0] == fs[1] {
		return false
	}
	a, b := t.Surface(fs[0]), t.Surface(fs[1])
	if a == nil || b == nil {
		return false
	}
	same := a.Sheet != 0 && a.Sheet == b.Sheet
	if id := t.Faces[fs[0]].ID; id != 0 && id == t.Faces[fs[1]].ID {
		same = true
	}
	if !same {
		return false
	}
	for _, p := range edgeSamples(ed.Points) {
		var ua, va, ub, vb float64
		a.ClosestPointToUV(p, &ua, &va, true)
		b.ClosestPointToUV(p, &ub, &vb, true)
		na, nb := a.NormalAt(ua, va), b.NormalAt(ub, vb)
		if na.Magnitude() == 0 || nb.Magnitude() == 0 {
			continue
		}
		if na.WithMagnitude(1).Dot(nb.WithMagnitude(1)) < seamCos {
			return false
		}
	}
	return true
}

// edgeSamples are a few points along an edge: its ends and its middle.
func edgeSamples(pts []geom.Vector) []geom.Vector {
	if len(pts) == 0 {
		return nil
	}
	return []geom.Vector{pts[0], midpoint(pts), pts[len(pts)-1]}
}

// SheetsWithID are the sheets with a patch that carries identity id,
// in the order of their first patches.
func (t *Topology) SheetsWithID(id srf.FaceID) []int {
	var out []int
	for s := range t.Sheets {
		for _, f := range t.Sheets[s].Faces {
			if t.Faces[f].ID == id {
				out = append(out, s)
				break
			}
		}
	}
	return out
}

// SheetOf is the sheet of a face (a patch).
func (t *Topology) SheetOf(f int) int { return t.Faces[f].Sheet }

// EdgeSheets are the sheets on the two sides of an edge, -1 where
// there is none.
func (t *Topology) EdgeSheets(e int) [2]int {
	out := [2]int{-1, -1}
	for i, f := range t.EdgeFaces(e) {
		if f >= 0 {
			out[i] = t.Faces[f].Sheet
		}
	}
	return out
}

// sameSheets reports whether edges a and b lie between the same two
// sheets.
func (t *Topology) sameSheets(a, b int) bool {
	sa, sb := t.EdgeSheets(a), t.EdgeSheets(b)
	return sa == sb || sa == [2]int{sb[1], sb[0]}
}

// through is the edge that goes on from e at vertex v as one edge of
// the solid: the only other edge there that is not a seam, between the
// same sheets; -1 when there is none, and so v is a vertex of the
// solid.
func (t *Topology) through(e, v int) int {
	other := -1
	for _, x := range t.Vertices[v].Edges {
		if x == e || t.Edges[x].Seam {
			continue
		}
		if other >= 0 {
			return -1
		}
		other = x
	}
	if other < 0 || t.Edges[other].V[0] == t.Edges[other].V[1] || !t.sameSheets(e, other) {
		return -1
	}
	return other
}

// Chain is the edge of the solid that edge e is a piece of: e and the
// edges it goes on into across vertices that only seams touch, in
// order from one end to the other, each with whether it runs reversed
// along the chain. A seam is a chain of its own.
func (t *Topology) Chain(e int) []ChainPiece {
	if t.Edges[e].Seam {
		return []ChainPiece{{Edge: e}}
	}
	seen := map[int]bool{e: true}
	// Back from e's start, then on from its end.
	var back []ChainPiece
	for cur, v := e, t.Edges[e].V[0]; ; {
		nx := t.through(cur, v)
		if nx < 0 || seen[nx] {
			break
		}
		seen[nx] = true
		// nx runs into v along the chain when its finish is v.
		rev := t.Edges[nx].V[0] == v
		back = append(back, ChainPiece{Edge: nx, Reversed: rev})
		cur = nx
		if rev {
			v = t.Edges[nx].V[1]
		} else {
			v = t.Edges[nx].V[0]
		}
	}
	out := make([]ChainPiece, 0, len(back)+1)
	for _, b := range slices.Backward(back) {
		out = append(out, b)
	}
	out = append(out, ChainPiece{Edge: e})
	for cur, v := e, t.Edges[e].V[1]; ; {
		nx := t.through(cur, v)
		if nx < 0 || seen[nx] {
			break
		}
		seen[nx] = true
		rev := t.Edges[nx].V[1] == v
		out = append(out, ChainPiece{Edge: nx, Reversed: rev})
		cur = nx
		if rev {
			v = t.Edges[nx].V[0]
		} else {
			v = t.Edges[nx].V[1]
		}
	}
	return out
}

// ChainPiece is an edge in a chain, and whether the chain runs along
// it from its V[1] to its V[0].
type ChainPiece struct {
	Edge     int
	Reversed bool
}

// Closed reports whether a chain comes back to where it starts, as a
// circle's does.
func (t *Topology) Closed(chain []ChainPiece) bool {
	first, last := chain[0], chain[len(chain)-1]
	start := t.Edges[first.Edge].V[0]
	if first.Reversed {
		start = t.Edges[first.Edge].V[1]
	}
	end := t.Edges[last.Edge].V[1]
	if last.Reversed {
		end = t.Edges[last.Edge].V[0]
	}
	return start == end
}

// ChainPoints are the points along a chain from one end to the other.
func (t *Topology) ChainPoints(chain []ChainPiece) []geom.Vector {
	var out []geom.Vector
	for _, c := range chain {
		pts := t.Edges[c.Edge].Points
		if c.Reversed {
			pts = reversed(pts)
		}
		if len(out) > 0 && len(pts) > 0 {
			pts = pts[1:]
		}
		out = append(out, pts...)
	}
	return out
}

// Lead is the piece of e's chain that names it: the one whose name is
// lowest, or e itself when no piece has a name.
func (t *Topology) Lead(e int) int {
	best, bestN := e, -1
	var bestName Name
	for _, c := range t.Chain(e) {
		n, err := t.NameOf(c.Edge)
		if err != nil {
			continue
		}
		if bestN < 0 || n.Less(bestName) {
			best, bestN, bestName = c.Edge, n.N, n
		}
	}
	return best
}

// SolidEdges are the edges of the solid: the lead piece of every chain
// that is not a seam, in the order of the pieces.
func (t *Topology) SolidEdges() []int {
	var out []int
	for e := range t.Edges {
		if !t.Edges[e].Seam && t.Lead(e) == e {
			out = append(out, e)
		}
	}
	return out
}

// SolidVertex reports whether a vertex is a vertex of the solid, and
// not where a seam meets a chain or other seams.
func (t *Topology) SolidVertex(v int) bool {
	var es []int
	for _, x := range t.Vertices[v].Edges {
		if !t.Edges[x].Seam {
			es = append(es, x)
		}
	}
	switch len(es) {
	case 0:
		return false
	case 2:
		return t.through(es[0], v) != es[1]
	}
	return true
}

// SheetEdges are the lead pieces of the edges round a sheet, each
// once, in the order its patches' loops meet them.
func (t *Topology) SheetEdges(s int) []int {
	var out []int
	seen := map[int]bool{}
	for _, f := range t.Sheets[s].Faces {
		for _, e := range t.FaceEdges(f) {
			if t.Edges[e].Seam {
				continue
			}
			if l := t.Lead(e); !seen[l] {
				seen[l] = true
				out = append(out, l)
			}
		}
	}
	return out
}

// SheetNeighbours are the sheets that share an edge with sheet s.
func (t *Topology) SheetNeighbours(s int) []int {
	var out []int
	seen := map[int]bool{s: true}
	for _, f := range t.Sheets[s].Faces {
		for _, g := range t.Neighbours(f) {
			if sg := t.Faces[g].Sheet; !seen[sg] {
				seen[sg] = true
				out = append(out, sg)
			}
		}
	}
	slices.Sort(out)
	return out
}
