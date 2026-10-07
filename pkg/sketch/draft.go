package sketch

import (
	"errors"
	"fmt"
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

// A draft may lean only some sides (DraftFaces), and may lean them from
// another plane than the sketch's, the neutral one, ValB along the
// extrusion from the sketch's plane (roadmap 28.2): there the solid
// has the profile's outline, and it narrows going away from it both
// ways, as a moulded part does from its parting line. The far end and
// the sides of an extrusion are its two prisms from the neutral plane,
// joined.

// DraftNeutral is how far along the extrusion, from the sketch's plane,
// the neutral plane of its draft lies, in mm.
func (g *Group) DraftNeutral() float64 { return g.ValB }

// draftAngles are the angles of the sides of loops: angle for a curve of
// one of DraftFaces, or of any when there are none, 0 for the rest.
func (g *Group) draftAngles(loops [][]srf.SBezier, angle float64) [][]float64 {
	chosen := map[uint32]bool{}
	for _, h := range g.DraftFaces {
		chosen[h.V] = true
	}
	out := make([][]float64, len(loops))
	for k, l := range loops {
		for _, sb := range l {
			a := angle
			if len(chosen) > 0 && !chosen[sb.Sheet] {
				a = 0
			}
			out[k] = append(out[k], a)
		}
	}
	return out
}

// SetDraftFaces makes the extrusion hg's draft lean the sides of the
// curves es of its sketch only, or every side when es is empty. A curve
// may be named by its entity in the sketch, by its copies at the
// extrusion's ends, or by its side's face.
func (sk *Sketch) SetDraftFaces(hg core.HGroup, es []core.HEntity) error {
	g := sk.GetGroup(hg)
	if g == nil || g.Type != GroupExtrude {
		return errors.New("only an extrusion has a draft")
	}
	var out []core.HEntity
	seen := map[core.HEntity]bool{}
	for _, he := range es {
		src, ok := sk.draftCurve(g, he)
		if !ok {
			return fmt.Errorf("%s is not a line, arc or circle of the extruded sketch, nor its side", sk.entityName(he))
		}
		if !seen[src] {
			seen[src] = true
			out = append(out, src)
		}
	}
	g.DraftFaces = out
	sk.MarkGroupDirty(hg, false)
	return nil
}

// draftCurve is the curve of g's sketch that he is or stands for: the
// curve itself, a copy of it at an end, or its side's face.
func (sk *Sketch) draftCurve(g *Group, he core.HEntity) (core.HEntity, bool) {
	curve := func(h core.HEntity) bool {
		e := sk.GetEntity(h)
		return e != nil && e.Group == g.OpA && !e.Construction &&
			(e.Type == core.EntityLineSegment || e.Type == core.EntityArcOfCircle || e.Type == core.EntityCircle)
	}
	if curve(he) {
		return he, true
	}
	for key, id := range g.Remap {
		if g.H.Entity(int(id.V)) == he && (key.CopyNumber == RemapBottom || key.CopyNumber == RemapTop || key.CopyNumber == RemapLineToFace) && curve(key.Input) {
			return key.Input, true
		}
	}
	return core.HEntity{}, false
}

// generateDraftShell makes the drafted extrusion of the source's loop
// sets from the sketch's plane to tbot and to ttop, or says why it
// can't and leaves the shell empty: two prisms from the neutral plane,
// one each way it reaches.
func (g *Group) generateDraftShell(src *Group, tbot, ttop geom.Vector) bool {
	angle := g.ValC * math.Pi / 180
	dir := ttop.Minus(tbot)
	if dir.Magnitude() < core.LengthEps {
		g.setProblem(nil, "the extrusion has no length")
		return false
	}
	dir = dir.WithMagnitude(1)
	lo, hi, s0 := tbot.Dot(dir), ttop.Dot(dir), g.ValB
	if s0 < lo-core.LengthEps || s0 > hi+core.LengthEps {
		g.setProblem(nil, "the draft's neutral plane, %g from the sketch, lies off the extrusion, which reaches from %g to %g", s0, lo, hi)
		return false
	}
	neutral := dir.ScaledBy(s0)
	var out srf.SShell
	first := true
	for k := range src.BezierLoops.Sets {
		set := &src.BezierLoops.Sets[k]
		loops := [][]srf.SBezier{translatedCurves(set.Outer.Curves, neutral)}
		for _, h := range set.Holes {
			loops = append(loops, translatedCurves(h.Curves, neutral))
		}
		angles := g.draftAngles(loops, angle)
		for _, along := range []geom.Vector{dir.ScaledBy(lo - s0), dir.ScaledBy(hi - s0)} {
			if along.Magnitude() < core.LengthEps {
				continue
			}
			sh, err := topo.DraftedPrismAngles(loops, angles, along, g.Color, 0)
			if err != nil {
				if se, ok := errors.AsType[*topo.ShellError](err); ok {
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

// translatedCurves are cs moved by by.
func translatedCurves(cs []srf.SBezier, by geom.Vector) []srf.SBezier {
	out := make([]srf.SBezier, len(cs))
	for i, c := range cs {
		for j := 0; j <= c.Degree; j++ {
			c.Ctrl[j] = c.Ctrl[j].Plus(by)
		}
		out[i] = c
	}
	return out
}
