package sweep

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

func grid() Grid {
	return Grid{
		Axes:        []Axis{Floats("r", 1, 2, 3), Strings("kind", "a", "b"), Bools("flip")},
		MaxFailRate: 0.25,
		Sample:      4,
	}
}

func TestGridPoints(t *testing.T) {
	g := grid()
	if g.Size() != 12 {
		t.Fatalf("size %d, want 12", g.Size())
	}
	seen := map[string]bool{}
	for i := range g.Size() {
		seen[g.At(i).Label()] = true
	}
	if len(seen) != 12 {
		t.Errorf("%d distinct points of 12", len(seen))
	}
	p := g.At(11)
	if p.Float("r") != 3 || p.String("kind") != "b" || !p.Bool("flip") || p.Value("kind") != "b" {
		t.Errorf("last point %s", p.Label())
	}
	if l := g.At(0).Label(); l != "r=1 kind=a flip=false" {
		t.Errorf("first point %q", l)
	}
}

func TestNoSuchAxis(t *testing.T) {
	defer func() {
		if r := recover(); r == nil || !strings.Contains(fmt.Sprint(r), `"depth"`) {
			t.Errorf("recovered %v", r)
		}
	}()
	grid().At(0).Float("depth")
}

func TestSampleAndAll(t *testing.T) {
	t.Setenv("TELESAPR_SWEEP", "")
	g := grid()
	var labels []string
	// run as Run is without the race detector, which thins it.
	r := g.run(false, false, func(p Point) error { labels = append(labels, p.Label()); return nil })
	if r.Runs != 4 || r.Full || len(labels) != 4 || r.GridSize != 12 {
		t.Fatalf("sample: %+v %v", r, labels)
	}
	// Distinct points, spread over the grid, so every radius is met.
	seen, radii := map[string]bool{}, map[string]bool{}
	for _, l := range labels {
		seen[l] = true
		radii[strings.Fields(l)[0]] = true
	}
	if len(seen) != 4 || len(radii) != 3 {
		t.Errorf("the sample is not spread: %v", labels)
	}
	if !strings.Contains(r.String(), "a sample of 12") {
		t.Errorf("summary %q", r.String())
	}

	if r := g.RunAll(func(Point) error { return nil }); r.Runs != 12 || !r.Full {
		t.Errorf("all: %+v", r)
	}
	t.Setenv("TELESAPR_SWEEP", "1")
	if r := g.Run(func(Point) error { return nil }); r.Runs != 12 || !r.Full {
		t.Errorf("full: %+v", r)
	}
	g.Sample = 0
	if r := g.run(false, false, func(Point) error { return nil }); r.Runs != 12 {
		t.Errorf("no sample size runs %d", r.Runs)
	}
}

// Under the race detector a sweep takes a fifth of its points, some of
// those it takes without, and is allowed as many failures as they are.
func TestSweepThinned(t *testing.T) {
	t.Parallel()
	g := Grid{Axes: []Axis{Floats("i", 0, 1, 2, 3, 4, 5, 6, 7, 8, 9)}, MaxFailRate: 0.1}
	var plain, thinned []string
	g.run(true, false, func(p Point) error { plain = append(plain, p.Label()); return nil })
	r := g.run(true, true, func(p Point) error {
		thinned = append(thinned, p.Label())
		return errors.New("fails")
	})
	if r.Runs != 2 || r.Full || len(thinned) != 2 {
		t.Fatalf("thinned: %+v %v", r, thinned)
	}
	for _, l := range thinned {
		if !slices.Contains(plain, l) {
			t.Errorf("%s is no point of the plain run %v", l, plain)
		}
	}
	// One failure of ten allowed, so one of the two, not both.
	if r.MaxFailRate != 0.5 {
		t.Errorf("allowed %g", r.MaxFailRate)
	}
	rec := &recorder{TB: t}
	r.Require(rec)
	if !rec.failed {
		t.Error("two failures passed where one is allowed")
	}
}

// recorder is a testing.TB that keeps what a report says.
type recorder struct {
	testing.TB
	logs   []string
	failed bool
}

func (r *recorder) Helper() {}
func (r *recorder) Logf(format string, args ...any) {
	r.logs = append(r.logs, fmt.Sprintf(format, args...))
}
func (r *recorder) Errorf(format string, args ...any) {
	r.failed = true
	r.Logf(format, args...)
}

