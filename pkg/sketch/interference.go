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
// groups before the first of them made is another. Two parts interfere
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
	// Failed is set when the Boolean failed, and nothing is known.
	Failed bool
}

// part is a solid of an assembly, by the group that made it.
type part struct {
	g      core.HGroup
	shell  *srf.SShell
	volume float64
	lo, hi geom.Vector
}

// Interferences checks the parts of the model up to group upTo against
// each other. meshOnly lists the parts that have no shell.
func (sk *Sketch) Interferences(upTo core.HGroup) (found []Interference, meshOnly []core.HGroup) {
	var parts []part
	add := func(h core.HGroup, sh *srf.SShell) {
		if len(sh.Surface) == 0 {
			return
		}
		p := part{g: h, shell: sh}
		p.hi = geom.Vector{X: math.Inf(-1), Y: math.Inf(-1), Z: math.Inf(-1)}
		p.lo = p.hi.ScaledBy(-1)
		for i := range sh.Surface {
			var smax, smin geom.Vector
			sh.Surface[i].GetAxisAlignedBounding(&smax, &smin)
			smax.MakeMaxMin(&p.hi, &p.lo)
			smin.MakeMaxMin(&p.hi, &p.lo)
		}
		var m srf.SMesh
		sh.TriangulateInto(&m)
		p.volume = m.CalculateVolume()
		parts = append(parts, p)
	}
	var before *Group // the last group before the first assembled one
	assembled := false
	for _, hg := range sk.GroupOrder {
		g := sk.GetGroup(hg)
		if g == nil || g.Suppress || sk.SuppressedReason(hg) != "" {
			if hg == upTo {
				break
			}
			continue
		}
		solid := g.IsMeshGroup() || g.Type == GroupLinked || g.Type == GroupMirror
		if solid && g.MeshCombine == CombineAssemble {
			if !assembled && before != nil {
				add(before.H, &before.RunningShell)
			}
			assembled = true
			if g.IsTriangleMeshAssembly() || g.ForceToMesh {
				meshOnly = append(meshOnly, hg)
			} else {
				add(hg, &g.ThisShell)
			}
		} else if !assembled && solid {
			before = g
		}
		if hg == upTo {
			break
		}
	}
	for i := range parts {
		for j := i + 1; j < len(parts); j++ {
			a, b := &parts[i], &parts[j]
			if a.lo.X > b.hi.X || b.lo.X > a.hi.X || a.lo.Y > b.hi.Y || b.lo.Y > a.hi.Y ||
				a.lo.Z > b.hi.Z || b.lo.Z > a.hi.Z {
				continue
			}
			v, m, ok := overlap(a.shell, b.shell)
			// Faces that lie on each other, as of a part placed twice
			// at one place, can leave the Boolean with nothing: where
			// the boxes share volume, again with the second part moved
			// by a ten-thousandth of a millimetre.
			box := (math.Min(a.hi.X, b.hi.X) - math.Max(a.lo.X, b.lo.X)) *
				(math.Min(a.hi.Y, b.hi.Y) - math.Max(a.lo.Y, b.lo.Y)) *
				(math.Min(a.hi.Z, b.hi.Z) - math.Max(a.lo.Z, b.lo.Z))
			if (!ok || v <= 0) && box > 1e-6*math.Min(a.volume, b.volume) {
				var moved srf.SShell
				moved.MakeFromTransformationOf(b.shell, geom.Vector{X: 1e-4, Y: 1.618e-4, Z: 1.414e-4}, geom.QuaternionIdentity, 1)
				v, m, ok = overlap(a.shell, &moved)
			}
			if !ok {
				found = append(found, Interference{A: a.g, B: b.g, Failed: true})
				continue
			}
			// Touching faces leave slivers of no volume.
			if v > 1e-6*math.Min(a.volume, b.volume) && v > 1e-9 {
				found = append(found, Interference{A: a.g, B: b.g, Volume: v, Mesh: m})
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
