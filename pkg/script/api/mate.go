package api

import (
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// Mates (roadmap 16.9, G1) are shortcuts that put two parts together
// with the constraints there are: what each makes is ordinary
// constraints, listed, edited and deleted as any. They are made in the
// active group, which places one of the parts, as a link group does.

// Concentric puts two circles or arcs on one axis. Drawing free in 3d,
// their axes are made parallel and the second's centre put on the
// first's axis, by its distance from the first's centre along two
// directions square to the axis, each zero; one part may still turn
// about the axis and slide along it, as a pin in a hole. The directions
// are lines or normals of the first circle's group, or the reference
// axes; with none square to the axis, the centres are put on each
// other. In a workplane, the centres are put on each other.
func (d *Doc) Concentric(a, b Entity) []Constraint {
	var ca, cb Entity
	var across []core.HEntity
	free := false
	d.read(func(sk *sketch.Sketch) {
		for _, e := range []Entity{a, b} {
			if t := e.get(sk).Type; t != core.EntityCircle && t != core.EntityArcOfCircle {
				fail("a concentric mate is of two circles or arcs, and %s isn't one", e.describe(sk))
			}
		}
		ca, cb = a.Center(), b.Center()
		free = core.IsFreeIn3D(sk.ActiveWorkplane())
		ea := a.get(sk)
		axis := ea.NormalEntity(sk).NormalN(sk).WithMagnitude(1)
		for i := range sk.Entities {
			e := &sk.Entities[i]
			if e.Group != ea.Group && e.Group.V != core.HGroupReferences || !e.HasVector() {
				continue
			}
			v := e.VectorGetNum(sk)
			if v.Magnitude() < core.LengthEps || math.Abs(v.WithMagnitude(1).Dot(axis)) > 1e-6 {
				continue
			}
			if len(across) == 1 {
				u := sk.GetEntity(across[0]).VectorGetNum(sk).WithMagnitude(1)
				if u.Cross(v.WithMagnitude(1)).Magnitude() < 0.1 {
					continue
				}
			}
			across = append(across, e.H)
			if len(across) == 2 {
				break
			}
		}
	})
	if !free {
		return []Constraint{d.Coincident(ca, cb)}
	}
	// The axes parallel first, as for Flush.
	out := d.addUnlessRedundant(nil, func() Constraint { return d.Parallel(a.Normal(), b.Normal()) })
	if len(across) < 2 {
		return append(out, d.Coincident(ca, cb))
	}
	for _, dir := range across {
		out = append(out, d.constrain(func(sk *sketch.Sketch, c *core.Constraint) {
			c.Type = core.ConstraintProjPtDistance
			c.Workplane = core.NoEntity // free in 3d
			c.PtA, c.PtB, c.EntityA = ca.H, cb.H, dir
		}))
	}
	return out
}

// Flush makes two faces parallel and puts a point of the first, one
// lying on it, on the second: the faces lie in one plane, facing as
// they did, and one part may still slide and turn on the other.
func (d *Doc) Flush(faceA, faceB Entity) []Constraint {
	var on Entity
	d.read(func(sk *sketch.Sketch) {
		fa := faceA.get(sk)
		for _, e := range []Entity{faceA, faceB} {
			if !e.get(sk).IsFace() {
				fail("a flush mate is of two faces, and %s isn't one", e.describe(sk))
			}
		}
		// A point of the first face's group, lying on it.
		p0, n := fa.FaceGetPointNum(sk), fa.FaceGetNormalNum(sk).WithMagnitude(1)
		for i := range sk.Entities {
			e := &sk.Entities[i]
			if e.Group == fa.Group && e.IsPoint() && math.Abs(e.PointGetNum(sk).Minus(p0).Dot(n)) < core.LengthEps {
				on = Entity{d, e.H}
				return
			}
		}
		fail("%s has no point lying on it to put on the other face", faceA.describe(sk))
	})
	// The faces parallel first: a point put on the plane first would
	// tilt the part to reach it, and the solver might not turn it back.
	out := d.addUnlessRedundant(nil, func() Constraint {
		// Parallel takes faces as vectors, their normals, though the
		// interface offers it for lines and normals only.
		return d.constrain(func(sk *sketch.Sketch, c *core.Constraint) {
			c.Type = core.ConstraintParallel
			c.Workplane = core.NoEntity // free in 3d
			c.EntityA, c.EntityB = faceA.H, faceB.H
		})
	})
	return append(out, d.On(on, faceB))
}

// Hinge puts the start of line b on the start of line a, and the end of
// b on a's line: b may only turn about a, as on a pin.
func (d *Doc) Hinge(a, b Entity) []Constraint {
	d.read(func(sk *sketch.Sketch) {
		for _, e := range []Entity{a, b} {
			if e.get(sk).Type != core.EntityLineSegment {
				fail("a hinge is of two line segments, and %s isn't one", e.describe(sk))
			}
		}
	})
	out := []Constraint{d.Coincident(a.Start(), b.Start())}
	return d.addUnlessRedundant(out, func() Constraint { return d.On(b.End(), a) })
}

// addUnlessRedundant adds the constraint make makes to out, unless the
// active group no longer solves with it, as when what it asks holds
// already: then it is deleted again.
func (d *Doc) addUnlessRedundant(out []Constraint, make func() Constraint) []Constraint {
	c := make()
	d.Regen()
	if d.Active().Solved() {
		return append(out, c)
	}
	c.Delete()
	d.Regen()
	return out
}
