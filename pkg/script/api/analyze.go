package api

import (
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// CurveLength is the total length in mm of the curves given, as Analyze >
// Measure Perimeter gives it: line segments, arcs, circles and cubics,
// each taken as the straight pieces it is drawn with.
func (d *Doc) CurveLength(curves ...Entity) (mm float64) {
	d.read(func(sk *sketch.Sketch) {
		hs := make([]core.HEntity, len(curves))
		for i, e := range curves {
			hs[i] = e.get(sk).H
		}
		var err error
		if mm, err = sk.Length(hs); err != nil {
			fail("%v", err)
		}
	})
	return mm
}

// FacesArea is the area in mm² of faces of the solid at the active
// group, as Analyze > Measure Area gives it with them selected: the
// face entities an extrusion or a lathe makes.
func (d *Doc) FacesArea(faces ...Entity) (mm2 float64) {
	d.read(func(sk *sketch.Sketch) {
		g := sk.Group(sk.ActiveGroup())
		displayMesh(sk, g)
		hs := make([]core.HEntity, len(faces))
		for i, e := range faces {
			hs[i] = e.get(sk).H
		}
		var err error
		if mm2, err = sk.FacesArea(g.H, hs); err != nil {
			fail("%v", err)
		}
	})
	return mm2
}

// Area is the area in mm² of the closed region the group's sketch
// draws, holes taken out, as Analyze > Measure Area gives it with
// nothing selected.
func (g Group) Area() (mm2 float64) {
	g.d.read(func(sk *sketch.Sketch) {
		var err error
		if mm2, err = sk.RegionArea(g.get(sk).H); err != nil {
			fail("%s: %v", groupDescription(g.get(sk)), err)
		}
	})
	return mm2
}

// CenterOfMass is the centre of mass of the model up to and with this
// group, of one density throughout, as Analyze > Show Center of Mass
// marks it.
func (g Group) CenterOfMass() (p Vec) {
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		displayMesh(sk, group)
		var err error
		if p, err = sk.CenterOfMass(group.H); err != nil {
			fail("%s: %v", groupDescription(g.get(sk)), err)
		}
	})
	return p
}
