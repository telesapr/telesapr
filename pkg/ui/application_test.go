package ui

import (
	"testing"
	"time"

	"github.com/telesapr/telesapr/pkg/sketch"
)

// A linked solid keeps the tolerance it was read at, so the links are
// read again when the tolerance in effect changes. The settings keep it
// through f32 (config.go), so from the second run on what they hold is
// 0.10000000149011612 where the sketch's own default is the clean 0.1;
// that is not a change of tolerance, and must not read every linked
// file again.
func TestApplyEngineSettingsReloadsLinksOnAToleranceChange(t *testing.T) {
	t.Parallel()
	ss, _ := newTestApp(t)
	reloads := 0
	ss.SK.ReloadAllLinked = func(*sketch.Sketch) { reloads++ }

	for _, tc := range []struct {
		tol  float64
		want int
	}{
		{0.1, 0},                   // the default, which the sketch already has
		{float64(float32(0.1)), 0}, // the default as a settings file holds it
		{0.5, 1},                   // a tolerance the user set
		{0.1, 2},                   // and back to the default
	} {
		ss.ChordTol = tc.tol
		ss.applyEngineSettings()
		if reloads != tc.want {
			t.Fatalf("with the tolerance at %g the links have been read %d times, want %d",
				tc.tol, reloads, tc.want)
		}
		if got := ss.SK.ChordTolPercent(); got != tc.tol {
			t.Errorf("the sketch's tolerance is %g, want %g", got, tc.tol)
		}
	}
}

// The configuration screen's time for finding the constraints to remove
// is what the sketch's solver is given.
func TestApplyEngineSettingsPassesFindToFixTimeout(t *testing.T) {
	t.Parallel()
	ss, _ := newTestApp(t)
	ss.TimeoutRedundantConstr = 2500
	ss.applyEngineSettings()
	if got, want := ss.SK.FindToFixTimeout, 2500*time.Millisecond; got != want {
		t.Errorf("the sketch's search may take %v, want %v", got, want)
	}

	// A test's own time takes its place.
	ss.findToFixTimeout = time.Hour
	ss.applyEngineSettings()
	if got := ss.SK.FindToFixTimeout; got != time.Hour {
		t.Errorf("with the test's time set the sketch's search may take %v, want an hour", got)
	}
}
