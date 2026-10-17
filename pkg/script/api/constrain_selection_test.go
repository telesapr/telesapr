package api

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
)

// A line segment and an arc that share no end are tangent at the end of
// the arc a script names; with none named, the call says what to give.
func TestTangentAtChosenEnd(t *testing.T) {
	t.Parallel()
	d := New()
	line := d.Sketch.Line(V(0, 0, 0), V(10, 0, 0))
	arc := d.Sketch.Arc(V(20, 0, 0), V(30, 0, 0), V(20, 10, 0))
	mustFail(t, "tangent with no end", "share an endpoint", func() { d.Constrain.Tangent(line, arc) })
	mustFail(t, "tangent at a foreign point", "does not belong", func() { d.Constrain.Tangent(line, arc, line.Start()) })

	atStart := d.Constrain.Tangent(line, arc, arc.Start())
	if atStart.Type() != core.ConstraintArcLineTangent || atStart.Other() {
		t.Errorf("at the start: %v, other %v; want arc-line-tangent at the first end", atStart, atStart.Other())
	}
	atEnd := d.Constrain.Tangent(arc, line, arc.End())
	if !atEnd.Other() {
		t.Errorf("at the end: other is false, want the arc's second end")
	}
}

// Two arcs that share no end are tangent at the ends a script names.
func TestTangentCurvesAtChosenEnds(t *testing.T) {
	t.Parallel()
	d := New()
	a := d.Sketch.Arc(V(0, 0, 0), V(10, 0, 0), V(0, 10, 0))
	b := d.Sketch.Arc(V(30, 0, 0), V(40, 0, 0), V(30, 10, 0))
	mustFail(t, "no ends", "share an endpoint", func() { d.Constrain.Tangent(a, b) })
	mustFail(t, "one end", "two curves take", func() { d.Constrain.Tangent(a, b, a.Start()) })
	c := d.Constrain.Tangent(a, b, a.End(), b.Start())
	if c.Type() != core.ConstraintCurveCurveTangent || !c.Other() {
		t.Errorf("%v, other %v; want curve-curve tangent at a's end", c, c.Other())
	}
}

// Two faces are parallel and perpendicular; a face and a line segment
// are neither.
func TestParallelPerpendicularFaces(t *testing.T) {
	t.Parallel()
	d := New()
	d.Sketch.Rect(V(0, 0, 0), V(10, 10, 0))
	ext := d.Features.Extrude(5, "")
	faces := ext.Faces()
	d.Features.SketchIn(d.Workplane("XY"), "")
	if len(faces) < 3 {
		t.Fatalf("the extrusion has %d faces", len(faces))
	}
	p := d.Constrain.Parallel(faces[0], faces[1])
	if p.Type() != core.ConstraintParallel {
		t.Errorf("%v, want parallel", p)
	}
	q := d.Constrain.Perpendicular(faces[0], faces[2])
	if q.Type() != core.ConstraintPerpendicular {
		t.Errorf("%v, want perpendicular", q)
	}
	line := d.Sketch.Line(V(0, 0, 0), V(1, 0, 0))
	mustFail(t, "face and line", "two faces", func() { d.Constrain.Parallel(faces[0], line) })
}

// The middle of a line segment is in a workplane, and the point at a
// middle loses the on-line constraint it had.
func TestMidpointInWorkplane(t *testing.T) {
	t.Parallel()
	d := New()
	line := d.Sketch.Line(V(0, 0, 0), V(10, 0, 0))
	c := d.Constrain.Midpoint(d.Workplane("XY"), line)
	if c.Type() != core.ConstraintAtMidpoint {
		t.Errorf("%v, want at-midpoint", c)
	}
	mustFail(t, "midpoint of a point", "line segment", func() { d.Constrain.Midpoint(line.Start(), line.End()) })

	p := d.Sketch.Point(V(5, 0, 0))
	d.Constrain.On(p, line)
	n := len(d.Host().Sketch().Constraints)
	d.Constrain.Midpoint(p, line)
	if got := len(d.Host().Sketch().Constraints); got != n {
		t.Errorf("%d constraints after the midpoint replaced the on-line one, want %d", got, n)
	}
}

// A comment is written next to a point.
func TestCommentOnPoint(t *testing.T) {
	t.Parallel()
	d := New()
	p := d.Sketch.Point(V(1, 2, 0))
	c := d.Constrain.CommentOn(p, "here")
	if c.Type() != core.ConstraintComment || c.Comment() != "here" {
		t.Errorf("%v with %q", c, c.Comment())
	}
	if got := d.Host().Sketch().Constraint(c.H).PtA; got != p.H {
		t.Errorf("the comment is on %v, want %v", got, p.H)
	}
	mustFail(t, "comment on a line", "point", func() { d.Constrain.CommentOn(d.Sketch.Line(V(0, 0, 0), V(1, 0, 0)), "x") })
}

// A line segment stands for its two ends in Symmetric, and symmetric
// about the axis it drops the horizontal constraint it had.
func TestSymmetricLineSegment(t *testing.T) {
	t.Parallel()
	d := New()
	d.Features.SketchIn(d.Workplane("XY"), "")
	line := d.Sketch.Line(V(-5, 0, 0), V(5, 0, 0))
	count := func(typ core.ConstraintType) (n int) {
		for _, cc := range d.Host().Sketch().Constraints {
			if cc.Type == typ {
				n++
			}
		}
		return n
	}
	d.Constrain.Horizontal(line)
	c := d.Constrain.Symmetric(line, Entity{}, Entity{})
	if c.Type() != core.ConstraintSymmetricHoriz {
		t.Errorf("%v, want symmetric about the vertical axis", c)
	}
	if n := count(core.ConstraintHorizontal); n != 0 {
		t.Errorf("%d horizontal constraints are still there", n)
	}

	// About a workplane the line keeps what it has.
	other := d.Sketch.Line(V(-3, 4, 0), V(3, 4, 0))
	d.Constrain.Horizontal(other)
	w := d.Constrain.Symmetric(other, Entity{}, d.Workplane("YZ"))
	if w.Type() != core.ConstraintSymmetric {
		t.Errorf("%v, want symmetric about a workplane", w)
	}
	if count(core.ConstraintHorizontal) != 1 {
		t.Errorf("symmetric about a workplane took the horizontal constraint away")
	}
	mustFail(t, "point and nothing", "line segment, or a point", func() { d.Constrain.Symmetric(line.Start(), Entity{}, Entity{}) })
}
