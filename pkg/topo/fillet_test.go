package topo_test

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// filletLoss is the volume a fillet of radius r takes off a straight
// edge of length l whose faces meet at interior angle beta: the area
// between the corner and the arc, times l.
func filletLoss(r, beta, l float64) float64 {
	s := r / math.Tan(beta/2)
	return l * (r*s - r*r*(math.Pi-beta)/2)
}

func TestFilletBoxEdge(t *testing.T) {
	old := srf.ChordTolMm()
	srf.SetChordTolMm(0.001)
	defer srf.SetChordTolMm(old)
	sh := box()
	if err := topo.FilletEdge(sh, v(0, 0, 30), v(10, 0, 30), 2, 1<<30|0x77<<16); err != nil {
		t.Fatal(err)
	}
	tp := build(t, sh)
	if len(tp.Faces) != 7 || len(tp.Edges) != 15 || len(tp.Vertices) != 10 {
		t.Errorf("V %d E %d F %d", len(tp.Vertices), len(tp.Edges), len(tp.Faces))
	}
	var m srf.SMesh
	sh.TriangulateInto(&m)
	want := 6000 - filletLoss(2, math.Pi/2, 10)
	// The mesh's chords cut inside the arc by up to the tolerance.
	if got := m.CalculateVolume(); math.Abs(got-want) > 2*0.001*(math.Pi/2*2)*10 {
		t.Errorf("volume %.6f, want %.6f", got, want)
	}
	if e, me := tp.Euler(), topo.MeshEuler(&m); e != 2 || me != 2 {
		t.Errorf("Euler %d, the mesh's %d", e, me)
	}
}

func TestFilletRefusals(t *testing.T) {
	for _, c := range []struct {
		name string
		a, b geom.Vector
		r    float64
		want string
	}{
		{"no edge", v(1, 1, 1), v(2, 2, 2), 1, "no edge"},
		{"part of an edge", v(0, 0, 30), v(5, 0, 30), 1, "not a whole edge"},
		{"too wide", v(0, 0, 30), v(10, 0, 30), 25, "wider than a face"},
		{"no radius", v(0, 0, 30), v(10, 0, 30), 0, "more than zero"},
	} {
		sh := box()
		err := topo.FilletEdge(sh, c.a, c.b, c.r, 0)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v", c.name, err)
		}
	}
}

// cornerLoss is what fillets of radius r take off a box: each edge its
// cross section times its length, less at each corner where two meet
// the part both count, and at each where three meet, the corner cube
// they count three times over less the sphere's eighth.
func cornerLoss(r, length float64, mitres, spheres int) float64 {
	a := r * r * (1 - math.Pi/4)
	return a*length - float64(mitres)*r*r*r*(5.0/3-math.Pi/2) + float64(spheres)*r*r*r*(7*math.Pi/12-2)
}

// Edges of the 10 x 20 x 30 box meeting at corners: two top edges (a
// mitre), three edges at a corner (a sphere), the four top edges (four
// mitres), and all twelve (eight spheres).
//
// The chord tolerance is a five-thousandth of the radius. Finer than a
// thousandth, the strips between the sphere's grid and its trims were
// ear-clipped into slivers that crossed, until the ear clipper stopped
// taking ears that fold over (roadmap 13.1 and 13.2).
func TestFilletCorners(t *testing.T) {
	old := srf.ChordTolMm()
	defer srf.SetChordTolMm(old)
	top := [][2]geom.Vector{{v(0, 0, 30), v(10, 0, 30)}, {v(10, 0, 30), v(10, 20, 30)},
		{v(10, 20, 30), v(0, 20, 30)}, {v(0, 20, 30), v(0, 0, 30)}}
	var all [][2]geom.Vector
	for _, z := range []float64{0, 30} {
		all = append(all, [2]geom.Vector{v(0, 0, z), v(10, 0, z)}, [2]geom.Vector{v(10, 0, z), v(10, 20, z)},
			[2]geom.Vector{v(10, 20, z), v(0, 20, z)}, [2]geom.Vector{v(0, 20, z), v(0, 0, z)})
	}
	for _, x := range []float64{0, 10} {
		for _, y := range []float64{0, 20} {
			all = append(all, [2]geom.Vector{v(x, y, 0), v(x, y, 30)})
		}
	}
	for _, c := range []struct {
		name            string
		segs            [][2]geom.Vector
		length          float64
		mitres, spheres int
	}{
		{"mitre", top[:2], 30, 1, 0},
		{"sphere", [][2]geom.Vector{top[0], top[1], {v(10, 0, 0), v(10, 0, 30)}}, 60, 0, 1},
		{"top", top, 60, 4, 0},
		{"all", all, 240, 0, 8},
	} {
		for _, r := range []float64{1, 3} {
			tol := 0.0002 * r
			srf.SetChordTolMm(tol)
			sh := box()
			if err := topo.FilletEdges(sh, c.segs, r, 1<<30); err != nil {
				t.Errorf("%s at %g: %v", c.name, r, err)
				continue
			}
			tp, err := topo.Build(sh)
			if err != nil {
				t.Fatal(err)
			}
			var m srf.SMesh
			sh.TriangulateInto(&m)
			rep := check.Solid(sh, &m)
			want := 6000 - cornerLoss(r, c.length, c.mitres, c.spheres)
			// The chords cut inside the rounded faces, by less than the
			// tolerance over their area.
			area := math.Pi/2*r*c.length + float64(c.spheres)*math.Pi*r*r/2
			allow := tol * area
			if !rep.Sound() || len(tp.Validate()) > 0 || math.Abs(rep.Volume-want) > allow {
				t.Errorf("%s at %g: %s, want volume %g", c.name, r, rep.String(), want)
			}
			if e, me := tp.Euler(), topo.MeshEuler(&m); e != 2 || me != 2 {
				t.Errorf("%s at %g: Euler %d, the mesh's %d", c.name, r, e, me)
			}
		}
	}
}

