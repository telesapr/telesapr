package sweep

import (
	"errors"
	"fmt"
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
	t.Setenv("SPACESOLVE_SWEEP", "")
	g := grid()
	var labels []string
	r := g.Run(func(p Point) error { labels = append(labels, p.Label()); return nil })
	if r.Runs != 4 || r.Full || len(labels) != 4 || r.GridSize != 12 {
		t.Fatalf("sample: %+v %v", r, labels)
	}
	// Spread over the grid, so every radius is met.
	if !strings.HasPrefix(labels[0], "r=1") || !strings.HasPrefix(labels[3], "r=3") {
		t.Errorf("the sample is not spread: %v", labels)
	}
	if !strings.Contains(r.String(), "a sample of 12") {
		t.Errorf("summary %q", r.String())
	}

	if r := g.RunAll(func(Point) error { return nil }); r.Runs != 12 || !r.Full {
		t.Errorf("all: %+v", r)
	}
	t.Setenv("SPACESOLVE_SWEEP", "1")
	if r := g.Run(func(Point) error { return nil }); r.Runs != 12 || !r.Full {
		t.Errorf("full: %+v", r)
	}
	g.Sample = 0
	if r := g.run(false, func(Point) error { return nil }); r.Runs != 12 {
		t.Errorf("no sample size runs %d", r.Runs)
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
	if (Report{}).FailRate() != 0 {
		t.Error("an empty report has failures")
	}
}
