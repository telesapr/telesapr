package api

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// A linked part is placed by seven parameters of its group, a move and
// a turn (Group.Param 0 to 2 and 3 to 6, the turn a quaternion). The
// calls below change them as Edit > Rotate Imported 90° and dragging
// the part's normal in the window do; a point of the part held by a
// constraint places it too, with Entity.MoveTo or Constraint.

// placed is the group, which must be a link.
func (v LinkGroup) placed(sk *sketch.Sketch, call string) *sketch.Group {
	g := v.g
	group := g.get(sk)
	g.mustBe(group, sketch.GroupLinked, call)
	return group
}

// Offset is how far the part is moved from where its file puts it, in
// mm.
func (v LinkGroup) Offset() (offset Vec) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		group := v.placed(sk, "Offset")
		offset = Vec{
			X: sk.MustParam(group.H.Param(0)).Val,
			Y: sk.MustParam(group.H.Param(1)).Val,
			Z: sk.MustParam(group.H.Param(2)).Val,
		}
	})
	return offset
}

// Move moves the part by t, in mm, and regenerates it.
func (v LinkGroup) Move(t Vec) {
	checkFinite("a move", t)
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		group := v.placed(sk, "Move")
		group.TransformImportedBy(sk, t, geom.Quaternion{W: 1})
		sk.MarkGroupDirty(group.H, false)
	})
}

// Orientation is where the part's own x and y directions point now, as
// unit vectors.
func (v LinkGroup) Orientation() (u, w Vec) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		group := v.placed(sk, "Orientation")
		q := linkQuaternion(sk, group)
		u, w = q.RotationU(), q.RotationV()
	})
	return u, w
}

// linkQuaternion is the part's turn.
func linkQuaternion(sk *sketch.Sketch, group *sketch.Group) geom.Quaternion {
	val := func(i int) float64 { return sk.MustParam(group.H.Param(i)).Val }
	return geom.NewQuaternion(val(3), val(4), val(5), val(6))
}

// SetOrientation turns the part so that its own x and y directions
// point along u and w, which must not be parallel (w is made square to
// u), as dragging the part's normal in the window does.
func (v LinkGroup) SetOrientation(u, w Vec) {
	checkFinite("an orientation", u)
	checkFinite("an orientation", w)
	if u.Magnitude() == 0 || u.Cross(w).Magnitude() == 0 {
		fail("the directions of an orientation must differ")
	}
	u = u.WithMagnitude(1)
	w = w.Minus(u.ScaledBy(w.Dot(u))).WithMagnitude(1)
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		group := v.placed(sk, "SetOrientation")
		q := geom.QuaternionFromBasis(u, w)
		for i, c := range []float64{q.W, q.X, q.Y, q.Z} {
			sk.MustParam(group.H.Param(3 + i)).Val = c
		}
		sk.MarkGroupDirty(group.H, false)
	})
}

// Rotate turns the part by degrees about the axis through its own
// origin, on top of the turn it has, and regenerates it. The turn is
// counter-clockwise looking against the axis.
func (v LinkGroup) Rotate(axis Vec, degrees float64) {
	checkFinite("an axis", axis)
	if axis.Magnitude() == 0 {
		fail("a turn needs an axis, not the zero vector")
	}
	if math.IsNaN(degrees) || math.IsInf(degrees, 0) {
		fail("a turn of %g degrees", degrees)
	}
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		group := v.placed(sk, "Rotate")
		q := geom.QuaternionFromAxisAngle(axis.WithMagnitude(1), degrees*math.Pi/180)
		group.TransformImportedBy(sk, geom.Vector{}, q)
		sk.MarkGroupDirty(group.H, false)
	})
}

// Rotate90 is Edit > Rotate Imported 90°: it turns the part a quarter
// about the coordinate axis closest to axis, which the window takes to
// be the direction of the view. It is one step of undo.
func (v LinkGroup) Rotate90(axis Vec) {
	checkFinite("an axis", axis)
	if axis.Magnitude() == 0 {
		fail("a turn needs an axis, not the zero vector")
	}
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		group := v.placed(sk, "Rotate90")
		n := axis.ClosestOrtho().WithMagnitude(1)
		group.TransformImportedBy(sk, geom.Vector{}, geom.QuaternionFromAxisAngle(n, math.Pi/2))
		sk.MarkGroupDirty(group.H, false)
	})
}

// checkFinite stops the call when a vector holds a NaN or an infinity.
func checkFinite(what string, v Vec) {
	if s := v.X + v.Y + v.Z; math.IsNaN(s) || math.IsInf(s, 0) {
		fail("%s is a vector of numbers, not %v", what, v)
	}
}
