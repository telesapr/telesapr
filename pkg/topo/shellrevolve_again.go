package topo

import (
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A solid of revolution made by Booleans, as lathes joined, keeps the
// surfaces of its operands, trimmed back: a cup's side runs on under a
// flange joined to it. The inside, taken away from such a solid, can
// leave a Boolean the kernel fails, where it takes it from the same
// solid turned whole from its profile. ShellRevolved has found every
// face of the solid on the profile turned, so it may hollow that one
// instead, its faces given the identities of the solid's own.

// revolvedAgain is the solid rev makes, at sh's tolerance, each of its
// surfaces with the identity and colour of the face of sh (of topology
// t) it lies on.
func revolvedAgain(sh *srf.SShell, t *Topology, rev Revolution) (*srf.SShell, error) {
	again := rev.solid(sh.Tol, sh.Surface[0].Color)
	for i := range again.Surface {
		s := &again.Surface[i]
		f, d := t.FaceAt(s.PointAt(0.5, 0.5))
		if f < 0 || d > 100*geom.LengthEps {
			return nil, ErrNotRevolution
		}
		from := t.Surface(f)
		s.Face, s.Origin, s.Color = from.Face, from.Origin, from.Color
	}
	return again, nil
}

// solid is the solid r makes, at tolerance tol, its faces of colour
// color named by no entity.
func (r Revolution) solid(tol srf.Tolerance, color core.RGBA) *srf.SShell {
	axis := r.Axis.WithMagnitude(1)
	out := &srf.SShell{Tol: tol}
	for i := range r.Profile.Sets {
		if r.Whole() {
			out.MakeFromRevolutionOf(&r.Profile.Sets[i], r.Pt, axis, color, noFaces{})
		} else {
			out.MakeFromHelicalRevolutionOf(&r.Profile.Sets[i], r.Pt, axis, color, noFaces{}, r.Start, r.End, 0, 0)
		}
	}
	return out
}
