package sketch

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// The thickness of walls (item 14 of doc/notes/modelling_upgrades.md),
// measured on the parts' meshes: from points of each triangle, a ray
// into the solid, square to the triangle, to where it leaves the part
// again. A triangle is measured at the middles of the pieces it is cut
// into, each no longer than a step that the size of the model and the
// thickness asked about set, so that a thin spot narrower than the
// mesh's triangles, as near the tip of a cone, isn't missed between
// their middles; about the thinnest points found the pieces are cut
// finer again, and between them the corners of where the triangles the
// rays joined cover each other are measured (wall_corners.go). The two
// ends of each ray lie on the mesh, each within
// the chord tolerance of the surface; where both ends are on surfaces
// of a part with a shell, the thinnest spots are followed onto those
// surfaces as the clearance between parts is (Clearance.refine).

// ThinSpot is a wall's thickness at a point: the part's group, the
// thickness, the point it was measured from and the one across the
// wall.
type ThinSpot struct {
	Group     core.HGroup
	Thickness float64
	From, To  geom.Vector
	// Refined is set when From and To were followed onto the part's
	// surfaces, so that they lie on the solid rather than on its mesh.
	Refined bool
	// surfaces are the places in the part's shell of the surfaces From
	// and To lie on, or -1.
	sa, sb int
}

// WallReport is the thickness of the walls of the model's parts.
type WallReport struct {
	// Thinnest is the thinnest wall found; its Thickness is +Inf when
	// nothing could be measured.
	Thinnest ThinSpot
	// Thin are the points measured thinner than the limit asked for,
	// the thinnest first.
	Thin []ThinSpot
	// Samples is how many points were measured, and Unmeasured how many
	// rays found no way out of the solid, or met a face the wrong way
	// round: where a mesh is open or parts stand in each other. The
	// corners measured between the points (measureCorners) are counted
	// in Samples where their rays left the part; one whose didn't is
	// left out, not counted as unmeasured.
	Samples, Unmeasured int
	// Step is the longest a piece of a triangle measured at its middle
	// was; about the thinnest spots, the pieces were finer.
	Step float64
	// Tolerance is how far a thickness may be from the true one at that
	// point: both ends of the ray lie on the mesh.
	Tolerance float64
}

// Most points a wall check measures in its first pass, and how many
// more it measures about the thinnest: a model of many triangles is
// measured at no more than their middles.
const (
	wallSamples       = 50000
	wallRefineSamples = 20000
	// wallSteps is how many steps the model's size is cut into when no
	// thickness is asked about.
	wallSteps = 50
	// wallRefineCut is how many times finer the pieces are about the
	// thinnest spots.
	wallRefineCut = 8
	// wallRefineMost is how many thin spots are followed onto surfaces.
	wallRefineMost = 500
)

// WallThickness measures the walls of the parts of the model up to group
// upTo, and lists the points thinner than below; with a below of 0 or
// less, only the thinnest. Each solid is measured alone, a part of
// several taken by its solids as ClearancesWithin takes them, each
// hollow's inside with the solid round it, so that a ray from one solid
// doesn't stop on another standing on it; a model that assembles
// nothing is one part. The model must have been generated.
func (sk *Sketch) WallThickness(upTo core.HGroup, below float64) (WallReport, error) {
	g := sk.Group(upTo)
	if g == nil {
		return WallReport{}, fmt.Errorf("g%03x doesn't exist", upTo.V)
	}
	parts := sk.clearanceParts(upTo, true)
	if len(parts) == 0 && len(g.RunningShell.Surface) > 0 && !g.RunningShell.BooleanFailed {
		parts = append(parts, newClearancePart(upTo, &g.RunningShell))
	}
	if len(parts) == 0 {
		g.GenerateDisplayItems(sk)
		if g.DisplayMesh.IsEmpty() {
			return WallReport{}, errors.New("there is no solid at the group")
		}
		w := &clearancePart{g: upTo, chord: sk.Tol().ChordTolMm()}
		w.mesh.Triangles = append(w.mesh.Triangles, g.DisplayMesh.Triangles...)
		for i := range w.mesh.Triangles {
			w.mesh.Triangles[i].Tag = -1
		}
		w.tree = srf.NewMeshTree(&w.mesh)
		parts = append(parts, w)
	}
	r := WallReport{Thinnest: ThinSpot{Thickness: math.Inf(1)}}
	r.Step = wallStep(parts, below)
	for _, w := range parts {
		r.Tolerance = max(r.Tolerance, 2*w.chord)
		r.measure(w, below)
	}
	slices.SortStableFunc(r.Thin, func(a, b ThinSpot) int { return cmp.Compare(a.Thickness, b.Thickness) })
	return r, nil
}

