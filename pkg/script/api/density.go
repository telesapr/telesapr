package api

import (
	"github.com/telesapr/telesapr/pkg/sketch"
)

// Density is the density of the group's solid in g/cm³, 0 when none is
// set; GroupAnalysis.MassProperties then takes the part at 1.
func (g Group) Density() (gPerCm3 float64) {
	g.d.read(func(sk *sketch.Sketch) { gPerCm3 = g.get(sk).Density })
	return gPerCm3
}

// SetDensity sets the density of the group's solid in g/cm³ (steel is
// about 7.85, aluminium 2.7, PLA 1.24), or clears it with 0, as the
// group screen's density does. A part of an assembly takes its own
// group's density; a solid made of several features takes the one last
// set among them. Only a group whose solid joins the model or is
// assembled into it has one: an extrusion, lathe, revolve, helix,
// sweep, loft or link. It fails for another, and for a value below 0.
func (g Group) SetDensity(gPerCm3 float64) {
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		if err := sk.SetGroupDensity(g.H, gPerCm3); err != nil {
			fail("%s: %v", groupDescription(group), err)
		}
	})
}
