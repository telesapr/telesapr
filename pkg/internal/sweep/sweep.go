// Package sweep runs a robustness sweep: a function over every
// combination of a few parameters, such as sizes, angles and
// placements, counting the combinations it fails on. The acceptable
// failure rate is fixed before the feature is built (ROADMAP-JUMP.md,
// "Definition of done"), and a sweep over it fails the test.
//
// By default a sweep runs an even sample of its grid, so that it fits
// in the ordinary test run; SPACESOLVE_SWEEP=1 runs all of it.
package sweep

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// Axis is a parameter and the values it takes.
type Axis struct {
	Name   string
	Values []any
}

// Floats is an axis of numbers.
func Floats(name string, values ...float64) Axis {
	a := Axis{Name: name}
	for _, v := range values {
		a.Values = append(a.Values, v)
	}
	return a
}

// Strings is an axis of names, such as thread designations.
func Strings(name string, values ...string) Axis {
	a := Axis{Name: name}
	for _, v := range values {
		a.Values = append(a.Values, v)
	}
	return a
}

// Bools is an axis that is off, then on.
func Bools(name string) Axis { return Axis{Name: name, Values: []any{false, true}} }

// Point is one combination of the axes' values.
type Point struct {
	names  []string
	values []any
}

func (p Point) value(name string) any {
	for i, n := range p.names {
		if n == name {
			return p.values[i]
		}
	}
	panic(fmt.Sprintf("sweep: no axis %q", name))
}

// Float is the value of a Floats axis.
func (p Point) Float(name string) float64 { return p.value(name).(float64) }

// String is the value of a Strings axis.
func (p Point) String(name string) string { return p.value(name).(string) }

// Bool is the value of a Bools axis.
func (p Point) Bool(name string) bool { return p.value(name).(bool) }

// Value is the value of an axis of any kind.
func (p Point) Value(name string) any { return p.value(name) }

// Label names the point: "r=2 angle=30".
func (p Point) Label() string {
	parts := make([]string, len(p.names))
	for i, n := range p.names {
		parts[i] = fmt.Sprintf("%s=%v", n, p.values[i])
	}
	return strings.Join(parts, " ")
}

// Grid is a sweep: every combination of its axes.
type Grid struct {
	Axes []Axis
	// MaxFailRate is the fraction of points allowed to fail, from 0 to
	// 1, fixed before the feature is built.
	MaxFailRate float64
	// Sample is how many points the ordinary test run takes, spread
	// evenly over the grid; 0 takes them all.
	Sample int
	// SampleMaxFailRate is the fraction of the sample allowed to fail,
	// when a sample is too small to show the grid's rate; 0 is
	// MaxFailRate.
	SampleMaxFailRate float64
}

// Size is the number of points in the grid.
func (g Grid) Size() int {
	n := 1
	for _, a := range g.Axes {
		n *= len(a.Values)
	}
	return n
}

// At is the i-th point, the last axis varying fastest.
func (g Grid) At(i int) Point {
	p := Point{names: make([]string, len(g.Axes)), values: make([]any, len(g.Axes))}
	for k := len(g.Axes) - 1; k >= 0; k-- {
		a := g.Axes[k]
		p.names[k] = a.Name
		p.values[k] = a.Values[i%len(a.Values)]
		i /= len(a.Values)
	}
	return p
}

// Full reports whether sweeps run their whole grid: when
// SPACESOLVE_SWEEP is set.
func Full() bool { return os.Getenv("SPACESOLVE_SWEEP") != "" }

// Failure is a point the function failed on, and why.
type Failure struct {
	Point Point
	Err   error
}

// Report is what a sweep found.
type Report struct {
	Runs     int
	Failed   []Failure
	Elapsed  time.Duration
	Full     bool
	GridSize int
	// MaxFailRate is the grid's, or for a sample its own.
	MaxFailRate float64
}

// FailRate is the fraction of the runs that failed.
func (r Report) FailRate() float64 {
	if r.Runs == 0 {
		return 0
	}
	return float64(len(r.Failed)) / float64(r.Runs)
}

// String sums the report up on one line.
func (r Report) String() string {
	of := ""
	if !r.Full {
		of = fmt.Sprintf(", a sample of %d", r.GridSize)
	}
	return fmt.Sprintf("%d of %d failed (%.1f%%, %.1f%% allowed%s) in %v",
		len(r.Failed), r.Runs, 100*r.FailRate(), 100*r.MaxFailRate, of, r.Elapsed.Round(time.Millisecond))
}

// Run calls fn on the sample, or on the whole grid when Full; an error
// or a panic is a failure.
func (g Grid) Run(fn func(p Point) error) Report {
	return g.run(Full(), fn)
}

// RunAll calls fn on every point of the grid.
func (g Grid) RunAll(fn func(p Point) error) Report {
	return g.run(true, fn)
}

func (g Grid) run(full bool, fn func(p Point) error) Report {
	size := g.Size()
	r := Report{Full: full || g.Sample == 0 || g.Sample >= size, GridSize: size, MaxFailRate: g.MaxFailRate}
	if !r.Full && g.SampleMaxFailRate > 0 {
		r.MaxFailRate = g.SampleMaxFailRate
	}
	start := time.Now()
	n, stride := size, 1
	if !r.Full {
		// A stride coprime with the size visits n distinct points
		// spread over every axis; one that divides it would keep to a
		// few values of the faster axes.
		n, stride = g.Sample, max(1, size/g.Sample)
		for gcd(stride, size) != 1 {
			stride++
		}
	}
	for k := range n {
		p := g.At(k * stride % size)
		if err := call(fn, p); err != nil {
			r.Failed = append(r.Failed, Failure{Point: p, Err: err})
		}
		r.Runs++
	}
	r.Elapsed = time.Since(start)
	return r
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// call runs fn, turning a panic into an error.
func call(fn func(p Point) error, p Point) (err error) {
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("panic: %v", rec)
		}
	}()
	return fn(p)
}

// Require logs every failure and the summary, and fails t when more
// failed than the grid allows.
func (r Report) Require(t testing.TB) {
	t.Helper()
	for _, f := range r.Failed {
		t.Logf("failed at %s: %v", f.Point.Label(), f.Err)
	}
	if r.FailRate() > r.MaxFailRate {
		t.Errorf("sweep: %s", r)
	} else {
		t.Logf("sweep: %s", r)
	}
}
