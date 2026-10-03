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
// starts at the end nearest the source's solid. The copies are united
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
	src := sk.GetGroup(sk.Active)
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
	src := sk.GetGroup(g.OpA)
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
		src.ThisShell.Surface[i].GetAxisAlignedBounding(&smax, &smin)
		smax.MakeMaxMin(&hi, &lo)
		smin.MakeMaxMin(&hi, &lo)
	}
	centre := hi.Plus(lo).ScaledBy(0.5)
	chain, err := sk.pathChain(g.Edges, "pattern", func(x geom.Vector) float64 { return x.Minus(centre).Magnitude() }, false)
	if err != nil {
		return nil, err
	}
	lengths := make([]float64, len(chain))
	total := 0.0
	for k, p := range chain {
		if p.arc {
			lengths[k] = math.Abs(p.angle) * p.start.Minus(p.center).Magnitude()
		} else {
			lengths[k] = p.end.Minus(p.start).Magnitude()
		}
		total += lengths[k]
	}
	out := make([]srf.Rigid, n)
	for a := range n {
		want := total * float64(a) / float64(n-1)
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

// checkPattern records what is wrong with the group's input.
func (g *Group) checkPattern(sk *Sketch) bool {
	if _, err := g.patternMotions(sk); err != nil {
		var at *geom.Vector
		var pe *pathError
		if errors.As(err, &pe) {
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
