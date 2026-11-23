package sketch_test

import (
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// Sharp corners beside a cubic whose runs, as first cut, don't join,
// made since 2026-11-23: a loop whose run from the profile round to the
// cubic comes back past where it started (cut in two between, a
// diamond), a loop twisted a whole turn whose run of lines would hold
// the twist beside its own corners where they are too short for it
// (held untwisted there, a bar 2 by 4), a circle at a corner rounded
// after a cubic first whose tubes touch tangent on a patch's seam (the
// Euler check counted the touching hole), a diamond twisted at a corner
// of 30° beside a cubic bent a quarter turn (held three times as far),
// and a circle twisted, the same solid as untwisted, by a quarter turn
// round a loop too. A bar 4 by 2 round the loop is refused by name: the
// line along its foot is too short for the mitres of both its corners.
func TestSweepSharpCornerBesideCubicRunsCutAgain(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("each sweep joins its runs by Booleans, the first cut failing: a few seconds")
	}
	type build func(d *api.Doc) []api.Entity
	loop := func(corner, bend float64) build {
		return func(d *api.Doc) []api.Entity { p, _ := cubicCornerLoop(d, corner, bend); return p }
	}
	open := func(cubicFirst bool, corner, bend float64) build {
		return func(d *api.Doc) []api.Entity { return cubicCornerPath(d, cubicFirst, corner, bend, 10, 12) }
	}
	sweepOf := func(b build, profile float64, set func(api.SweepGroup)) api.Group {
		d := api.New()
		path := b(d)
		cubicCornerProfile(d, profile)
		s := d.Features.Sweep(path, "")
		if set != nil {
			set(s.AsSweep())
		}
		return s
	}
	twist := func(deg float64) func(api.SweepGroup) { return func(s api.SweepGroup) { s.SetTwist(deg) } }
	for _, c := range []struct {
		name    string
		path    build
		profile float64
		set     func(api.SweepGroup)
	}{
		{"diamond round a loop coming back past its start", loop(150, 30), 3, nil},
		{"bar twisted a turn round a loop", loop(150, 30), 0, twist(360)},
		{"circle rounded after a cubic", func(d *api.Doc) []api.Entity { return cubicCornerRoundedPath(d, true, 150, 0) }, 1,
			func(s api.SweepGroup) { s.SetCornerRadius(3) }},
		{"diamond twisted at 30°, the cubic bent a quarter turn", open(false, 30, 90), 3, twist(90)},
	} {
		s := sweepOf(c.path, c.profile, c.set)
		if p, r := s.Problem(), s.Analyze().Check(); p != "" || !r.Sound() {
			t.Errorf("%s: %q %s, want made sound", c.name, p, r.String())
		}
	}
	for _, c := range []struct {
		name string
		path build
		deg  float64
	}{
		{"circle twisted at 150°, the cubic bent back", open(false, 150, 90), 90},
		{"circle twisted a quarter turn round a loop", loop(150, 90), 90},
	} {
		plain := sweepOf(c.path, 1, nil).Analyze().Check()
		s := sweepOf(c.path, 1, twist(c.deg))
		r := s.Analyze().Check()
		if p := s.Problem(); p != "" || !r.Sound() || math.Abs(r.Volume-plain.Volume) > 1e-9*plain.Volume {
			t.Errorf("%s: %q %s, want made sound, its volume the untwisted %.9f", c.name, p, r.String(), plain.Volume)
		}
	}
	s := sweepOf(loop(150, 30), 2, nil)
	if p, r := s.Problem(), s.Analyze().Check(); !strings.Contains(p, "too short") || !r.Empty() {
		t.Errorf("bar 4 by 2 round the loop: %q, want refused saying the line is too short, with no solid", p)
	}
}
