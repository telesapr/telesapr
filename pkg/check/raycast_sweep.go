package check

import (
	"errors"
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// halfSpace is the side of the plane through p that n points to.
type halfSpace struct{ p, n geom.Vector }

func (h halfSpace) holds(x geom.Vector) bool { return x.Minus(h.p).Dot(h.n) >= 0 }

// crossing appends where the ray o + s d crosses the plane, if it does.
func (h halfSpace) crossing(o, d geom.Vector, out []float64) []float64 {
	if dn := d.Dot(h.n); dn != 0 {
		out = append(out, h.p.Minus(o).Dot(h.n)/dn)
	}
	return out
}

// newCutPrism is the profile pr moved along `along`, ended by the
// half-spaces start and end in place of the profile's plane at either
// end: a run of a sweep, which a mitre cuts at a corner.
func newCutPrism(pr profile, along geom.Vector, start, end halfSpace) *prism {
	p := newPrism(pr, geom.Vector{}, along)
	p.cuts = []halfSpace{start, end}
	p.cornerLo = geom.Vector{X: math.Inf(1), Y: math.Inf(1), Z: math.Inf(1)}
	p.cornerHi = p.cornerLo.ScaledBy(-1)
	// Each corner of the profile runs along the prism from one cut to
	// the other.
	for _, poly := range pr.polys {
		for _, v := range poly {
			q := p.base.Plus(pr.eu.ScaledBy(v[0])).Plus(pr.ev.ScaledBy(v[1]))
			for _, h := range p.cuts {
				if dn := p.dir.Dot(h.n); dn != 0 {
					q.Plus(p.dir.ScaledBy(h.p.Minus(q).Dot(h.n)/dn)).MakeMaxMin(&p.cornerHi, &p.cornerLo)
				}
			}
		}
	}
	return p
}

// sweepLeaves is a sweep along lines and arcs as its steps move each of
// the profile's loop sets: a prism along each line, ended by the
// profile's plane or by the plane halving a mitred corner, and a
// revolution about each arc's axis. A run along an arc is a revolve
// only where the profile, moved to the arc's start, lies in a plane
// through the arc's axis, as a profile square to the path does; one
// leaning to it is refused.
func sweepLeaves(sk *sketch.Sketch, g, src *sketch.Group) (union, error) {
	steps, err := g.SweepSteps(sk)
	if errors.Is(err, sketch.ErrFittedSweep) {
		return nil, fmt.Errorf("%w: %w", err, ErrUnsupported)
	}
	if err != nil {
		return nil, err
	}
	// The direction of the line before or after a corner, step k.
	lineAt := func(k int) geom.Vector { return steps[k].Along.WithMagnitude(1) }
	var leaves union
	for i := range src.BezierLoops.Sets {
		frame := srf.RigidIdentity
		for k, st := range steps {
			moved := srf.TransformedLoopSet(&src.BezierLoops.Sets[i], frame)
			n := moved.Normal.WithMagnitude(1)
			switch {
			case st.Corner:
			case st.Arc:
				axis := st.Axis.WithMagnitude(1)
				if math.Abs(n.Dot(axis)) > 1e-9 || math.Abs(moved.Point.Minus(st.Center).Dot(n)) > 1e-6 {
					return nil, fmt.Errorf("%s: the profile leans to an arc's axis: %w", g.DescriptionString(), ErrUnsupported)
				}
				leaves = append(leaves, newRevolution(moved, st.Center, axis, st.Angle >= 2*math.Pi, 0, st.Angle))
			default:
				d := lineAt(k)
				if n.Dot(d) < 0 {
					n = n.ScaledBy(-1)
				}
				start := halfSpace{p: moved.Point, n: n}
				end := halfSpace{p: moved.Point.Plus(st.Along), n: n.ScaledBy(-1)}
				if k > 0 && steps[k-1].Corner {
					start = halfSpace{p: steps[k-1].Center, n: lineAt(k - 2).Plus(d).WithMagnitude(1)}
				}
				if k+1 < len(steps) && steps[k+1].Corner {
					end = halfSpace{p: steps[k+1].Center, n: d.Plus(lineAt(k + 2)).WithMagnitude(1).ScaledBy(-1)}
				}
				eu := n.NormalDir(0)
				leaves = append(leaves, newCutPrism(newProfile(moved, moved.Point, eu, n.Cross(eu)), st.Along, start, end))
			}
			frame = frame.Then(st.Motion())
		}
	}
	return leaves, nil
}
