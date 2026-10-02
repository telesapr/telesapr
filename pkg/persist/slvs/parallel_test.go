package slvs

import (
	"bytes"
	"fmt"
	"github.com/spacesolve/spacesolve/pkg/internal/slvswrite"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/internal/par"
	"github.com/spacesolve/spacesolve/pkg/sketch"
	"github.com/spacesolve/spacesolve/pkg/srf"
)

// generatedDump loads a fixture, generates it, and returns all that
// depends on the generation: the save, the STL export, and every
// group's running shell and mesh to the bit. copies gives the number
// of copies that step and repeat groups are to make instead of the
// fixture's.
func generatedDump(t *testing.T, path, stl string, copies map[uint32]float64) []byte {
	srf.SetChordTolMm(0)
	doc, err := LoadFile(path)
	if err != nil {
		return nil
	}
	sk := doc.Sketch
	for h, n := range copies {
		sk.GetGroup(core.HGroup{V: h}).ValA = n
	}
	sk.GenerateAllTyped(sketch.GenerateAll)

	var b bytes.Buffer
	vec := func(v geom.Vector) {
		fmt.Fprintf(&b, "%x %x %x ", math.Float64bits(v.X), math.Float64bits(v.Y), math.Float64bits(v.Z))
	}
	for _, hg := range sk.GroupOrder {
		g := sk.GetGroup(hg)
		fmt.Fprintf(&b, "group %x failed=%v\n", hg.V, g.BooleanFailed)
		for i := range g.RunningShell.Surface {
			s := &g.RunningShell.Surface[i]
			fmt.Fprintf(&b, "surface %x face=%x", s.H.V, s.Face)
			for _, stb := range s.Trim {
				fmt.Fprintf(&b, " trim %x %v ", stb.Curve.V, stb.Backwards)
				vec(stb.StartP)
				vec(stb.FinishP)
			}
			b.WriteByte('\n')
		}
		for i := range g.RunningShell.Curve {
			c := &g.RunningShell.Curve[i]
			fmt.Fprintf(&b, "curve %x %x %x %v ", c.H.V, c.SurfA.V, c.SurfB.V, c.IsExact)
			for _, p := range c.Points {
				vec(p.P)
			}
			b.WriteByte('\n')
		}
		for _, m := range []*srf.SMesh{&g.RunningMesh, &g.DisplayMesh} {
			fmt.Fprintf(&b, "mesh %d\n", len(m.Triangles))
			for i := range m.Triangles {
				tr := &m.Triangles[i]
				vec(tr.A)
				vec(tr.B)
				vec(tr.C)
				vec(tr.NA)
				vec(tr.NB)
				vec(tr.NC)
				fmt.Fprintf(&b, "%x\n", tr.Meta.Face)
			}
		}
	}
	b.WriteString("save\n")
	b.Write(slvswrite.Save(doc.Sketch))

	es := sketch.ExportSettings{ChordTol: 0.1, MaxSegments: 64, Scale: 1}
	if err := sk.ExportMeshTo(stl, es); err != nil {
		fmt.Fprintf(&b, "stl: %v\n", err)
	} else {
		data, err := os.ReadFile(stl)
		if err != nil {
			t.Fatal(err)
		}
		b.WriteString("stl\n")
		b.Write(data)
	}
	return b.Bytes()
}

// generatedWith is generatedDump in the given mode, on the given number
// of goroutines.
func generatedWith(t *testing.T, mode srf.ParallelMode, workers int, path, stl string) []byte {
	return generatedCopiesWith(t, mode, workers, path, stl, nil)
}

func generatedCopiesWith(t *testing.T, mode srf.ParallelMode, workers int, path, stl string, copies map[uint32]float64) []byte {
	srf.SetParallelMode(mode)
	if mode != srf.ParallelOff {
		par.SetWorkers(workers)
	}
	defer srf.SetParallelMode(srf.ParallelExact)
	return generatedDump(t, path, stl, copies)
}

