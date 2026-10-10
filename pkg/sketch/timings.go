package sketch

import (
	"slices"
	"sync"
	"time"

	"github.com/telesapr/telesapr/pkg/core"
)

// Timings collects how long regenerations spend on each group: solving
// it, making its loops and building its solid. A sketch is given one
// (Sketch.Timings) by whoever wants the figures, as telesapr-cli's
// --profile does; with none, nothing is timed. Solids may be built on
// several goroutines, so it locks.
type Timings struct {
	mu     sync.Mutex
	groups []GroupTiming
}

// GroupTiming is what one group took, summed over every regeneration
// that worked on it, and how many times each step ran.
type GroupTiming struct {
	Group                    core.HGroup
	Solve, Loops, Solid      time.Duration
	Solves, LoopRuns, Solids int
}

// timingStep is a step a regeneration times.
type timingStep int

const (
	timingSolve timingStep = iota
	timingLoops
	timingSolid
)

// add counts d for group hg's step.
func (t *Timings) add(hg core.HGroup, step timingStep, d time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	i := slices.IndexFunc(t.groups, func(g GroupTiming) bool { return g.Group == hg })
	if i < 0 {
		t.groups = append(t.groups, GroupTiming{Group: hg})
		i = len(t.groups) - 1
	}
	g := &t.groups[i]
	switch step {
	case timingSolve:
		g.Solve += d
		g.Solves++
	case timingLoops:
		g.Loops += d
		g.LoopRuns++
	case timingSolid:
		g.Solid += d
		g.Solids++
	}
}

// Groups are the groups' figures, in the order they were first timed.
func (t *Timings) Groups() []GroupTiming {
	t.mu.Lock()
	defer t.mu.Unlock()
	return slices.Clone(t.groups)
}

// timed starts timing group hg's step when the sketch has Timings, and
// returns what stops it.
func (sk *Sketch) timed(hg core.HGroup, step timingStep) func() {
	if sk.Timings == nil {
		return func() {}
	}
	start := time.Now()
	return func() { sk.Timings.add(hg, step, time.Since(start)) }
}
