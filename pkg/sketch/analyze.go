package sketch

import (
	"errors"
	"fmt"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// The measurements of the Analyze menu that read a group's solid or its
// sketch. Curves are taken as the straight pieces they are drawn with,
// at the sketch's chord tolerance, so a curved length or area is short
// by around one percent.

// ErrNoClosedRegion is returned when a group's sketch is not a closed
// region whose area can be taken.
var ErrNoClosedRegion = errors.New("the group's sketch is not a closed region: it is open, not in one plane, or crosses itself")

// Length is the total length of the curves of entities hs: line
// segments, arcs, circles, cubics and text.
func (sk *Sketch) Length(hs []core.HEntity) (float64, error) {
	if len(hs) == 0 {
		return 0, errors.New("there are no curves to measure")
	}
	total := 0.0
	for _, he := range hs {
		e := sk.Entity(he)
		if e == nil {
			return 0, fmt.Errorf("e%03x doesn't exist", he.V)
		}
		var sbl srf.SBezierList
		GenerateEntityBeziers(e, &sbl, &Runtime{Sketch: sk})
		if len(sbl.Curves) == 0 {
			return 0, fmt.Errorf("e%03x is no curve, and has no length", he.V)
		}
		for i := range sbl.Curves {
			var pts []geom.Vector
			sbl.Curves[i].MakePwlIntoPoints(&pts, sk.Tol(), 0)
			for j := 1; j < len(pts); j++ {
				total += pts[j].Minus(pts[j-1]).Magnitude()
			}
		}
	}
	return total, nil
}

// RegionArea is the area of the closed region group hg's sketch draws,
// holes taken out.
func (sk *Sketch) RegionArea(hg core.HGroup) (float64, error) {
	g := sk.Group(hg)
	if g == nil {
		return 0, fmt.Errorf("g%03x doesn't exist", hg.V)
	}
	if g.PolyError.How != int(core.PolyGood) {
		return 0, ErrNoClosedRegion
	}
	var el srf.SEdgeList
	g.PolyLoops.MakeEdgesInto(&el)
	var sp srf.SPolygon
	el.AssemblePolygon(&sp, nil, true)
	sp.Normal = sp.ComputeNormal()
	sp.FixContourDirections()
	return sp.SignedArea(), nil
}

// FacesArea is the area of the faces of group hg's solid that the face
// entities hs stand for.
func (sk *Sketch) FacesArea(hg core.HGroup, hs []core.HEntity) (float64, error) {
	g := sk.Group(hg)
	if g == nil {
		return 0, fmt.Errorf("g%03x doesn't exist", hg.V)
	}
	faces := make([]uint32, 0, len(hs))
	for _, he := range hs {
		if e := sk.Entity(he); e == nil || !e.IsFace() {
			return 0, fmt.Errorf("e%03x is no face of the solid", he.V)
		}
		faces = append(faces, he.V)
	}
	return g.DisplayMesh.CalculateSurfaceArea(faces), nil
}

// CenterOfMass is the centre of mass of the solid at group hg, taken
// as of one density throughout.
func (sk *Sketch) CenterOfMass(hg core.HGroup) (geom.Vector, error) {
	g := sk.Group(hg)
	if g == nil {
		return geom.Vector{}, fmt.Errorf("g%03x doesn't exist", hg.V)
	}
	if g.DisplayMesh.IsEmpty() {
		return geom.Vector{}, errors.New("there is no solid at the group")
	}
	return g.DisplayMesh.CenterOfMass(), nil
}
