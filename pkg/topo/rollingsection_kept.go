package topo

import (
	"math"
	"slices"
	"sync"
)

// A mitred group's blends find the ball at the same arc lengths many
// times over: the searches for a mitre's arc lengths (mitreRoot, from
// one bracket each, which agree for their first steps), for its feet
// (mitreFoot, on one grid each side) and the scans. While the chains are
// blended (blendGroups) a section depends only on its arc length: the
// chain's edge, faces, ends and radius are fixed once it is traced and
// mitred, and every projection is found from the faces alone
// (srf.SShell.ProjectIndependently). So each section found then is kept,
// by its arc length's bits, and handed out again (keptSection), the same
// to the bit; about half of them were found before. A foot put on a pole
// (poleSnaps) is put on it after (section), so the kept section is the
// one before.

// rbKept are a chain's sections found while it is blended, by the bits
// of their arc lengths; nil where they are not kept.
type rbKept struct {
	m sync.Map // uint64 to rbSection
}

// blendingAtOnce runs fn, the chains' blends, with each chain's
// sections found at once (atOnce) and kept, and drops them after. The
// chains must be traced and mitred, and their shell's surfaces projected
// on independently.
func blendingAtOnce(rbs []*rollingBall, fn func()) {
	for _, rb := range rbs {
		rb.atOnce, rb.kept = true, &rbKept{}
	}
	defer func() {
		for _, rb := range rbs {
			rb.atOnce, rb.kept = false, nil
		}
	}()
	fn()
}

// keptSection is the section at arc length s if it is kept.
func (rb *rollingBall) keptSection(s float64) (rbSection, bool) {
	if rb.kept == nil {
		return rbSection{}, false
	}
	v, ok := rb.kept.m.Load(math.Float64bits(s))
	if !ok {
		return rbSection{}, false
	}
	return cloneSection(v.(rbSection)), true
}

// keepSection keeps sec, the section at arc length s, if sections are
// kept.
func (rb *rollingBall) keepSection(s float64, sec rbSection) {
	if rb.kept != nil {
		rb.kept.m.Store(math.Float64bits(s), cloneSection(sec))
	}
}

// cloneSection is sec with an arc of its own, so that what one caller
// does to it is not another's.
func cloneSection(sec rbSection) rbSection {
	sec.arc.Knots = slices.Clone(sec.arc.Knots)
	sec.arc.Ctrl = slices.Clone(sec.arc.Ctrl)
	sec.arc.Weight = slices.Clone(sec.arc.Weight)
	return sec
}

// warmSections finds the sections at arc lengths ss at once, to be kept,
// where sections are kept: a search that takes them in order then finds
// them kept. What it finds is dropped, so the search is as it was; it
// only finds sooner what it would have found.
func (rb *rollingBall) warmSections(ss ...float64) {
	if rb.kept == nil || len(ss) < 2 {
		return
	}
	rb.each(len(ss), func(i int) { _, _ = rb.section(ss[i]) })
}
