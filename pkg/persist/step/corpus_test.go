package step

import (
	"bufio"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
)

// corpusEntry is a line of testdata/step/corpus/reference.txt.
type corpusEntry struct {
	name         string
	volume, area float64
	expect       string
}

func readReference(t *testing.T, path string) []corpusEntry {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	var out []corpusEntry
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) != 4 {
			t.Fatalf("%s: bad line %q", path, line)
		}
		v, err1 := strconv.ParseFloat(parts[1], 64)
		a, err2 := strconv.ParseFloat(parts[2], 64)
		if err1 != nil || err2 != nil {
			t.Fatalf("%s: bad numbers in %q", path, line)
		}
		out = append(out, corpusEntry{parts[0], v, a, parts[3]})
	}
	return out
}

// checkAgainst reads a file and holds it to its reference: a closed
// mesh whose volume is within the chord tolerance of the reference
// (the area times the tolerance, and a little for rounding), or the
// failure it is listed with.
func checkAgainst(t *testing.T, path string, e corpusEntry) {
	t.Helper()
	res, err := ReadFile(path, Options{})
	if err != nil {
		t.Fatalf("%s: %v", e.name, err)
	}
	r := check.Solid(nil, &res.Mesh)
	switch e.expect {
	case "ok", "open":
		if p := res.Report.Problem(); p != "" {
			t.Errorf("%s: %s", e.name, p)
		}
		if e.expect == "ok" && r.NakedEdges != 0 {
			t.Errorf("%s: %d naked edges", e.name, r.NakedEdges)
		}
		allow := e.area*res.Report.ChordTol + 1e-6*e.volume
		if e.expect == "open" {
			allow *= 4
		}
		if d := math.Abs(r.Volume - e.volume); d > allow {
			t.Errorf("%s: volume %.6g, want %.6g within %.3g", e.name, r.Volume, e.volume, allow)
		}
	default:
		if p := res.Report.Problem(); !strings.HasPrefix(p, e.expect) {
			t.Errorf("%s: problem %q, want it to start %q", e.name, p, e.expect)
		}
	}
}

// TestCorpus reads the files of testdata/step/corpus.
func TestCorpus(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "testdata", "step", "corpus")
	for _, e := range readReference(t, filepath.Join(dir, "reference.txt")) {
		t.Run(e.name, func(t *testing.T) { checkAgainst(t, filepath.Join(dir, e.name), e) })
	}
}

// TestExternalCorpus reads the files external.txt lists that it finds
// under $TELESAPR_STEP_CORPUS.
func TestExternalCorpus(t *testing.T) {
	root := os.Getenv("TELESAPR_STEP_CORPUS")
	if root == "" {
		t.Skip("TELESAPR_STEP_CORPUS is not set")
	}
	found := map[string]string{}
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if _, ok := found[d.Name()]; !ok {
				found[d.Name()] = p
			}
		}
		return nil
	})
	dir := filepath.Join("..", "..", "..", "testdata", "step", "corpus")
	for _, e := range readReference(t, filepath.Join(dir, "external.txt")) {
		path, ok := found[e.name]
		if !ok {
			t.Logf("%s: not under %s", e.name, root)
			continue
		}
		t.Run(e.name, func(t *testing.T) { checkAgainst(t, path, e) })
	}
}

func BenchmarkReadNIST5(b *testing.B) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "step", "corpus", "NIST_MBE_PMI_5.stp"))
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Read(data, Options{}); err != nil {
			b.Fatal(err)
		}
	}
}

// TestKnifeEdgeReadBack reads the export of the boolean_knife_edge
// fixture, whose faces meet along an edge four times. One face's loop
// has a vertex inside an edge its neighbour runs past, a T-junction the
// Boolean left; the mesh is mended there and has no naked edge
// (roadmap 13.5). The knife edge itself, shared by four faces, is what
// the check still finds crossing.
func TestKnifeEdgeReadBack(t *testing.T) {
	path := filepath.Join("..", "..", "..", "testdata", "export", "group", "boolean_knife_edge", "normal", "surfaces.step")
	res, err := ReadFile(path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	r := check.Solid(nil, &res.Mesh)
	if r.NakedEdges != 0 || math.Abs(r.Volume-1.83039) > 1e-4 {
		t.Errorf("%d naked edges, volume %g", r.NakedEdges, r.Volume)
	}
}