// eachFixture calls fn for every sketch under testdata; under the race
// detector, for a sample of them (raceSample).
func eachFixture(t *testing.T, fn func(path string)) {
	var paths []string
	err := filepath.WalkDir(fixture(""), func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && filepath.Ext(path) == ".slvs" {
			paths = append(paths, path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range raceSample(paths) {
		fn(path)
	}
}

// TestParallelSameAsSerial generates every fixture on one goroutine and
// on several, and requires the same shells, meshes, save and STL file
// to the bit.
func TestParallelSameAsSerial(t *testing.T) {
	stl := filepath.Join(t.TempDir(), "out.stl")
	count := 0
	eachFixture(t, func(path string) {
		serial := generatedWith(t, srf.ParallelOff, 1, path, stl)
		if serial == nil {
			return // not a valid file
		}
		count++
		for _, workers := range []int{1, 8} {
			if !bytes.Equal(serial, generatedWith(t, srf.ParallelExact, workers, path, stl)) {
				t.Errorf("%s: generated on %d goroutines differs from serial", path, workers)
			}
		}
	})
	if count == 0 {
		t.Fatal("no fixtures")
	}
}

// TestParallelSameAsSerialStepAndRepeat is TestParallelSameAsSerial for
// a step and repeat of a step and repeat with 6 by 6 copies, whose
// pairs are combined at once, each with its own loops.
func TestParallelSameAsSerialStepAndRepeat(t *testing.T) {
	stl := filepath.Join(t.TempDir(), "out.stl")
	path := fixture("group/translate_nd/normal.slvs")
	copies := map[uint32]float64{6: 6, 7: 6}
	serial := generatedCopiesWith(t, srf.ParallelOff, 1, path, stl, copies)
	if bytes.Equal(serial, generatedWith(t, srf.ParallelOff, 1, path, stl)) {
		t.Fatal("the numbers of copies didn't change")
	}
	for _, mode := range []srf.ParallelMode{srf.ParallelExact, srf.ParallelFast} {
		for _, workers := range []int{1, 3, 8} {
			if !bytes.Equal(serial, generatedCopiesWith(t, mode, workers, path, stl, copies)) {
				t.Errorf("mode %d on %d goroutines differs from serial", mode, workers)
			}
		}
	}
}

// TestParallelFastSameOnAnyGoroutines generates every fixture in the
// fast mode, whose result mustn't depend on the number of goroutines.
func TestParallelFastSameOnAnyGoroutines(t *testing.T) {
	stl := filepath.Join(t.TempDir(), "out.stl")
	eachFixture(t, func(path string) {
		one := generatedWith(t, srf.ParallelFast, 1, path, stl)
		for _, workers := range []int{2, 8} {
			if !bytes.Equal(one, generatedWith(t, srf.ParallelFast, workers, path, stl)) {
				t.Errorf("%s: generated on %d goroutines differs from one", path, workers)
			}
		}
	})
}

// generatedShape loads a fixture, generates it in the given mode, and
// returns what shouldn't depend on the last bits: for every group,
// whether its Boolean failed, the numbers of its surfaces, curves and
// triangles, and its volume.
func generatedShape(mode srf.ParallelMode, path string) (counts []int, volumes []float64) {
	srf.SetParallelMode(mode)
	defer srf.SetParallelMode(srf.ParallelExact)
	srf.SetChordTolMm(0)
	doc, err := LoadFile(path)
	if err != nil {
		return nil, nil
	}
	sk := doc.Sketch
	sk.GenerateAllTyped(sketch.GenerateAll)
	for _, hg := range sk.GroupOrder {
		g := sk.GetGroup(hg)
		failed := 0
		if g.BooleanFailed {
			failed = 1
		}
		var m srf.SMesh
		g.RunningShell.TriangulateInto(&m)
		counts = append(counts, failed, len(g.RunningShell.Surface), len(g.RunningShell.Curve),
			len(m.Triangles), len(g.RunningMesh.Triangles))
		volumes = append(volumes, m.CalculateVolume(), g.RunningMesh.CalculateVolume())
	}
	return counts, volumes
}

// TestParallelFastSameShapeAsSerial requires of the fast mode the
// solids of the serial one, up to the last bits.
func TestParallelFastSameShapeAsSerial(t *testing.T) {
	eachFixture(t, func(path string) {
		wantN, wantV := generatedShape(srf.ParallelOff, path)
		gotN, gotV := generatedShape(srf.ParallelFast, path)
		if !slices.Equal(gotN, wantN) {
			t.Errorf("%s: fast mode has\n%v, serial\n%v", path, gotN, wantN)
			return
		}
		for i := range wantV {
			if math.Abs(gotV[i]-wantV[i]) > 1e-9*math.Max(1, math.Abs(wantV[i])) {
				t.Errorf("%s: fast mode has volume %v, serial %v", path, gotV[i], wantV[i])
			}
		}
	})
}

// raceEvery is how many fixtures and cases make one that runs under the
// race detector, which is about eight times slower. The race detector
// needs the code that runs on several goroutines, not every model; the
// plain and the coverage runs take them all.
const raceEvery = 5

// raceSample is every raceEvery-th of xs under the race detector, and
// all of them otherwise.
func raceSample[T any](xs []T) []T {
	if !raceDetector {
		return xs
	}
	var out []T
	for i, x := range xs {
		if i%raceEvery == 0 {
			out = append(out, x)
		}
	}
	return out
}
