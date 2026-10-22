package ui

import (
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A screw thread on a drawing is drawn by the drafting convention (ISO
// 6410), not turn by turn: the thread is a plain cylinder at its crest,
// its outline and its ends in solid lines; the root is two thin lines
// along it in a view across the axis, and three quarters of a thin
// circle in a view along it. A screw's root lies inside the crest's
// cylinder and is shown all the same; a nut's lies in the part and is
// hidden, like any edge behind material.

// threadTol is how near the band of a thread a point is taken to lie
// in it, in mm.
const threadTol = 1e-3

// threadSides are the segments a thread's cylinder and circles are
// drawn with.
const threadSides = 72

// threadConvention takes the turns of the threads bands out of the
// solid's edges sel and its mesh m, and returns the mesh with each
// thread a cylinder at its crest in their place, the edges to add that
// the solid hides as it hides its own (the thread's ends and a nut's
// root), and those it doesn't (a screw's root). right and up are the
// view's x and y.
func threadConvention(sel *srf.SEdgeList, m *srf.SMesh, bands []sketch.ThreadBand, right, up geom.Vector) (srf.SMesh, srf.SEdgeList, srf.SEdgeList) {
	n := right.Cross(up).WithMagnitude(1)
	in := func(p geom.Vector) bool {
		for _, b := range bands {
			if b.Holds(p, threadTol) {
				return true
			}
		}
		return false
	}
	kept := sel.Edges[:0]
	for _, e := range sel.Edges {
		if !in(e.A) || !in(e.B) {
			kept = append(kept, e)
		}
	}
	sel.Edges = kept

	var out srf.SMesh
	for _, tr := range m.Triangles {
		if !in(tr.A) || !in(tr.B) || !in(tr.C) {
			out.AddTriangle(tr)
		}
	}
	var extra, always srf.SEdgeList
	for _, b := range bands {
		u, v := b.Axis.NormalDir(0), b.Axis.NormalDir(1)
		at := func(r, a, t float64) geom.Vector {
			return b.Origin.Plus(u.ScaledBy(r * math.Cos(a))).Plus(v.ScaledBy(r * math.Sin(a))).Plus(b.Axis.ScaledBy(t))
		}
		endOn := math.Abs(n.Dot(b.Axis)) > 1-1e-6
		circled := false
		for _, e := range sel.Edges {
			if b.OnCylinder(e.A, b.Crest, threadTol) && b.OnCylinder(e.B, b.Crest, threadTol) {
				circled = true
				break
			}
		}
		// The crest's cylinder, facing out of the material: away from
		// the axis on a screw, towards it in a nut.
		for i := range threadSides {
			a0, a1 := 2*math.Pi*float64(i)/threadSides, 2*math.Pi*float64(i+1)/threadSides
			p00, p01 := at(b.Crest, a0, 0), at(b.Crest, a1, 0)
			p10, p11 := at(b.Crest, a0, b.Length), at(b.Crest, a1, b.Length)
			out0 := at(b.Crest, (a0+a1)/2, 0).Minus(b.Origin)
			if b.Internal {
				out0 = out0.ScaledBy(-1)
			}
			out.AddTriangleN(srf.STriMeta{}, out0, p00, p01, p11)
			out.AddTriangleN(srf.STriMeta{}, out0, p00, p11, p10)
			// The thread's ends; looking along the axis, not where the
			// solid has a circle of the crest's size already, as a rod
			// as thick as its thread does at its far end, which would
			// draw two outlines a chord apart.
			if !endOn || !circled {
				extra.AddEdge(p00, p01, core.StyleSolidEdge, 0, 0)
				extra.AddEdge(p10, p11, core.StyleSolidEdge, 0, 0)
			}
		}

		// The root: along a view across the axis, two lines; looking
		// along it, three quarters of a circle at the end nearer the
		// viewer, a little in front of it, so that its own face doesn't
		// hide it.
		root := &extra
		if !b.Internal {
			root = &always
		}
		switch along := n.Dot(b.Axis); {
		case math.Abs(along) < 1e-6:
			side := n.Cross(b.Axis).WithMagnitude(b.Root)
			for _, s := range []float64{-1, 1} {
				root.AddEdge(b.Origin.Plus(side.ScaledBy(s)), b.Origin.Plus(side.ScaledBy(s)).Plus(b.Axis.ScaledBy(b.Length)),
					core.StyleDimSolid, 0, 0)
			}
		case math.Abs(along) > 1-1e-6:
			end, toward := 0.0, -threadTol
			if along > 0 {
				end, toward = b.Length, threadTol
			}
			// The quarter left open is the one up and to the right of
			// the view, as drafting leaves it.
			c := b.Origin.Plus(b.Axis.ScaledBy(end + toward))
			onRoot := func(a float64) geom.Vector {
				return c.Plus(right.WithMagnitude(b.Root * math.Cos(a))).Plus(up.WithMagnitude(b.Root * math.Sin(a)))
			}
			for i := range threadSides * 3 / 4 {
				a0, a1 := 2*math.Pi*float64(i)/threadSides, 2*math.Pi*float64(i+1)/threadSides
				root.AddEdge(onRoot(a0+math.Pi/2), onRoot(a1+math.Pi/2), core.StyleDimSolid, 0, 0)
			}
		}
	}
	return out, extra, always
}
