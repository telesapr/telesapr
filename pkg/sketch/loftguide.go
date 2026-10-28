package sketch

import (
	"errors"
	"fmt"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// A loft's guide curves (roadmap 28.3) are the line segments, arcs and
// cubics Edges, drawn in sketches before it: each chain of them that
// joins end to end is one guide, from a point of the first profile
// through every profile to the last (topo.MakeGuidedLoft), or on a
// closed loft round the ring to where it started
// (topo.MakeClosedGuidedLoft). One guide
// moves the loft's sections; two move, turn and scale them.

// guideSteps is how many points a guide takes along each of its Bezier
// curves.
const guideSteps = 64

// SetLoftGuides makes the line segments, arcs and cubics es the guide
// curves of the loft hg, or takes its guides away when es is empty.
func (sk *Sketch) SetLoftGuides(hg core.HGroup, es []core.HEntity) error {
	g := sk.Group(hg)
	if g == nil || g.Type != GroupLoft {
		return errors.New("only a loft has guide curves")
	}
	var out []core.HEntity
	seen := map[core.HEntity]bool{}
	for _, he := range es {
		e := sk.Entity(he)
		if e == nil {
			return fmt.Errorf("entity %s doesn't exist", sk.entityName(he))
		}
		switch e.Type {
		case core.EntityLineSegment, core.EntityArcOfCircle, core.EntityCubic:
		default:
			return fmt.Errorf("%s is not a line segment, an arc or a cubic: a guide is made of those", sk.entityName(he))
		}
		if !sk.GroupsInOrder(e.Group, hg) || e.Group == hg {
			return fmt.Errorf("%s is drawn after the loft: a guide must come before it", sk.entityName(he))
		}
		if !seen[he] {
			seen[he] = true
			out = append(out, he)
		}
	}
	g.Edges = out
	sk.MarkGroupDirty(hg, false)
	return nil
}

// loftGuides are the loft's guide curves as points along each, or why
// they don't make guides. A closed loft's guides close on themselves,
// round the ring; an open loft's run from its first profile to its last.
func (g *Group) loftGuides(sk *Sketch) ([]topo.LoftGuide, error) {
	if len(g.Edges) == 0 {
		return nil, nil
	}
	var curves []srf.SBezier
	for _, he := range g.Edges {
		e := sk.Entity(he)
		if e == nil {
			return nil, fmt.Errorf("guide curve %s doesn't exist", sk.entityName(he))
		}
		var sbl srf.SBezierList
		GenerateEntityBeziers(e, &sbl, &Runtime{Sketch: sk})
		for _, sb := range sbl.Curves {
			if sb.Start().Minus(sb.Finish()).Magnitude() < sweepTol {
				return nil, pathErr(nil, "guide curve %s has no length", sk.entityName(he))
			}
			curves = append(curves, sb)
		}
	}
	near := func(p, q geom.Vector) bool { return p.Minus(q).Magnitude() < sweepTol }
	degree := func(p geom.Vector) int {
		n := 0
		for _, sb := range curves {
			if near(p, sb.Start()) {
				n++
			}
			if near(p, sb.Finish()) {
				n++
			}
		}
		return n
	}
	for _, sb := range curves {
		for _, x := range []geom.Vector{sb.Start(), sb.Finish()} {
			if degree(x) > 2 {
				return nil, pathErr(&x, "the guides branch at (%g, %g, %g)", x.X, x.Y, x.Z)
			}
		}
	}
	used := make([]bool, len(curves))
	var guides []topo.LoftGuide
	for {
		// A chain starts at an end that meets no other.
		first := -1
		for i, sb := range curves {
			if !used[i] && (degree(sb.Start()) == 1 || degree(sb.Finish()) == 1) {
				first = i
				break
			}
		}
		if first >= 0 && g.LoftClosed {
			x := curves[first].Start()
			if degree(x) != 1 {
				x = curves[first].Finish()
			}
			return nil, pathErr(&x, "a guide ends at (%g, %g, %g): a closed loft's guide goes round the ring, closing on itself", x.X, x.Y, x.Z)
		}
		if first < 0 {
			for i, sb := range curves {
				if used[i] {
					continue
				}
				x := sb.Start()
				if !g.LoftClosed {
					return nil, pathErr(&x, "a guide closes on itself at (%g, %g, %g): a guide runs from the first profile to the last", x.X, x.Y, x.Z)
				}
				// A closed chain: from any of its curves round to it.
				first = i
				break
			}
			if first < 0 {
				break
			}
		}
		at := curves[first].Start()
		if degree(at) != 1 && !g.LoftClosed {
			at = curves[first].Finish()
		}
		var pts topo.LoftGuide
		pts = append(pts, at)
		for {
			found := false
			for i, sb := range curves {
				if used[i] {
					continue
				}
				forward := near(at, sb.Start())
				if !forward && !near(at, sb.Finish()) {
					continue
				}
				used[i], found = true, true
				for k := 1; k <= guideSteps; k++ {
					t := float64(k) / guideSteps
					if !forward {
						t = 1 - t
					}
					pts = append(pts, sb.PointAt(t))
				}
				at = pts[len(pts)-1]
				break
			}
			if !found {
				break
			}
		}
		guides = append(guides, pts)
	}
	return guides, nil
}
