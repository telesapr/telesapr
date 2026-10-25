package sketch_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/fixtures"
	"github.com/telesapr/telesapr/pkg/internal/golden"
)

// displayFixtures are the models of the corpus whose display outlines
// TestDisplayOutlines compares with testdata/display.
var displayFixtures = []string{
	"group/boolean_coplanar_union/normal.tlspr",
	"group/boolean_knife_edge/normal.tlspr",
	"group/boolean_tangent_crossing/normal.tlspr",
	"group/boolean_tangent_edge/normal.tlspr",
	"group/boolean_tangent_fillet/normal.tlspr",
	"group/boolean_tangent_spline/normal.tlspr",
	"group/force_to_mesh/union.tlspr",
	"group/force_to_mesh/difference.tlspr",
	"group/force_to_mesh/fillet.tlspr",
	"group/force_to_mesh/translate.tlspr",
	"group/link_idf/normal.tlspr",
	"group/link_stl/normal.tlspr",
	"group/translate_asy/normal.tlspr",
	"group/translate_nd/normal.tlspr",
	"request/ttf_text/extrude.tlspr",
}

// TestDisplayOutlines checks each group's display items against the
// golden: the mesh's size, and the outlines as PolylineBuilder joins
// them, bit for bit.
func TestDisplayOutlines(t *testing.T) {
	t.Parallel()
	for _, f := range displayFixtures {
		t.Run(f, func(t *testing.T) {
			t.Parallel()
			goldenPath := fixtures.Path("display/" + strings.TrimSuffix(f, ".tlspr") + ".outlines")
			doc, err := openCorpus(f)
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
