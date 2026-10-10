package check_test

import (
	"errors"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// oracleOf builds the oracle of the document's active group.
func oracleOf(t *testing.T, d *api.Doc) *check.Oracle {
	t.Helper()
	sk := d.Host().Sketch()
	sk.GenerateAll()
	o, err := check.NewOracle(sk, sk.ActiveGroup())
	if err != nil || o == nil {
		t.Fatalf("oracle: %v, %v", o, err)
	}
	return o
}

// meshOf is the active group's solid's mesh.
func meshOf(d *api.Doc) *srf.SMesh {
	sk := d.Host().Sketch()
	g := sk.Group(sk.ActiveGroup())
	g.GenerateDisplayItems(sk)
	m := g.DisplayMesh
	return &m
}

// near reports whether a is within rel of b.
func near(a, b, rel float64) bool { return math.Abs(a-b) <= rel*math.Abs(b) }

// A box's volume and centroid come out exact on the coarsest grid,
// and a probe through it enters and leaves at its faces.
func TestOracleBox(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Rect(api.V(0, 0, 0), api.V(30, 20, 0))
	d.Extrude(10, "")
	o := oracleOf(t, d)
	m := o.Measure(1e-9, 64)
	if !near(m.Volume, 6000, 1e-9) || m.CenterOfMass.Minus(api.V(15, 10, 5)).Magnitude() > 1e-9 {
		t.Errorf("volume %g, centroid %v", m.Volume, m.CenterOfMass)
	}
	iv := o.Probe(api.V(-5, 7, 3), api.V(1, 0, 0))
	if len(iv) != 1 || math.Abs(iv[0].From-5) > 1e-9 || math.Abs(iv[0].To-35) > 1e-9 {
		t.Errorf("probe %v", iv)
	}
	if !o.Inside(api.V(1, 1, 1)) || o.Inside(api.V(31, 1, 1)) || o.Inside(api.V(1, 1, 11)) {
		t.Error("inside")
	}
	if iv := o.Probe(api.V(100, 100, 100), api.V(1, 0, 0)); len(iv) != 0 {
		t.Errorf("a probe that misses: %v", iv)
	}
}

// A plate with a hole cut, a boss joined and a slab kept where it
// meets: the oracle's volume is the closed form, and the B-rep's mesh
// agrees with it ray by ray within its chord tolerance.
func TestOracleBooleans(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Rect(api.V(0, 0, 0), api.V(40, 40, 0))
	d.Extrude(10, "plate")
	d.SketchIn(d.Workplane("XY"), "")
	d.Circle(api.V(20, 20, 0), 5)
	hole := d.Extrude(10, "hole")
	hole.SetCombine(api.Difference)
	d.SketchIn(d.Workplane("XY"), "")
	d.Rect(api.V(30, 30, 0), api.V(50, 50, 0))
	d.Extrude(20, "boss")
	want := 40*40*10 - math.Pi*25*10 + 20*20*20 - 10*10*10
	o := oracleOf(t, d)
	m := o.Measure(1e-4, 256)
	if !near(m.Volume, want, 2e-3) {
		t.Errorf("volume %g, want %g (spread %g at %d rays)", m.Volume, want, m.Spread(), m.Rays)
	}
	// The mesh's circle is chords: a ray through the hole misses each
	// side's arc by up to the chord tolerance.
	if diffs := o.Diff(meshOf(d), 24, 0.1); len(diffs) != 0 {
		t.Errorf("%d rays disagree with the mesh, as %+v", len(diffs), diffs[0])
	}
	// The mesh made wrong: a face's triangles dropped, and the rays
	// through it disagree.
	bad := meshOf(d)
	bad.Triangles = bad.Triangles[2:]
	if diffs := o.Diff(bad, 24, 0.1); len(diffs) == 0 {
		t.Error("a mesh with a hole in it agrees")
	}
}

// Kept where they meet: an intersection of two boxes, the second 5
// tall.
func TestOracleIntersection(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Rect(api.V(0, 0, 0), api.V(20, 20, 0))
	d.Extrude(20, "")
	d.SketchIn(d.Workplane("XY"), "")
	d.Rect(api.V(10, 10, 0), api.V(30, 30, 0))
	g := d.Extrude(5, "")
	g.SetCombine(api.Intersection)
	if m := oracleOf(t, d).Measure(1e-9, 64); !near(m.Volume, 10*10*5, 1e-9) {
		t.Errorf("volume %g", m.Volume)
	}
}

// A lathed cylinder and cone, and a quarter turn revolved: the volumes
// are the closed forms, and the quarter's centroid lies where the
// kernel's does, on the side the turn goes.
func TestOracleRevolutions(t *testing.T) {
	t.Parallel()
	lathe := func(angle float64) (*api.Doc, api.Group) {
		d := api.New()
		d.SketchIn(d.Workplane("ZX"), "")
		axis := d.Line(api.V(0, 0, -1), api.V(0, 0, 21))
		axis.SetConstruction(true)
		d.Polyline(true, api.V(0, 0, 0), api.V(8, 0, 0), api.V(4, 0, 20), api.V(0, 0, 20))
		if angle == 0 {
			return d, d.Lathe(axis, "")
		}
		return d, d.Revolve(axis, angle, "")
	}
	cone := math.Pi * 20 / 3 * (64 + 32 + 16)
	d, _ := lathe(0)
	if m := oracleOf(t, d).Measure(5e-4, 256); !near(m.Volume, cone, 3e-3) {
		t.Errorf("lathe: volume %g, want %g (spread %g)", m.Volume, cone, m.Spread())
	}
	d, g := lathe(90)
	m := oracleOf(t, d).Measure(5e-4, 256)
	if !near(m.Volume, cone/4, 3e-3) {
		t.Errorf("revolve: volume %g, want %g", m.Volume, cone/4)
	}
	if c := g.Check().CenterOfMass; m.CenterOfMass.Minus(c).Magnitude() > 0.05 {
		t.Errorf("revolve: centroid %v, the kernel's %v", m.CenterOfMass, c)
	}
	if diffs := oracleOf(t, d).Diff(meshOf(d), 16, 0.1); len(diffs) != 0 {
		t.Errorf("revolve: %d rays disagree with the mesh, as %+v", len(diffs), diffs[0])
	}
}

// Two loops in one sketch extrude to two prisms, taken together; a
// group the oracle can't build is named.
func TestOracleLoopsAndUnsupported(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Rect(api.V(0, 0, 0), api.V(10, 10, 0))
	d.Rect(api.V(20, 0, 0), api.V(30, 10, 0))
	ex := d.Extrude(10, "")
	// The rectangles' sides fall inside the grid's cells, which it
	// samples at their middles: the error is a cell's width along the
	// outline, which three grids' agreement within 0.1% leaves at 0.3%.
	if m := oracleOf(t, d).Measure(1e-3, 256); !near(m.Volume, 2000, 5e-3) {
		t.Errorf("two loops: volume %g", m.Volume)
	}
	d.ChamferSolid(ex.Solid().Edges()[:1], 1, "")
	sk := d.Host().Sketch()
	sk.GenerateAll()
	if _, err := check.NewOracle(sk, sk.ActiveGroup()); !errors.Is(err, check.ErrUnsupported) {
		t.Errorf("a chamfer: %v", err)
	}
	if o, err := check.NewOracle(sk, core.HGroup{V: sketch.HGroupReferences}); o != nil || err != nil {
		t.Errorf("no solid: %v, %v", o, err)
	}
	if got := check.MeshProbe(&srf.SMesh{}, geom.Vector{}, geom.Vector{X: 1}); len(got) != 0 {
		t.Errorf("an empty mesh: %v", got)
	}
}

// A box's mesh is a closed manifold; dropping a triangle opens it,
// doubling one makes edges used thrice, turning one round misorients
// its edges, and a triangle with two corners at one place is
// degenerate.
func TestMeshManifold(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Rect(api.V(0, 0, 0), api.V(10, 10, 0))
	d.Extrude(10, "")
	oracleOf(t, d)
	good := meshOf(d)
	if m := check.MeshManifold(good); !m.Closed() {
		t.Errorf("a box: %+v", m)
	}
	open := *good
	open.Triangles = good.Triangles[1:]
	if m := check.MeshManifold(&open); m.Unmatched != 3 {
		t.Errorf("a triangle dropped: %+v", m)
	}
	excess := *good
	excess.Triangles = append(append([]srf.STriangle(nil), good.Triangles...), good.Triangles[0])
	if m := check.MeshManifold(&excess); m.Excess != 3 {
		t.Errorf("a triangle doubled: %+v", m)
	}
	flipped := *good
	flipped.Triangles = append([]srf.STriangle(nil), good.Triangles...)
	tr := &flipped.Triangles[0]
	tr.A, tr.B = tr.B, tr.A
	if m := check.MeshManifold(&flipped); m.Misoriented != 3 {
		t.Errorf("a triangle turned round: %+v", m)
	}
	degen := *good
	degen.Triangles = append(append([]srf.STriangle(nil), good.Triangles...), srf.STriangle{A: geom.Vector{}, B: geom.Vector{}, C: geom.Vector{X: 1}})
	if m := check.MeshManifold(&degen); m.Degenerate != 1 {
		t.Errorf("a degenerate triangle: %+v", m)
	}
}

// The ray-cast check of a sound solid agrees with its mesh ray by ray;
// one the oracle can't build is refused, naming the group.
func TestRayCheck(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Rect(api.V(0, 0, 0), api.V(40, 40, 0))
	d.Extrude(10, "")
	d.SketchIn(d.Workplane("XY"), "")
	d.Circle(api.V(20, 20, 0), 5)
	d.Extrude(10, "").SetCombine(api.Difference)
	sk := d.Host().Sketch()
	sk.GenerateAll()
	r, err := check.RayCheck(sk, sk.Group(sk.ActiveGroup()), 16)
	if err != nil || len(r.Disagree) != 0 || !r.Manifold.Closed() || !near(r.Measure.Volume, r.MeshVolume, 5e-3) || r.Rays != 768 {
		t.Errorf("%v: %+v", err, r)
	}
}
