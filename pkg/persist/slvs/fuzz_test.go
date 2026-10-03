package slvs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// FuzzLoad reads any bytes as a file: they load or give an error,
// never a panic.
func FuzzLoad(f *testing.F) {
	for _, name := range []string{
		"request/line_segment/normal.slvs",
		"request/ttf_text/normal.slvs",
		"group/boolean_coplanar_union/normal.slvs",
		"group/link/normal.slvs",
		"constraint/angle/normal.slvs",
		"solver/param_point_distance.slvs",
	} {
		data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", filepath.FromSlash(name)))
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	f.Add([]byte(""))
	f.Add([]byte("\xb1\xb2\xb3SolveSpaceREVa\n\nGroup.h.v=1\nAddGroup\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		d := &Document{Sketch: sketch.NewSketch(), groupRemap: make(map[core.HGroup][]RemapEntry)}
		_ = d.parse(data)
	})
}
