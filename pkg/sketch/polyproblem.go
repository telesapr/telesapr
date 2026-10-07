package sketch

import (
	"math"
	"slices"
	"strconv"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// PolyProblem says what is wrong with a sketch's contours, as
// GenerateLoops found it, and where: an open end, a point off the
// plane of the rest, where the contour crosses itself, or a curve of
// no length. ok is false when the contours are good, or the group is
// not one that has them.
func (g *Group) PolyProblem() (msg string, at geom.Vector, ok bool) {
	switch core.PolyError(g.PolyError.How) {
	case core.PolyNotClosed:
		at = g.PolyError.NotClosed.A
		msg = "is not closed: a contour has an open end at " + pointText(at) +
			" (or its curves are not all of one style)"
	case core.PolyNotCoplanar:
		at = g.PolyError.ErrorPoint
		msg = "is not all in one plane: " + pointText(at) + " is off it"
	case core.PolySelfIntersecting:
		at = g.PolyError.ErrorPoint
		msg = "crosses itself at " + pointText(at)
	case core.PolyZeroLenEdge:
		at = g.PolyError.ErrorPoint
		msg = "has a curve of zero length at " + pointText(at)
	default:
		return "", at, false
	}
	return msg, at, true
}

// pointText is a point as "(x, y, z)" in mm, to three decimals.
func pointText(p geom.Vector) string {
	n := func(v float64) string {
		v = math.Round(v*1000) / 1000
		if v == 0 {
			v = 0
		}
		return strconv.FormatFloat(v, 'g', -1, 64)
	}
	return "(" + n(p.X) + ", " + n(p.Y) + ", " + n(p.Z) + ")"
}

// MakesSolidFromSketch reports whether the group makes a solid from
// the contours of a sketch: an extrusion, lathe, revolve, helix or
// sweep, or a loft. Such a group with a Problem added no solid.
func (g *Group) MakesSolidFromSketch() bool {
	switch g.Type {
	case GroupExtrude, GroupLathe, GroupRevolve, GroupHelix, GroupSweep, GroupLoft:
		return true
	}
	return false
}

// takesProfile reports whether the group makes a solid from the
// contours of group h.
func (g *Group) takesProfile(h core.HGroup) bool {
	switch {
	case !g.MakesSolidFromSketch():
		return false
	case g.Type == GroupLoft:
		return slices.Contains(g.LoftProfiles(), h)
	}
	return g.OpA == h
}

// reportPolyProblem puts a sketch's PolyProblem into the Problem of the
// sketch and of a group that makes a solid from it, since such a group
// then adds none. A sketch says it only when a solid is made from it:
// a sweep's path is open and may leave the plane, and a text's glyphs
// may have curves of no length, by design.
func (g *Group) reportPolyProblem(sk *Sketch) {
	switch g.Type {
	case GroupDrawing3D, GroupDrawingWorkplane:
		msg, at, ok := g.PolyProblem()
		if !ok {
			return
		}
		if !sk.profileOfSolid(g.H) {
			return
		}
		g.setProblem(&at, "the sketch %s", msg)
	case GroupExtrude, GroupLathe, GroupRevolve, GroupHelix, GroupSweep:
		src := sk.Group(g.OpA)
		if src == nil {
			return
		}
		if msg, at, ok := src.PolyProblem(); ok {
			g.setProblem(&at, "the sketch %s %s", src.DescriptionString(), msg)
		}
	case GroupLoft:
		for _, h := range g.LoftProfiles() {
			src := sk.Group(h)
			if src == nil {
				continue
			}
			if msg, at, ok := src.PolyProblem(); ok {
				g.setProblem(&at, "the sketch %s %s", src.DescriptionString(), msg)
				return
			}
		}
	}
}

// profileOfSolid reports whether a group makes a solid from the
// contours of group h.
func (sk *Sketch) profileOfSolid(h core.HGroup) bool {
	for i := range sk.Groups {
		g := &sk.Groups[i]
		if g.SuppressedBy.V == 0 && g.takesProfile(h) {
			return true
		}
	}
	return false
}
