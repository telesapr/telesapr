package sketch_test

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// jointCase is a mate of the plates of matePart and the freedom it
// leaves the second: its counts, invariant and joint, and a point the
// invariant goes through.
type jointCase struct {
	name          string
	mate          func(t *testing.T, d *api.Doc, a, b api.Group) ([]api.Constraint, api.Vec)
	constraints   int
	turns, slides int
	invariant     string
	joint         string
}

// checkJoint makes each case's mate, one step of undo that redo brings
// back identical, and requires the part solved with the freedom the
// joint leaves, its invariant through the point the case gives.
func checkJoint(t *testing.T, cases []jointCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d, a, b := matePart(t)
			d.Sketch.Anywhere3D()
			d.Edit.Regen()
			step(d)
			before := saved(t, d)
			cs, at := tc.mate(t, d, a, b)
			if len(cs) != tc.constraints || !b.Solved() {
				t.Fatalf("%d constraints, want %d; solved %v", len(cs), tc.constraints, b.Solved())
			}
			f := b.AsLink().Freedom()
			if f.Turns != tc.turns || f.Slides != tc.slides || f.Invariant != tc.invariant || f.Joint != tc.joint || f.Moving {
				t.Fatalf("%+v; want %d turns, %d slides, a fixed %s, a %s", f, tc.turns, tc.slides, tc.invariant, tc.joint)
			}
			off := at.Minus(f.Point)
			switch tc.invariant {
			case "axis", "line":
				off = off.Cross(f.Dir)
			case "plane":
				off = f.Dir.ScaledBy(off.Dot(f.Dir))
			}
			if off.Magnitude() > 1e-6 {
				t.Errorf("%s through %v, %v: %v is %g off it", tc.invariant, f.Point, f.Dir, at, off.Magnitude())
			}
			undoRedo(t, d, before)
		})
	}
}

// jointsRefused requires each call to fail with an error, not a panic of
// another kind, and to leave the document as it was. The calls are made
// in 3d, but for those named "in a workplane", made in XY.
func jointsRefused(t *testing.T, calls map[string]func(d *api.Doc, a, b api.Group)) {
	t.Helper()
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			d, a, b := matePart(t)
			if name == "in a workplane" {
				d.Sketch.InWorkplane(d.Workplane("XY"))
			} else {
				d.Sketch.Anywhere3D()
			}
			d.Edit.Regen()
			before := saved(t, d)
			func() {
				defer func() {
					if _, ok := recover().(error); !ok {
						t.Errorf("%s: no error", name)
					}
				}()
				call(d, a, b)
			}()
			if string(saved(t, d)) != string(before) {
				t.Errorf("%s: the refused mate changed the document", name)
			}
		})
	}
}

var anyEntity = func(api.Entity) bool { return true }

// A ball mate puts a point of the second plate on a point of the
// first, or the centres of two circles on each other: the plate turns
// every way about it and doesn't slide. Lines and faces are refused,
// and so is a ball mate in a workplane.
func TestBallMate(t *testing.T) {
	t.Parallel()
	checkJoint(t, []jointCase{
		{"points", func(t *testing.T, d *api.Doc, a, b api.Group) ([]api.Constraint, api.Vec) {
			p := first(t, a, "point", anyEntity)
			return d.Assembly.Ball(p, first(t, b, "point", anyEntity)), p.Pos()
		}, 1, 3, 0, "point", "ball"},
		{"centres", func(t *testing.T, d *api.Doc, a, b api.Group) ([]api.Constraint, api.Vec) {
			c := first(t, a, "circle", anyEntity)
			return d.Assembly.Ball(c, first(t, b, "circle", anyEntity)), c.Center().Pos()
		}, 1, 3, 0, "point", "ball"},
	})
	jointsRefused(t, map[string]func(d *api.Doc, a, b api.Group){
		"lines": func(d *api.Doc, a, b api.Group) {
			d.Assembly.Ball(first(t, a, "line", anyEntity), first(t, b, "line", anyEntity))
		},
		"in a workplane": func(d *api.Doc, a, b api.Group) {
			d.Assembly.Ball(first(t, a, "point", anyEntity), first(t, b, "point", anyEntity))
		},
	})
}

