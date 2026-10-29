package thread

import "math"

// A rounded root's arc, as the standard draws it, is tangent to both
// flanks of the groove and touches the root's diameter at its bottom.
// Modelled so, it would lie on the part's turned surface along a helix,
// which no Boolean takes. Each side of the groove keeps the standard's
// arc from its flank to where it turns rootTurn short of the bottom,
// and goes on from there along the arc's tangent, which crosses the
// root steeply, to rootDepth of the radius past the root, inside the
// part; the root keeps a short flat between the two. The model lies
// within a quarter of the radius of the standard's arc (0.019 of a
// B1.9 pitch, at the load flank's crossing), where the flat root's
// corners were up to 1.6 radii from it. Shallower crossings, which
// keep closer to the arc, leave the root's flat narrower and its
// Booleans failing at coarser chord tolerances (tried 60° and two
// thirds of the way: sound at a chord tolerance of 0.12 of the radius
// where these are at 0.2).
const (
	// rootTurn is the most the arc turns short of its bottom, in
	// degrees; never more than nine tenths of the way from the flank,
	// so that the arc beside a steep flank keeps a length.
	rootTurn = 80.0
	// rootSpan is that part of the way.
	rootSpan = 0.9
	// rootDepth is how far past the root, as a part of the radius, the
	// tangent goes into the part.
	rootDepth = 0.2
)

// RootArc is one side of a ridge's rounded root (RootArcs). Each point
// is a radius from the axis and a distance along it from the middle of
// the ridge's crest.
type RootArc struct {
	// Centre is the arc's centre; it leaves the flank at Tangent and
	// turns into a line at Turn, which crosses the root at Cross and
	// ends inside the part at End.
	Centre, Tangent, Turn, Cross, End [2]float64
	// Foot is where the flank, carried on, crosses the root.
	Foot [2]float64
}

// RootArcs are the sides of a rounded root (RootRoundRadius) on the
// ridge's lower and upper flanks.
func (p Profile) RootArcs() (lower, upper RootArc) {
	r := p.RootRoundRadius
	// u is the way from the root to the crest along a radius; b below
	// is how far a point lies from the crest towards the root.
	u := 1.0
	if p.RootRadius > p.CrestRadius {
		u = -1
	}
	h := p.Height()
	radius := func(b float64) float64 { return p.CrestRadius - u*b }
	bc := h - r
	tl, tu := p.flankTans()
	side := func(angle, tan, way float64) RootArc {
		a := angle * math.Pi / 180
		turn := math.Min(rootTurn, (90-angle)*rootSpan) * math.Pi / 180
		zc := way * (p.CrestWidth/2 + bc*tan + r/math.Cos(a))
		// From Turn the tangent runs deeper and towards the groove's
		// middle, along (sin, cos) of the turn in (b, way z).
		bt, zt := bc+r*math.Cos(turn), zc-way*r*math.Sin(turn)
		along := func(b float64) [2]float64 {
			return [2]float64{radius(b), zt + way*(b-bt)/math.Tan(turn)}
		}
		return RootArc{
			Centre:  [2]float64{radius(bc), zc},
			Tangent: [2]float64{radius(bc + r*math.Sin(a)), zc - way*r*math.Cos(a)},
			Turn:    [2]float64{radius(bt), zt},
			Cross:   along(h),
			End:     along(h + rootDepth*r),
			Foot:    [2]float64{p.RootRadius, way * (p.CrestWidth/2 + h*tan)},
		}
	}
	return side(p.FlankAngle-p.UpperFlank, tl, -1), side(p.UpperFlank, tu, 1)
}
