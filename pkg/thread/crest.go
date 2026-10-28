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
	// where the standard crest is.
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
// crest keeps CrestRadius as its top and sets CrestRoundRadius, with
// CrestWidth and RoundFootRadius where the arc meets the flanks; only a
// symmetric ridge is rounded, and a buttress ridge is returned as it
// is.
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
			return p
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
