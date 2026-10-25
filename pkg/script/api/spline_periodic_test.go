package api

import "testing"

// A spline drawn through its points is closed into a periodic one, as
// the mouse does by clicking the first point again.
func TestMakePeriodic(t *testing.T) {
	t.Parallel()
	d := New()
	s := d.Sketch.Cubic(V(-30, 0, 0), V(-20, 15, 0), V(20, -15, 0), V(30, 0, 0))
	for _, p := range []Vec{V(-20, 15, 0), V(0, 0, 0), V(20, -15, 0)} {
		s.AddSplinePoint(p)
	}
	if s.Kind() != "cubic" || len(s.Points()) != 7 {
		t.Fatalf("the spline is a %s of %d points", s.Kind(), len(s.Points()))
	}
	s.MakePeriodic()
	if s.Kind() != "periodic-cubic" {
		t.Fatalf("after closing, the spline is a %s", s.Kind())
	}
	points := s.Points()
	if len(points) != 4 {
		t.Fatalf("a closed spline has %d points", len(points))
	}
	for i, p := range points {
		if !p.Pos().Equals(V(-30, 0, 0)) && !p.Pos().Equals(V(-20, 15, 0)) &&
			!p.Pos().Equals(V(0, 0, 0)) && !p.Pos().Equals(V(20, -15, 0)) {
			t.Errorf("point %d is at %v, where the spline was not drawn", i, p.Pos())
		}
	}
	if !d.Active().Solved() {
		t.Errorf("the sketch does not solve: %s", d.Active().Problem())
	}
	// Closing it again is what it is.
	s.MakePeriodic()
	if s.Kind() != "periodic-cubic" || len(s.Points()) != 4 {
		t.Errorf("closing it twice: a %s of %d points", s.Kind(), len(s.Points()))
	}
}

// Only a spline with four points to go round can be closed, and
// nothing else can be closed at all.
func TestMakePeriodicMisuse(t *testing.T) {
	t.Parallel()
	d := New()
	short := d.Sketch.Cubic(V(0, 0, 0), V(10, 0, 0), V(20, 0, 0), V(30, 0, 0))
	line := d.Sketch.Line(V(0, 0, 0), V(10, 0, 0))
	for name, fn := range map[string]func(){
		"too few points": func() { short.MakePeriodic() },
		"a line segment": func() { line.MakePeriodic() },
	} {
		func() {
			defer func() {
				if _, ok := recover().(error); !ok {
					t.Errorf("%s: no error", name)
				}
			}()
			fn()
		}()
	}
	if short.Kind() != "cubic" || len(short.Points()) != 4 {
		t.Errorf("the spline was closed after all: a %s of %d points", short.Kind(), len(short.Points()))
	}
}
