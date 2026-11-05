package sketch_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/thread"
)

// roundRootCases are B1.9 threads with rounded roots, screws and nuts,
// flat and rounded crests, screws chamfered or not, from 1/2-20 to 4-4.
func roundRootCases() []threadCase {
	var cases []threadCase
	for _, s := range []struct {
		designation string
		d           float64
	}{
		{"1/2-20 BUTT", 12.7}, {"1/2-12 BUTT", 12.7}, {"3/4-10 BUTT", 19.05}, {"1-12 BUTT", 25.4},
		{"1-8 BUTT", 25.4}, {"2-8 BUTT", 50.8}, {"2-4 BUTT", 50.8}, {"4-4 BUTT", 101.6},
	} {
		for _, crest := range []thread.Crest{thread.CrestStandard, thread.CrestRounded} {
			o := sketch.ThreadOptions{Clearance: 0.05, RoundRoot: true, Crest: crest}
			screw, chamfered, nut := o, o, o
			screw.Length = 25
			chamfered.Length, chamfered.Chamfer = 25, true
			nut.Length, nut.Through, nut.Internal = 15, true, true
			name := fmt.Sprintf("%s %v", s.designation, crest)
			cases = append(cases,
				threadCase{name + " screw", s.d / 2, 40, s.designation, screw},
				threadCase{name + " chamfered screw", s.d / 2, 40, s.designation, chamfered},
				threadCase{name + " nut", 0.8 * s.d, 15, s.designation, nut})
		}
	}
	return cases
}

// TestThreadRoundRootSweep makes B1.9 threads with rounded roots at
// chord tolerances from fine to coarse and requires every one sound.
// Coarser than the arcs take, the thread and the part it is made in
// are made at a tolerance of their own (threadSolidTol): the part's
// solid at the thread's. Before 2026-11-04 a rounded root was refused
// coarser than a fifth of its radius, and with only the thread's groups
// at the finer tolerance the screws came out with edges open along the
// root. An even sample by default, the whole grid (240 threads) with
// TELESAPR_SWEEP=1.
func TestThreadRoundRootSweep(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("threads with rounded roots")
	}
	byName := map[string]threadCase{}
	var names []string
	for _, tc := range roundRootCases() {
		byName[tc.name] = tc
		names = append(names, tc.name)
	}
	grid := sweep.Grid{
		Parallel: true,
		Axes: []sweep.Axis{
			sweep.Floats("tol", 2, 1, 0.5, 0.2, 0.1),
			sweep.Strings("case", names...),
		},
		Sample: 12,
	}
	grid.Run(func(p sweep.Point) error {
		sk, _, res, err := byName[p.String("case")].tryBuild(t, p.Float("tol"))
		if err != nil {
			return err
		}
		if !res.Sound {
			return errors.New("not sound")
		}
		// The part, the extrusion, is made at the thread's tolerance.
		var part *sketch.Group
		for i := range sk.Groups {
			if sk.Groups[i].Type == sketch.GroupExtrude {
				part = &sk.Groups[i]
			}
		}
		if part == nil {
			return errors.New("no extrusion")
		}
		if a, b := part.ThisShell.Tol.ChordTolMm(), sk.Group(res.Group).ThisShell.Tol.ChordTolMm(); a != b {
			return fmt.Errorf("the part made at %g mm, the thread at %g", a, b)
		}
		return nil
	}).Require(t)
}
