package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/golden"
	"github.com/telesapr/telesapr/pkg/persist/slvs"
	"github.com/telesapr/telesapr/pkg/platform/headless"
	"github.com/telesapr/telesapr/pkg/ttf"
)

// drawFixtures are the fixtures TestDrawCalls draws.
var drawFixtures = []string{
	"constraint/angle/free_in_3d.slvs",
	"constraint/angle/normal.slvs",
	"constraint/angle/reference_free_in_3d.slvs",
	"constraint/angle/reference.slvs",
	"constraint/angle/skew.slvs",
	"constraint/arc_line_tangent/normal.slvs",
	"constraint/at_midpoint/line_plane_free_in_3d.slvs",
	"constraint/at_midpoint/line_plane_normal.slvs",
	"constraint/at_midpoint/line_pt_free_in_3d.slvs",
	"constraint/at_midpoint/line_pt_normal.slvs",
	"constraint/comment/normal.slvs",
	"constraint/cubic_line_tangent/free_in_3d.slvs",
	"constraint/cubic_line_tangent/normal.slvs",
	"constraint/curve_curve_tangent/arc_arc.slvs",
	"constraint/curve_curve_tangent/arc_cubic.slvs",
	"constraint/diameter/normal.slvs",
	"constraint/diameter/reference.slvs",
	"constraint/eq_len_pt_line_d/normal.slvs",
	"constraint/eq_pt_ln_distances/normal.slvs",
	"constraint/equal_angle/normal_old_version.slvs",
	"constraint/equal_angle/normal.slvs",
	"constraint/equal_angle/other.slvs",
	"constraint/equal_length_lines/normal.slvs",
	"constraint/equal_line_arc_len/normal.slvs",
	"constraint/equal_line_arc_len/pi.slvs",
	"constraint/equal_line_arc_len/tau.slvs",
	"constraint/equal_radius/normal.slvs",
	"constraint/horizontal/line.slvs",
	"constraint/horizontal/pt_pt.slvs",
	"constraint/large_dimensions/normal.slvs",
	"constraint/length_difference/normal.slvs",
	"constraint/length_difference/reference.slvs",
	"constraint/length_ratio/normal.slvs",
	"constraint/length_ratio/reference.slvs",
	"constraint/parallel/free_in_3d.slvs",
	"constraint/parallel/normal.slvs",
	"constraint/perpendicular/normal.slvs",
	"constraint/points_coincident/free_in_3d.slvs",
	"constraint/points_coincident/normal.slvs",
	"constraint/proj_pt_distance/normal.slvs",
	"constraint/proj_pt_distance/reference.slvs",
	"constraint/pt_face_distance/normal.slvs",
	"constraint/pt_face_distance/reference.slvs",
	"constraint/pt_in_plane/normal.slvs",
	"constraint/pt_line_distance/extended.slvs",
	"constraint/pt_line_distance/free_in_3d.slvs",
	"constraint/pt_line_distance/normal.slvs",
	"constraint/pt_line_distance/reference.slvs",
	"constraint/pt_on_circle/negative_dia.slvs",
	"constraint/pt_on_circle/normal.slvs",
	"constraint/pt_on_face/normal.slvs",
	"constraint/pt_on_line/left_free_in_3d.slvs",
	"constraint/pt_on_line/normal.slvs",
	"constraint/pt_on_line/right_free_in_3d.slvs",
	"constraint/pt_plane_distance/normal.slvs",
	"constraint/pt_plane_distance/reference.slvs",
	"constraint/pt_pt_distance/free_in_3d.slvs",
	"constraint/pt_pt_distance/normal.slvs",
	"constraint/pt_pt_distance/reference.slvs",
	"constraint/same_orientation/normal.slvs",
	"constraint/same_orientation/same_group.slvs",
	"constraint/symmetric/free_in_3d.slvs",
	"constraint/symmetric_horiz/normal.slvs",
	"constraint/symmetric_line/normal.slvs",
	"constraint/symmetric/normal.slvs",
	"constraint/symmetric_vert/normal.slvs",
	"constraint/vertical/line.slvs",
	"constraint/vertical/pt_pt.slvs",
	"constraint/where_dragged/free_in_3d.slvs",
	"constraint/where_dragged/normal.slvs",
	"group/boolean_coplanar_union/normal.slvs",
	"group/boolean_knife_edge/normal.slvs",
	"group/boolean_tangent_crossing/normal.slvs",
	"group/boolean_tangent_edge/normal.slvs",
	"group/boolean_tangent_fillet/normal.slvs",
	"group/boolean_tangent_spline/normal.slvs",
	"group/force_to_mesh/difference.slvs",
	"group/force_to_mesh/fillet.slvs",
	"group/force_to_mesh/translate.slvs",
	"group/force_to_mesh/union.slvs",
	"group/link_idf/normal.slvs",
	"group/link/normal.slvs",
	"group/link/rect_v20.slvs",
	"group/link_stl/normal.slvs",
	"group/translate_asy/normal.slvs",
	"group/translate_nd/normal.slvs",
	"request/arc_of_circle/normal.slvs",
	"request/circle/free_in_3d.slvs",
	"request/circle/normal.slvs",
	"request/cubic/normal.slvs",
	"request/cubic_periodic/normal.slvs",
	"request/datum_point/normal.slvs",
	"request/image/linked.slvs",
	"request/image/normal.slvs",
	"request/line_segment/normal.slvs",
	"request/ttf_text/extrude_kerning.slvs",
	"request/ttf_text/extrude.slvs",
	"request/ttf_text/kerning.slvs",
	"request/ttf_text/normal.slvs",
	"request/workplane/normal.slvs",
}

