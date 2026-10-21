package sketch

import (
	"errors"
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// Mass properties (item 13 of doc/notes/modelling_upgrades.md). A
// density belongs to a part, not to a feature: once a Boolean has
// joined two features' solids, nothing says which volume was whose. So
// a part is what the interference check takes for one (assemblyParts):
// each group assembled into the model is a part at its own density,
// and the solid the groups before the first of them made is a part at
// the density last set among those groups. A model that assembles
// nothing is one part so. A part with no density set is taken at
// DefaultDensity, and says so.

// DefaultDensity is the density, in g/cm³, of a part none is set for:
// water's, so that its mass in g is its volume in cm³.
const DefaultDensity = 1.0

// ErrBadDensity is a density that is not a positive number.
var ErrBadDensity = errors.New("a density is a positive number of g/cm³, or 0 for none")

// HasDensity reports whether the group's solid can be given a density:
// a group whose solid joins the model, or is assembled into it, as a
// part of its own. A fillet or a shell edits a solid that has one
// already, and a step and repeat copies it.
func (g *Group) HasDensity() bool { return g.HasCombine() }

// SetGroupDensity sets the density of the group's solid, in g/cm³, or
// clears it with 0. The caller remembers the undo state. It changes no
// geometry, so nothing regenerates.
func (sk *Sketch) SetGroupDensity(hg core.HGroup, gPerCm3 float64) error {
	g := sk.Group(hg)
	if g == nil {
		return fmt.Errorf("no group %08x", hg.V)
	}
	if !g.HasDensity() {
		return errors.New("this kind of group makes no part of its own to give a density")
	}
	if math.IsNaN(gPerCm3) || math.IsInf(gPerCm3, 0) || gPerCm3 < 0 {
		return ErrBadDensity
	}
	g.Density = gPerCm3
	return nil
}

// PartMass is the mass properties of one part. Lengths are mm, masses
// g.
type PartMass struct {
	// Group is the group that made the part: an assembled group, or
	// the last group of the solid before the first of them.
	Group core.HGroup
	// Density is what the part is taken at, in g/cm³; DensitySet is
	// false when it is DefaultDensity because none is set.
	Density    float64
	DensitySet bool
	// Volume in mm³ and Mass in g.
	Volume, Mass float64
	CenterOfMass geom.Vector
	// Inertia is the inertia tensor about the part's centre of mass, in
	// g·mm²: on the diagonal the moments about the axes through it
	// along x, y and z, off it the products of inertia, negated.
	Inertia [3][3]float64
}

// MassReport is the mass properties of the model up to a group: its
// parts', and the whole's, the inertia about the whole's centre of
// mass.
type MassReport struct {
	Parts        []PartMass
	Volume, Mass float64
	CenterOfMass geom.Vector
	Inertia      [3][3]float64
}

// DensityUnset reports whether some part is taken at DefaultDensity
// because none is set for it.
func (r MassReport) DensityUnset() bool {
	for _, p := range r.Parts {
		if !p.DensitySet {
			return true
		}
	}
	return false
}

// MassProperties are the mass properties of the model up to group upTo,
// from the triangles of its parts' solids, so a curved part is short
// of its volume as its mesh is, by the chord tolerance. The model must
// have been generated; the group's mesh to display is made if it is
// not.
func (sk *Sketch) MassProperties(upTo core.HGroup) (MassReport, error) {
	g := sk.Group(upTo)
	if g == nil {
		return MassReport{}, fmt.Errorf("g%03x doesn't exist", upTo.V)
	}
	parts, meshOnly, before := sk.assemblyParts(upTo)
	var r MassReport
	if len(parts) == 0 && len(meshOnly) == 0 {
		g.GenerateDisplayItems(sk)
		if g.DisplayMesh.IsEmpty() {
			return MassReport{}, errors.New("there is no solid at the group")
		}
		r.Parts = append(r.Parts, partMass(upTo, &g.DisplayMesh, sk.densityThrough(upTo)))
	} else {
		baseFound := false
		for _, p := range parts {
			var m srf.SMesh
			p.shell.TriangulateInto(&m)
			d := sk.Group(p.g).Density
			if before != nil && p.g == before.H {
				d = sk.densityThrough(p.g)
				baseFound = true
			}
			r.Parts = append(r.Parts, partMass(p.g, &m, d))
		}
		// A solid before the assembly that was made as triangles has
		// no shell to be a part by.
		if before != nil && !baseFound && !before.RunningMesh.IsEmpty() {
			base := partMass(before.H, &before.RunningMesh, sk.densityThrough(before.H))
			r.Parts = append([]PartMass{base}, r.Parts...)
		}
		for _, hg := range meshOnly {
			mg := sk.Group(hg)
			r.Parts = append(r.Parts, partMass(hg, &mg.ThisMesh, mg.Density))
		}
	}
	r.combine()
	return r, nil
}

// densityThrough is the density last set among the groups up to and
// with hg that are left in the model, 0 when there is none.
func (sk *Sketch) densityThrough(hg core.HGroup) float64 {
	d := 0.0
	for _, h := range sk.GroupOrder {
		g := sk.Group(h)
		if g != nil && !g.Suppress && sk.SuppressedReason(h) == "" && g.Density > 0 {
			d = g.Density
		}
		if h == hg {
			break
		}
	}
	return d
}

// partMass is the part of group hg whose solid's triangles are m, at
// density d in g/cm³, or DefaultDensity for a d of 0.
func partMass(hg core.HGroup, m *srf.SMesh, d float64) PartMass {
	p := PartMass{Group: hg, Density: d, DensitySet: d > 0}
	if d == 0 {
		p.Density = DefaultDensity
	}
	mm := m.Mass()
	// g/cm³ is 1e-3 g/mm³.
	k := p.Density / 1000
	p.Volume, p.Mass, p.CenterOfMass = mm.Volume, mm.Volume*k, mm.CenterOfMass
	for a := range 3 {
		for b := range 3 {
			p.Inertia[a][b] = mm.Inertia[a][b] * k
		}
	}
	return p
}

// combine sums the parts: the whole's mass, its centre of mass, and its
// inertia about that centre, each part's moved there by the parallel
// axis theorem.
func (r *MassReport) combine() {
	var moment geom.Vector
	for _, p := range r.Parts {
		r.Volume += p.Volume
		r.Mass += p.Mass
		moment = moment.Plus(p.CenterOfMass.ScaledBy(p.Mass))
	}
	if r.Mass == 0 {
		return
	}
	r.CenterOfMass = moment.ScaledBy(1 / r.Mass)
	for _, p := range r.Parts {
		d := p.CenterOfMass.Minus(r.CenterOfMass)
		dd := d.Dot(d)
		for a := range 3 {
			for b := range 3 {
				x := -d.Element(a) * d.Element(b)
				if a == b {
					x += dd
				}
				r.Inertia[a][b] += p.Inertia[a][b] + p.Mass*x
			}
		}
	}
}
