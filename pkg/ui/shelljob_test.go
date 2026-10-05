package ui

import (
	"math"
	"testing"
	"time"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// waitShells runs the headless timers until the job running ends.
func waitShells(t *testing.T, ss *Application, p interface{ RunDueTimers(int) int }) {
	t.Helper()
	for range 5000 {
		if ss.shellJobs.job == nil {
			return
		}
		p.RunDueTimers(10)
		time.Sleep(time.Millisecond)
	}
	t.Fatal("the job didn't end")
}

// In the application the solids of a scheduled regeneration are made
// off the thread (roadmap 17.3): the entities follow at once, the solid
// stays as it was until the job ends, then is the one a regeneration in
// full makes; Esc stops a job, leaving the solid out of date.
func TestBackgroundShells(t *testing.T) {
	ss, p := newTestApp(t)
	ss.BackgroundShells = true
	doc := api.NewDoc(ss.ScriptHost())
	sides := doc.Rect(api.V(0, 0, 0), api.V(20, 10, 0))
	width := doc.Length(sides[0], 20)
	block := doc.Extrude(10, "")
	if v := block.Check().Volume; math.Abs(v-2000) > 1e-6 {
		t.Fatalf("volume %g", v)
	}

	// An edit the interface schedules.
	c := ss.SK.GetConstraint(width.H)
	c.ValA = 30
	ss.SK.MarkGroupDirty(c.Group, false)
	ss.Refresh()
	g := ss.SK.GetGroup(block.H)
	if !g.ShellsStale || ss.shellJobs.job == nil {
		t.Fatalf("stale %v, job %v after a scheduled regeneration", g.ShellsStale, ss.shellJobs.job)
	}
	waitShells(t, ss, p)
	g = ss.SK.GetGroup(block.H)
	if g.ShellsStale {
		t.Fatal("still stale after the job")
	}
	if v := block.Check().Volume; math.Abs(v-3000) > 1e-6 {
		t.Errorf("volume %g after the job, want 3000", v)
	}

	// Esc stops the next one.
	c = ss.SK.GetConstraint(width.H)
	c.ValA = 40
	ss.SK.MarkGroupDirty(c.Group, false)
	ss.Refresh()
	if !ss.stopShellJob() {
		t.Fatal("no job to stop")
	}
	p.RunDueTimers(10)
	if !ss.SK.GetGroup(block.H).ShellsStale {
		t.Error("a stopped job's solid isn't out of date")
	}
}
