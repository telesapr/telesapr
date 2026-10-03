package link

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// linkGroups is a sketch with n link groups of file.
func linkGroups(file string, n int) *sketch.Sketch {
	sk := &sketch.Sketch{}
	for i := range n {
		sk.Groups = append(sk.Groups, sketch.Group{H: core.HGroup{V: uint32(i + 2)}, Type: sketch.GroupLinked, LinkFile: file})
	}
	return sk
}

// A file linked by three groups is read once; reloaded, not again;
// changed, again. Each group has its own copy (roadmap 17.4).
func TestLinkCache(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "box.obj")
	if err := os.WriteFile(file, []byte(boxObj), 0o644); err != nil {
		t.Fatal(err)
	}
	read := filesRead.Load()
	sk := linkGroups(file, 3)
	if !ReloadAll(sk, "", Options{Quiet: true}) {
		t.Fatal("not loaded")
	}
	if n := filesRead.Load() - read; n != 1 {
		t.Errorf("%d reads for three links", n)
	}
	for i := range sk.Groups {
		if len(sk.Groups[i].ImpMesh.Triangles) != 12 {
			t.Fatalf("group %d has %d triangles", i, len(sk.Groups[i].ImpMesh.Triangles))
		}
	}
	// Their own copies: changing one changes no other.
	sk.Groups[0].ImpMesh.Triangles[0].A.X = 99
	sk.Groups[0].ImpEntity[0].ActPoint.X = 99
	if sk.Groups[1].ImpMesh.Triangles[0].A.X == 99 || sk.Groups[1].ImpEntity[0].ActPoint.X == 99 {
		t.Error("the groups share what they hold")
	}
	ReloadAll(sk, "", Options{Quiet: true})
	if n := filesRead.Load() - read; n != 1 {
		t.Errorf("%d reads after a reload", n)
	}
	// Changed: read again.
	later := time.Now().Add(time.Minute)
	if err := os.Chtimes(file, later, later); err != nil {
		t.Fatal(err)
	}
	ReloadAll(sk, "", Options{Quiet: true})
	if n := filesRead.Load() - read; n != 2 {
		t.Errorf("%d reads after the file changed", n)
	}
}

// A STEP file's tessellation is kept on disk: with the memory's
// forgotten, as in a new session, it comes from there, the same.
func TestStepDiskCache(t *testing.T) {
	t.Setenv("TELESAPR_CACHE_DIR", t.TempDir())
	path := filepath.Join(stepShapes, "cylinder.step")
	forget := func() {
		stepCacheMu.Lock()
		stepCache = map[stepKey]*stepEntry{}
		stepCacheMu.Unlock()
	}
	forget()
	writes, hits := stepDiskWrites.Load(), stepDiskHits.Load()
	first, err := readStep(path, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	if stepDiskWrites.Load() != writes+1 {
		t.Fatal("the tessellation wasn't kept")
	}
	forget()
	again, err := readStep(path, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	if stepDiskHits.Load() != hits+1 {
		t.Fatal("the kept tessellation wasn't used")
	}
	if !reflect.DeepEqual(first.Mesh.Triangles, again.Mesh.Triangles) || !reflect.DeepEqual(first.Report, again.Report) ||
		!reflect.DeepEqual(first.Vertices, again.Vertices) {
		t.Error("the kept tessellation differs")
	}
	// Another tolerance is another tessellation.
	forget()
	if _, err := readStep(path, 0.2); err != nil || stepDiskHits.Load() != hits+1 {
		t.Errorf("another tolerance hit the cache (%v)", err)
	}
}

// BenchmarkReloadAssembly reloads twenty links of a part of 50,000
// triangles, as undo does, with what was read kept ("warm") or
// forgotten each time, as before roadmap 17.4 ("cold").
func BenchmarkReloadAssembly(b *testing.B) {
	var m srf.SMesh
	for i := range 25000 {
		x := float64(i % 500)
		y := float64(i / 500)
		a, c, d, e := geom.Vector{X: x, Y: y}, geom.Vector{X: x + 1, Y: y}, geom.Vector{X: x + 1, Y: y + 1}, geom.Vector{X: x, Y: y + 1}
		m.AddTriangle(srf.STriangle{A: a, B: c, C: d})
		m.AddTriangle(srf.STriangle{A: a, B: d, C: e})
	}
	var bin bytes.Buffer
	if err := sketch.ExportMeshAsStlTo(&bin, &m, 1); err != nil {
		b.Fatal(err)
	}
	file := filepath.Join(b.TempDir(), "part.stl")
	if err := os.WriteFile(file, bin.Bytes(), 0o644); err != nil {
		b.Fatal(err)
	}
	for _, warm := range []bool{false, true} {
		name := map[bool]string{false: "cold", true: "warm"}[warm]
		b.Run(name, func(b *testing.B) {
			sk := linkGroups(file, 20)
			for b.Loop() {
				if !warm {
					loadMu.Lock()
					loadCache = map[loadKey]*loadEntry{}
					loadMu.Unlock()
				}
				if !ReloadAll(sk, "", Options{Quiet: true}) || len(sk.Groups[19].ImpMesh.Triangles) != 50000 {
					b.Fatal("not loaded")
				}
			}
		})
	}
}