// wallStep is the longest piece of a triangle a first pass measures at
// its middle: half the thickness asked about, or a part of the model's
// size, and longer where that would measure more than wallSamples
// points.
func wallStep(parts []*clearancePart, below float64) float64 {
	hi := geom.Vector{X: math.Inf(-1), Y: math.Inf(-1), Z: math.Inf(-1)}
	lo := hi.ScaledBy(-1)
	area := 0.0
	for _, w := range parts {
		for i := range w.mesh.Triangles {
			tr := &w.mesh.Triangles[i]
			for _, v := range []geom.Vector{tr.A, tr.B, tr.C} {
				v.MakeMaxMin(&hi, &lo)
			}
			area += tr.Area()
		}
	}
	step := hi.Minus(lo).Magnitude() / wallSteps
	if below > 0 {
		step = min(step, below/2)
	}
	// A triangle cut into pieces of side step holds about 2·area/step²
	// of them.
	return max(step, math.Sqrt(2*area/wallSamples))
}

// wallSample is a measured point: its spot, the triangle it is on and
// the one its ray left the part through.
type wallSample struct {
	spot     ThinSpot
	tri, hit int
}

// measure casts rays into the part from the pieces of each of its
// triangles, then from finer pieces of the triangles about the thinnest
// it found, and takes in what each found.
func (r *WallReport) measure(w *clearancePart, below float64) {
	var found []wallSample
	for i := range w.mesh.Triangles {
		found = r.measureTriangle(w, i, cutsFor(&w.mesh.Triangles[i], r.Step), found)
	}
	// About the thinnest: the triangles whose pieces measured within a
	// quarter again of the thinnest of the part.
	least := math.Inf(1)
	for _, f := range found {
		least = min(least, f.spot.Thickness)
	}
	var again []int
	seen := map[int]bool{}
	for _, f := range found {
		if f.spot.Thickness <= 1.25*least && !seen[f.tri] {
			seen[f.tri] = true
			again = append(again, f.tri)
		}
	}
	budget := wallRefineSamples
	for _, i := range again {
		n := cutsFor(&w.mesh.Triangles[i], r.Step) * wallRefineCut
		if n*n > budget {
			break
		}
		budget -= n * n
		found = r.measureTriangle(w, i, n, found)
	}
	found = r.measureCorners(w, below, found)
	slices.SortStableFunc(found, func(a, b wallSample) int { return cmp.Compare(a.spot.Thickness, b.spot.Thickness) })
	// The thinnest are followed onto the surfaces, each of which may move
	// it by the meshes' tolerance: the thinnest after is among them.
	for k, f := range found {
		s := f.spot
		if k < wallRefineMost {
			s = w.refineWall(s)
			if s.Thickness < r.Thinnest.Thickness {
				r.Thinnest = s
			}
		}
		if s.Thickness < below {
			r.Thin = append(r.Thin, s)
		}
	}
}

// cutsFor is how many pieces a side of the triangle is cut into for
// pieces no longer than step.
func cutsFor(tr *srf.STriangle, step float64) int {
	long := max(tr.B.Minus(tr.A).Magnitude(), tr.C.Minus(tr.B).Magnitude(), tr.A.Minus(tr.C).Magnitude())
	return max(1, int(math.Ceil(long/step)))
}

