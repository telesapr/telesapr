package api

import (
	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// GroupAnalysis measures and checks the solid up to and with a group,
// as the Analyze menu does for the active group: g.Analyze().
type GroupAnalysis struct{ g Group }

// Area is the area in mm² of the closed region the group's sketch
// draws, holes taken out, as Analyze > Measure Area gives it with
// nothing selected.
func (v GroupAnalysis) Area() (mm2 float64) {
	g := v.g
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
func (v GroupAnalysis) CenterOfMass() (p Vec) {
	g := v.g
	g.d.readMesh(func(sk *sketch.Sketch) {
		group := g.get(sk)
		displayMesh(sk, group)
		var err error
		if p, err = sk.CenterOfMass(group.H); err != nil {
			fail("%s: %v", groupDescription(g.get(sk)), err)
		}
	})
	return p
}

// MassProperties is the mass, centre of mass and inertia of the model
// up to and with this group, in g and mm, and of each of its parts, as
// Analyze > Measure Volume gives them. Each part is at its density
// (Group.SetDensity): a group assembled into the model is a part of its
// own, and the solid the groups before the first of them made is one at
// the density last set among them; a part with none set is taken at
// 1 g/cm³, which MassReport.DensityUnset says. The inertia tensors are
// about each one's centre of mass. They come from the solid's
// triangles, so a curved part is short as its mesh is, by the chord
// tolerance.
func (v GroupAnalysis) MassProperties() (r sketch.MassReport) {
	g := v.g
	g.d.readMesh(func(sk *sketch.Sketch) {
		group := g.get(sk)
		displayMesh(sk, group)
		var err error
		if r, err = sk.MassProperties(group.H); err != nil {
			fail("%s: %v", groupDescription(group), err)
		}
	})
	return r
}

// WallThickness measures the walls of the model up to and with this
// group, in mm, as Analyze > Clearance and Walls does: from the middle
// of each triangle of each part's mesh, square to it, into the solid to
// where it leaves the part again. It lists the points thinner than
// below, the thinnest first, or with a below of 0 only the thinnest
// (WallReport.Thinnest). Each part is measured alone, as Assembly.
// Interferences takes them. A spot thinner than the triangles about it
// can fall between them; WallReport.Tolerance is how far each
// thickness may be from the true one at its point.
func (v GroupAnalysis) WallThickness(below float64) (r sketch.WallReport) {
	g := v.g
	g.d.readMesh(func(sk *sketch.Sketch) {
		group := g.get(sk)
		var err error
		if r, err = sk.WallThickness(group.H, below); err != nil {
			fail("%s: %v", groupDescription(group), err)
		}
	})
	return r
}

// Mesh is the triangle mesh of the model up to and with this group.
// It is the sketch's own, and is replaced when the sketch
// regenerates.
func (v GroupAnalysis) Mesh() (m *srf.SMesh) {
	g := v.g
	g.d.readMesh(func(sk *sketch.Sketch) { m = displayMesh(sk, g.get(sk)) })
	return m
}

// Shell is the exact surfaces of the model up to and with this group;
// like Mesh, it is the sketch's own.
func (v GroupAnalysis) Shell() (s *srf.SShell) {
	g := v.g
	g.d.readMesh(func(sk *sketch.Sketch) { s = &g.get(sk).RunningShell })
	return s
}

// Volume is the volume in mm³ of the model up to and with this group.
func (v GroupAnalysis) Volume() (mm3 float64) {
	g := v.g
	g.d.readMesh(func(sk *sketch.Sketch) { mm3 = displayMesh(sk, g.get(sk)).CalculateVolume() })
	return mm3
}

// Triangles is how many triangles the model has up to and with this
// group.
func (v GroupAnalysis) Triangles() (n int) {
	g := v.g
	g.d.readMesh(func(sk *sketch.Sketch) { n = len(displayMesh(sk, g.get(sk)).Triangles) })
	return n
}

// Bounds are the corners of the box around the model up to and with
// this group.
func (v GroupAnalysis) Bounds() (lo, hi Vec) {
	g := v.g
	g.d.readMesh(func(sk *sketch.Sketch) {
		m := displayMesh(sk, g.get(sk))
		if m.IsEmpty() {
			return
		}
		hi, lo = m.Bounding()
	})
	return lo, hi
}

// Check checks the solid of the model up to and with this group: it
// reports naked and self-intersecting edges, whether the Boolean
// failed, the face and triangle counts, and the volume, centre of mass
// and bounding box, in mm. Report.Sound() says whether the solid is
// closed, does not cross itself, and came from a Boolean that worked,
// and is false too when the group added no solid for a Problem (as an
// extrusion of a sketch that crosses itself), which Report.Problem
// holds; Report.Summary() says it in words. Analyze > Check Solid shows the
// same report.
func (v GroupAnalysis) Check() (r check.Report) {
	g := v.g
	g.d.readMesh(func(sk *sketch.Sketch) { r = check.Group(sk, g.get(sk)) })
	return r
}

// RayCheck checks the solid of the model up to and with this group by
// the ray-cast oracle, which rebuilds it from the groups' definitions
// (profiles, extrusions, axes, a sweep's steps, where step and repeats,
// patterns along a path and mirrors put their copies) without the
// kernel's surface intersection: its volume, centre of mass and inertia
// tensor about it (Measure.Inertia, for a density of one, in mm⁵;
// Measure.PrincipalMoments) by three grids of rays, the rays along
// which it and the kernel's mesh disagree by more than twice the chord
// tolerance, and the mesh's half-edges (each edge used twice, once each
// way). It fails for a model with a group the oracle can't build, as a
// fillet or a loft: extrusions, drafted or not, lathes, revolves,
// helices, sweeps along lines and arcs, and copies of them only.
// Analyze > Ray-Cast Check shows the same.
func (v GroupAnalysis) RayCheck() (r check.RayReport) {
	g := v.g
	g.d.readMesh(func(sk *sketch.Sketch) {
		var err error
		if r, err = check.RayCheck(sk, g.get(sk), 24); err != nil {
			fail("%v", err)
		}
	})
	return r
}

// Probe is where the line through from along dir passes through the
// solid of the model up to and with this group, by the ray-cast oracle
// (GroupAnalysis.RayCheck): the stretches inside it, in the line's parameter,
// dir's length to one. It fails as RayCheck does.
func (v GroupAnalysis) Probe(from, dir Vec) (inside []check.Interval) {
	g := v.g
	g.d.readMesh(func(sk *sketch.Sketch) {
		o, err := check.NewOracle(sk, g.get(sk).H)
		if err != nil {
			fail("%v", err)
		}
		if o != nil {
			inside = o.Probe(from, dir)
		}
	})
	return inside
}
