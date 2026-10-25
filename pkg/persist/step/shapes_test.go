package step

import (
	"bufio"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/srf"
)

var shapesDir = filepath.Join("..", "..", "..", "testdata", "step", "shapes")

// shapeExpect is a line of testdata/step/shapes/expected.txt.
type shapeExpect struct {
	name         string
	volume, area float64
}

func readShapes(t *testing.T) []shapeExpect {
	t.Helper()
	return readExpected(t, shapesDir)
}

// readExpected reads a directory's expected.txt.
func readExpected(t *testing.T, dir string) []shapeExpect {
	t.Helper()
	f, err := os.Open(filepath.Join(dir, "expected.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	var out []shapeExpect
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fs := strings.Fields(sc.Text())
		if len(fs) < 3 || strings.HasPrefix(fs[0], "#") {
			continue
		}
		v, _ := strconv.ParseFloat(fs[1], 64)
		a, _ := strconv.ParseFloat(fs[2], 64)
		out = append(out, shapeExpect{fs[0], v, a})
	}
	if len(out) == 0 {
		t.Fatal("no shapes")
	}
	return out
}

func readShape(t *testing.T, name string, o Options) *Result {
	t.Helper()
	res, err := ReadFile(filepath.Join(shapesDir, name+".step"), o)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// TestShapes reads each shape of the analytic set at three tolerances:
// every face is read, the mesh is closed and doesn't cross itself, and
// its volume is within the chord tolerance of the exact one.
func TestShapes(t *testing.T) {
	t.Parallel()
	for _, e := range readShapes(t) {
		for _, pct := range []float64{0.5, 0.1, 0.02} {
			res := readShape(t, e.name, Options{ChordTolPct: pct})
			if p := res.Report.Problem(); p != "" {
				t.Errorf("%s at %g%%: %s", e.name, pct, p)
			}
			r := check.Solid(nil, &res.Mesh)
			if r.NakedEdges != 0 || r.SelfIntersecting != 0 {
				t.Errorf("%s at %g%%: %d naked and %d self-intersecting edges", e.name, pct, r.NakedEdges, r.SelfIntersecting)
			}
			allow := e.area*res.Report.ChordTol + 1e-9*e.volume
			if d := math.Abs(r.Volume - e.volume); d > allow {
				t.Errorf("%s at %g%%: volume %.9g, want %.9g within %.3g", e.name, pct, r.Volume, e.volume, allow)
			}
		}
	}
}

// TestShapesConverge: halving the tolerance brings the volume nearer.
func TestShapesConverge(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"sphere", "torus", "cone_apex", "vase_revolved"} {
		exact := 0.0
		for _, e := range readShapes(t) {
			if e.name == name {
				exact = e.volume
			}
		}
		prev := math.Inf(1)
		for _, tol := range []float64{0.2, 0.05, 0.0125} {
			res := readShape(t, name, Options{ChordTol: tol})
			err := math.Abs(check.Solid(nil, &res.Mesh).Volume - exact)
			if err >= prev {
				t.Errorf("%s: error %g at %g mm, %g at the coarser tolerance", name, err, tol, prev)
			}
			prev = err
		}
	}
}

// TestShapesDeterministic: the mesh is the same on one goroutine as
// on several.
func TestShapesDeterministic(t *testing.T) {
	read := func() []srf.STriangle {
		var all []srf.STriangle
		for _, e := range readShapes(t) {
			all = append(all, readShape(t, e.name, Options{}).Mesh.Triangles...)
		}
		return all
	}
	old := runtime.GOMAXPROCS(1)
	one := read()
	runtime.GOMAXPROCS(8)
	eight := read()
	runtime.GOMAXPROCS(old)
	if len(one) != len(eight) {
		t.Fatalf("%d triangles on one goroutine, %d on eight", len(one), len(eight))
	}
	for i := range one {
		if one[i] != eight[i] {
			t.Fatalf("triangle %d differs", i)
		}
	}
}

// TestShapeVertices: the box's eight corners and the cylinder's circle
// centres come back as points.
func TestShapeVertices(t *testing.T) {
	t.Parallel()
	res := readShape(t, "box", Options{})
	if len(res.Vertices) != 8 || len(res.Centers) != 0 {
		t.Errorf("box: %d vertices, %d centres", len(res.Vertices), len(res.Centers))
	}
	res = readShape(t, "cylinder", Options{})
	if len(res.Centers) != 2 {
		t.Errorf("cylinder: %d centres, want 2", len(res.Centers))
	}
	for _, c := range res.Centers {
		if c.X != 0 || c.Y != 0 || (c.Z != 0 && c.Z != 12) {
			t.Errorf("cylinder centre %v", c)
		}
	}
	if res.Report.Schema != "AUTOMOTIVE_DESIGN" || Protocol(res.Report.Schema) != "AP214" {
		t.Errorf("schema %q", res.Report.Schema)
	}
	if res.Report.Unit != "millimetres" || res.Report.Shapes != 1 || res.Report.Placements != 1 {
		t.Errorf("report %+v", res.Report)
	}
}