// measureTriangle measures triangle i of the part at the middles of the
// n² pieces its sides cut into n make, and adds what it found to found.
func (r *WallReport) measureTriangle(w *clearancePart, i, n int, found []wallSample) []wallSample {
	tr := w.tree.Triangle(i)
	nv := tr.Normal()
	if nv.Magnitude() < 1e-12 {
		return found
	}
	in := nv.WithMagnitude(-1)
	at := func(a, b int) geom.Vector {
		// The point a/n of the way along AB and b/n along AC.
		return tr.A.Plus(tr.B.Minus(tr.A).ScaledBy(float64(a) / float64(n))).Plus(tr.C.Minus(tr.A).ScaledBy(float64(b) / float64(n)))
	}
	piece := func(p, q, s geom.Vector) {
		r.Samples++
		sp, hit, ok := thicknessAt(w.tree, i, p, q, s, in)
		if !ok {
			r.Unmeasured++
			return
		}
		sp.Group = w.g
		sp.sa = tr.Tag
		found = append(found, wallSample{spot: sp, tri: i, hit: hit})
	}
	for a := range n {
		for b := range n - a {
			piece(at(a, b), at(a+1, b), at(a, b+1))
			if a+b+1 < n {
				piece(at(a+1, b), at(a+1, b+1), at(a, b+1))
			}
		}
	}
	return found
}

// rayStarts are the points of a piece of a triangle a ray is cast from,
// by their weights of its corners: the middle, and if the ray from
// there passes exactly along an edge between two triangles across the
// wall, as where the two faces of a plate are cut into triangles alike,
// and slips through, three points off it.
var rayStarts = [][3]float64{{1.0 / 3, 1.0 / 3, 1.0 / 3}, {0.5, 0.3, 0.2}, {0.2, 0.5, 0.3}, {0.3, 0.2, 0.5}}

// thicknessAt is the wall's thickness at the piece a, b, c of triangle
// i of the tree's mesh, in along in, with the triangle its ray left
// through, and false where no ray from it leaves the solid through a
// face that faces out.
func thicknessAt(tree *srf.MeshTree, i int, a, b, c, in geom.Vector) (ThinSpot, int, bool) {
	for _, wt := range rayStarts {
		from := a.ScaledBy(wt[0]).Plus(b.ScaledBy(wt[1])).Plus(c.ScaledBy(wt[2]))
		if s, hit, ok := thicknessFrom(tree, i, from, in); ok {
			return s, hit, true
		}
	}
	return ThinSpot{}, -1, false
}

// thicknessFrom is the wall's thickness from point from of triangle i,
// in along in, and the triangle the ray left through.
func thicknessFrom(tree *srf.MeshTree, i int, from, in geom.Vector) (ThinSpot, int, bool) {
	h, ok := tree.Ray(from, in, 1e-9, i)
	if !ok || h.Cos <= 0 {
		return ThinSpot{}, -1, false
	}
	return ThinSpot{Thickness: h.T, From: from, To: from.Plus(in.ScaledBy(h.T)), sb: tree.Triangle(h.Tri).Tag}, h.Tri, true
}

// refineWall follows a thin spot's two ends onto the surfaces of the
// part they lie on, to the nearest points of the two about them, and
// keeps them there where they stay on the part and the thickness moves
// no more than the tolerance of the meshes.
func (w *clearancePart) refineWall(s ThinSpot) ThinSpot {
	if w.shell == nil || s.sa < 0 || s.sb < 0 || s.sa == s.sb {
		return s
	}
	pa, pb, ok := follow(w, w, &w.shell.Surface[s.sa], &w.shell.Surface[s.sb], s.From, s.To)
	t := pa.Minus(pb).Magnitude()
	if !ok || math.Abs(t-s.Thickness) > 2*w.chord+geom.LengthEps {
		return s
	}
	s.Thickness, s.From, s.To, s.Refined = t, pa, pb, true
	return s
}