func TestFilletEdges(t *testing.T) {
	sh := box()
	// Two parallel edges of the top, apart.
	if err := topo.FilletEdges(sh, [][2]geom.Vector{{v(0, 0, 30), v(10, 0, 30)}, {v(0, 20, 30), v(10, 20, 30)}}, 3, 1<<30); err != nil {
		t.Fatal(err)
	}
	tp := build(t, sh)
	if len(tp.Faces) != 8 {
		t.Errorf("%d faces", len(tp.Faces))
	}
	// The fillets' faces have names through their origins.
	named := 0
	for e := range tp.Edges {
		if _, err := tp.NameOf(e); err == nil {
			named++
		}
	}
	if named != len(tp.Edges) {
		t.Errorf("%d of %d edges named", named, len(tp.Edges))
	}
}

// TestFilletSweep fillets the edge of a triangular prism along a grid
// of angles, radii, lengths and leans of the prism, which makes the end
// faces oblique to the edge. Every point must give a closed, valid
// solid with the volume the formula gives; the rate allowed is 0, set
// before the fillet was built.
func TestFilletSweep(t *testing.T) {
	old := srf.ChordTolMm()
	srf.SetChordTolMm(0.001)
	defer srf.SetChordTolMm(old)
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Floats("angle", 20, 45, 60, 90, 120, 150, 165),
			sweep.Floats("k", 0.05, 0.4, 0.8),
			sweep.Floats("length", 0.5, 5, 40),
			sweep.Floats("lean", 0, 0.6),
		},
		MaxFailRate: 0,
		Sample:      24,
	}
	grid.Run(func(p sweep.Point) error {
		th := p.Float("angle") * math.Pi / 180
		length := p.Float("length")
		lean := v(p.Float("lean")*length, 0, length)
		// The directions into the two faces square to the edge, and the
		// largest radius whose fillet fits their 10 long ends.
		e := lean.WithMagnitude(1)
		d1, d2 := v(1, 0, 0), v(math.Cos(th), math.Sin(th), 0)
		u1 := d1.Minus(e.ScaledBy(d1.Dot(e))).WithMagnitude(1)
		u2 := d2.Minus(e.ScaledBy(d2.Dot(e))).WithMagnitude(1)
		half := math.Acos(math.Max(-1, math.Min(1, u1.Dot(u2)))) / 2
		r := p.Float("k") * 10 * math.Min(d1.Dot(u1), d2.Dot(u2)) * math.Tan(half)
		var sh srf.SShell
		sh.MakePrism([]geom.Vector{v(0, 0, 0), v(10, 0, 0), v(10*math.Cos(th), 10*math.Sin(th), 0)},
			v(0, 0, 0), lean, core.NewRgbaColor(100, 100, 100, 255))
		if err := topo.FilletEdge(&sh, v(0, 0, 0), lean, r, 1<<30); err != nil {
			return err
		}
		tp, err := topo.Build(&sh)
		if err != nil {
			return err
		}
		if probs := tp.Validate(); len(probs) > 0 {
			return fmt.Errorf("%s", probs[0].Msg)
		}
		var m srf.SMesh
		sh.TriangulateInto(&m)
		rep := check.Solid(&sh, &m)
		if !rep.Sound() {
			return fmt.Errorf("unsound: %s", rep.String())
		}
		// The faces' angle across the edge, which leans with the prism.
		n1 := lean.Cross(v(1, 0, 0)).WithMagnitude(1)
		n2 := v(math.Cos(th), math.Sin(th), 0).Cross(lean).WithMagnitude(1)
		beta := math.Pi - math.Acos(math.Max(-1, math.Min(1, n1.Dot(n2))))
		want := 50*math.Sin(th)*length - filletLoss(r, beta, lean.Magnitude())
		arcLen := r * (math.Pi - beta)
		if d := math.Abs(rep.Volume - want); d > 2*0.001*arcLen*lean.Magnitude()+1e-9*want {
			return fmt.Errorf("volume %g, want %g", rep.Volume, want)
		}
		return nil
	}).Require(t)
}