// A cylindrical mate puts the second plate's axis on the first's: two
// edges, an edge and a hole's axis, or two holes' axes, as Concentric
// does. The plate turns about the axis and slides along it. Faces and
// points are refused.
func TestCylindricalMate(t *testing.T) {
	t.Parallel()
	checkJoint(t, []jointCase{
		{"lines", func(t *testing.T, d *api.Doc, a, b api.Group) ([]api.Constraint, api.Vec) {
			l := first(t, a, "line", anyEntity)
			return d.Assembly.Cylindrical(l, first(t, b, "line", anyEntity)), l.Start().Pos()
		}, 2, 1, 1, "axis", "cylindrical"},
		{"line and circle", func(t *testing.T, d *api.Doc, a, b api.Group) ([]api.Constraint, api.Vec) {
			// An upright edge, near the hole's axis in direction, as a
			// rod put into a hole.
			l := first(t, a, "line", func(e api.Entity) bool {
				v := e.End().Pos().Minus(e.Start().Pos())
				return v.Cross(api.V(0, 0, 1)).Magnitude() < 1e-9
			})
			return d.Assembly.Cylindrical(l, first(t, b, "circle", anyEntity)), l.Start().Pos()
		}, 2, 1, 1, "axis", "cylindrical"},
		{"circles", func(t *testing.T, d *api.Doc, a, b api.Group) ([]api.Constraint, api.Vec) {
			c := first(t, a, "circle", anyEntity)
			return d.Assembly.Cylindrical(c, first(t, b, "circle", anyEntity)), c.Center().Pos()
		}, 3, 1, 1, "axis", "cylindrical"},
	})
	jointsRefused(t, map[string]func(d *api.Doc, a, b api.Group){
		"faces": func(d *api.Doc, a, b api.Group) {
			_, top := ends(a)
			bot, _ := ends(b)
			d.Assembly.Cylindrical(top, bot)
		},
		"points": func(d *api.Doc, a, b api.Group) {
			d.Assembly.Cylindrical(first(t, a, "point", anyEntity), first(t, b, "point", anyEntity))
		},
		"in a workplane": func(d *api.Doc, a, b api.Group) {
			d.Assembly.Cylindrical(first(t, a, "line", anyEntity), first(t, b, "line", anyEntity))
		},
		// A flat edge for a hole's axis: the part would have to turn
		// a quarter turn, from where the solve can't make them parallel.
		"square to the axis": func(d *api.Doc, a, b api.Group) {
			d.Assembly.Cylindrical(first(t, a, "line", anyEntity), first(t, b, "circle", anyEntity))
		},
	})
}

// A planar mate lays the second plate's bottom on the first's top: it
// slides two ways on it and turns about its normal. Lines are refused.
func TestPlanarMate(t *testing.T) {
	t.Parallel()
	checkJoint(t, []jointCase{
		{"faces", func(t *testing.T, d *api.Doc, a, b api.Group) ([]api.Constraint, api.Vec) {
			_, top := ends(a)
			bot, _ := ends(b)
			return d.Assembly.Planar(top, bot), top.FacePoint()
		}, 2, 1, 2, "plane", "planar"},
	})
	jointsRefused(t, map[string]func(d *api.Doc, a, b api.Group){
		"lines": func(d *api.Doc, a, b api.Group) {
			d.Assembly.Planar(first(t, a, "line", anyEntity), first(t, b, "line", anyEntity))
		},
	})
}

// A slot mate puts a corner of the second plate on an edge of the
// first, or the first's corner on the second's edge: the corner slides
// along the edge and the plate turns every way about it. Two points or
// two lines are refused.
func TestSlotMate(t *testing.T) {
	t.Parallel()
	checkJoint(t, []jointCase{
		{"line then point", func(t *testing.T, d *api.Doc, a, b api.Group) ([]api.Constraint, api.Vec) {
			l := first(t, a, "line", anyEntity)
			return d.Assembly.Slot(l, first(t, b, "point", anyEntity)), l.Start().Pos()
		}, 1, 3, 1, "line", "slot"},
		{"centre then line", func(t *testing.T, d *api.Doc, a, b api.Group) ([]api.Constraint, api.Vec) {
			c := first(t, a, "circle", anyEntity)
			return d.Assembly.Slot(c, first(t, b, "line", anyEntity)), c.Center().Pos()
		}, 1, 3, 1, "line", "slot"},
	})
	jointsRefused(t, map[string]func(d *api.Doc, a, b api.Group){
		"points": func(d *api.Doc, a, b api.Group) {
			d.Assembly.Slot(first(t, a, "point", anyEntity), first(t, b, "point", anyEntity))
		},
		"lines": func(d *api.Doc, a, b api.Group) {
			d.Assembly.Slot(first(t, a, "line", anyEntity), first(t, b, "line", anyEntity))
		},
	})
}
