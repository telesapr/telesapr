package sketch

import (
	"errors"
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A wrap lays the closed loops of its profile sketch (OpA), text
// included, onto a cylinder or a cone of the solid so far,
// SolidFaces[0], as paper is rolled onto it, and raises them ValB above
// the face (combined as a union: an emboss) or sinks them ValB into it
// (a difference: an engraving). The sketch lies in a workplane parallel
// to the face's axis, and lands on the side of the face towards it,
// looking from there as it was drawn (srf.Wrap). The band reaches
// through the face by a part of its height, so that the Boolean crosses
// the face rather than touching it, which it can't do; how far is
// hidden in the result, inside the solid or in the air. The group makes
// no entities; its faces are named from it.
//
// The Boolean of a band with a cone or a cylinder can fail where it
// can't close the trims of the face at a corner of the profile, by
// chance of where its marched curves end (no case of the text sweeps
// does since 2026-10-23). Where the whole band fails, the group joins it
// loop set by loop set, each through the face as far as each of
// wrapOverlaps in turn until one joins: the result is the same solid.
// Where a loop set joins at none, the group refuses, the solid so far
// passing through, rather than leave an unsound solid.

// wrapOverlaps are how far through the face the band reaches, as
// fractions of its height, in the order they are tried.
var wrapOverlaps = []float64{0.25, 0.05, 0.4, 0.15, 0.7}

// NewWrap starts a wrap of the active group's sketch onto the face of
// the solid so far, raised height mm above it, or with engrave sunk
// height mm into it. It tries the wrap, so a face, sketch or height it
// can't take is an error here. It is what the interface and scripts add
// with AddGroupAfterActive.
func (sk *Sketch) NewWrap(face srf.FaceID, height float64, engrave bool, name string) (Group, error) {
	g := NewGroup()
	g.Type = GroupWrap
	g.Name = name
	if name == "" {
		g.Name = "wrap"
	}
	g.MeshCombine = CombineUnion
	if engrave {
		g.MeshCombine = CombineDifference
	}
	g.ValB = height
	src := sk.Group(sk.Active)
	if src == nil || src.Type != GroupDrawingWorkplane {
		return g, errors.New("the active group must be a sketch in a workplane: its loops are what is wrapped")
	}
	if src.PolyError.How != int(core.PolyGood) || len(src.BezierLoops.Sets) == 0 {
		return g, errors.New("the active sketch must be closed loops to wrap")
	}
	g.OpA = src.H
	g.SolidFaces = []srf.FaceID{face}
	w, sets, err := g.wrapOf(sk, sk.solidAt(sk.Active))
	if err != nil {
		return g, err
	}
	if _, err := g.bandOf(sk, w, sets, wrapOverlaps[0]); err != nil {
		return g, err
	}
	return g, nil
}

// wrapOf is the wrap of the group's profile onto its face of the solid
// of group solid, and the profile's loop sets.
func (g *Group) wrapOf(sk *Sketch, solid *Group) (*srf.Wrap, []srf.SBezierLoopSet, error) {
	h := g.ValB
	switch {
	case len(g.SolidFaces) == 0:
		return nil, nil, errors.New("the wrap names no face to lie on")
	case h <= 0 || math.IsNaN(h) || math.IsInf(h, 0):
		return nil, nil, fmt.Errorf("a wrap's height must be more than zero, not %g", h)
	case g.MeshCombine != CombineUnion && g.MeshCombine != CombineDifference:
		return nil, nil, errors.New("a wrap embosses (union) or engraves (difference) the face")
	case solid == nil || len(solid.RunningShell.Surface) == 0:
		return nil, nil, errors.New("there is no solid with faces to wrap onto")
	case solid.IsForcedToMesh(sk):
		return nil, nil, errors.New("the solid is a triangle mesh, which has no faces to wrap onto")
	}
	src := sk.Group(g.OpA)
	if src == nil || src.Type != GroupDrawingWorkplane {
		return nil, nil, errors.New("the profile is not a sketch in a workplane")
	}
	if src.PolyError.How != int(core.PolyGood) || len(src.BezierLoops.Sets) == 0 {
		return nil, nil, errors.New("the profile is not closed loops")
	}
	t, err := solid.Topology()
	if err != nil {
		return nil, nil, fmt.Errorf("finding the solid's faces: %w", err)
	}
	fs := t.FacesWithID(g.SolidFaces[0])
	if len(fs) == 0 {
		return nil, nil, fmt.Errorf("face %x is no longer in the solid", uint32(g.SolidFaces[0]))
	}
	face, ok := srf.RoundFaceOf(t.Surface(fs[0]))
	if !ok {
		return nil, nil, errors.New("the face is neither a cylinder nor a cone: a sketch wraps onto those")
	}
	wp := sk.MustEntity(src.H.Entity(0))
	sets := src.BezierLoops.Sets
	w, err := srf.NewWrap(face, wp.WorkplaneGetOffset(sk), wp.NormalEntity(sk).NormalN(sk), sets)
	if err != nil {
		return nil, nil, err
	}
	return w, sets, nil
}

// bandOf is the band of the loop sets sets wrapped by w, reaching
// overlap of its height through the face.
func (g *Group) bandOf(sk *Sketch, w *srf.Wrap, sets []srf.SBezierLoopSet, overlap float64) (*srf.SShell, error) {
	h := g.ValB
	h0, h1 := -overlap*h, h
	if g.MeshCombine == CombineDifference {
		h0, h1 = -h, overlap*h
	}
	sh := &srf.SShell{Tol: sk.tol}
	for i := range sets {
		if err := sh.MakeFromWrapOf(&sets[i], w, h0, h1, g.Color); err != nil {
			return nil, err
		}
	}
	return sh, nil
}

// wrapOverlapsIn are the overlaps of wrapOverlaps that keep the part
// of the band through the face within the wall under it, for an emboss,
// or within the air over it, for an engraving: a band reaching through
// a wall thinner than its overlap would stand out of the wall's other
// side. Where the wall is thinner than all of them, the one overlap is
// half the wall; where the wall is too thin to hold even that, an
// error.
func (g *Group) wrapOverlapsIn(solid *srf.SShell, w *srf.Wrap, sets []srf.SBezierLoopSet) ([]float64, error) {
	h := g.ValB
	room, at := wrapRoom(solid, w, sets, g.MeshCombine == CombineDifference)
	var out []float64
	for _, o := range wrapOverlaps {
		if o*h <= room/2 {
			out = append(out, o)
		}
	}
	if len(out) > 0 {
		return out, nil
	}
	if room/2 > 2*solid.Tol.ChordTolMm() {
		return []float64{room / 2 / h}, nil
	}
	what := "wall under the emboss"
	if g.MeshCombine == CombineDifference {
		what = "room over the engraving, before the solid again,"
	}
	return nil, fmt.Errorf("the %s is %.3g mm at its thinnest, at (%.3f, %.3f, %.3f): too little to join the band through",
		what, room, at.X, at.Y, at.Z)
}

// wrapRoom is the least distance from the face under the wrapped sets
// to the solid's surface beyond, along the face's normal: inwards,
// across the wall to where the ray leaves the solid, or with out,
// outwards, across the air over the face to where it enters the solid
// again; +Inf where the rays meet nothing. It is measured on the
// solid's mesh, and a ray from the face that meets the face's own
// triangles, the other way, passes them.
func wrapRoom(solid *srf.SShell, w *srf.Wrap, sets []srf.SBezierLoopSet, out bool) (float64, geom.Vector) {
	var m srf.SMesh
	solid.TriangulateInto(&m)
	tree := srf.NewMeshTree(&m)
	room, at := math.Inf(1), geom.Vector{}
	pts, ns := w.FacePoints(sets)
	for i, p := range pts {
		dir := ns[i]
		if !out {
			dir = dir.ScaledBy(-1)
		}
		tmin := geom.LengthEps
		for range 4 {
			hit, ok := tree.Ray(p, dir, tmin, -1)
			if !ok || hit.T >= room {
				break
			}
			if (hit.Cos > 0) != out {
				room, at = hit.T, p
				break
			}
			tmin = hit.T + geom.LengthEps
		}
	}
	return room, at
}

// wrapNamer names a band's surfaces from the group, in order, so that a
// band made loop set by loop set names its faces as the whole one does.
type wrapNamer struct {
	g    *Group
	next int
}

// name gives the band's surfaces the group's next names.
func (n *wrapNamer) name(sh *srf.SShell) {
	for i := range sh.Surface {
		sh.Surface[i].Origin = srf.OriginFor(n.g.H.V, n.next)
		n.next++
	}
}

// generateWrapShell wraps the profile onto the face of the solid so far
// and combines the band with it, into the group's running shell; its own
// shell is the band. When it can't, the solid passes through and the
// group says why.
func (g *Group) generateWrapShell(sk *Sketch) {
	prev := g.PreviousGroup(sk)
	var solid *Group
	var prevs srf.SShell
	if prev != nil {
		solid = sk.solidAt(prev.H)
		prevs.MakeFromCopyOf(&prev.RunningShell)
	}
	g.RunningShell.MakeFromCopyOf(&prevs)
	w, sets, err := g.wrapOf(sk, solid)
	if err != nil {
		g.setProblem(nil, "%v", err)
		return
	}
	overlaps, err := g.wrapOverlapsIn(&prevs, w, sets)
	if err != nil {
		g.setProblem(nil, "%v", err)
		return
	}
	band, err := g.bandOf(sk, w, sets, overlaps[0])
	if err != nil {
		g.setProblem(nil, "%v", err)
		return
	}
	(&wrapNamer{g: g}).name(band)
	g.ThisShell = *band
	var out srf.SShell
	out.Tol = sk.tol
	g.generateForBooleanShell(&prevs, band, &out, g.MeshCombine)
	if out.BooleanFailed {
		out = g.wrapBySets(sk, &prevs, w, sets, overlaps)
	}
	if out.BooleanFailed {
		// Refused rather than left unsound: the solid so far passes
		// through.
		g.setProblem(nil, "the wrapped profile can't be joined to the face at a height of %g: "+
			"the Boolean fails at a corner of it; a smaller height may join", g.ValB)
		return
	}
	out.MergeCoincidentSurfaces()
	g.RunningShell = out
}

// wrapBySets combines the band with prev loop set by loop set, each
// reaching through the face as far as each of overlaps in turn, until
// it joins.
func (g *Group) wrapBySets(sk *Sketch, prev *srf.SShell, w *srf.Wrap, sets []srf.SBezierLoopSet, overlaps []float64) srf.SShell {
	cur := *prev
	namer := &wrapNamer{g: g}
	for i := range sets {
		var out srf.SShell
		start := namer.next
		for _, o := range overlaps {
			namer.next = start
			band, err := g.bandOf(sk, w, sets[i:i+1], o)
			if err != nil {
				continue
			}
			namer.name(band)
			out = srf.SShell{Tol: sk.tol}
			g.generateForBooleanShell(&cur, band, &out, g.MeshCombine)
			if !out.BooleanFailed {
				break
			}
		}
		if out.BooleanFailed || len(out.Surface) == 0 {
			return out
		}
		cur = out
	}
	return cur
}

// generateWrapBand makes the band alone, into the group's own shell, for
// the pipeline to combine as meshes.
func (g *Group) generateWrapBand(sk *Sketch) {
	var solid *Group
	if prev := g.PreviousGroup(sk); prev != nil {
		solid = sk.solidAt(prev.H)
	}
	w, sets, err := g.wrapOf(sk, solid)
	if err == nil {
		var band *srf.SShell
		if band, err = g.bandOf(sk, w, sets, wrapOverlaps[0]); err == nil {
			g.ThisShell = *band
			return
		}
	}
	g.setProblem(nil, "%v", err)
}