// TestFilletCornerSweep rounds the top edges of prisms of regular
// polygons (mitres at every corner), one corner's three edges (a sphere
// where the faces don't meet square), or every edge, over the polygon,
// the height and the radius, against the volumes OpenCASCADE gives in
// testdata/topo/fillet-corners.txt (scripts/fillet-fixtures.py); the
// rate allowed, 0, was set before.
func TestFilletCornerSweep(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "topo", "fillet-corners.txt"))
	if err != nil {
		t.Fatal(err)
	}
	type key struct {
		n     int
		h, k  float64
		edges string
	}
	want := map[key][2]float64{}
	for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
		var c key
		var r, vol float64
		if _, err := fmt.Sscan(line, &c.n, &c.h, &c.k, &c.edges, &r, &vol); err != nil {
			t.Fatalf("%q: %v", line, err)
		}
		want[c] = [2]float64{r, vol}
	}
	old := srf.ChordTolMm()
	defer srf.SetChordTolMm(old)
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Floats("sides", 3, 4, 5, 6, 8),
			sweep.Floats("height", 5, 20),
			sweep.Floats("k", 0.2, 0.5, 0.8),
			sweep.Strings("edges", "top", "corner", "all"),
		},
		MaxFailRate: 0,
		Sample:      20,
	}
	grid.Run(func(p sweep.Point) error {
		c := key{int(p.Float("sides")), p.Float("height"), p.Float("k"), p.String("edges")}
		w, ok := want[c]
		if !ok {
			return fmt.Errorf("no volume for %+v in the fixture", c)
		}
		r := w[0]
		pts := regular(c.n, 10)
		var sh srf.SShell
		sh.MakePrism(pts, v(0, 0, 0), v(0, 0, c.h), core.NewRgbaColor(100, 100, 100, 255))
		var segs [][2]geom.Vector
		first := v(10, 0, c.h)
		for i := range pts {
			a, b := pts[i].Plus(v(0, 0, c.h)), pts[(i+1)%c.n].Plus(v(0, 0, c.h))
			switch {
			case c.edges != "corner":
				segs = append(segs, [2]geom.Vector{a, b})
			case a.Equals(first) || b.Equals(first):
				segs = append(segs, [2]geom.Vector{a, b})
			}
			if c.edges == "all" {
				segs = append(segs, [2]geom.Vector{pts[i], pts[(i+1)%c.n]}, [2]geom.Vector{pts[i], a})
			}
		}
		if c.edges == "corner" {
			segs = append(segs, [2]geom.Vector{pts[0], first})
		}
		tol := 0.003 * r
		srf.SetChordTolMm(tol)
		if err := topo.FilletEdges(&sh, segs, r, 1<<30); err != nil {
			return err
		}
		tp, err := topo.Build(&sh)
		if err != nil {
			return err
		}
		var m srf.SMesh
		sh.TriangulateInto(&m)
		rep := check.Solid(&sh, &m)
		// The rounded faces' area is less than pi r times the edges'
		// length and the spheres', so the chords lose less than tol
		// times that.
		length := 0.0
		for _, sg := range segs {
			length += sg[1].Minus(sg[0]).Magnitude()
		}
		allow := tol * (math.Pi*r*length + 4*math.Pi*r*r*float64(c.n))
		switch {
		case !rep.Sound():
			return fmt.Errorf("unsound: %s", rep.String())
		case len(tp.Validate()) > 0:
			return fmt.Errorf("topology: %s", tp.Validate()[0].Msg)
		case math.Abs(rep.Volume-w[1]) > allow:
			return fmt.Errorf("volume %g, OpenCASCADE's %g, %g allowed", rep.Volume, w[1], allow)
		}
		return nil
	}).Require(t)
}

