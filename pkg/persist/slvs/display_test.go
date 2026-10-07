package slvs_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/golden"
	"github.com/telesapr/telesapr/pkg/persist/slvs"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// displayFixtures are the fixtures whose display outlines
// TestDisplayOutlines compares with testdata/display.
var displayFixtures = []string{
	"group/boolean_coplanar_union/normal.slvs",
	"group/boolean_knife_edge/normal.slvs",
	"group/boolean_tangent_crossing/normal.slvs",
	"group/boolean_tangent_edge/normal.slvs",
	"group/boolean_tangent_fillet/normal.slvs",
	"group/boolean_tangent_spline/normal.slvs",
	"group/force_to_mesh/union.slvs",
	"group/force_to_mesh/difference.slvs",
	"group/force_to_mesh/fillet.slvs",
	"group/force_to_mesh/translate.slvs",
	"group/link_idf/normal.slvs",
	"group/link_stl/normal.slvs",
	"group/translate_asy/normal.slvs",
	"group/translate_nd/normal.slvs",
	"request/ttf_text/extrude.slvs",
}

// TestDisplayOutlines checks each group's display items against the
// golden, first recorded from C++ Group::GenerateDisplayItems: the
// mesh's size, and the outlines as PolylineBuilder joins them, bit for
// bit.
func TestDisplayOutlines(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata")
	for _, f := range displayFixtures {
		t.Run(f, func(t *testing.T) {
			goldenPath := filepath.Join(root, "display", strings.TrimSuffix(f, ".slvs")+".outlines")
			// As the harness's SS.Init, generate a new file first: the
			// linked files load with the chord tolerance it leaves.
			blank := sketch.NewSketch()
			blank.NewFile()
			blank.GenerateAll()

			doc, err := slvs.LoadFile(filepath.Join(root, f))
			if err != nil {
				t.Fatal(err)
			}
			sk := doc.Sketch
			sk.GenerateAll()

			var got strings.Builder
			for _, hg := range sk.GroupOrder {
				g := sk.Group(hg)
				g.GenerateDisplayItems(sk)
				fmt.Fprintf(&got, "group %x mesh %d outlines %d\n", hg.V,
					len(g.DisplayMesh.Triangles), len(g.DisplayOutlines.Outlines))
				for _, so := range g.DisplayOutlines.Outlines {
					fmt.Fprintf(&got, "%d %s %s %s %s\n", so.Tag,
						hexVector(so.A), hexVector(so.B), hexVector(so.NL), hexVector(so.NR))
				}
			}
			golden.CheckDump(t, goldenPath, got.String())
		})
	}
}

func hexVector(v geom.Vector) string {
	return fmt.Sprintf("%x %x %x", v.X, v.Y, v.Z)
}
