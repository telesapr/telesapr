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

// part is a solid of an assembly, by the group that made it.
type part struct {
	g      core.HGroup
	shell  *srf.SShell
	mesh   srf.SMesh
	volume float64
	area   float64
	lo, hi geom.Vector
	// uncut is set when a cut after the part failed on it, so that its
	// shell is the part as made, before the cut.
	uncut bool
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
		meets := p.lo.X <= hi.X && lo.X <= p.hi.X && p.lo.Y <= hi.Y && lo.Y <= p.hi.Y &&
			p.lo.Z <= hi.Z && lo.Z <= p.hi.Z
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

// Interferences checks the parts of the model up to group upTo against
// each other. meshOnly lists the parts that have no shell.
func (sk *Sketch) Interferences(upTo core.HGroup) (found []Interference, meshOnly []core.HGroup) {
	var parts []*part
	add := func(h core.HGroup, sh *srf.SShell) {
		if len(sh.Surface) == 0 {
			return
		}
		p := &part{g: h, shell: sh}
		p.lo, p.hi = shellBox(sh)
		parts = append(parts, p)
	}
	var before *Group // the last group before the first assembled one
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
	for _, p := range parts {
		p.shell.TriangulateInto(&p.mesh)
		p.volume = p.mesh.CalculateVolume()
		for i := range p.mesh.Triangles {
			p.area += p.mesh.Triangles[i].Area()
		}
	}
	for i := range parts {
		for j := i + 1; j < len(parts); j++ {
			a, b := parts[i], parts[j]
			if a.lo.X > b.hi.X || b.lo.X > a.hi.X || a.lo.Y > b.hi.Y || b.lo.Y > a.hi.Y ||
				a.lo.Z > b.hi.Z || b.lo.Z > a.hi.Z {
				continue
			}
			v, m, ok := overlap(a.shell, b.shell)
			// Faces that lie on each other, as of a part placed twice
			// at one place, can leave the Boolean with nothing: where
			// the boxes share volume, again with the second part moved
			// a ten-thousandth of a millimetre each way. A part placed
			// twice overlaps itself either way; parts that only touch
			// come apart one way, or share a sliver no thicker than the
			// move, and a sliver is too thin for the Boolean to be sure
			// of: the result moved apart stands, and the first one
			// stands when both moved fail.
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
				// Surfaces that lie on each other, as a shank in its
				// nut, the Boolean can't always follow: their meshes,
				// within the chord tolerance of them, share no more
				// than that tolerance through the parts' faces where
				// they only touch.
				f := Interference{A: a.g, B: b.g, Approximate: true}
				f.Mesh.MakeFromIntersectionOf(&a.mesh, &b.mesh)
				f.Volume = f.Mesh.CalculateVolume()
				if math.IsNaN(f.Volume) || f.Volume < -1e-9 {
					found = append(found, Interference{A: a.g, B: b.g, Failed: true})
				} else if f.Volume > 2*a.shell.Tol.Or(b.shell.Tol).ChordTolMm()*math.Min(a.area, b.area) {
					found = append(found, f)
				}
				continue
			}
			// Touching faces leave slivers of no volume.
			if v > 1e-6*math.Min(a.volume, b.volume) && v > 1e-9 {
				found = append(found, Interference{A: a.g, B: b.g, Volume: v, Mesh: m, Approximate: a.uncut || b.uncut})
			}
		}
	}
	return found, meshOnly
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
