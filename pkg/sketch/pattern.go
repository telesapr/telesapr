package sketch

import (
	"errors"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A pattern along a path repeats the solid of its source group (OpA)
// along a chain of line segments and arcs that meet tangent (Edges),
// as a sweep's path: ValA copies, the first where the source is and the
// last at the far end of the path, spaced evenly by length along it,
// each turned with the path as a sweep turns its profile. The path
// starts at the end nearest the source's solid. A closed path, a circle
// or a chain that comes back to its start meeting it tangent, starts at
// its joint nearest the source's solid, or a circle at its point
// nearest it, and the copies share all of it out evenly (roadmap 28.5). The copies are united
// with the solid before the source, as a step and repeat's are; the
// sketch's entities are not copied (roadmap 15.6, E2).

// RemapPattern is the remap copy number of a pattern's first copy.
const RemapPattern = 2000

// NewPathPattern starts a pattern of the active group's solid along
// path, line segments and arcs joined end to end and meeting tangent,
// copies times in all. It is what the interface and scripts add with
// AddGroupAfterActive.
func (sk *Sketch) NewPathPattern(path []core.HEntity, copies int, name string) (Group, error) {
	g := NewGroup()
	g.Type = GroupPathPattern
	g.Name = name
	if name == "" {
		g.Name = "pattern"
	}
	src := sk.Group(sk.Active)
	if src == nil || len(src.ThisShell.Surface) == 0 {
		return g, errors.New("the active group makes no solid to repeat")
	}
	if len(path) == 0 {
		return g, errors.New("there is no path to repeat along")
	}
	if copies < 2 {
		return g, errors.New("a pattern makes at least two copies")
	}
	g.OpA = src.H
	g.Edges = append([]core.HEntity(nil), path...)
	g.ValA = float64(copies)
	g.MeshCombine = src.MeshCombine
	if _, err := g.patternMotions(sk); err != nil {
		return g, err
	}
	return g, nil
}

// patternMotions are the rigid motions that take the source to each
// copy, the first none.
func (g *Group) patternMotions(sk *Sketch) ([]srf.Rigid, error) {
	src := sk.Group(g.OpA)
	if src == nil {
		return nil, errors.New("the group to repeat is gone")
	}
	n := int(math.Round(g.ValA))
	if n < 2 {
		return nil, errors.New("a pattern makes at least two copies")
	}
	hi := geom.Vector{X: math.Inf(-1), Y: math.Inf(-1), Z: math.Inf(-1)}
	lo := hi.ScaledBy(-1)
	for i := range src.ThisShell.Surface {
		var smax, smin geom.Vector
		src.ThisShell.Surface[i].AxisAlignedBounding(&smax, &smin)
		smax.MakeMaxMin(&hi, &lo)
		smin.MakeMaxMin(&hi, &lo)
	}
	centre := hi.Plus(lo).ScaledBy(0.5)
	dist := func(x geom.Vector) float64 { return x.Minus(centre).Magnitude() }
	chain, closed, err := sk.pathChainOf(g.Edges, "pattern", dist, false, true)
	if err != nil {
		return nil, err
	}
	if closed {
		chain = startNearest(chain, dist)
	}
	lengths := make([]float64, len(chain))
	total := 0.0
	for k, p := range chain {
		lengths[k] = pieceLength(p)
		total += lengths[k]
	}
	// Along an open path the last copy is at its far end; round a closed
	// one the copies share it out, the one after the last being the first.
	spaces := n - 1
	if closed {
		spaces = n
	}
	out := make([]srf.Rigid, n)
	for a := range n {
		want := total * float64(a) / float64(spaces)
		r := srf.RigidIdentity
		for k, p := range chain {
			st := p.step()
			if want >= lengths[k] && k < len(chain)-1 {
				r = r.Then(st.Motion())
				want -= lengths[k]
				continue
			}
			f := math.Min(1, want/lengths[k])
			if st.Arc {
				st.Angle *= f
			} else {
				st.Along = st.Along.ScaledBy(f)
			}
			r = r.Then(st.Motion())
			break
		}
		out[a] = r
	}
	return out, nil
}

// pointAt is the point of a line or arc piece at fraction t along it.
func (p pathPiece) pointAt(t float64) geom.Vector {
	if !p.arc {
		return p.start.Plus(p.end.Minus(p.start).ScaledBy(t))
	}
	return p.center.Plus(p.start.Minus(p.center).RotatedAroundAxis(p.axis, p.angle*t))
}

// split cuts a line or arc piece at fraction t into two.
func (p pathPiece) split(t float64) (pathPiece, pathPiece) {
	a, b := p, p
	m := p.pointAt(t)
	a.end, b.start = m, m
	if p.arc {
		a.angle, b.angle = p.angle*t, p.angle*(1-t)
	}
	a.curves, b.curves = nil, nil
	return a, b
}

// startNearest turns a closed chain of lines and arcs to start at its
// point where dist is least, splitting the piece it lies on: so that a
// pattern round it starts at the solid it repeats, wherever that sits
// on it.
func startNearest(chain []pathPiece, dist func(geom.Vector) float64) []pathPiece {
	const samples = 512
	bestK, bestT, best := 0, 0.0, math.Inf(1)
	for k, p := range chain {
		for i := 0; i <= samples; i++ {
			t := float64(i) / samples
			if d := dist(p.pointAt(t)); d < best {
				bestK, bestT, best = k, t, d
			}
		}
	}
	// Refine by golden section between the neighbouring samples.
	p := chain[bestK]
	lo, hi := math.Max(0, bestT-1.0/samples), math.Min(1, bestT+1.0/samples)
	for range 60 {
		m1, m2 := lo+(hi-lo)*0.382, lo+(hi-lo)*0.618
		if dist(p.pointAt(m1)) < dist(p.pointAt(m2)) {
			hi = m2
		} else {
			lo = m1
		}
	}
	t := (lo + hi) / 2
	var out []pathPiece
	switch {
	case t*pieceLength(p) < sweepTol:
		out = append(out, chain[bestK:]...)
		out = append(out, chain[:bestK]...)
	case (1-t)*pieceLength(p) < sweepTol:
		out = append(out, chain[bestK+1:]...)
		out = append(out, chain[:bestK+1]...)
	default:
		a, b := p.split(t)
		out = append(out, b)
		out = append(out, chain[bestK+1:]...)
		out = append(out, chain[:bestK]...)
		out = append(out, a)
	}
	return out
}

// pieceLength is a line or arc piece's length.
func pieceLength(p pathPiece) float64 {
	if p.arc {
		return math.Abs(p.angle) * p.start.Minus(p.center).Magnitude()
	}
	return p.end.Minus(p.start).Magnitude()
}

// checkPattern records what is wrong with the group's input.
func (g *Group) checkPattern(sk *Sketch) bool {
	if _, err := g.patternMotions(sk); err != nil {
		var at *geom.Vector
		if pe, ok := errors.AsType[*pathError](err); ok {
			at = pe.at
		}
		g.setProblem(at, "%v", err)
		return false
	}
	return true
}

// generatePatternShell unites the copies of the source's solid.
func (g *Group) generatePatternShell(sk *Sketch, src, out *srf.SShell, forWhat CombineAs) {
	ms, err := g.patternMotions(sk)
	if err != nil {
		return
	}
	ts, qs := make([]geom.Vector, len(ms)), make([]geom.Quaternion, len(ms))
	for a, m := range ms {
		ts[a], qs[a] = m.T, m.Q
	}
	g.unionCopies(sk, src, out, forWhat, ts, qs, 0, func(a int) int { return RemapPattern + a })
}
