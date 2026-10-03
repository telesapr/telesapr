package api

import (
	"math"
	"path/filepath"
	"testing"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

// A plate with a hole is drawn, dimensioned and extruded, and comes
// back from a file as it was.
func TestPlate(t *testing.T) {
	d := New()
	if got := len(d.Groups()); got != 1 {
		t.Fatalf("a new sketch has %d groups, want 1", got)
	}
	sides := d.Rect(V(0, 0, 0), V(30, 20, 0))
	width := d.Length(sides[0], 40)
	d.Length(sides[1], 25)
	d.Coincident(sides[0].Start(), d.Origin())
	hole := d.Circle(V(10, 10, 0), 3)
	d.Diameter(hole, 8)
	d.Distance(hole.Center(), sides[0], 12)
	d.Distance(hole.Center(), sides[3], 15)

	g := d.Active()
	if !g.Solved() {
		t.Fatalf("the sketch didn't solve")
	}
	if dof := g.Dof(); dof != 0 {
		t.Errorf("the sketch has %d degrees of freedom, want 0", dof)
	}
	if p := sides[0].End().Pos(); !near(p.X, 40) || !near(p.Y, 0) {
		t.Errorf("the bottom side ends at %v, want (40, 0)", p)
	}
	if p := hole.Center().Pos(); !near(p.X, 15) || !near(p.Y, 12) {
		t.Errorf("the hole is at %v, want (15, 12)", p)
	}
	if r := hole.Radius(); !near(r, 4) {
		t.Errorf("the hole's radius is %g, want 4", r)
	}

	ex := d.Extrude(5, "plate")
	if ex.Name() != "plate" || ex.Kind() != "extrude" || d.Active().H != ex.H {
		t.Errorf("the extrusion is %v (%s), active %v", ex, ex.Kind(), d.Active())
	}
	want := (40*25 - math.Pi*16) * 5
	if v := ex.Volume(); math.Abs(v-want) > want*0.01 {
		t.Errorf("the plate's volume is %g, want about %g", v, want)
	}
	if lo, hi := ex.Bounds(); !near(hi.Z-lo.Z, 5) {
		t.Errorf("the plate is %g thick, want 5", hi.Z-lo.Z)
	}

	r := ex.Check()
	// Four sides, two caps, and the hole's wall, of four patches.
	if !r.Sound() || r.Faces != 7 || r.Patches != 10 || !near(r.Volume, ex.Volume()) || !near(r.Max.Z-r.Min.Z, 5) {
		t.Errorf("the plate does not check sound with 7 faces of 10 patches: %s", r.String())
	}
	if r := g.Check(); !r.Empty() {
		t.Errorf("the sketch before the extrusion has a solid: %s", r.String())
	}

	// A dimension changes what depends on it.
	width.Set(50)
	want = (50*25 - math.Pi*16) * 5
	if v := ex.Volume(); math.Abs(v-want) > want*0.01 {
		t.Errorf("the wider plate's volume is %g, want about %g", v, want)
	}

	// Undo takes back everything since the document was made, which
	// was one step.
	file := filepath.Join(t.TempDir(), "plate.spcs")
	if err := d.Save(file); err != nil {
		t.Fatal(err)
	}
	back, err := Open(file)
	if err != nil {
		t.Fatal(err)
	}
	bg := back.Group("plate")
	if !bg.Valid() {
		t.Fatalf("the saved sketch has no group 'plate': %v", back.Groups())
	}
	if v := bg.Volume(); math.Abs(v-want) > want*0.01 {
		t.Errorf("the reloaded plate's volume is %g, want about %g", v, want)
	}
	if got := back.Group("g003"); got.H != bg.H {
		t.Errorf("g003 is %v, want %v", got, bg)
	}
}

// What can't be done panics with an error that says why.
func TestMisuse(t *testing.T) {
	d := New()
	line := d.Line(V(0, 0, 0), V(10, 0, 0))
	for name, fn := range map[string]func(){
		"position of a line": func() { line.Pos() },
		"radius of a line":   func() { line.Radius() },
		"diameter of a line": func() { d.Diameter(line, 3) },
		"no workplane":       func() { d.Workplane("AB") },
		"deleted entity":     func() { line.Delete(); line.Length() },
		"check of a deleted group": func() {
			g := d.Sketch3D("gone")
			g.Delete()
			g.Check()
		},
	} {
		func() {
			defer func() {
				r := recover()
				if _, ok := r.(error); !ok {
					t.Errorf("%s: recovered %v, want an error", name, r)
				}
			}()
			fn()
		}()
	}
	if line.Valid() {
		t.Errorf("the deleted line is still valid")
	}
}
