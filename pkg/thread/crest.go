package thread

import (
	"fmt"
	"math"
)

// Crest is how a ridge's top is shaped. The standards give it flat;
// a printed thread is stronger and mates more easily with less of a
// point, so the crest can be cut lower or rounded.
type Crest int

const (
	// CrestStandard is the flat crest of the standard profile.
	CrestStandard Crest = iota
	// CrestTruncated cuts the crest down by a quarter of the ridge's
	// height, which widens its flat.
	CrestTruncated
	// CrestRounded is an arc tangent to both flanks, whose top is
	// where the standard crest is: over the crest's middle on a
	// symmetric ridge, off it towards the steeper flank on a buttress
	// ridge's.
	CrestRounded
)

// crestNames are the crests' names in files and scripts.
var crestNames = [...]string{CrestStandard: "standard", CrestTruncated: "truncated", CrestRounded: "rounded"}

func (c Crest) String() string {
	if c >= 0 && int(c) < len(crestNames) {
		return crestNames[c]
	}
	return fmt.Sprintf("Crest(%d)", int(c))
}

// ParseCrest is the crest of a name, as Crest.String writes it; ""
// is the standard crest.
func ParseCrest(s string) (Crest, error) {
	if s == "" {
		return CrestStandard, nil
	}
	for c, name := range crestNames {
		if s == name {
			return Crest(c), nil
		}
	}
	return 0, fmt.Errorf("unknown thread crest %q: standard, truncated or rounded", s)
}

// truncation is the fraction of the ridge's height that a truncated
// crest takes off.
const truncation = 0.25

// WithCrest returns the profile with its crest shaped as c. A rounded
// crest keeps CrestRadius as its top and sets CrestRoundRadius; a
// symmetric ridge's CrestWidth and RoundFootRadius are then where the
// arc meets the flanks, and a buttress ridge's, whose arc meets its
// flanks at two radii, stay at the crest's (CrestArc).
func (p Profile) WithCrest(c Crest) Profile {
	toRoot := 1.0 // the way from the crest to the root, along a radius
	if p.RootRadius < p.CrestRadius {
		toRoot = -1
	}
	switch c {
	case CrestTruncated:
		r := p.CrestRadius + toRoot*truncation*p.Height()
		p.CrestRadius, p.CrestWidth = r, p.WidthAt(r)
	case CrestRounded:
		if !p.Symmetric() {
			return p.withLeaningRound()
		}
		half := p.FlankAngle / 2 * math.Pi / 180
		sin := math.Sin(half)
		// The flanks meet at an apex beyond the flat crest; the circle
		// tangent to both whose top is the crest's radius has its centre
		// d from the apex, and touches the flanks at d·cos²(half) in from
		// it, d·sin·cos to either side.
		apex := p.CrestWidth / 2 / math.Tan(half)
		r := apex * sin / (1 - sin)
		d := apex + r
		p.CrestRoundRadius = r
		p.RoundFootRadius = p.CrestRadius - toRoot*apex + toRoot*d*math.Cos(half)*math.Cos(half)
		p.CrestWidth = 2 * d * sin * math.Cos(half)
	}
	return p
}

// withLeaningRound rounds a buttress ridge's crest. The circle tangent
// to the crest's line and to a flank at angle a from the plane square
// to the axis has its centre r·tan(45° − a/2) in from where the flank
// meets the crest, along the axis, so the circle tangent to both
// flanks and to the crest has r = CrestWidth / (tan(45° − a/2) +
// tan(45° − b/2)). Its top lies off the crest's middle, so the crest's
// width and radius are kept as the frame CrestArc measures from: the
// flanks are those of the flat crest, carried on to it.
func (p Profile) withLeaningRound() Profile {
	lower, upper := p.flankAngles()
	p.CrestRoundRadius = p.CrestWidth / (math.Tan(math.Pi/4-lower/2) + math.Tan(math.Pi/4-upper/2))
	p.RoundFootRadius = p.CrestRadius
	return p
}

// flankAngles are the lower and the upper flank's angles from the
// plane square to the axis, in radians.
func (p Profile) flankAngles() (lower, upper float64) {
	return (p.FlankAngle - p.UpperFlank) * math.Pi / 180, p.UpperFlank * math.Pi / 180
}

// CrestArc is a rounded crest's arc (CrestRoundRadius): its centre and
// its feet on the lower and the upper flank, each a radius from the
// axis and a distance along it from the middle of the ridge at
// RoundFootRadius, towards the upper flank. A symmetric ridge's centre
// lies on that middle; a buttress ridge's lies off it, and its feet at
// two radii.
func (p Profile) CrestArc() (centre, lower, upper [2]float64) {
	r := p.CrestRoundRadius
	toRoot := math.Copysign(1, p.RootRadius-p.CrestRadius)
	radius := func(b float64) float64 { return p.RoundFootRadius + toRoot*b }
	al, au := p.flankAngles()
	// b is how far a point lies from RoundFootRadius towards the root;
	// the centre lies r in from the upper flank, which leaves the crest
	// at CrestWidth/2 and moves tan(au) along the axis for each unit of
	// b.
	bc := p.belowCrest(p.CrestRadius + toRoot*r)
	zc := p.CrestWidth/2 + bc*math.Tan(au) - r/math.Cos(au)
	centre = [2]float64{radius(bc), zc}
	lower = [2]float64{radius(bc - r*math.Sin(al)), zc - r*math.Cos(al)}
	upper = [2]float64{radius(bc - r*math.Sin(au)), zc + r*math.Cos(au)}
	return centre, lower, upper
}
