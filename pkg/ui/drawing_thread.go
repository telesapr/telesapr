package ui

import (
	"math"
	"slices"

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
// root), and those it doesn't (a screw's root). What another group
// made in a band, as a hole across the thread, stays: its edges are
// taken out onto the crest's cylinder, where the convention draws the
// thread's surface. right and up are the view's x and y.
func threadConvention(sel *srf.SEdgeList, m *srf.SMesh, bands []sketch.ThreadBand, right, up geom.Vector) (srf.SMesh, srf.SEdgeList, srf.SEdgeList) {
	n := right.Cross(up).WithMagnitude(1)
	// bandOf is the band p lies in, or -1.
	bandOf := func(p geom.Vector) int {
		for i, b := range bands {
			if b.Holds(p, threadTol) {
				return i
			}
		}
		return -1
	}
	// threads tells whether a triangle is a thread's: in its band and
	// made by its groups.
	threads := func(tr srf.STriangle) bool {
		i := bandOf(tr.A)
		return i >= 0 && bandOf(tr.B) == i && bandOf(tr.C) == i && bands[i].Makes(uint32(tr.Meta.Face))
	}
	var out srf.SMesh
	// own are the corners in a band of what the part has there of its
	// own, to a micrometre: an outline's ends are its mesh's corners,
	// but not always to the last bit.
	type corner [3]int64
	at := func(p geom.Vector) corner {
		return corner{int64(math.Round(p.X * 1e6)), int64(math.Round(p.Y * 1e6)), int64(math.Round(p.Z * 1e6))}
	}
	own := map[corner]bool{}
	for _, tr := range m.Triangles {
		if threads(tr) {
			continue
		}
		out.AddTriangle(tr)
		for _, p := range []geom.Vector{tr.A, tr.B, tr.C} {
			if bandOf(p) >= 0 {
				own[at(p)] = true
			}
		}
	}
	kept := sel.Edges[:0]
	for _, e := range sel.Edges {
		ia, ib := bandOf(e.A), bandOf(e.B)
		switch {
		case ia < 0 || ib < 0:
			kept = append(kept, e)
		// The part's own, but for its faces at the band's ends, whose
		// edges with the thread are the thread's ends, drawn below.
		case own[at(e.A)] && own[at(e.B)] && within(bands[ia], e.A) && within(bands[ib], e.B):
			e.A, e.B = onCrest(bands[ia], e.A), onCrest(bands[ib], e.B)
			kept = append(kept, e)
		}
	}
	sel.Edges = kept

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
		case math.Abs(along) <= 1-1e-6:
			// Across the axis, or askew to it as in the isometric view,
			// two lines where the root's cylinder turns away; with a
			// run-out they slant out to the crest at the thread's end.
			side := n.Cross(b.Axis).WithMagnitude(1)
			full := b.Length - b.RunOut
			for _, s := range []float64{-1, 1} {
				at := func(r, t float64) geom.Vector {
					return b.Origin.Plus(side.ScaledBy(s * r)).Plus(b.Axis.ScaledBy(t))
				}
				root.AddEdge(at(b.Root, 0), at(b.Root, full), core.StyleDimSolid, 0, 0)
				if b.RunOut > 0 {
					root.AddEdge(at(b.Root, full), at(b.Crest, b.Length), core.StyleDimSolid, 0, 0)
				}
			}
		default:
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

// within reports whether p lies in the band short of its ends.
func within(b sketch.ThreadBand, p geom.Vector) bool {
	t := p.Minus(b.Origin).Dot(b.Axis)
	return t > threadTol && t < b.Length-threadTol
}

// onCrest is p taken out, or in, onto the crest's cylinder as the
// drawing has it, of threadSides flat sides: just outside them on a
// screw and inside them in a nut, so that they hide no edge on it.
func onCrest(b sketch.ThreadBand, p geom.Vector) geom.Vector {
	q := b.ToCrest(p)
	k := (1 + 1e-6) / math.Cos(math.Pi/threadSides)
	if b.Internal {
		k = (1 - 1e-6) * math.Cos(math.Pi/threadSides)
	}
	d := q.Minus(b.Origin)
	along := b.Axis.ScaledBy(d.Dot(b.Axis))
	return b.Origin.Plus(along).Plus(d.Minus(along).ScaledBy(k))
}

// threadCut gives a section's cut face, poly in the plane with normal
// n, the convention's shape: what of it lies in a thread's band is
// taken out to the crest, so the face, and the hatch across it, reach
// the crest as drafting has them. The plane is along the axis or
// across it, as a drawing's sections are; one askew is left as it is.
func threadCut(poly *srf.SPolygon, bands []sketch.ThreadBand, n geom.Vector) {
	for ci := range poly.Contours {
		c := &poly.Contours[ci]
		for i := range c.Points {
			p := c.Points[i].P
			for _, b := range bands {
				if b.Holds(p, threadTol) {
					c.Points[i].P = crestInPlane(b, p, n)
					break
				}
			}
		}
		c.Points = simplified(c.Points)
	}
	poly.Contours = slices.DeleteFunc(poly.Contours, func(c srf.SContour) bool { return len(c.Points) < 3 })
}

// crestInPlane is p, in a plane with normal n along the band's axis or
// across it, moved in that plane onto the crest's cylinder.
func crestInPlane(b sketch.ThreadBand, p, n geom.Vector) geom.Vector {
	d := p.Minus(b.Origin)
	t := d.Dot(b.Axis)
	w := d.Minus(b.Axis.ScaledBy(t))
	switch along := math.Abs(n.Dot(b.Axis)); {
	case along > 1-1e-6:
		return b.ToCrest(p)
	case along < 1e-6:
		s := n.Cross(b.Axis).WithMagnitude(1)
		x, h := w.Dot(s), w.Dot(n)
		if b.Crest*b.Crest <= h*h {
			return p
		}
		x = math.Copysign(math.Sqrt(b.Crest*b.Crest-h*h), x)
		return b.Origin.Plus(b.Axis.ScaledBy(t)).Plus(s.ScaledBy(x)).Plus(n.ScaledBy(h))
	}
	return p
}

// simplified is a contour without points that repeat the one before or
// lie on the line between their neighbours, which the cut's points
// taken out to the crest leave many of.
func simplified(pts []srf.SPoint) []srf.SPoint {
	const eps = 1e-9
	out := make([]srf.SPoint, 0, len(pts))
	for _, p := range pts {
		if len(out) > 0 && p.P.Minus(out[len(out)-1].P).Magnitude() < eps {
			continue
		}
		out = append(out, p)
	}
	for len(out) > 1 && out[0].P.Minus(out[len(out)-1].P).Magnitude() < eps {
		out = out[:len(out)-1]
	}
	for changed := true; changed && len(out) >= 3; {
		changed = false
		for i := 0; i < len(out) && len(out) >= 3; i++ {
			a, b, c := out[(i+len(out)-1)%len(out)].P, out[i].P, out[(i+1)%len(out)].P
			if b.Minus(a).Cross(c.Minus(b)).Magnitude() < eps*max(1, c.Minus(a).Magnitude()) {
				out = slices.Delete(out, i, i+1)
				changed = true
			}
		}
	}
	return out
}

// threadCutRoots adds the roots of the threads the plane keep cuts, on
// the cut face, thin, as drafting has them: two lines along the axis
// for a plane along it, slanting out with a run-out; three quarters of
// a circle for one across it.
func threadCutRoots(el *srf.SEdgeList, bands []sketch.ThreadBand, keep srf.HalfSpace) {
	n := keep.N.WithMagnitude(1)
	for _, b := range bands {
		switch along := n.Dot(b.Axis); {
		case math.Abs(along) < 1e-6:
			s := n.Cross(b.Axis).WithMagnitude(1)
			h := keep.D - n.Dot(b.Origin)
			full := b.Length - b.RunOut
			for _, side := range []float64{-1, 1} {
				on := func(r, t float64) (geom.Vector, bool) {
					if r*r <= h*h {
						return geom.Vector{}, false
					}
					return b.Origin.Plus(n.ScaledBy(h)).Plus(s.ScaledBy(side * math.Sqrt(r*r-h*h))).Plus(b.Axis.ScaledBy(t)), true
				}
				p0, ok0 := on(b.Root, 0)
				p1, ok1 := on(b.Root, full)
				if !ok0 || !ok1 {
					continue
				}
				el.AddEdge(p0, p1, core.StyleDimSolid, 0, 0)
				if p2, ok := on(b.Crest, b.Length); ok && b.RunOut > 0 {
					el.AddEdge(p1, p2, core.StyleDimSolid, 0, 0)
				}
			}
		case math.Abs(along) > 1-1e-6:
			t := (keep.D - n.Dot(b.Origin)) / along
			if t < 0 || t > b.Length-b.RunOut {
				continue
			}
			c := b.Origin.Plus(b.Axis.ScaledBy(t))
			u, v := b.Axis.NormalDir(0).WithMagnitude(b.Root), b.Axis.NormalDir(1).WithMagnitude(b.Root)
			for i := range threadSides * 3 / 4 {
				a0, a1 := 2*math.Pi*float64(i)/threadSides, 2*math.Pi*float64(i+1)/threadSides
				el.AddEdge(c.Plus(u.ScaledBy(math.Cos(a0))).Plus(v.ScaledBy(math.Sin(a0))),
					c.Plus(u.ScaledBy(math.Cos(a1))).Plus(v.ScaledBy(math.Sin(a1))), core.StyleDimSolid, 0, 0)
			}
		}
	}
}
