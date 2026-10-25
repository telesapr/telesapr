package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/golden"
	"github.com/telesapr/telesapr/pkg/platform/headless"
)

// drawFixtures are the fixtures TestDrawCalls draws.
var drawFixtures = []string{
	"constraint/angle/free_in_3d.tlspr",
	"constraint/angle/normal.tlspr",
	"constraint/angle/reference_free_in_3d.tlspr",
	"constraint/angle/reference.tlspr",
	"constraint/angle/skew.tlspr",
	"constraint/arc_line_tangent/normal.tlspr",
	"constraint/at_midpoint/line_plane_free_in_3d.tlspr",
	"constraint/at_midpoint/line_plane_normal.tlspr",
	"constraint/at_midpoint/line_pt_free_in_3d.tlspr",
	"constraint/at_midpoint/line_pt_normal.tlspr",
	"constraint/comment/normal.tlspr",
	"constraint/cubic_line_tangent/free_in_3d.tlspr",
	"constraint/cubic_line_tangent/normal.tlspr",
	"constraint/curve_curve_tangent/arc_arc.tlspr",
	"constraint/curve_curve_tangent/arc_cubic.tlspr",
	"constraint/diameter/normal.tlspr",
	"constraint/diameter/reference.tlspr",
	"constraint/eq_len_pt_line_d/normal.tlspr",
	"constraint/eq_pt_ln_distances/normal.tlspr",
	"constraint/equal_angle/normal_old_version.tlspr",
	"constraint/equal_angle/normal.tlspr",
	"constraint/equal_angle/other.tlspr",
	"constraint/equal_length_lines/normal.tlspr",
	"constraint/equal_line_arc_len/normal.tlspr",
	"constraint/equal_line_arc_len/pi.tlspr",
	"constraint/equal_line_arc_len/tau.tlspr",
	"constraint/equal_radius/normal.tlspr",
	"constraint/horizontal/line.tlspr",
	"constraint/horizontal/pt_pt.tlspr",
	"constraint/large_dimensions/normal.tlspr",
	"constraint/length_difference/normal.tlspr",
	"constraint/length_difference/reference.tlspr",
	"constraint/length_ratio/normal.tlspr",
	"constraint/length_ratio/reference.tlspr",
	"constraint/parallel/free_in_3d.tlspr",
	"constraint/parallel/normal.tlspr",
	"constraint/perpendicular/normal.tlspr",
	"constraint/points_coincident/free_in_3d.tlspr",
	"constraint/points_coincident/normal.tlspr",
	"constraint/proj_pt_distance/normal.tlspr",
	"constraint/proj_pt_distance/reference.tlspr",
	"constraint/pt_face_distance/normal.tlspr",
	"constraint/pt_face_distance/reference.tlspr",
	"constraint/pt_in_plane/normal.tlspr",
	"constraint/pt_line_distance/extended.tlspr",
	"constraint/pt_line_distance/free_in_3d.tlspr",
	"constraint/pt_line_distance/normal.tlspr",
	"constraint/pt_line_distance/reference.tlspr",
	"constraint/pt_on_circle/negative_dia.tlspr",
	"constraint/pt_on_circle/normal.tlspr",
	"constraint/pt_on_face/normal.tlspr",
	"constraint/pt_on_line/left_free_in_3d.tlspr",
	"constraint/pt_on_line/normal.tlspr",
	"constraint/pt_on_line/right_free_in_3d.tlspr",
	"constraint/pt_plane_distance/normal.tlspr",
	"constraint/pt_plane_distance/reference.tlspr",
	"constraint/pt_pt_distance/free_in_3d.tlspr",
	"constraint/pt_pt_distance/normal.tlspr",
	"constraint/pt_pt_distance/reference.tlspr",
	"constraint/same_orientation/normal.tlspr",
	"constraint/same_orientation/same_group.tlspr",
	"constraint/symmetric/free_in_3d.tlspr",
	"constraint/symmetric_horiz/normal.tlspr",
	"constraint/symmetric_line/normal.tlspr",
	"constraint/symmetric/normal.tlspr",
	"constraint/symmetric_vert/normal.tlspr",
	"constraint/vertical/line.tlspr",
	"constraint/vertical/pt_pt.tlspr",
	"constraint/where_dragged/free_in_3d.tlspr",
	"constraint/where_dragged/normal.tlspr",
	"group/boolean_coplanar_union/normal.tlspr",
	"group/boolean_knife_edge/normal.tlspr",
	"group/boolean_tangent_crossing/normal.tlspr",
	"group/boolean_tangent_edge/normal.tlspr",
	"group/boolean_tangent_fillet/normal.tlspr",
	"group/boolean_tangent_spline/normal.tlspr",
	"group/force_to_mesh/difference.tlspr",
	"group/force_to_mesh/fillet.tlspr",
	"group/force_to_mesh/translate.tlspr",
	"group/force_to_mesh/union.tlspr",
	"group/link_idf/normal.tlspr",
	"group/link/normal.tlspr",
	"group/link/rect_v20.tlspr",
	"group/link_stl/normal.tlspr",
	"group/translate_asy/normal.tlspr",
	"group/translate_nd/normal.tlspr",
	"request/arc_of_circle/normal.tlspr",
	"request/circle/free_in_3d.tlspr",
	"request/circle/normal.tlspr",
	"request/cubic/normal.tlspr",
	"request/cubic_periodic/normal.tlspr",
	"request/datum_point/normal.tlspr",
	"request/image/linked.tlspr",
	"request/image/normal.tlspr",
	"request/line_segment/normal.tlspr",
	"request/ttf_text/extrude_kerning.tlspr",
	"request/ttf_text/extrude.tlspr",
	"request/ttf_text/kerning.tlspr",
	"request/ttf_text/normal.tlspr",
	"request/workplane/normal.tlspr",
}

// TestDrawCalls compares every canvas call graphicsWindow.Draw makes
// with the golden (testdata/draw), in the default view and a rotated
// one, bit for bit.
func TestDrawCalls(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..", "testdata")
	for _, f := range drawFixtures {
		t.Run(f, func(t *testing.T) {
			t.Parallel()
			goldenPath := filepath.Join(root, "draw", strings.TrimSuffix(f, ".tlspr")+".draw")
			golden.CheckDump(t, goldenPath, drawCallsFor(t, filepath.Join(root, "tlspr", "golden", f)))
		})
	}
}

// drawCallsFor opens a model and records the draw calls of both views.
func drawCallsFor(t testing.TB, path string) string {
	p := headless.New()
	ss := New(p)
	ss.LoadSettings()
	if err := ss.Load(path); err != nil {
		t.Fatal(err)
	}
	// The goldens are drawn by a camera of 297x210.
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