// A sample whose even stride divides the grid still meets every value
// of the fastest axis.
func TestSampleSpread(t *testing.T) {
	g := Grid{Axes: []Axis{Floats("a", 1, 2, 3, 4, 5, 6, 7, 8), Floats("b", 0, 1, 2, 3, 4, 5, 6, 7, 8, 9)}, Sample: 16}
	seen, bs := map[string]bool{}, map[float64]bool{}
	g.run(false, false, func(p Point) error {
		seen[p.Label()] = true
		bs[p.Float("b")] = true
		return nil
	})
	if len(seen) != 16 || len(bs) < 8 {
		t.Errorf("%d distinct points, %d values of b", len(seen), len(bs))
	}
}

func TestFailures(t *testing.T) {
	g := grid()
	fails := func(p Point) error {
		switch {
		case p.Float("r") == 3 && p.Bool("flip"):
			return errors.New("too big")
		case p.Float("r") == 2 && p.String("kind") == "b" && p.Bool("flip"):
			panic("boom")
		}
		return nil
	}
	r := g.RunAll(fails)
	if len(r.Failed) != 3 || r.FailRate() != 0.25 {
		t.Fatalf("%d failed, rate %g: %v", len(r.Failed), r.FailRate(), r.Failed)
	}
	if !strings.Contains(r.Failed[0].Err.Error(), "panic: boom") {
		t.Errorf("first failure %v", r.Failed[0].Err)
	}
	rec := &recorder{}
	r.Require(rec)
	if rec.failed || len(rec.logs) != 4 {
		t.Errorf("at the allowed rate: failed %t, logs %v", rec.failed, rec.logs)
	}

	g.MaxFailRate = 0.2
	rec = &recorder{}
	g.RunAll(fails).Require(rec)
	if !rec.failed || !strings.Contains(rec.logs[len(rec.logs)-1], "3 of 12 failed (25.0%, 20.0% allowed)") {
		t.Errorf("over the rate: failed %t, logs %v", rec.failed, rec.logs)
	}
	// A sample may have its own allowance.
	g.SampleMaxFailRate = 0.5
	if r := g.run(false, false, fails); r.MaxFailRate != 0.5 || r.Full {
		t.Errorf("sample allowance %g, full %t", r.MaxFailRate, r.Full)
	}
	if r := g.RunAll(fails); r.MaxFailRate != 0.2 {
		t.Errorf("full allowance %g", r.MaxFailRate)
	}
	// A whole grid that fails less than it allows is a stale allowance;
	// a sample tells nothing about it.
	g.MaxFailRate, g.SampleMaxFailRate = 0.5, 0
	rec = &recorder{}
	g.RunAll(fails).Require(rec)
	if !rec.failed || !strings.Contains(rec.logs[len(rec.logs)-1], "lower the allowance to 3 of 12") {
		t.Errorf("under the rate: failed %t, logs %v", rec.failed, rec.logs)
	}
	rec = &recorder{}
	g.run(false, false, fails).Require(rec)
	if rec.failed {
		t.Errorf("a sample under the rate: logs %v", rec.logs)
	}
	// A rate that allows part of a failure allows none more.
	g.MaxFailRate = 3.5 / 12
	rec = &recorder{}
	g.RunAll(fails).Require(rec)
	if rec.failed {
		t.Errorf("at the rate rounded down: logs %v", rec.logs)
	}
	if (Report{}).FailRate() != 0 {
		t.Error("an empty report has failures")
	}
}

// A grid run in parallel fails at the same points, in the same order,
// as run on one goroutine.
func TestParallelFailsAtSamePoints(t *testing.T) {
	t.Parallel()
	g := grid()
	fails := func(p Point) error {
		if p.Float("r") != 2 && p.Bool("flip") {
			return fmt.Errorf("at %s", p.Label())
		}
		return nil
	}
	serial := g.RunAll(fails)
	g.Parallel = true
	parallel := g.RunAll(fails)
	if parallel.Runs != serial.Runs || len(parallel.Failed) != len(serial.Failed) {
		t.Fatalf("parallel %s, serial %s", parallel, serial)
	}
	for i, f := range serial.Failed {
		if p := parallel.Failed[i]; p.Point.Label() != f.Point.Label() || p.Err.Error() != f.Err.Error() {
			t.Errorf("failure %d: parallel %s %v, serial %s %v", i, p.Point.Label(), p.Err, f.Point.Label(), f.Err)
		}
	}
}