// TestDrawCalls compares every canvas call GraphicsWindow.Draw makes
// with the golden (testdata/draw, first recorded from C++), in the
// default view and a rotated one, bit for bit.
func TestDrawCalls(t *testing.T) {
	root := filepath.Join("..", "..", "testdata")
	ttf.DefaultFonts = ttf.NewFontList([]string{filepath.Join(root, "Gentium-R.ttf")})
	for _, f := range drawFixtures {
		t.Run(f, func(t *testing.T) {
			goldenPath := filepath.Join(root, "draw", strings.TrimSuffix(f, ".slvs")+".draw")
			if want, err := os.ReadFile(goldenPath); err == nil && len(want) == 0 {
				t.Skip("C++ couldn't draw this fixture")
			}
			golden.CheckDump(t, goldenPath, drawCallsFor(t, filepath.Join(root, f)))
		})
	}
}

// drawCallsFor loads a fixture as the C++ harness does and records the
// draw calls of both views.
func drawCallsFor(t testing.TB, path string) string {
	p := headless.New()
	ss := New(p)
	// As SS.Init: an empty new file first, whose generation leaves the
	// chord tolerance that linked files are then loaded with.
	ss.LoadSettings()
	ss.NewFile()
	ss.AfterNewFile()
	doc, err := slvs.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	ss.LoadSketchForTest(doc.Sketch, path)
	// C++'s headless build has no window: its camera is 297x210.
	w := ss.GW.window.(*headless.Window)
	w.Width, w.Height = 297, 210

	var b strings.Builder
	draw := func(name string) {
		fmt.Fprintf(&b, "view %s\n", name)
		rc := newRecordingCanvas(&b)
		rc.camera = ss.GW.Camera()
		rc.SetBitmapFont(ss.GW.bitmapFont())
		ss.GW.Draw(rc)
	}
	v := &ss.GW.view
	v.Scale = 5
	v.Offset = geom.Vector{}
	v.ProjRight = geom.Vector{X: 1}
	v.ProjUp = geom.Vector{Y: 1}
	draw("default")

	v.Scale = 3.25
	v.Offset = geom.Vector{X: -4.5, Y: 2.25, Z: 1.5}
	v.ProjRight = geom.Vector{X: 0.8, Y: -0.5, Z: 0.2}
	v.ProjUp = geom.Vector{X: 0.3, Y: 0.6, Z: 0.75}
	ss.GW.NormalizeProjectionVectors()
	draw("rotated")
	return b.String()
}
