package sketch

import (
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// ThreadBand is where a thread lies, for a drawing that shows it by
// the drafting convention rather than turn by turn: along Axis from
// Origin for Length, between the radii Inner and Outer that its ridge,
// the turning or boring and the ridge's foot reach, the crest at Crest
// and the root at Root.
type ThreadBand struct {
	Origin, Axis geom.Vector
	Length       float64
	Crest, Root  float64
	Inner, Outer float64
	Internal     bool
	// Helix is the thread's helix group.
	Helix core.HGroup
}

// ThreadBands are the threads of the solid of group hg: those whose
// helix group comes at or before it and is generated.
func (sk *Sketch) ThreadBands(hg core.HGroup) []ThreadBand {
	upTo := sk.Group(hg)
	if upTo == nil {
		return nil
	}
	var out []ThreadBand
	for _, h := range sk.GroupOrder {
		g := sk.Group(h)
		if g.Order > upTo.Order {
			break
		}
		if !g.Thread.IsThread() || g.Suppress || g.SuppressedBy.V != 0 {
			continue
		}
		if b, ok := sk.threadBand(g); ok {
			out = append(out, b)
		}
	}
	return out
}

// threadBand is where the thread of the helix group g lies.
func (sk *Sketch) threadBand(g *Group) (ThreadBand, bool) {
	t := g.Thread
	circle := sk.Entity(t.Circle)
	if circle == nil || !circle.IsCircle() {
		return ThreadBand{}, false
	}
	l, err := t.layout(circle.CircleGetRadiusNum(sk))
	if err != nil {
		return ThreadBand{}, false
	}
	axis := sk.MustEntity(circle.Normal).NormalGetNum(sk).RotationN()
	if t.Reverse {
		axis = axis.ScaledBy(-1)
	}
	p := l.profile
	b := ThreadBand{
		Origin: sk.MustEntity(circle.Point[0]).PointGetNum(sk), Axis: axis, Length: t.Length,
		Crest: p.CrestRadius, Root: p.RootRadius, Internal: t.Internal, Helix: g.H,
	}
	b.Inner = math.Min(math.Min(p.CrestRadius, p.RootRadius), l.baseR)
	b.Outer = math.Max(math.Max(p.CrestRadius, p.RootRadius), l.baseR)
	return b, true
}

// Holds reports whether p lies in the band, within tol.
func (b ThreadBand) Holds(p geom.Vector, tol float64) bool {
	d := p.Minus(b.Origin)
	along := d.Dot(b.Axis)
	r := d.Minus(b.Axis.ScaledBy(along)).Magnitude()
	return along >= -tol && along <= b.Length+tol && r >= b.Inner-tol && r <= b.Outer+tol
}

// OnCylinder reports whether p lies within tol of the cylinder of
// radius r about the band's axis, at any height.
func (b ThreadBand) OnCylinder(p geom.Vector, r, tol float64) bool {
	d := p.Minus(b.Origin)
	return math.Abs(d.Minus(b.Axis.ScaledBy(d.Dot(b.Axis))).Magnitude()-r) <= tol
}
