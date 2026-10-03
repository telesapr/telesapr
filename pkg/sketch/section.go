package sketch

import (
	"errors"
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// Section is the section view (an extension): the solid is drawn cut
// by a plane, the part on the side the plane's normal points to taken
// away and the cut capped. A face's normal is taken to point out of
// the solid. It is a way of looking at the model, as the
// view is, so it is neither saved nor undone.
type Section struct {
	// Plane is the workplane or planar face the cut is parallel to.
	Plane core.HEntity
	// Offset is how far the cut lies from Plane along its normal, as
	// typed: a number or an expression in the document's unit.
	Offset string
	// Flip takes away the other side.
	Flip bool
}

// SetSection turns the section view on, cutting parallel to plane, a
// workplane or a planar face, offset (typed, in the document's unit;
// "" is zero) along its normal, away from the side flip picks.
func (sk *Sketch) SetSection(plane core.HEntity, offset string, flip bool) error {
	e := sk.GetEntity(plane)
	if e == nil {
		return fmt.Errorf("no entity %08x", plane.V)
	}
	if !e.IsWorkplane() && !e.IsFace() {
		return errors.New("a section is cut parallel to a workplane or a planar face")
	}
	s := Section{Plane: plane, Offset: cleanText(offset), Flip: flip}
	if _, err := sk.sectionOffset(s.Offset); err != nil {
		return err
	}
	sk.Section = &s
	return nil
}

// ClearSection turns the section view off.
func (sk *Sketch) ClearSection() { sk.Section = nil }

// sectionOffset is typed offset text in millimetres.
func (sk *Sketch) sectionOffset(text string) (float64, error) {
	if text == "" {
		return 0, nil
	}
	e, _, err := sk.parseNamed(text)
	if err != nil {
		return 0, err
	}
	vars := make(map[string]float64, len(sk.Parameters))
	for _, p := range sk.Parameters {
		vars[p.Name] = p.Value
	}
	v := e.Eval(nil, vars) * sk.Units.MmPer()
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, fmt.Errorf("the offset '%s' is not a number", text)
	}
	return v, nil
}

// SectionCut is where the section view cuts now: the half space it
// keeps, and a point on its plane. ok is false when the view is off,
// and err says why when it is on but can't cut, as when its plane was
// deleted or its offset names a parameter that went away.
func (sk *Sketch) SectionCut() (keep srf.HalfSpace, origin geom.Vector, ok bool, err error) {
	s := sk.Section
	if s == nil {
		return keep, origin, false, nil
	}
	e := sk.GetEntity(s.Plane)
	var n geom.Vector
	switch {
	case e == nil:
		return keep, origin, false, errors.New("the section's plane is gone")
	case e.IsWorkplane():
		origin = e.WorkplaneGetOffset(sk)
		n = e.NormalEntity(sk).NormalN(sk)
	case e.IsFace():
		origin = e.FaceGetPointNum(sk)
		n = sk.OutwardFaceNormal(e)
	default:
		return keep, origin, false, errors.New("the section's plane is not a plane")
	}
	if n.Magnitude() < geom.LengthEps {
		return keep, origin, false, errors.New("the section's plane has no normal")
	}
	n = n.WithMagnitude(1)
	off, err := sk.sectionOffset(s.Offset)
	if err != nil {
		return keep, origin, false, err
	}
	origin = origin.Plus(n.ScaledBy(off))
	if s.Flip {
		n = n.ScaledBy(-1)
	}
	return srf.HalfSpace{N: n, D: n.Dot(origin)}, origin, true, nil
}

// OutwardFaceNormal is a face's normal, turned to point out of the
// solid, as its triangles' do. A face entity's own normal follows the
// sketch it came from, and points in on some faces of an extrusion; it
// stays so, since a constraint on a face, a distance from it, keeps its
// sign in the file, and turning the normal would move the point to the
// other side. What reports a direction to the user, the section view
// and Measure, takes this one (roadmap 14.5).
func (sk *Sketch) OutwardFaceNormal(e *core.Entity) geom.Vector {
	n := e.FaceGetNormalNum(sk)
	for _, hg := range []core.HGroup{sk.ActiveGroup(), e.Group} {
		g := sk.GetGroup(hg)
		if g == nil {
			continue
		}
		g.GenerateDisplayItems(sk)
		for i := range g.DisplayMesh.Triangles {
			tr := &g.DisplayMesh.Triangles[i]
			if uint32(tr.Meta.Face) != e.H.V {
				continue
			}
			if tr.Normal().Dot(n) < 0 {
				return n.ScaledBy(-1)
			}
			return n
		}
	}
	return n
}

// SectionPlaneName names the section's plane for the user: a
// workplane by its request or group, a face by its group.
func (sk *Sketch) SectionPlaneName() string {
	s := sk.Section
	if s == nil {
		return ""
	}
	e := sk.GetEntity(s.Plane)
	switch {
	case e == nil:
		return sk.entityName(s.Plane)
	case e.IsFace():
		return "face of " + sk.anyGroupName(e.Group)
	case e.IsWorkplane() && !s.Plane.IsFromRequest():
		return "workplane of " + sk.anyGroupName(e.Group)
	}
	return sk.entityName(s.Plane)
}
