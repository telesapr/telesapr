package api

import (
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// AnalyzeMenu holds the calls of the Analyze menu that measure what is
// selected or named. What measures a group's solid is on Group.Analyze.
// A script reaches it as doc.Analyze.
type AnalyzeMenu struct{ d *Doc }

// CurveLength is the total length in mm of the curves given, as Analyze >
// Measure Perimeter gives it: line segments, arcs, circles and cubics,
// each taken as the straight pieces it is drawn with.
func (m AnalyzeMenu) CurveLength(curves ...Entity) (mm float64) {
	d := m.d
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
func (m AnalyzeMenu) FacesArea(faces ...Entity) (mm2 float64) {
	d := m.d
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

// Measure measures what it is given, as the description of the
// selection and Analyze > Measure do, without adding a constraint: one
// line segment (its length) or circle (its radius); two points; a point
// and a circle, line segment or plane face; two lines, normals or
// plane faces (their angle in degrees); or two points and a line to
// project their distance on. Lengths are in mm. A face counts by its
// outward normal: a point outside the solid is at a positive distance
// from it, and two opposite faces are at 180 degrees.
func (m AnalyzeMenu) Measure(entities ...Entity) (out sketch.Measurement) {
	d := m.d
	d.read(func(sk *sketch.Sketch) {
		hs := make([]core.HEntity, len(entities))
		for i, e := range entities {
			hs[i] = e.get(sk).H
		}
		var err error
		if out, err = sk.Measure(hs...); err != nil {
			fail("%v", err)
		}
	})
	return out
}
