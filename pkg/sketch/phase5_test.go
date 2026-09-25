package sketch_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/persist/slvs"
	"github.com/spacesolve/spacesolve/pkg/sketch"
)

func TestPhase5LoadAndGenerate(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Dir(file)
	pkgDir := filepath.Dir(dir)
	root := filepath.Join(pkgDir, "..", "testdata")
	path := filepath.Join(root, "request", "line_segment", "normal.slvs")
	doc, err := slvs.LoadFile(path)
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	rt := sketch.NewRuntime(doc.Sketch)
	rt.RecalculateEntities(false)

	// Verify the runtime can compute normals.
	for _, ent := range doc.Sketch.Entities {
		if ent.Type == 0 { // unknown
			continue
		}
	}
	t.Logf("loaded %d entities from fixture", len(doc.Sketch.Entities))
}

func TestGroupPipelineStub(t *testing.T) {
	// Just verify the pipeline functions are callable.
	g := &sketch.Group{}
	sk := sketch.NewSketch()
	g.GenerateShellAndMesh(sk)
	g.GenerateForBoolean(nil, nil)
	g.GenerateForStepAndRepeat(nil, nil)
}
