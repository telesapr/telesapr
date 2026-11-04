package link

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

var stepShapes = filepath.Join("..", "..", "..", "testdata", "step", "shapes")

func TestLinkStep(t *testing.T) {
	t.Parallel()
	sk := &sketch.Sketch{}
	var m srf.SMesh
	var rep Report
	el, ok := linkStep(filepath.Join(stepShapes, "cylinder.step"), sk, &m, nil, &rep)
	if !ok {
		t.Fatal("not linked")
	}
	if len(m.Triangles) == 0 || rep.Problem != "" {
		t.Errorf("%d triangles, problem %q", len(m.Triangles), rep.Problem)
	}
	// The origin, three normals, two vertices and two centres.
	points, normals := 0, 0
	for _, e := range el {
		switch e.Type {
		case core.EntityPointNCopy:
			points++
		case core.EntityNormalNCopy:
			normals++
		}
	}
	if points != 5 || normals != 3 {
		t.Errorf("%d points and %d normals", points, normals)
	}
	want := []string{"STEP AP214, in millimetres", "1 shape, 3 faces, ", "chord tolerance "}
	if len(rep.Info) != 3 {
		t.Fatalf("info %q", rep.Info)
	}
	for i, w := range want {
		if !strings.HasPrefix(rep.Info[i], w) {
			t.Errorf("info %q, want it to start %q", rep.Info[i], w)
		}
	}
}

func TestLinkStepProblems(t *testing.T) {
	t.Parallel()
	sk := &sketch.Sketch{}
	var m srf.SMesh
	var rep Report
	// Missing: not linked, so the user is asked to find it.
	if _, ok := linkStep(filepath.Join(stepShapes, "missing.step"), sk, &m, nil, &rep); ok {
		t.Error("a missing file linked")
	}
	// Not STEP: linked, empty, with the reason.
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.step")
	if err := os.WriteFile(bad, []byte("ISO-10303-21;\nHEADER;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep = Report{}
	if _, ok := linkStep(bad, sk, &m, nil, &rep); !ok {
		t.Fatal("a broken file counts as missing")
	}
	if !strings.HasPrefix(rep.Problem, "the file can't be read") || len(m.Triangles) != 0 {
		t.Errorf("problem %q, %d triangles", rep.Problem, len(m.Triangles))
	}
	// Nothing it can read: the unsupported entities are named.
	csg := filepath.Join("..", "..", "..", "testdata", "step", "corpus", "ap203e2_csg.stp")
	rep = Report{}
	if _, ok := linkStep(csg, sk, &m, nil, &rep); !ok || !strings.Contains(rep.Problem, "CSG_SOLID") {
		t.Errorf("problem %q", rep.Problem)
	}
}

func TestLinkStepCache(t *testing.T) {
	t.Parallel()
	// A memo of its own: the package's other tests read STEP files
	// through stepCache side by side, and queued behind the first read
	// they used to push box.step out of it before the second.
	memo := newStepMemo()
	path := filepath.Join(stepShapes, "box.step")
	a, err := memo.read(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := memo.read(path, 0)
	if a != b {
		t.Error("the same file was read twice")
	}
	c, _ := memo.read(path, 0.5)
	if c == a {
		t.Error("another tolerance gave the cached mesh")
	}
	// The cache keeps a few files only.
	for _, name := range []string{"sphere", "torus", "cone_apex", "cylinder", "box_hole"} {
		if _, err := memo.read(filepath.Join(stepShapes, name+".step"), 0); err != nil {
			t.Fatal(err)
		}
	}
	n := len(memo.entries)
	if n > stepCacheSize {
		t.Errorf("%d files cached", n)
	}
}

func TestReloadAllStep(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	data, err := os.ReadFile(filepath.Join(stepShapes, "box.step"))
	if err != nil {
		t.Fatal(err)
	}
	// Linked by a name relative to the sketch, upper case extension.
	if err := os.WriteFile(filepath.Join(dir, "part.STP"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	sk := &sketch.Sketch{}
	sk.Groups = append(sk.Groups, sketch.Group{Type: sketch.GroupLinked, LinkFile: "part.STP"})
	if !ReloadAll(sk, filepath.Join(dir, "model.tlspr"), Options{}) {
		t.Fatal("cancelled")
	}
	g := &sk.Groups[0]
	if len(g.ImpMesh.Triangles) != 12 || !g.ForceToMesh || len(g.LinkInfo) != 3 || g.LinkProblem != "" {
		t.Errorf("%d triangles, forced %v, info %q, problem %q", len(g.ImpMesh.Triangles), g.ForceToMesh, g.LinkInfo, g.LinkProblem)
	}
	// A file another loader reads reports nothing.
	g.LinkFile = "missing.stl"
	ReloadAll(sk, filepath.Join(dir, "model.tlspr"), Options{})
	if g.LinkInfo != nil || g.LinkProblem != "" {
		t.Errorf("info %q problem %q left from before", g.LinkInfo, g.LinkProblem)
	}
}

func TestPlural(t *testing.T) {
	t.Parallel()
	if plural(1, "face") != "1 face" || plural(3, "face") != "3 faces" {
		t.Error(plural(1, "face"), plural(3, "face"))
	}
}

// A STEP file linked as a solid by two groups is read once: the second
// group's shell, the cache's copy, is followed at the tolerance the
// file was read at, as the first's is. It was left unset, so a part of
// the assembly made from it (Sketch.AssemblyParts) had its surfaces at
// DefaultTolerance.
func TestLinkStepSolidCacheTolerance(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	data, err := os.ReadFile(filepath.Join(stepShapes, "cylinder.step"))
	if err != nil {
		t.Fatal(err)
	}
	// A file of its own, so no other test's read is in the cache.
	if err := os.WriteFile(filepath.Join(dir, "part.step"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	sk := &sketch.Sketch{ChordTolPct: 0.05}
	for i := range 2 {
		sk.Groups = append(sk.Groups, sketch.Group{H: core.HGroup{V: uint32(i + 2)}, Type: sketch.GroupLinked,
			LinkFile: "part.step", LinkAsSolid: true})
	}
	if !ReloadAll(sk, filepath.Join(dir, "model.tlspr"), Options{}) {
		t.Fatal("cancelled")
	}
	a, b := &sk.Groups[0].ImpShell, &sk.Groups[1].ImpShell
	if len(a.Surface) == 0 || len(b.Surface) == 0 {
		t.Fatalf("read as a solid: %d and %d surfaces (%q)", len(a.Surface), len(b.Surface), sk.Groups[0].LinkProblem)
	}
	if a.Tol != b.Tol || a.Tol.ChordTolMm() == srf.DefaultTolerance.ChordTolMm() {
		t.Errorf("first read at %g mm, the cache's copy at %g mm (unset %v)",
			a.Tol.ChordTolMm(), b.Tol.ChordTolMm(), b.Tol == srf.Tolerance{})
	}
}
