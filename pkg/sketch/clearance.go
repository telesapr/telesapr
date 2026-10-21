package sketch

import (
	"cmp"
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// The clearance between parts (item 14 of doc/notes/modelling_upgrades.md).
// The parts are the interference check's (assemblyParts). Two parts'
// gap is first the least distance between their meshes, which is
// within the two chord tolerances of the true one: each mesh lies
// within its chord tolerance of its surfaces. Where both parts have
// shells, the nearest points are then followed onto the two surfaces
// the triangles there stand for, and kept where they stay on the
// parts, near their meshes; the gap is then between points of the
// solids, and the tolerance says how far the true one may still be
// below it.

// Clearance is the gap between two parts: the groups that made them,
// the least distance between them, 0 where they touch or overlap, and
// the nearest points of each.
type Clearance struct {
	A, B   core.HGroup
	Gap    float64
	PA, PB geom.Vector
	// Tolerance is how far the true gap may be from Gap: the chord
	// tolerances of the two meshes it was found on, more where the
	// points followed onto the surfaces moved it.
	Tolerance float64
	// Refined is set when PA and PB were followed onto the parts'
	// surfaces, so that they lie on the solids rather than on their
	// meshes.
	Refined bool
}

// clearancePart is a part's mesh, its tree, and its shell and the
// surface of each triangle, where it has one.
type clearancePart struct {
	g     core.HGroup
	shell *srf.SShell
	mesh  srf.SMesh
	tree  *srf.MeshTree
	chord float64
}

// newClearancePart is the part of group hg with shell sh, its triangles
// tagged by their surfaces.
func newClearancePart(hg core.HGroup, sh *srf.SShell) *clearancePart {
	p := &clearancePart{g: hg, shell: sh, chord: sh.Tol.ChordTolMm()}
	for i, m := range sh.TriangulateBySurface() {
		for _, tr := range m.Triangles {
			tr.Tag = i
			p.mesh.Triangles = append(p.mesh.Triangles, tr)
		}
	}
	p.tree = srf.NewMeshTree(&p.mesh)
	return p
}

// clearanceParts are the parts of the model up to group upTo, with
// shells or as meshes: a part with no shell is taken by its triangles,
// which are its geometry, so its chord tolerance is 0.
func (sk *Sketch) clearanceParts(upTo core.HGroup) []*clearancePart {
	parts, meshOnly, _ := sk.assemblyParts(upTo)
	var out []*clearancePart
	for _, p := range parts {
		out = append(out, newClearancePart(p.g, p.shell))
	}
	for _, hg := range meshOnly {
		mp := &clearancePart{g: hg}
		mp.mesh.Triangles = append(mp.mesh.Triangles, sk.Group(hg).ThisMesh.Triangles...)
		mp.tree = srf.NewMeshTree(&mp.mesh)
		out = append(out, mp)
	}
	return out
}

// Clearances are the gaps between the parts of the model up to group
// upTo that are less than below, the nearest first; a below of 0 or
// less lists every pair's. Parts that touch or overlap have a gap of 0:
// Interferences tells which overlap.
func (sk *Sketch) Clearances(upTo core.HGroup, below float64) []Clearance {
	limit := below
	if limit <= 0 {
		limit = math.Inf(1)
	}
	parts := sk.clearanceParts(upTo)
	var found []Clearance
	for i := range parts {
		for j := i + 1; j < len(parts); j++ {
			if c, ok := clearanceOf(parts[i], parts[j], limit); ok {
				found = append(found, c)
			}
		}
	}
	slices.SortStableFunc(found, func(x, y Clearance) int { return cmp.Compare(x.Gap, y.Gap) })
	return found
}

// clearanceOf is the gap between parts a and b, if it is less than
// limit.
func clearanceOf(a, b *clearancePart, limit float64) (Clearance, bool) {
	tol := a.chord + b.chord
	// A pair the meshes put a little past the limit may still be inside
	// it.
	n, ok := a.tree.Nearest(b.tree, limit+tol)
	if !ok {
		return Clearance{}, false
	}
	c := Clearance{A: a.g, B: b.g, Gap: n.Dist, PA: n.PA, PB: n.PB, Tolerance: tol}
	if n.Dist > 0 && a.shell != nil && b.shell != nil {
		c.refine(a, b, n)
	}
	if c.Gap >= limit {
		return Clearance{}, false
	}
	return c, true
}

// refine follows the nearest points of two parts' meshes onto the
// surfaces their triangles stand for, and keeps the nearest pair of
// points that stay on the parts. The nearest points of two meshes are
// often on an edge of each, between triangles of two surfaces, so each
// surface about each point is tried.
func (c *Clearance) refine(a, b *clearancePart, n srf.MeshNearest) {
	best := math.Inf(1)
	var bpa, bpb geom.Vector
	for _, ia := range surfacesAt(a, n.PA) {
		for _, ib := range surfacesAt(b, n.PB) {
			pa, pb, ok := follow(a, b, &a.shell.Surface[ia], &b.shell.Surface[ib], n.PA, n.PB)
			if gap := pa.Minus(pb).Magnitude(); ok && gap < best {
				best, bpa, bpb = gap, pa, pb
			}
		}
	}
	if math.IsInf(best, 1) || math.Abs(best-n.Dist) > c.Tolerance+geom.LengthEps {
		return
	}
	// The true gap is no more than that between two points of the solids,
	// and no less than the meshes' less the chord tolerances.
	c.Tolerance = max(best-(n.Dist-c.Tolerance), 0)
	c.Gap, c.PA, c.PB, c.Refined = best, bpa, bpb, true
}

// surfacesAt are the surfaces, by their places in the part's shell, of
// the triangles of its mesh at p.
func surfacesAt(p *clearancePart, at geom.Vector) []int {
	var found []int
	for _, i := range p.tree.Within(at, geom.LengthEps) {
		if s := p.tree.Triangle(i).Tag; !slices.Contains(found, s) {
			found = append(found, s)
		}
	}
	return found
}

// follow moves pa and pb, each in turn, to the point of its surface
// nearest the other, until they stop, and reports whether they did and
// stayed on the parts: near their meshes, within the chord tolerances.
// A point that has run off its face, past a trim, onto the rest of its
// surface, is far from the mesh.
func follow(a, b *clearancePart, sa, sb *srf.SSurface, pa, pb geom.Vector) (geom.Vector, geom.Vector, bool) {
	for range 50 {
		qa := sa.Project(pb, 0).P
		qb := sb.Project(qa, 0).P
		moved := qa.Minus(pa).Magnitude() + qb.Minus(pb).Magnitude()
		pa, pb = qa, qb
		if moved < 1e-9 {
			da, _, _ := a.tree.NearestPoint(pa)
			db, _, _ := b.tree.NearestPoint(pb)
			return pa, pb, da <= a.chord+geom.LengthEps && db <= b.chord+geom.LengthEps
		}
	}
	return pa, pb, false
}
