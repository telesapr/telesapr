package sketch

import (
	"errors"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// An extrusion's draft, ValC degrees, leans its sides into the solid
// from the sketch's plane, the neutral one, so that the far end is
// smaller, or larger when it is negative (topo.DraftedPrism, roadmap
// 15.4). A two-sided extrusion leans both ways from the plane. The
// entities the extrusion makes at its ends and along its sides keep
// where they would be without the draft, so that what refers to them
// keeps working; those off the solid are hidden. A draft the profile
// can't take is a problem on the group, and the extrusion is made
// without it.

// HasDraft reports whether the group is an extrusion with a draft.
func (g *Group) HasDraft() bool { return g.Type == GroupExtrude && g.ValC != 0 }

// generateDraftShell makes the drafted extrusion of the source's loop
// sets from the sketch's plane to tbot and to ttop, or says why it
// can't and leaves the shell empty.
func (g *Group) generateDraftShell(src *Group, tbot, ttop geom.Vector) bool {
	angle := g.ValC * math.Pi / 180
	var out srf.SShell
	first := true
	for k := range src.BezierLoops.Sets {
		set := &src.BezierLoops.Sets[k]
		loops := [][]srf.SBezier{set.Outer.Curves}
		for _, h := range set.Holes {
			loops = append(loops, h.Curves)
		}
		for _, along := range []geom.Vector{tbot, ttop} {
			if along.Magnitude() < core.LengthEps {
				continue
			}
			sh, err := topo.DraftedPrism(loops, along, angle, g.Color, 0)
			if err != nil {
				var se *topo.ShellError
				if errors.As(err, &se) {
					at := se.At
					g.setProblem(&at, "%v", err)
				} else {
					g.setProblem(nil, "%v", err)
				}
				return false
			}
			if first {
				out, first = *sh, false
				continue
			}
			var u srf.SShell
			u.MakeFromUnionOf(&out, sh)
			if u.BooleanFailed {
				g.setProblem(nil, "the Boolean that joins the drafted parts failed")
				return false
			}
			out = u
		}
	}
	g.ThisShell = out
	return true
}

// annotateDraftFaces gives the drafted shell's ends and the sides of
// the source's line segments their entities, as an extrusion's, so that
// they can be picked: a side by the segment it passes through on the
// sketch's plane.
func (g *Group) annotateDraftFaces(sk *Sketch, onOrig, tbot, ttop geom.Vector) {
	for i := range g.ThisShell.Surface {
		ss := &g.ThisShell.Surface[i]
		if ss.DegM != 1 || ss.DegN != 1 {
			continue
		}
		p := ss.PointAt(0, 0)
		n := ss.NormalAt(0, 0).WithMagnitude(1)
		d := n.Dot(p)
		if math.Abs(math.Abs(n.Dot(ttop.Minus(tbot).WithMagnitude(1)))-1) < 1e-9 {
			if math.Abs(onOrig.Plus(ttop).Dot(n)-d) < core.LengthEps {
				ss.Face = srf.FaceID(g.RemapEntity(core.NoEntity, RemapTop).V)
			}
			if math.Abs(onOrig.Plus(tbot).Dot(n)-d) < core.LengthEps {
				ss.Face = srf.FaceID(g.RemapEntity(core.NoEntity, RemapBottom).V)
			}
			continue
		}
		for ei := range sk.Entities {
			e := &sk.Entities[ei]
			if e.Group != g.OpA || e.Type != core.EntityLineSegment {
				continue
			}
			a := sk.MustGetEntity(e.Point[0]).PointGetNum(sk)
			b := sk.MustGetEntity(e.Point[1]).PointGetNum(sk)
			if math.Abs(n.Dot(a)-d) < core.LengthEps && math.Abs(n.Dot(b)-d) < core.LengthEps {
				ss.Face = srf.FaceID(g.RemapEntity(e.H, RemapLineToFace).V)
				break
			}
		}
	}
}
