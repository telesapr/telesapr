package sketch

import (
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// The interference check by shells (roadmap 16.10, G3). An assembly's
// parts are the solids its groups put side by side: each group that
// combines as an assembly is one, its own solid, and the solid the
// groups before the first of them made is another; a group cutting the
// assembly after a part cuts the part. Two parts interfere
// where the Boolean intersection of their shells has volume; parts
// that only touch don't. A part with no shell, as a linked mesh or STEP
// file, can't be checked so: the triangles of the whole model are, as
// before (MeshOnly).

// Interference is two parts that overlap: the groups that made them,
// the volume they share, and its triangles.
type Interference struct {
	A, B   core.HGroup
	Volume float64
	Mesh   srf.SMesh
	// Approximate is set when the Boolean of the shells failed and the
	// volume is that of their meshes' intersection instead, within the
	// chord tolerance times the parts' area of the true one.
	Approximate bool
	// Failed is set when the Boolean failed, of the meshes too, and
	// nothing is known.
	Failed bool
}

// part is a solid of an assembly, by the group that made it: whole,
// and its pieces when they are needed.
type part struct {
	g      core.HGroup
	shell  *srf.SShell
	whole  *piece
	pieces []*piece
	lo, hi geom.Vector
	// uncut is set when a cut after the part failed on it, so that its
	// shell is the part as made, before the cut.
	uncut bool
}

// piece is a closed piece of a part's shell (srf.SShell.Pieces).
type piece struct {
	shell  *srf.SShell
	mesh   srf.SMesh
	volume float64
	area   float64
	lo, hi geom.Vector
}

// newPiece is sh with its box, mesh, volume and area.
func newPiece(sh *srf.SShell) *piece {
	pc := &piece{shell: sh}
	pc.lo, pc.hi = shellBox(sh)
	sh.TriangulateInto(&pc.mesh)
	pc.volume = pc.mesh.CalculateVolume()
	for i := range pc.mesh.Triangles {
		pc.area += pc.mesh.Triangles[i].Area()
	}
	return pc
}

// piecesOf are the part's pieces, made once; a part of one piece is
// its whole.
func (p *part) piecesOf() []*piece {
	if p.pieces == nil {
		shells := p.shell.Pieces()
		if len(shells) == 1 {
			p.pieces = []*piece{p.whole}
		} else {
			for _, sh := range shells {
				p.pieces = append(p.pieces, newPiece(sh))
			}
		}
	}
	return p.pieces
}

// shellBox is the box about the shell's surfaces' control points.
func shellBox(sh *srf.SShell) (lo, hi geom.Vector) {
	hi = geom.Vector{X: math.Inf(-1), Y: math.Inf(-1), Z: math.Inf(-1)}
	lo = hi.ScaledBy(-1)
	for i := range sh.Surface {
		var smax, smin geom.Vector
		sh.Surface[i].AxisAlignedBounding(&smax, &smin)
		smax.MakeMaxMin(&hi, &lo)
		smin.MakeMaxMin(&hi, &lo)
	}
	return lo, hi
}

// cutParts takes the solid of g, a group cutting the assembly after
// its parts, from each part it reaches, or keeps of each only what lies
// in it, as the group's Boolean does to the assembly as a whole: a hole
// cut through a bracket after the bracket's screws were placed is a
// hole in the bracket part, not a screw through a solid one.
func cutParts(parts []*part, g *Group) {
	tool := &g.ThisShell
	if len(tool.Surface) == 0 {
		return
	}
	lo, hi := shellBox(tool)
	for _, p := range parts {
		meets := boxesMeet(p.lo, p.hi, lo, hi)
		out := &srf.SShell{Tol: p.shell.Tol}
		switch {
		case g.MeshCombine == CombineDifference && !meets:
			continue
		case g.MeshCombine == CombineDifference:
			out.MakeFromDifferenceOf(p.shell, tool)
		case meets:
			out.MakeFromIntersectionOf(p.shell, tool)
		}
		if out.BooleanFailed {
			p.uncut = true
			continue
		}
		p.shell = out
		p.lo, p.hi = shellBox(out)
	}
}

// nudge is how far a part is moved to try again where the Boolean of
// two parts found nothing, or failed.
var nudge = geom.Vector{X: 1e-4, Y: 1.618e-4, Z: 1.414e-4}

// assemblyParts are the parts of the model up to group upTo, cut by
// the groups that cut the assembly after them, and the parts that have
// no shell, by group. before is the last group before the first
// assembled one, whose solid so far is a part, or nil when the model is
// no assembly, or one that starts with an assembled group.
func (sk *Sketch) assemblyParts(upTo core.HGroup) (parts []*part, meshOnly []core.HGroup, before *Group) {
	add := func(h core.HGroup, sh *srf.SShell) {
		if len(sh.Surface) == 0 {
			return
		}
		p := &part{g: h, shell: sh}
		p.lo, p.hi = shellBox(sh)
		parts = append(parts, p)
	}
	assembled := false
	for _, hg := range sk.GroupOrder {
		g := sk.Group(hg)
		if g == nil || g.Suppress || sk.SuppressedReason(hg) != "" {
			if hg == upTo {
				break
			}
			continue
		}
		solid := g.IsMeshGroup() || g.Type == GroupLinked || g.Type == GroupMirror
		switch {
		case solid && g.MeshCombine == CombineAssemble:
			if !assembled && before != nil {
				add(before.H, &before.RunningShell)
			}
			assembled = true
			if g.IsTriangleMeshAssembly() || g.ForceToMesh {
				meshOnly = append(meshOnly, hg)
			} else {
				add(hg, &g.ThisShell)
			}
		case assembled && solid && (g.MeshCombine == CombineDifference || g.MeshCombine == CombineIntersection):
			cutParts(parts, g)
		case !assembled && solid:
			before = g
		}
		if hg == upTo {
			break
		}
	}
	if !assembled {
		before = nil
	}
	return parts, meshOnly, before
}

// Interferences checks the parts of the model up to group upTo against
// each other. meshOnly lists the parts that have no shell.
func (sk *Sketch) Interferences(upTo core.HGroup) (found []Interference, meshOnly []core.HGroup) {
	parts, meshOnly, _ := sk.assemblyParts(upTo)
	for _, p := range parts {
		p.whole = newPiece(p.shell)
	}
	for i := range parts {
		for j := i + 1; j < len(parts); j++ {
			a, b := parts[i], parts[j]
			if !boxesMeet(a.lo, a.hi, b.lo, b.hi) {
				continue
			}
			f := Interference{A: a.g, B: b.g, Approximate: a.uncut || b.uncut}
			if len(a.piecesOf()) > 1 || len(b.piecesOf()) > 1 {
				f = a.overlapByPieces(b, f)
			} else if v, m, approximate, ok := pieceOverlap(a.whole, b.whole); ok {
				f.Volume, f.Mesh, f.Approximate = v, m, f.Approximate || approximate
			} else {
				f.Failed = true
			}
			if f.Failed {
				found = append(found, Interference{A: a.g, B: b.g, Failed: true})
				continue
			}
			// Touching faces leave slivers of no volume.
			if f.Volume > 1e-9 {
				found = append(found, f)
			}
		}
	}
	return found, meshOnly
}

// overlapByPieces is f, two parts' interference, found piece by piece:
// a part may be an assembly of its own, of solids standing in each
// other, as a linked model of parts, and a Boolean of such a shell is
// not defined, where it doesn't fail it is wrong. Where more than one
// pair of pieces overlaps, the volume is their sum, which counts twice
// what pieces of one part share, and is marked approximate.
func (p *part) overlapByPieces(o *part, f Interference) Interference {
	pairs := 0
	for _, pa := range p.piecesOf() {
		for _, pb := range o.piecesOf() {
			if !boxesMeet(pa.lo, pa.hi, pb.lo, pb.hi) {
				continue
			}
			v, m, approximate, ok := pieceOverlap(pa, pb)
			if !ok {
				return Interference{A: f.A, B: f.B, Failed: true}
			}
			if v > 0 {
				pairs++
				f.Volume += v
				f.Mesh.Triangles = append(f.Mesh.Triangles, m.Triangles...)
				f.Approximate = f.Approximate || approximate || pairs > 1
			}
		}
	}
	return f
}

// boxesMeet reports whether two boxes share a point.
func boxesMeet(alo, ahi, blo, bhi geom.Vector) bool {
	return alo.X <= bhi.X && blo.X <= ahi.X && alo.Y <= bhi.Y && blo.Y <= ahi.Y &&
		alo.Z <= bhi.Z && blo.Z <= ahi.Z
}

// pieceOverlap is the volume and the triangles two pieces of parts
// share, zero where they only touch, whether it is of their meshes
// rather than their shells, and false if nothing is known.
func pieceOverlap(a, b *piece) (v float64, m srf.SMesh, approximate, ok bool) {
	v, m, ok = overlap(a.shell, b.shell)
	// Faces that lie on each other, as of a part placed twice at one
	// place, can leave the Boolean with nothing: where the boxes share
	// volume, again with the second part moved a ten-thousandth of a
	// millimetre each way. A part placed twice overlaps itself either
	// way; parts that only touch come apart one way, or share a sliver
	// no thicker than the move, and a sliver is too thin for the
	// Boolean to be sure of: the result moved apart stands, and the
	// first one stands when both moved fail.
	box := (math.Min(a.hi.X, b.hi.X) - math.Max(a.lo.X, b.lo.X)) *
		(math.Min(a.hi.Y, b.hi.Y) - math.Max(a.lo.Y, b.lo.Y)) *
		(math.Min(a.hi.Z, b.hi.Z) - math.Max(a.lo.Z, b.lo.Z))
	if (!ok || v <= 0) && box > 1e-6*math.Min(a.volume, b.volume) {
		retried := false
		var least float64
		var leastMesh srf.SMesh
		for _, by := range []geom.Vector{nudge, nudge.ScaledBy(-1)} {
			var moved srf.SShell
			moved.MakeFromTransformationOf(b.shell, by, geom.QuaternionIdentity, 1)
			mv, mm, mok := overlap(a.shell, &moved)
			if mok && (!retried || mv < least) {
				least, leastMesh, retried = mv, mm, true
			}
		}
		switch {
		case retried && least > nudge.Magnitude()*math.Min(a.area, b.area):
			v, m, ok = least, leastMesh, true
		case retried || ok:
			v, m, ok = 0, srf.SMesh{}, true
		}
	}
	if !ok {
		// Surfaces that lie on each other, as a shank in its nut, the
		// Boolean can't always follow: their meshes, within the chord
		// tolerance of them, share no more than that tolerance through
		// the parts' faces where they only touch.
		m.MakeFromIntersectionOf(&a.mesh, &b.mesh)
		v = m.CalculateVolume()
		if math.IsNaN(v) || v < -1e-9 {
			return 0, srf.SMesh{}, false, false
		}
		if v <= 2*a.shell.Tol.Or(b.shell.Tol).ChordTolMm()*math.Min(a.area, b.area) {
			return 0, srf.SMesh{}, true, true
		}
		return v, m, true, true
	}
	// Touching faces leave slivers of no volume.
	if v <= 1e-6*math.Min(a.volume, b.volume) {
		return 0, srf.SMesh{}, false, true
	}
	return v, m, false, true
}

// overlap is the volume and the triangles a and b share, and false if
// the Boolean failed.
func overlap(a, b *srf.SShell) (float64, srf.SMesh, bool) {
	var x srf.SShell
	x.MakeFromIntersectionOf(a, b)
	var m srf.SMesh
	if x.BooleanFailed {
		return 0, m, false
	}
	x.TriangulateInto(&m)
	return m.CalculateVolume(), m, true
}

// JoinsAssembly reports whether the solid so far that g's Boolean joins
// holds parts put side by side: a group before it combines as an
// assembly onto a solid made before that. Parts that stand in each
// other make a solid whose Boolean with another is not defined, which
// a failed Boolean then most likely comes of.
func (g *Group) JoinsAssembly(sk *Sketch) bool {
	assembled := false
	for p := g.RunningMeshGroup(sk); p != nil; p = p.RunningMeshGroup(sk) {
		if !p.IsMeshGroup() || p.Suppress || len(p.ThisShell.Surface) == 0 && p.ThisMesh.IsEmpty() {
			continue
		}
		// A part with a solid before it is one of several; the first
		// solid is a part alone, however it combines.
		if assembled {
			return true
		}
		assembled = p.MeshCombine == CombineAssemble
	}
	return false
}
