package step

import (
	"bufio"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

var exchangeDir = filepath.Join("..", "..", "..", "testdata", "step", "exchange")

// exchangeShape is a line of testdata/step/exchange/reference.txt: a
// shape OpenCASCADE built, as it measures it.
type exchangeShape struct {
	name         string
	volume, area float64
	centre       geom.Vector
}

func readExchange(t *testing.T) []exchangeShape {
	t.Helper()
	f, err := os.Open(filepath.Join(exchangeDir, "reference.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	var out []exchangeShape
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}
		fs := strings.Split(line, "\t")
		if len(fs) != 6 {
			t.Fatalf("bad line %q", line)
		}
		var v [5]float64
		for i := range v {
			if v[i], err = strconv.ParseFloat(fs[i+1], 64); err != nil {
				t.Fatalf("bad number in %q: %v", line, err)
			}
		}
		out = append(out, exchangeShape{fs[0], v[0], v[1], geom.Vector{X: v[2], Y: v[3], Z: v[4]}})
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Fatal("no shapes")
	}
	return out
}

// TestExchangeFixtures reads the STEP files OpenCASCADE wrote of shapes
// it built (scripts/step-exchange/occ-fixtures.tcl), each as AP242 and
// two also as AP203 and AP214, as a mesh and as a solid: every file is
// read whole, closed and not crossing itself, its volume within the
// chord tolerance of OpenCASCADE's and its centre within the tolerance.
// The ball fused at the holed block's corner, whose pole lies on the
// block's edge, came in as a mesh open and 9% short before 2026-11-23.
func TestExchangeFixtures(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob(filepath.Join(exchangeDir, "*.step"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no fixtures: %v", err)
	}
	ref := map[string]exchangeShape{}
	for _, e := range readExchange(t) {
		ref[e.name] = e
	}
	for _, path := range files {
		file := strings.TrimSuffix(filepath.Base(path), ".step")
		name, schema, ok := strings.Cut(file, "_AP")
		e, known := ref[name]
		if !ok || !known {
			t.Fatalf("%s: not in reference.txt", file)
		}
		t.Run(file, func(t *testing.T) {
			t.Parallel()
			res, err := ReadFile(path, Options{ChordTolPct: 0.1})
			if err != nil {
				t.Fatal(err)
			}
			if got, want := Protocol(res.Report.Schema), "AP"+schema; got != want {
				t.Errorf("protocol %q, want %s", got, want)
			}
			if p := res.Report.Problem(); p != "" {
				t.Errorf("mesh: %s", p)
			}
			tol := res.Report.ChordTol
			r := check.Solid(nil, &res.Mesh)
			checkExchanged(t, "mesh", e, r, tol)

			const solidTol = 0.005
			m := readModel(t, path)
			sh, err := m.ShellOf(0, srf.OriginFor(9, 0), fineMesh(solidTol))
			if err != nil {
				t.Fatalf("solid: %v", err)
			}
			checkExchanged(t, "solid", e, solidReport(sh), solidTol)
		})
	}
}

// checkExchanged checks a read shape against OpenCASCADE's numbers: the
// volume within the area times the chord tolerance, the centre within
// the tolerance.
func checkExchanged(t *testing.T, how string, e exchangeShape, r check.Report, tol float64) {
	t.Helper()
	if r.NakedEdges != 0 || r.SelfIntersecting != 0 {
		t.Errorf("%s: %d naked and %d self-intersecting edges", how, r.NakedEdges, r.SelfIntersecting)
	}
	if allow := e.area*tol + 1e-9*e.volume; math.Abs(r.Volume-e.volume) > allow {
		t.Errorf("%s: volume %.6f, want %.6f within %.3g", how, r.Volume, e.volume, allow)
	}
	if d := r.CenterOfMass.Minus(e.centre).Magnitude(); d > tol {
		t.Errorf("%s: centre %v, %.3g from %v", how, r.CenterOfMass, d, e.centre)
	}
}
