package sketch_test

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/telesapr/telesapr/pkg/persist/tlspr"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/fixtures"
	"github.com/telesapr/telesapr/pkg/internal/par"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// generatedDump loads a fixture, generates it, and returns all that
// depends on the generation: the save, the STL export, and every
// group's running shell and mesh to the bit. copies gives the number
// of copies that step and repeat groups are to make instead of the
// fixture's.
func generatedDump(t *testing.T, path, stl string, copies map[uint32]float64) []byte {
	doc, err := tlspr.LoadFile(path)
	if err != nil {
		return nil
	}
	sk := doc.Sketch
	for h, n := range copies {
		sk.Group(core.HGroup{V: h}).ValA = n
	}
	sk.GenerateAllTyped(sketch.GenerateAll)

	var b bytes.Buffer
	vec := func(v geom.Vector) {
		fmt.Fprintf(&b, "%x %x %x ", math.Float64bits(v.X), math.Float64bits(v.Y), math.Float64bits(v.Z))
	}
	for _, hg := range sk.GroupOrder {
		g := sk.Group(hg)
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
	b.Write(tlspr.Encode(doc.Sketch, tlspr.Options{}))

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
	defer srf.SetParallelMode(srf.ParallelOn)
	return generatedDump(t, path, stl, copies)
}

// eachFixture calls fn for every model of the corpus and every solid
// golden of testdata/solid; under the race detector, for a sample of
// them (fixtures.Sample).
func eachFixture(t *testing.T, fn func(path string)) {
	var paths []string
	for _, rel := range corpus(t) {
		paths = append(paths, corpusPath(rel))
	}
	solids, err := filepath.Glob(fixtures.Path("solid/*.tlspr"))
	if err != nil {
		t.Fatal(err)
	}
	paths = append(paths, solids...)
	for _, path := range fixtures.Sample(paths) {
		fn(path)
	}
}

// TestParallelSameAsSerial generates every fixture on one goroutine and
// on several, and requires the same shells, meshes, save and STL file
// to the bit.
func TestParallelSameAsSerial(t *testing.T) {
	if testing.Short() {
		t.Skip("every fixture, twice")
	}
	stl := filepath.Join(t.TempDir(), "out.stl")
	count := 0
	eachFixture(t, func(path string) {
		serial := generatedWith(t, srf.ParallelOff, 1, path, stl)
		if serial == nil {
			return // not a valid file
		}
		count++
		for _, workers := range []int{1, 8} {
			if !bytes.Equal(serial, generatedWith(t, srf.ParallelOn, workers, path, stl)) {
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
	path := corpusPath("group/translate_nd/normal.tlspr")
	copies := map[uint32]float64{6: 6, 7: 6}
	serial := generatedCopiesWith(t, srf.ParallelOff, 1, path, stl, copies)
	if bytes.Equal(serial, generatedWith(t, srf.ParallelOff, 1, path, stl)) {
		t.Fatal("the numbers of copies didn't change")
	}
	for _, workers := range []int{1, 3, 8} {
		if !bytes.Equal(serial, generatedCopiesWith(t, srf.ParallelOn, workers, path, stl, copies)) {
			t.Errorf("on %d goroutines differs from serial", workers)
		}
	}
}
