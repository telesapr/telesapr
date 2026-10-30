package sketch_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/thread"
)

// buttressCrestCases are buttress threads with rounded crests across
// DIN 513's and B1.9's sizes: screws and nuts, the load flank either
// way, B1.9's with and without its rounded roots, and a few chamfered
// and run out.
func buttressCrestCases() []threadCase {
	var cases []threadCase
	for _, c := range []struct {
		designation string
		d           float64
		roundRoot   bool
	}{
		{"S10x2", 10, false}, {"S20x4", 20, false}, {"S40x7", 40, false},
		{"1/2-16 BUTT", 12.7, false}, {"1-12 BUTT", 25.4, false}, {"2-8 BUTT", 50.8, false},
		{"1/2-16 BUTT", 12.7, true}, {"1-12 BUTT", 25.4, true},
	} {
		name := c.designation
		if c.roundRoot {
			name += " round root"
		}
		for _, back := range []bool{false, true} {
			o := sketch.ThreadOptions{Crest: thread.CrestRounded, LoadFlankBack: back, RoundRoot: c.roundRoot}
			screw, nut := o, o
			screw.Length = c.d
			nut.Length, nut.Through, nut.Internal = 0.8*c.d, true, true
			suffix := ""
			if back {
				suffix = " back"
			}
			cases = append(cases,
				threadCase{name + " screw" + suffix, c.d / 2, 2 * c.d, loadWay(c.designation, false, back), screw},
				threadCase{name + " nut" + suffix, c.d, 0.8 * c.d, loadWay(c.designation, true, back), nut})
		}
	}
	// Chamfered ends and run-outs, whose cones cut the arcs.
	cases = append(cases,
		threadCase{"S20x4 screw chamfer", 10, 40, "S20x4", sketch.ThreadOptions{Length: 20, Through: true, Chamfer: true, Crest: thread.CrestRounded}},
		// A chamfered 1-12 BUTT nut 20 mm long is unsound at chord
		// tolerances of 0.3% and coarser with a flat crest as with a
		// rounded one: not the crest's; an S20x4 nut stands in for it.
		threadCase{"S20x4 nut chamfer", 20, 16, "S20x4", sketch.ThreadOptions{Length: 16, Through: true, Internal: true, Chamfer: true, Crest: thread.CrestRounded}},
		threadCase{"S20x4 screw run-out", 10, 40, "S20x4", sketch.ThreadOptions{Length: 20, RunOut: 6, Crest: thread.CrestRounded}},
		threadCase{"1-12 BUTT screw run-out back", 12.7, 40, "1-12 PUSH-BUTT", sketch.ThreadOptions{Length: 20, RunOut: 5, Crest: thread.CrestRounded}})
	return cases
}

// loadWay is a B1.9 designation that turns the ridge of a screw or a
// nut back, or leaves it, as LoadFlankBack does a DIN 513 one's: a
// screw's is turned pushing, a nut's pulling.
func loadWay(designation string, internal, back bool) string {
	if !strings.HasSuffix(designation, "BUTT") || internal == back {
		return designation
	}
	return strings.Replace(designation, "BUTT", "PUSH-BUTT", 1)
}

// TestThreadButtressCrestSweep makes buttress threads with rounded
// crests at chord tolerances from coarse to fine and requires every one
// sound, a rounded root refused by name where it is too fine for the
// tolerance: an even sample by default, the whole grid with
// TELESAPR_SWEEP=1.
func TestThreadButtressCrestSweep(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("thread solids")
	}
	byName := map[string]threadCase{}
	var names []string
	for _, tc := range buttressCrestCases() {
		byName[tc.name] = tc
		names = append(names, tc.name)
	}
	grid := sweep.Grid{
		Parallel: true,
		Axes: []sweep.Axis{
			sweep.Floats("tol", 0.5, 0.2, 0.1, 0.05, 0.02, 0.01),
			sweep.Strings("case", names...),
		},
		Sample: 8,
	}
	grid.Run(func(p sweep.Point) error {
		_, _, res, err := byName[p.String("case")].tryBuild(t, p.Float("tol"))
		switch {
		case errors.Is(err, sketch.ErrRootRoundCoarse):
			return nil
		case err != nil:
			return err
		}
		if !res.Sound {
			return errors.New("not sound")
		}
		return nil
	}).Require(t)
}
