package link

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/sketch"
	"github.com/spacesolve/spacesolve/pkg/srf"
)

var stepShapes = filepath.Join("..", "..", "..", "testdata", "step", "shapes")

func TestLinkStep(t *testing.T) {
	sk := &sketch.Sketch{}
	var el []core.Entity
	var m srf.SMesh
	var rep Report
	if !linkStep(filepath.Join(stepShapes, "cylinder.step"), sk, &el, &m, nil, &rep) {
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
	sk := &sketch.Sketch{}
	var el []core.Entity
	var m srf.SMesh
	var rep Report
	// Missing: not linked, so the user is asked to find it.
	if linkStep(filepath.Join(stepShapes, "missing.step"), sk, &el, &m, nil, &rep) {
		t.Error("a missing file linked")
	}
	// Not STEP: linked, empty, with the reason.
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.step")
	if err := os.WriteFile(bad, []byte("ISO-10303-21;\nHEADER;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep = Report{}
	if !linkStep(bad, sk, &el, &m, nil, &rep) {
		t.Fatal("a broken file counts as missing")
	}
	if !strings.HasPrefix(rep.Problem, "the file can't be read") || len(m.Triangles) != 0 {
		t.Errorf("problem %q, %d triangles", rep.Problem, len(m.Triangles))
	}
	// Nothing it can read: the unsupported entities are named.
	csg := filepath.Join("..", "..", "..", "testdata", "step", "corpus", "ap203e2_csg.stp")
	rep = Report{}
	if !linkStep(csg, sk, &el, &m, nil, &rep) || !strings.Contains(rep.Problem, "CSG_SOLID") {
		t.Errorf("problem %q", rep.Problem)
	}
}

func TestLinkStepCache(t *testing.T) {
	path := filepath.Join(stepShapes, "box.step")
	a, err := readStep(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := readStep(path, 0)
	if a != b {
		t.Error("the same file was read twice")
	}
	c, _ := readStep(path, 0.5)
	if c == a {
		t.Error("another tolerance gave the cached mesh")
	}
	// The cache keeps a few files only.
	for _, name := range []string{"sphere", "torus", "cone_apex", "cylinder", "box_hole"} {
		if _, err := readStep(filepath.Join(stepShapes, name+".step"), 0); err != nil {
			t.Fatal(err)
		}
	}
	stepCacheMu.Lock()
	n := len(stepCache)
	stepCacheMu.Unlock()
	if n > stepCacheSize {
		t.Errorf("%d files cached", n)
	}
}

func TestReloadAllStep(t *testing.T) {
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
	if !ReloadAll(sk, filepath.Join(dir, "model.spcs"), Options{}) {
		t.Fatal("cancelled")
	}
	g := &sk.Groups[0]
	if len(g.ImpMesh.Triangles) != 12 || !g.ForceToMesh || len(g.LinkInfo) != 3 || g.LinkProblem != "" {
		t.Errorf("%d triangles, forced %v, info %q, problem %q", len(g.ImpMesh.Triangles), g.ForceToMesh, g.LinkInfo, g.LinkProblem)
	}
	// A file another loader reads reports nothing.
	g.LinkFile = "missing.stl"
	ReloadAll(sk, filepath.Join(dir, "model.spcs"), Options{Quiet: true})
	if g.LinkInfo != nil || g.LinkProblem != "" {
		t.Errorf("info %q problem %q left from before", g.LinkInfo, g.LinkProblem)
	}
}

func TestPlural(t *testing.T) {
	if plural(1, "face") != "1 face" || plural(3, "face") != "3 faces" {
		t.Error(plural(1, "face"), plural(3, "face"))
	}
}
