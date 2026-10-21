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
// measured on the parts' meshes: from the middle of each triangle, a
// ray into the solid, square to the triangle, to where it leaves the
// part again. A wall is measured so across itself wherever a triangle
// of either of its sides stands; a thin spot narrower than the
// triangles about it, as the tip of a cone, can fall between them. The
// two ends of each ray lie on the mesh, each within the chord
// tolerance of the surface.

// ThinSpot is a wall's thickness at a point: the part's group, the
// thickness, the point it was measured from and the one across the
// wall.
type ThinSpot struct {
	Group     core.HGroup
	Thickness float64
	From, To  geom.Vector
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
	// round: where a mesh is open or parts stand in each other.
	Samples, Unmeasured int
	// Tolerance is how far a thickness may be from the true one at that
	// point: both ends of the ray lie on the mesh.
	Tolerance float64
}

// wallMesh is a part's triangles, to measure its walls by.
type wallMesh struct {
	g     core.HGroup
	mesh  srf.SMesh
	chord float64
}

// WallThickness measures the walls of the parts of the model up to group
// upTo, and lists the points thinner than below; with a below of 0 or
// less, only the thinnest. Each part is measured alone, as the
// interference check takes them (assemblyParts); a model that
// assembles nothing is one part. The model must have been generated.
func (sk *Sketch) WallThickness(upTo core.HGroup, below float64) (WallReport, error) {
	g := sk.Group(upTo)
	if g == nil {
		return WallReport{}, fmt.Errorf("g%03x doesn't exist", upTo.V)
	}
	parts, meshOnly, _ := sk.assemblyParts(upTo)
	var meshes []*wallMesh
	for _, p := range parts {
		w := &wallMesh{g: p.g, chord: p.shell.Tol.ChordTolMm()}
		p.shell.TriangulateInto(&w.mesh)
		meshes = append(meshes, w)
	}
	for _, hg := range meshOnly {
		w := &wallMesh{g: hg}
		w.mesh.Triangles = append(w.mesh.Triangles, sk.Group(hg).ThisMesh.Triangles...)
		meshes = append(meshes, w)
	}
	if len(meshes) == 0 {
		g.GenerateDisplayItems(sk)
		if g.DisplayMesh.IsEmpty() {
			return WallReport{}, errors.New("there is no solid at the group")
		}
		w := &wallMesh{g: upTo, chord: sk.Tol().ChordTolMm()}
		w.mesh.Triangles = append(w.mesh.Triangles, g.DisplayMesh.Triangles...)
		meshes = append(meshes, w)
	}
	r := WallReport{Thinnest: ThinSpot{Thickness: math.Inf(1)}}
	for _, w := range meshes {
		r.Tolerance = max(r.Tolerance, 2*w.chord)
		r.measure(w, below)
	}
	slices.SortStableFunc(r.Thin, func(a, b ThinSpot) int { return cmp.Compare(a.Thickness, b.Thickness) })
	return r, nil
}

// measure casts a ray from the middle of each of the part's triangles
// into it, and takes in what each found.
func (r *WallReport) measure(w *wallMesh, below float64) {
	tree := srf.NewMeshTree(&w.mesh)
	for i := range w.mesh.Triangles {
		tr := &w.mesh.Triangles[i]
		n := tr.Normal()
		if n.Magnitude() < 1e-12 {
			continue
		}
		r.Samples++
		s, ok := thicknessAt(tree, i, n.WithMagnitude(-1))
		if !ok {
			r.Unmeasured++
			continue
		}
		s.Group = w.g
		if s.Thickness < r.Thinnest.Thickness {
			r.Thinnest = s
		}
		if s.Thickness < below {
			r.Thin = append(r.Thin, s)
		}
	}
}

// rayStarts are the points of a triangle a ray is cast from, by their
// weights of its corners: the middle, and if the ray from there passes
// exactly along an edge between two triangles across the wall, as
// where the two faces of a plate are cut into triangles alike, and
// slips through, three points off it.
var rayStarts = [][3]float64{{1.0 / 3, 1.0 / 3, 1.0 / 3}, {0.5, 0.3, 0.2}, {0.2, 0.5, 0.3}, {0.3, 0.2, 0.5}}

// thicknessAt is the wall's thickness at triangle i of the tree's mesh,
// in along in, and false where no ray from it leaves the solid through
// a face that faces out.
func thicknessAt(tree *srf.MeshTree, i int, in geom.Vector) (ThinSpot, bool) {
	tr := tree.Triangle(i)
	for _, wt := range rayStarts {
		from := tr.A.ScaledBy(wt[0]).Plus(tr.B.ScaledBy(wt[1])).Plus(tr.C.ScaledBy(wt[2]))
		if h, ok := tree.Ray(from, in, 1e-9, i); ok && h.Cos > 0 {
			return ThinSpot{Thickness: h.T, From: from, To: from.Plus(in.ScaledBy(h.T))}, true
		}
	}
	return ThinSpot{}, false
}
