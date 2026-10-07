package topo_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/persist/slvs"
	"github.com/telesapr/telesapr/pkg/persist/tlspr"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/topo"
)

var testdata = filepath.Join("..", "..", "testdata")

// fixtures lists the models under testdata with a solid to check: the
// .slvs fixtures, and the .tlspr goldens and solids that have no .slvs
// beside them.
func fixtures(t *testing.T) []string {
	var files []string
	err := filepath.WalkDir(testdata, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "v1" || d.Name() == "v2" || d.Name() == "ui" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(testdata, path)
		switch {
		case strings.HasSuffix(path, ".slvs"):
			files = append(files, filepath.ToSlash(rel))
		case strings.HasSuffix(path, ".tlspr") && !strings.HasPrefix(filepath.ToSlash(rel), "tlspr/golden"):
			if _, err := os.Stat(strings.TrimSuffix(path, ".tlspr") + ".slvs"); err != nil {
				files = append(files, filepath.ToSlash(rel))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(files)
	return files
}

// load reads and regenerates a fixture.
func load(t *testing.T, rel string) *sketch.Sketch {
	path := filepath.Join(testdata, filepath.FromSlash(rel))
	var sk *sketch.Sketch
	if strings.HasSuffix(rel, ".slvs") {
		doc, err := slvs.LoadFile(path)
		if err != nil {
			return nil
		}
		sk = doc.Sketch
	} else {
		doc, err := tlspr.LoadFile(path)
		if err != nil {
			return nil
		}
		sk = doc.Sketch
	}
	sk.GenerateAll()
	return sk
}

// solidGroups are the groups whose shell is the solid so far: those
// that made one and are not forced to a mesh.
func solidGroups(sk *sketch.Sketch) []*sketch.Group {
	var out []*sketch.Group
	for _, hg := range sk.GroupOrder {
		g := sk.Group(hg)
		if len(g.RunningShell.Surface) == 0 || g.IsForcedToMesh(sk) || len(g.ThisShell.Surface) == 0 {
			continue
		}
		out = append(out, g)
	}
	return out
}

// nonManifold are the fixtures whose solids have an edge where four
// faces meet, on purpose.
var nonManifold = map[string]bool{"group/boolean_knife_edge/normal.slvs": true}

// TestFixtures builds the topology of every solid of every fixture,
// validates it, and checks Euler's formula against the mesh made from
// the same shell: every solid but the knife edge's is sound.
func TestFixtures(t *testing.T) {
	var lines []string
	shells, sound, eulerOK := 0, 0, 0
	for _, f := range fixtures(t) {
		sk := load(t, f)
		if sk == nil {
			continue
		}
		for _, g := range solidGroups(sk) {
			shells++
			tp, err := topo.Build(&g.RunningShell)
			probs := tp.Validate()
			g.GenerateDisplayItems(sk)
			want := topo.MeshEuler(&g.DisplayMesh)
			got := tp.Euler()
			if err == nil && len(probs) == 0 {
				sound++
			}
			if got == want {
				eulerOK++
			}
			if (err != nil || len(probs) > 0 || got != want) && !nonManifold[f] {
				t.Errorf("%s g%x: err %v, %d problems, Euler %d, the mesh's %d", f, g.H.V, err, len(probs), got, want)
			}
			if nonManifold[f] && (len(probs) != 1 || !strings.Contains(probs[0].Msg, "4 co-edges")) {
				t.Errorf("%s g%x: %v", f, g.H.V, probs)
			}
			if err != nil || len(probs) > 0 || got != want {
				msg := ""
				if err != nil {
					msg = err.Error()
				} else if len(probs) > 0 {
					msg = fmt.Sprintf("%d problems, first %s", len(probs), probs[0].Msg)
				}
				lines = append(lines, fmt.Sprintf("%s g%x: V %d E %d F %d L %d euler %d mesh %d booleanFailed %v %s",
					f, g.H.V, len(tp.Vertices), len(tp.Edges), len(tp.Faces), len(tp.Loops), got, want, g.RunningShell.BooleanFailed, msg))
			}
		}
	}
	for _, l := range lines {
		t.Log(l)
	}
	t.Logf("%d shells, %d sound, %d with the mesh's Euler characteristic", shells, sound, eulerOK)
}

// BenchmarkBuildThread derives the topology of a threaded bolt, about
// 270 faces, as a chamfer by name does on each regeneration.
func BenchmarkBuildThread(b *testing.B) {
	t := &testing.T{}
	sk := load(t, "thread/bolt_m8x30.slvs")
	gs := solidGroups(sk)
	sh := &gs[len(gs)-1].RunningShell
	for b.Loop() {
		if _, err := topo.Build(sh); err != nil {
			b.Fatal(err)
		}
	}
}
