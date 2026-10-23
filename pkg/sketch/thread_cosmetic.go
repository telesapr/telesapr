package sketch

import (
	"math"
	"slices"

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
	// RunOut is how far before Length the root starts to taper to the
	// crest, 0 for none.
	RunOut float64
	// Chamfer is the radius at which a chamfered free end's cone, at
	// 45° to the axis, leaves the end's plane, out to the crest; 0 for
	// an end that isn't chamfered. ChamferFar tells whether the end at
	// Length is chamfered too.
	Chamfer    float64
	ChamferFar bool
	// Cosmetic is a thread recorded on the part, not modelled: the
	// part's own surface lies at its crest.
	Cosmetic bool
	// Helix is the group that keeps the thread (its helix, or a cosmetic
	// thread's sketch), and Groups those of its groups
	// that make its solid: what lies in the band and was made by
	// another group, as a hole across the thread, is the part's own.
	Helix  core.HGroup
	Groups []core.HGroup
}

// Makes reports whether the thread made what face carries, a handle
// of an entity of the group that made it.
func (b ThreadBand) Makes(face uint32) bool {
	return slices.Contains(b.Groups, core.HEntity{V: face}.Group())
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
		Crest: p.CrestRadius, Root: p.RootRadius, Internal: t.Internal, RunOut: t.RunOut, Cosmetic: t.Cosmetic, Helix: g.H,
		Groups: []core.HGroup{g.H},
	}
	if l.chamfer {
		b.Chamfer, b.ChamferFar = l.chamferR, l.farChamfer
	}
	if tg, err := sk.threadGroupsOf(g.H); err == nil {
		b.Groups = tg.solids()
	}
	b.Inner = math.Min(math.Min(p.CrestRadius, p.RootRadius), l.baseR)
	b.Outer = math.Max(math.Max(p.CrestRadius, p.RootRadius), l.baseR)
	// A chamfer's cone, the thread's too, reaches past the ridges' feet.
	if b.Chamfer != 0 {
		b.Inner, b.Outer = math.Min(b.Inner, b.Chamfer), math.Max(b.Outer, b.Chamfer)
	}
	return b, true
}

// Holds reports whether p lies in the band, within tol.
func (b ThreadBand) Holds(p geom.Vector, tol float64) bool {
	d := p.Minus(b.Origin)
	along := d.Dot(b.Axis)
	r := d.Minus(b.Axis.ScaledBy(along)).Magnitude()
	return along >= -tol && along <= b.Length+tol && r >= b.Inner-tol && r <= b.Outer+tol
}

// Along is how far along the axis from the band's origin p lies.
func (b ThreadBand) Along(p geom.Vector) float64 { return p.Minus(b.Origin).Dot(b.Axis) }

// OnCylinder reports whether p lies within tol of the cylinder of
// radius r about the band's axis, at any height.
func (b ThreadBand) OnCylinder(p geom.Vector, r, tol float64) bool {
	d := p.Minus(b.Origin)
	return math.Abs(d.Minus(b.Axis.ScaledBy(d.Dot(b.Axis))).Magnitude()-r) <= tol
}

// ToCrest is p moved straight away from the axis, or towards it, onto
// the crest's cylinder, or a chamfer's cone where it is short of it.
func (b ThreadBand) ToCrest(p geom.Vector) geom.Vector {
	d := p.Minus(b.Origin)
	t := d.Dot(b.Axis)
	along := b.Axis.ScaledBy(t)
	r := d.Minus(along)
	if r.Magnitude() == 0 {
		return p
	}
	return b.Origin.Plus(along).Plus(r.WithMagnitude(b.CrestAt(t)))
}

// ChamferReach is how far along the axis from a chamfered end its cone
// reaches the crest, 0 for a band with no chamfer.
func (b ThreadBand) ChamferReach() float64 {
	if b.Chamfer == 0 {
		return 0
	}
	return math.Abs(b.Crest - b.Chamfer)
}

// CrestAt is the radius of the thread's surface at t along the axis
// from the band's origin: its crest, or a chamfer's cone at 45° where
// it runs in from the crest to a chamfered end.
func (b ThreadBand) CrestAt(t float64) float64 {
	if b.Chamfer == 0 {
		return b.Crest
	}
	from := t
	if b.ChamferFar {
		from = min(from, b.Length-t)
	}
	if from >= b.ChamferReach() {
		return b.Crest
	}
	if b.Internal {
		return b.Chamfer - from
	}
	return b.Chamfer + from
}

// RootSpan is where along the axis the thread's root lies inside its
// surface, which drawings draw it over: from where the root's cylinder
// leaves a chamfer's cone, or the end, to the end of the full thread
// short of a run-out, or where it meets the cone at a chamfered far end.
func (b ThreadBand) RootSpan() (from, to float64) {
	to = b.Length - b.RunOut
	if b.Chamfer != 0 {
		from = math.Abs(b.Root - b.Chamfer)
		if b.ChamferFar {
			to = b.Length - from
		}
	}
	return from, to
}