// TestFilletUnevenMitreSweep rounds two top edges at a corner, or every
// top edge, of leaning prisms of regular polygons, whose side faces
// don't lie alike about the top edges, so their corners are uneven
// mitres, over the polygon, the lean, its direction and the radius,
// against the volumes OpenCASCADE gives in
// testdata/topo/fillet-mitres.txt (scripts/fillet-mitre-fixtures.py);
// the rate allowed, 0, was set before. OpenCASCADE's own result is
// exact for 62 of the 96: in the rest a spline it fits where two
// cylinders meet strays from them, by up to 0.15 mm, or the shape is
// invalid, while the curve this makes lies on both (exported to STEP,
// these solids measure as OpenCASCADE's where its are exact, to 1e-4
// mm³). Those are checked for soundness only. The mesh is fine and its
// segments many, so the chords lose little.
func TestFilletUnevenMitreSweep(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "topo", "fillet-mitres.txt"))
	if err != nil {
		t.Fatal(err)
	}
	type key struct {
		n     int
		lean  float64
		dir   string
		k     float64
		edges string
	}
	want := map[key][3]float64{}
	for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
		var c key
		var r, vol, exact float64
		if _, err := fmt.Sscan(line, &c.n, &c.lean, &c.dir, &c.k, &c.edges, &r, &vol, &exact); err != nil {
			t.Fatalf("%q: %v", line, err)
		}
		want[c] = [3]float64{r, vol, exact}
	}
	old := srf.ChordTolMm()
	defer srf.SetChordTolMm(old)
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Floats("sides", 3, 4, 5, 6),
			sweep.Floats("lean", 2, 5),
			sweep.Strings("dir", "x", "xy"),
			sweep.Floats("k", 0.2, 0.5, 0.8),
			sweep.Strings("edges", "pair", "top"),
		},
		MaxFailRate: 0,
		Sample:      16,
	}
	grid.Run(func(p sweep.Point) error {
		c := key{int(p.Float("sides")), p.Float("lean"), p.String("dir"), p.Float("k"), p.String("edges")}
		w, ok := want[c]
		if !ok {
			return fmt.Errorf("no volume for %+v in the fixture", c)
		}
		r := w[0]
		pts := regular(c.n, 10)
		d := v(c.lean, 0, 10)
		if c.dir == "xy" {
			d.Y = c.lean / 2
		}
		var sh srf.SShell
		sh.MakePrism(pts, v(0, 0, 0), d, core.NewRgbaColor(100, 100, 100, 255))
		var segs [][2]geom.Vector
		first := v(10, 0, 0).Plus(d)
		for i := range pts {
			a, b := pts[i].Plus(d), pts[(i+1)%c.n].Plus(d)
			if c.edges == "top" || a.Equals(first) || b.Equals(first) {
				segs = append(segs, [2]geom.Vector{a, b})
			}
		}
		tol := 0.0003 * r
		srf.SetMaxSegments(400)
		defer srf.SetMaxSegments(20)
		srf.SetChordTolMm(tol)
		if err := topo.FilletEdges(&sh, segs, r, 1<<30); err != nil {
			return err
		}
		tp, err := topo.Build(&sh)
		if err != nil {
			return err
		}
		var m srf.SMesh
		sh.TriangulateInto(&m)
		rep := check.Solid(&sh, &m)
		length := 0.0
		for _, sg := range segs {
			length += sg[1].Minus(sg[0]).Magnitude()
		}
		allow := tol * math.Pi * r * (length + 4*r*float64(c.n))
		switch {
		case !rep.Sound():
			return fmt.Errorf("unsound: %s", rep.String())
		case len(tp.Validate()) > 0:
			return fmt.Errorf("topology: %s", tp.Validate()[0].Msg)
		case w[2] == 1 && math.Abs(rep.Volume-w[1]) > allow:
			return fmt.Errorf("volume %g, OpenCASCADE's %g, %g allowed", rep.Volume, w[1], allow)
		}
		return nil
	}).Require(t)
}
