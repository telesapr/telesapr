package tlspr_test

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/persist/tlspr"
	"github.com/telesapr/telesapr/pkg/srf"
)

// BenchmarkMeshers regenerates each solid and thread golden with the ear
// clipper (and its CDT mender) and with the CDT on every curved face,
// and reports the active group's triangles and soundness: the
// measurement roadmap K4 asks for before the CDT could be the default.
// Run it once a model: go test ./pkg/persist/tlspr -run '^$'
// -bench BenchmarkMeshers -benchtime 1x; doc/dev-notes/k4-cdt-spike.md
// holds what it measured.
func BenchmarkMeshers(b *testing.B) {
	var files []string
	for _, dir := range []string{"solid", "thread"} {
		m, err := filepath.Glob(filepath.Join("..", "..", "..", "testdata", dir, "*.tlspr"))
		if err != nil {
			b.Fatal(err)
		}
		files = append(files, m...)
	}
	slices.Sort(files)
	meshers := []struct {
		name string
		m    srf.Mesher
	}{{"ear", srf.MesherEarClip}, {"cdt", srf.MesherCDT}}
	for _, f := range files {
		name := filepath.Base(filepath.Dir(f)) + "/" + strings.TrimSuffix(filepath.Base(f), ".tlspr")
		for _, ms := range meshers {
			b.Run(name+"/"+ms.name, func(b *testing.B) {
				doc, err := tlspr.LoadFile(f)
				if err != nil {
					b.Skip(err)
				}
				sk := doc.Sketch
				sk.NoSolidCache = true
				sk.Mesher = ms.m
				for range b.N {
					sk.GenerateAll()
				}
				g := sk.Group(sk.ActiveGroup())
				g.GenerateDisplayItems(sk)
				r := check.Solid(&g.RunningShell, &g.DisplayMesh)
				b.ReportMetric(float64(r.Triangles), "tris")
				b.ReportMetric(float64(r.NakedEdges), "naked")
				b.ReportMetric(float64(r.SelfIntersecting), "selfx")
				b.ReportMetric(r.Volume, "vol")
			})
		}
	}
}
