package ui

import (
	"context"
	"io"
	"maps"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/golden"
	"github.com/telesapr/telesapr/pkg/persist/tlspr"
	"github.com/telesapr/telesapr/pkg/script"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// requireSamePaths builds a model twice, through the interface and
// through a script, and requires the same saved file from both: the
// interface script testdata/ui/scripts/<uiScript>.script runs on the
// headless platform, and the Go script testdata/ui/scripts/<goScript>.go
// runs on a new sketch with no interface, as telesapr-cli run does.
// Both paths must reach the same engine entry points, so neither can
// drift from the other (doc/done-audit.md, "Definition of done").
// The pairs are named path_*, and have no golden of their own: each
// path is the other's judge. An interface script must not save: a save
// regenerates, and the solver is not a fixed point, so a solved sketch
// solved again can move by an ulp. TELESAPR_PATHEQUIV_DIR names a
// directory to write both saves to.
func requireSamePaths(t *testing.T, uiScript, goScript string) (fromUI, fromScript *sketch.Sketch) {
	t.Helper()
	var r *uiScriptRunner
	runUIScriptWith(t, uiScript, func(rr *uiScriptRunner) { r = rr })
	savedUI := tlspr.Encode(r.ss.SK, tlspr.Options{})

	path := filepath.Join("..", "..", "testdata", "ui", "scripts", goScript+".go")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	doc := api.New()
	opts := script.Options{Host: doc.Host(), Stdout: io.Discard, Stderr: io.Discard}
	if err := script.RunSource(context.Background(), opts, path, string(src)); err != nil {
		t.Fatalf("%s: %v", goScript, err)
	}
	savedScript := tlspr.Encode(doc.Host().Sketch(), tlspr.Options{})

	if dir := os.Getenv("TELESAPR_PATHEQUIV_DIR"); dir != "" {
		// Both saves, for a diff.
		for name, data := range map[string][]byte{uiScript + ".ui.tlspr": savedUI, goScript + ".go.tlspr": savedScript} {
			if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	if string(savedUI) != string(savedScript) {
		t.Fatalf("the interface (%s) and the script (%s) save different files, at %s",
			uiScript, goScript, golden.FirstDifference(savedScript, savedUI))
	}
	return r.ss.SK, doc.Host().Sketch()
}

// pathCheck is what a path pair asserts beyond the two saves being the
// same, of the sketch the interface built and the one the script built.
type pathCheck func(t *testing.T, ui, script *sketch.Sketch)

// pathEquivalences holds every pair testdata/ui/scripts/path_<name>.script
// and path_<name>.go by its name, with its check or nil.
var pathEquivalences = map[string]pathCheck{
	// A rectangle drawn with the mouse and extruded, and the same made by
	// a script.
	"extrude": nil,
	// A screw thread made through its screen, and by a script.
	"thread": nil,
	// A screw thread hidden by its row's box in the group list and
	// suppressed whole from its screen, and the same by a script.
	"thread_whole": nil,
	// A screw thread whose size and pitch are chosen from the list and
	// whose hand is a choice, made through its screen, and by a script.
	"thread_choose": nil,
	// A screw thread with a chamfered end, made through its screen, and by
	// a script.
	"thread_chamfer": nil,
	// A screw thread made on a face picked on the model, from its other
	// end, through its screen, and by a script.
	"thread_face": nil,
	// A cosmetic screw thread made on a face through its screen, and by a
	// script.
	"thread_cosmetic": nil,
	// A screw thread made, then edited as one from its helix group's
	// screen, and by a script through ThreadGroup.Edit.
	"thread_edit": nil,
	// A screw thread with level starts and a run-out, made through its
	// screen, and by a script.
	"thread_runout": nil,
	// A rectangle whose sides come from parameters, made through the
	// parameters screen and the dimension editor, and by a script.
	"parameters": nil,
	// A sketch in 3d moved earlier in the history through the Edit menu,
	// and by a script.
	"move_group": nil,
	// A sketch suppressed whole through its group screen, and by a script.
	"suppress_all": nil,
	// A group's solve options set through its screen, and by a script: the
	// flags reach the same fields either way.
	"group_solve": nil,
	// An inch document made through View > Dimension Units, and by a
	// script: the file's units line comes from either.
	"units": func(t *testing.T, ui, _ *sketch.Sketch) {
		if ui.Units != core.UnitInches {
			t.Errorf("the interface's document is in %v", ui.Units)
		}
	},
	// An extrusion deleted from its group screen, and by a script.
	"delete_group": nil,
	// A section view cut parallel to a face picked with the mouse, its
	// offset typed and its side flipped on its screen, and the same set by
	// a script. The section isn't saved, so its state and where it cuts
	// are compared too.
	"section": func(t *testing.T, ui, script *sketch.Sketch) {
		if ui.Section == nil || script.Section == nil {
			t.Fatalf("section view on: interface %v, script %v", ui.Section != nil, script.Section != nil)
		}
		if *ui.Section != *script.Section {
			t.Errorf("the interface's section %+v, the script's %+v", *ui.Section, *script.Section)
		}
		keepUI, _, okUI, _ := ui.SectionCut()
		keepScript, _, okScript, _ := script.SectionCut()
		if !okUI || !okScript || keepUI != keepScript {
			t.Errorf("the interface cuts by %v (%v), the script by %v (%v)", keepUI, okUI, keepScript, okScript)
		}
	},
	// A circle swept along a path picked with the mouse, and by a script.
	"sweep": nil,
	// An extrusion mirrored in a face picked with the mouse, and by a
	// script.
	"mirror": nil,
	// A mirror set on its screen to reflect the whole solid so far, and by
	// a script (roadmap 15.5).
	"mirror_solid": func(t *testing.T, ui, _ *sketch.Sketch) {
		if g := ui.Group(ui.Active); g.Type != sketch.GroupMirror || !g.MirrorSolid || g.Problem.Msg != "" {
			t.Errorf("the mirror: type %d, whole %v, problem %q", g.Type, g.MirrorSolid, g.Problem.Msg)
		}
	},
	// A post repeated along a construction line picked with the mouse, and
	// by a script (roadmap 15.6).
	"pattern_path": func(t *testing.T, ui, _ *sketch.Sketch) {
		if g := ui.Group(ui.Active); g.Type != sketch.GroupPathPattern || g.Problem.Msg != "" || g.ValA != 3 {
			t.Errorf("the pattern: type %d, problem %q, copies %g", g.Type, g.Problem.Msg, g.ValA)
		}
	},
	// A post repeated round a circle picked with the mouse, and by a
	// script (roadmap 28.5).
	"pattern_ring": func(t *testing.T, ui, _ *sketch.Sketch) {
		g := ui.Group(ui.Active)
		if g.Type != sketch.GroupPathPattern || g.Problem.Msg != "" || g.ValA != 3 {
			t.Errorf("the pattern: type %d, problem %q, copies %g", g.Type, g.Problem.Msg, g.ValA)
		}
	},
	// A draft's sides chosen by the menu and its neutral plane set on its
	// screen, and by a script (roadmap 28.2).
	"draft_sides": func(t *testing.T, ui, _ *sketch.Sketch) {
		if g := ui.Group(ui.Active); len(g.DraftFaces) != 2 || g.ValB != 10 || g.Problem.Msg != "" {
			t.Errorf("the draft: %d sides, neutral %g, problem %q", len(g.DraftFaces), g.ValB, g.Problem.Msg)
		}
	},
	// A sweep round a corner, picked with the mouse, and by a script
	// (roadmap 28.4).
	"sweep_corner": func(t *testing.T, ui, _ *sketch.Sketch) {
		if g := ui.Group(ui.Active); g.Type != sketch.GroupSweep || g.Problem.Msg != "" || len(g.Edges) != 2 {
			t.Errorf("the sweep: type %d, problem %q, %d edges", g.Type, g.Problem.Msg, len(g.Edges))
		}
	},
	// A sweep round a closed path, picked with the mouse, and by a script.
	"sweep_ring": func(t *testing.T, ui, _ *sketch.Sketch) {
		if g := ui.Group(ui.Active); g.Type != sketch.GroupSweep || g.Problem.Msg != "" || len(g.Edges) != 1 {
			t.Errorf("the sweep: type %d, problem %q, %d edges", g.Type, g.Problem.Msg, len(g.Edges))
		}
	},
	// A sweep's corner rounded on its screen, and by a script (F4).
	"sweep_rounded": func(t *testing.T, ui, _ *sketch.Sketch) {
		if g := ui.Group(ui.Active); g.Type != sketch.GroupSweep || g.Problem.Msg != "" || g.ValB != 3 {
			t.Errorf("the sweep: type %d, problem %q, corner radius %g", g.Type, g.Problem.Msg, g.ValB)
		}
	},
	// A loft's ends set on its screen, and by a script (roadmap 28.3).
	"loft_tangency": func(t *testing.T, ui, _ *sketch.Sketch) {
		if g := ui.Group(ui.Active); g.Type != sketch.GroupLoft || g.ValB != 1 || g.ValC != 0.5 || g.Problem.Msg != "" {
			t.Errorf("the loft: type %d, ends %g and %g, problem %q", g.Type, g.ValB, g.ValC, g.Problem.Msg)
		}
	},
	// A loft set on its screen to leave along a drafted post's sides, and
	// by a script.
	"loft_sides": func(t *testing.T, ui, _ *sketch.Sketch) {
		if g := ui.Group(ui.Active); g.Type != sketch.GroupLoft || g.LoftAlongSides != [2]bool{true, false} || g.Problem.Msg != "" {
			t.Errorf("the loft: type %d, along sides %v, problem %q", g.Type, g.LoftAlongSides, g.Problem.Msg)
		}
	},
	// A smooth loft closed into a ring on its screen, and by a script.
	"loft_ring": func(t *testing.T, ui, _ *sketch.Sketch) {
		if g := ui.Group(ui.Active); g.Type != sketch.GroupLoft || !g.LoftClosed || g.Problem.Msg != "" {
			t.Errorf("the loft: type %d, closed %v, problem %q", g.Type, g.LoftClosed, g.Problem.Msg)
		}
	},
	// A smooth loft held square to a profile between its ends on its
	// screen, and by a script.
	"loft_profile_tangency": func(t *testing.T, ui, _ *sketch.Sketch) {
		if g := ui.Group(ui.Active); g.Type != sketch.GroupLoft || !slices.Equal(g.LoftThroughTangency, []float64{0, 1, 0, 0}) || g.Problem.Msg != "" {
			t.Errorf("the loft: type %d, tangencies %v, problem %q", g.Type, g.LoftThroughTangency, g.Problem.Msg)
		}
	},
	// A loft's guide picked with the mouse and set by Features > Guide
	// Loft by Selected Curves, and by a script (roadmap 28.3).
	"loft_guides": func(t *testing.T, ui, _ *sketch.Sketch) {
		if g := ui.Group(ui.Active); g.Type != sketch.GroupLoft || len(g.Edges) != 1 || g.Problem.Msg != "" {
			t.Errorf("the loft: type %d, guides %v, problem %q", g.Type, g.Edges, g.Problem.Msg)
		}
	},
	// Two edges picked with the mouse and chamfered, and by a script.
	"chamfer": nil,
	// The top face of an extrusion picked with the mouse, its edges
	// selected from the context menu and chamfered, and the same made by a
	// script.
	"face_edges": func(t *testing.T, ui, _ *sketch.Sketch) {
		if g := ui.Group(ui.Active); g.Type != sketch.GroupChamfer || g.Problem.Msg != "" {
			t.Errorf("the chamfer: type %d, problem %q", g.Type, g.Problem.Msg)
		}
	},
	// A chamfer given a second distance and swapped on its screen, and by
	// a script (roadmap 28.1).
	"chamfer_two": func(t *testing.T, ui, _ *sketch.Sketch) {
		if g := ui.Group(ui.Active); g.ValB != 2 || g.ValC != 1 {
			t.Errorf("the chamfer's distances are %g and %g, want 2 and 1", g.ValB, g.ValC)
		}
	},
	// The three edges at a corner picked with the mouse and chamfered at
	// two distances, the corner filled, and by a script.
	"chamfer_corner": func(t *testing.T, ui, _ *sketch.Sketch) {
		g := ui.Group(ui.Active)
		if g.Type != sketch.GroupChamfer || g.Problem.Msg != "" || len(g.Edges) != 3 || g.ValC != 2 {
			t.Fatalf("the chamfer: type %d, problem %q, %d edges, second distance %g", g.Type, g.Problem.Msg, len(g.Edges), g.ValC)
		}
		var m srf.SMesh
		g.RunningShell.TriangulateInto(&m)
		if r := check.Solid(&g.RunningShell, &m); !r.Sound() {
			t.Errorf("the chamfered corner: %s", r.String())
		}
	},
	// A STEP file linked from the menu, and by a script.
	"link_step": func(t *testing.T, ui, _ *sketch.Sketch) {
		if g := ui.Group(ui.Active); len(g.ImpMesh.Triangles) == 0 || len(g.LinkInfo) == 0 {
			t.Errorf("the link has %d triangles, info %q", len(g.ImpMesh.Triangles), g.LinkInfo)
		}
	},
	// A STEP file linked and read as a solid on its group screen, and the
	// same linked as a solid by a script (roadmap 21.4).
	"link_step_solid": func(t *testing.T, ui, _ *sketch.Sketch) {
		g := ui.Group(ui.Active)
		if !g.LinkAsSolid || len(g.ImpShell.Surface) == 0 || g.ForceToMesh {
			t.Errorf("the link: as solid %v, %d surfaces, forced to mesh %v", g.LinkAsSolid, len(g.ImpShell.Surface), g.ForceToMesh)
		}
	},
	// A point picked and named on the datums screen, and the same named by
	// a script.
	"datums": func(t *testing.T, ui, _ *sketch.Sketch) {
		if len(ui.Datums) != 1 || ui.Datums[0].Name != "corner" {
			t.Errorf("datums %v", ui.Datums)
		}
	},
	// A part linked, given a configuration and a parameter value and then
	// relinked on its group screen, and the same by a script.
	"link_params": func(t *testing.T, ui, _ *sketch.Sketch) {
		g := ui.Group(ui.Active)
		if g.LinkConfiguration != "long" || len(g.LinkParams) != 1 || filepath.Base(g.LinkFile) != "block.tlspr" {
			t.Errorf("the link: configuration %q, params %v, file %q", g.LinkConfiguration, g.LinkParams, g.LinkFile)
		}
	},
	// An edge of the solid picked on the model and chamfered, and the same
	// edge chamfered by its name in a script.
	"chamfer_solid_edge": func(t *testing.T, ui, _ *sketch.Sketch) {
		if g := ui.Group(ui.Active); len(g.SolidEdges) != 1 || g.Problem.Msg != "" {
			t.Errorf("the chamfer has %d solid edges, problem %q", len(g.SolidEdges), g.Problem.Msg)
		}
	},
	// An edge of the solid that no entity draws, picked on the model and
	// named as a datum on the datums screen, and the same made by a script:
	// a workplane that follows the edge.
	"datum_solid_edge": func(t *testing.T, ui, _ *sketch.Sketch) {
		d := ui.Datum("notch")
		if d == nil {
			t.Fatalf("datums %v", ui.Datums)
		}
		if g := ui.Group(d.Entity.Group()); g.Subtype != sketch.GroupWorkplaneByDatum || len(g.SolidEdges) != 1 || g.Problem.Msg != "" {
			t.Errorf("the datum's group: subtype %d, edges %v, problem %q", g.Subtype, g.SolidEdges, g.Problem.Msg)
		}
	},
	// A side of a loft, neither flat nor round, picked on the model and
	// named as a datum on the datums screen, and the same made by a script:
	// a workplane touching the face at its middle.
	"datum_freeform": func(t *testing.T, ui, _ *sketch.Sketch) {
		d := ui.Datum("side")
		if d == nil {
			t.Fatalf("datums %v", ui.Datums)
		}
		if g := ui.Group(d.Entity.Group()); g.Subtype != sketch.GroupWorkplaneByDatum || len(g.SolidFaces) != 1 ||
			g.Problem.Msg != "" || d.Kind != topo.DatumFacePoint {
			t.Errorf("the datum's group: subtype %d, faces %v, problem %q, kind %s", g.Subtype, g.SolidFaces, g.Problem.Msg, d.Kind)
		}
	},
	// An edge of an extrusion picked with the mouse and rounded, and the
	// same made by a script.
	"fillet": func(t *testing.T, ui, _ *sketch.Sketch) {
		if g := ui.Group(ui.Active); g.Type != sketch.GroupFillet || g.Problem.Msg != "" {
			t.Errorf("the fillet: type %d, problem %q", g.Type, g.Problem.Msg)
		}
	},
	// A loft from the active sketch to one an entity is picked from with
	// the mouse, and the same made by a script.
	"loft": func(t *testing.T, ui, _ *sketch.Sketch) {
		if g := ui.Group(ui.Active); g.Type != sketch.GroupLoft || g.Problem.Msg != "" {
			t.Errorf("the loft: type %d, problem %q", g.Type, g.Problem.Msg)
		}
	},
	// A smooth loft from the active sketch through two picked with the
	// mouse, and the same made by a script (roadmap 20).
	"loft_smooth": func(t *testing.T, ui, _ *sketch.Sketch) {
		g := ui.Group(ui.Active)
		if g.Type != sketch.GroupLoft || g.Problem.Msg != "" || len(g.LoftThrough) != 1 {
			t.Errorf("the loft: type %d, problem %q, through %v", g.Type, g.Problem.Msg, g.LoftThrough)
		}
	},
	// A sweep along a cubic picked with the mouse, twisted and held to an
	// axis on the group screen, and the same made by a script (roadmap 20).
	"sweep_cubic": func(t *testing.T, ui, _ *sketch.Sketch) {
		g := ui.Group(ui.Active)
		if g.Type != sketch.GroupSweep || g.Problem.Msg != "" || g.ValC != 90 || g.SweepUp.X != 1 {
			t.Errorf("the sweep: type %d, problem %q, twist %g, up %v", g.Type, g.Problem.Msg, g.ValC, g.SweepUp)
		}
	},
	// A loft's side, a face no entity names, picked with the mouse on the
	// model and taken away by Shell Solid, and the same face by a script
	// (roadmap 14.1).
	"shell_loft": func(t *testing.T, ui, _ *sketch.Sketch) {
		g := ui.Group(ui.Active)
		if g.Type != sketch.GroupShell || g.Problem.Msg != "" || len(g.SolidFaces) != 1 {
			t.Errorf("the shell: type %d, problem %q, faces %v", g.Type, g.Problem.Msg, g.SolidFaces)
		}
	},
	// A face of an extrusion picked with the mouse and the solid hollowed,
	// and the same made by a script.
	"shell": func(t *testing.T, ui, _ *sketch.Sketch) {
		if g := ui.Group(ui.Active); g.Type != sketch.GroupShell || g.Problem.Msg != "" {
			t.Errorf("the shell: type %d, problem %q", g.Type, g.Problem.Msg)
		}
	},
	// A cylinder picked with the mouse and a sketch wrapped onto it, and the
	// same made by a script (WP-C2).
	"wrap": func(t *testing.T, ui, _ *sketch.Sketch) {
		g := ui.Group(ui.Active)
		if g.Type != sketch.GroupWrap || g.Problem.Msg != "" || len(g.SolidFaces) != 1 {
			t.Errorf("the wrap: type %d, problem %q, faces %v", g.Type, g.Problem.Msg, g.SolidFaces)
		}
	},
	// The top of an extrusion picked with the mouse and a sketch started on
	// it, and the same made by a script (F1).
	"sketch_on_face": func(t *testing.T, ui, _ *sketch.Sketch) {
		g := ui.Group(ui.Active)
		if g.Type != sketch.GroupDrawingWorkplane || g.Subtype != sketch.GroupWorkplaneByFace ||
			g.Problem.Msg != "" || len(g.SolidFaces) != 1 {
			t.Errorf("the sketch: type %d, subtype %d, problem %q, faces %v", g.Type, g.Subtype, g.Problem.Msg, g.SolidFaces)
		}
	},
	// The top of an extruded circle picked with the mouse and the cylinder
	// hollowed into a cup, and the same made by a script (roadmap 15.2).
	"shell_cup": func(t *testing.T, ui, _ *sketch.Sketch) {
		if g := ui.Group(ui.Active); g.Type != sketch.GroupShell || g.Problem.Msg != "" {
			t.Errorf("the shell: type %d, problem %q", g.Type, g.Problem.Msg)
		}
	},
	// A ring turned by a lathe, its outer side picked with the mouse and
	// the ring hollowed open there, and the same made by a script: a shell
	// by the lathe's profile (WP-C1).
	"shell_lathe": func(t *testing.T, ui, _ *sketch.Sketch) {
		g := ui.Group(ui.Active)
		if g.Type != sketch.GroupShell || g.Problem.Msg != "" {
			t.Fatalf("the shell: type %d, problem %q", g.Type, g.Problem.Msg)
		}
		// The ring less the channel: the rectangle 10..24 by 0..20 less 11..24
		// by 1..19, turned.
		var m srf.SMesh
		g.RunningShell.TriangulateInto(&m)
		r := check.Solid(&g.RunningShell, &m)
		want := math.Pi * ((24*24-10*10)*20 - (24*24-11*11)*18)
		if !r.Sound() || math.Abs(r.Volume-want) > 0.002*want {
			t.Errorf("the channel ring: %s, want volume %g", r.String(), want)
		}
	},
	// An extrusion's draft set on its group screen, and the same set by a
	// script (roadmap 15.4).
	"draft": func(t *testing.T, ui, _ *sketch.Sketch) {
		if g := ui.Group(ui.Active); g.Type != sketch.GroupExtrude || g.ValC != 5 || g.Problem.Msg != "" {
			t.Errorf("the extrusion: type %d, draft %g, problem %q", g.Type, g.ValC, g.Problem.Msg)
		}
	},
	// A screw mate of two picked circles, and the same made by a script,
	// with the lead the window takes for the hole's diameter.
	"mate_screw": func(t *testing.T, ui, _ *sketch.Sketch) {
		if p := ui.Parameter("lead"); p == nil || p.Value != 0.7 || ui.Parameter("turn") == nil {
			t.Errorf("the screw's parameters: %+v", ui.Parameters)
		}
	},
	// A thread made with its class chosen and its phase typed on its
	// screen, and the same made by a script.
	"thread_fit": func(t *testing.T, ui, _ *sketch.Sketch) {
		for i := range ui.Groups {
			if o := ui.Groups[i].Thread; o.IsThread() {
				if o.Class != "7H" || o.Phase != 90 || o.Clearance <= 0 {
					t.Errorf("the thread: class %q, phase %g, clearance %g", o.Class, o.Phase, o.Clearance)
				}
				return
			}
		}
		t.Error("no thread")
	},
	// A screw mate of the rims of a threaded bolt and nut, picked, and the
	// same made by a script: the threads' lead, and the nut moved to where
	// the threads fit.
	"mate_screw_threads": func(t *testing.T, ui, _ *sketch.Sketch) {
		if p := ui.Parameter("lead"); p == nil || p.Value != 1.25 {
			t.Errorf("the screw's lead: %+v", ui.Parameters)
		}
		move, err := ui.ThreadFitMove(core.HEntity{V: 0x8003002d}, core.HEntity{V: 0x8004005d})
		if err != nil || math.Abs(math.Remainder(move, 1.25)) > 1e-6 {
			t.Errorf("the nut is %g from where it fits (%v)", move, err)
		}
	},
	// An extrusion's depth held on its group screen, and the same held by
	// a script.
	"extrude_depth": func(t *testing.T, ui, _ *sketch.Sketch) {
		if g := ui.Group(ui.Active); g.Type != sketch.GroupExtrude || g.Extent != 25 || g.Solved.Dof != 0 {
			t.Errorf("the extrusion: type %d, depth %g, dof %d", g.Type, g.Extent, g.Solved.Dof)
		}
	},
	// A group made with an earlier group active, from the mouse and from a
	// script, goes into the history after that group, before the groups
	// that follow it (roadmap 16.4).
	"insert_group": func(t *testing.T, ui, _ *sketch.Sketch) {
		var names []string
		for _, hg := range ui.GroupOrder {
			names = append(names, ui.Group(hg).Name)
		}
		if got := strings.Join(names, " "); got != "#references sketch-in-plane extrude extrude" ||
			ui.Group(ui.GroupOrder[2]).H.V < ui.Group(ui.GroupOrder[3]).H.V {
			t.Errorf("the history: %s", got)
		}
	},
	// A line's minimum and maximum lengths and a point on it as a segment,
	// made with the mouse and the Constrain menu, and the same made by a
	// script (F2).
	"distance_bounds": func(t *testing.T, ui, _ *sketch.Sketch) {
		var kinds []string
		for i := range ui.Constraints {
			kinds = append(kinds, ui.Constraints[i].DescriptionString())
		}
		want := []string{"c001-min-pt-pt-distance", "c002-max-pt-pt-distance", "c003-pt-on-segment"}
		if !slices.Equal(kinds, want) {
			t.Errorf("constraints %v, want %v", kinds, want)
		}
		// The point, drawn onto the line, draws the line a little past its
		// minimum, which then holds by itself: the line's four freedoms and
		// the point's one along it.
		if g := ui.Group(ui.Active); !g.IsSolvedOkay() || g.Solved.Dof != 5 {
			t.Errorf("the sketch: %v, %d DOF", g.Solved.How, g.Solved.Dof)
		}
	},
	// A line nudged with the arrow keys, and the same moved by a script
	// (F5).
	"nudge": func(t *testing.T, ui, _ *sketch.Sketch) {
		for i := range ui.Entities {
			if e := &ui.Entities[i]; e.Type == core.EntityLineSegment && e.Group == ui.Active {
				a := ui.MustEntity(e.Point[0]).PointGetNum(ui)
				if a.Minus(geom.Vector{X: -19.8, Y: 12}).Magnitude() > 1e-9 {
					t.Errorf("the line starts at %v", a)
				}
			}
		}
	},
	// A diagonal dimension added to a rectangle that has its width and
	// height is a reference dimension, made with the mouse and by a script
	// alike, and the group solves.
	"redundant_dimension": func(t *testing.T, ui, _ *sketch.Sketch) {
		refs := 0
		for i := range ui.Constraints {
			if ui.Constraints[i].Reference {
				refs++
			}
		}
		if refs != 1 {
			t.Errorf("%d reference dimensions, want 1", refs)
		}
	},
	// A dimension named in its label's editor and read by name in another
	// one's, and the same made by a script (roadmap 16.1).
	"dimension_name": func(t *testing.T, ui, _ *sketch.Sketch) {
		if c := ui.NamedConstraint("width"); c == nil || c.ValA != 40 {
			t.Errorf("the dimension named width: %+v", c)
		}
	},
	// Configurations made and chosen on the parameters screen, and the
	// same made by a script (roadmap 16.2).
	"configurations": func(t *testing.T, ui, _ *sketch.Sketch) {
		if ui.ActiveConfiguration != "small" || len(ui.Configurations) != 2 {
			t.Errorf("configurations %v, active %q", ui.Configurations, ui.ActiveConfiguration)
		}
	},
	// Two circles picked with the mouse and made a concentric mate, and the
	// same made by a script (roadmap 16.9).
	"mate": func(t *testing.T, ui, _ *sketch.Sketch) {
		n := 0
		for _, c := range ui.Constraints {
			if c.Type == core.ConstraintPointsCoincident {
				n++
			}
		}
		if n != 1 {
			t.Errorf("%d coincident constraints", n)
		}
	},
	// A drawing set on its screen, and the same set by a script (roadmap
	// 18.1).
	"drawing": func(t *testing.T, ui, _ *sketch.Sketch) {
		if d := ui.Drawing; d == nil || d.Sheet != "A2" || d.Title != "PLATE" {
			t.Errorf("the drawing: %+v", d)
		}
	},
	// A thread's drawing set to draw it as modelled on the drawing's
	// screen, and the same set by a script.
	"drawing_thread": func(t *testing.T, ui, _ *sketch.Sketch) {
		if d := ui.Drawing; d == nil || !d.ModelledThreads {
			t.Errorf("the drawing: %+v", d)
		}
	},
	// An appearance set on the group screen, and the same set by a script
	// (roadmap 19.2).
	"appearance": func(t *testing.T, ui, _ *sketch.Sketch) {
		if g := ui.Group(ui.Active); g.Appearance != "metal" {
			t.Errorf("the extrusion's appearance %q", g.Appearance)
		}
	},
	// Two edges that meet at a corner picked with the mouse and rounded,
	// mitred, and the same made by a script.
	"fillet_mitre": func(t *testing.T, ui, _ *sketch.Sketch) {
		if g := ui.Group(ui.Active); g.Type != sketch.GroupFillet || g.Problem.Msg != "" || len(g.Edges) != 2 {
			t.Errorf("the fillet: type %d, problem %q, %d edges", g.Type, g.Problem.Msg, len(g.Edges))
		}
	},
	// A block's foot edge on a block, which goes in, picked with the mouse
	// in a turned view and filled, and the same made by a script; the fill
	// adds to the solid.
	"fillet_concave": func(t *testing.T, ui, _ *sketch.Sketch) {
		g := ui.Group(ui.Active)
		if g.Type != sketch.GroupFillet || g.Problem.Msg != "" || len(g.Edges) != 1 {
			t.Fatalf("the fillet: type %d, problem %q, %d edges", g.Type, g.Problem.Msg, len(g.Edges))
		}
		var before, after srf.SMesh
		g.PreviousGroup(ui).RunningShell.TriangulateInto(&before)
		g.RunningShell.TriangulateInto(&after)
		if gain := after.CalculateVolume() - before.CalculateVolume(); gain <= 0 {
			t.Errorf("the fill adds %g", gain)
		}
	},
	// A side of a rectangle with a corner rounded by a tangent arc, extruded,
	// picked with the mouse and rounded on round the arc, and the same made
	// by a script.
	"fillet_outline": func(t *testing.T, ui, _ *sketch.Sketch) {
		if g := ui.Group(ui.Active); g.Type != sketch.GroupFillet || g.Problem.Msg != "" {
			t.Errorf("the fillet: type %d, problem %q", g.Type, g.Problem.Msg)
		}
	},
	// Two edges of a ramp picked with the mouse on the turned view and
	// rounded, meeting in an uneven mitre, and the same made by a script.
	"fillet_uneven": func(t *testing.T, ui, _ *sketch.Sketch) {
		if g := ui.Group(ui.Active); g.Type != sketch.GroupFillet || g.Problem.Msg != "" || len(g.Edges) != 2 {
			t.Errorf("the fillet: type %d, problem %q, %d edges", g.Type, g.Problem.Msg, len(g.Edges))
		}
	},
	// The circle a lathed frustum's top rim traces, where the top meets the
	// cone, picked with the mouse and rounded, and the same made by a
	// script.
	"fillet_cone": func(t *testing.T, ui, _ *sketch.Sketch) {
		if g := ui.Group(ui.Active); g.Type != sketch.GroupFillet || g.Problem.Msg != "" {
			t.Errorf("the fillet: type %d, problem %q", g.Type, g.Problem.Msg)
		}
	},
	// Two edges picked with the mouse and rounded, the second given its own
	// radius on the group's screen, and the same made by a script.
	"fillet_radii": func(t *testing.T, ui, _ *sketch.Sketch) {
		g := ui.Group(ui.Active)
		if g.Type != sketch.GroupFillet || g.Problem.Msg != "" || len(g.EdgeRadii) != 1 || g.EdgeRadii[0].Val != 3 {
			t.Errorf("the fillet: type %d, problem %q, radii %+v", g.Type, g.Problem.Msg, g.EdgeRadii)
		}
	},
	// A circle of an extrusion picked with the mouse and rounded, and the
	// same made by a script.
	"fillet_circle": func(t *testing.T, ui, _ *sketch.Sketch) {
		if g := ui.Group(ui.Active); g.Type != sketch.GroupFillet || g.Problem.Msg != "" {
			t.Errorf("the fillet: type %d, problem %q", g.Type, g.Problem.Msg)
		}
	},
	// The circle at a boss's foot, one edge of four pieces, picked on a
	// piece whose name is not the lowest and filled, and the same edge
	// filled by a script: both name it by its lowest piece, and the fill
	// goes all round.
	"fillet_rim": func(t *testing.T, ui, _ *sketch.Sketch) {
		g := ui.Group(ui.Active)
		if g.Type != sketch.GroupFillet || g.Problem.Msg != "" || len(g.SolidEdges) != 1 {
			t.Fatalf("the fillet: type %d, problem %q, %d edges", g.Type, g.Problem.Msg, len(g.SolidEdges))
		}
		tp, _ := g.PreviousGroup(ui).Topology()
		e, err := tp.Resolve(g.SolidEdges[0])
		if err != nil || tp.Lead(e) != e || len(tp.Chain(e)) != 4 {
			t.Errorf("the fillet's edge %v: %v, a chain of %d", g.SolidEdges[0], err, len(tp.Chain(e)))
		}
		// A torus all round fills the foot: the spandrel of radius 1 turned
		// about the axis at 8 plus its centroid's distance out. The meshes
		// are at the interface's chord tolerance, whose curves' points the
		// shells keep, so this is a bound; TestFilletConcaveSweep in
		// pkg/topo measures the fill exactly.
		var before, after srf.SMesh
		g.PreviousGroup(ui).RunningShell.TriangulateInto(&before)
		g.RunningShell.TriangulateInto(&after)
		c := (10 - 3*math.Pi) / (12 - 3*math.Pi)
		want := 2 * math.Pi * (8 + c) * (1 - math.Pi/4)
		if gain := after.CalculateVolume() - before.CalculateVolume(); math.Abs(gain-want) > 0.1*want {
			t.Errorf("the fill adds %g, want %g", gain, want)
		}
	},
	// A circle made as big as another as it is drawn, and by a script
	// through Constrain.Auto (roadmap 29.4).
	"auto_constrain": nil,
	// A face's diameter on the drawing, the face picked with the mouse on
	// the drawing's screen, and by a script (roadmap 29.2).
	"drawing_faces": func(t *testing.T, ui, _ *sketch.Sketch) {
		if ui.Drawing == nil || len(ui.Drawing.Dimensions) != 1 || len(ui.Drawing.Dimensions[0].Faces) != 1 {
			t.Errorf("the interface's drawing: %+v", ui.Drawing)
		}
	},
	// A projected distance and two equal angles, made with the mouse and
	// the Constrain menu, and by a script.
	"equal": nil,
	// A point as far from a line segment as another line segment is long,
	// made with the mouse and the Constrain menu, and by a script.
	"equal_length": nil,
	// How far one point is from another along a line segment, made with the
	// mouse and the Constrain menu, and by a script.
	"projected_distance": nil,
	// A custom line style drawn at a level of its own, made on the styles
	// screen, and by a script.
	"style": nil,
	// A point and two line segments picked with the mouse and a workplane
	// made through the point parallel to the lines, and the same made by a
	// script.
	"sketch_on_lines": func(t *testing.T, ui, _ *sketch.Sketch) {
		g := ui.Group(ui.Active)
		if g.Type != sketch.GroupDrawingWorkplane || g.Subtype != sketch.GroupWorkplaneByLineSegments || g.Problem.Msg != "" {
			t.Errorf("the sketch: type %d, subtype %d, problem %q", g.Type, g.Subtype, g.Problem.Msg)
		}
	},
	// A rectangle turned about a construction line with a point chosen
	// with it, which is the axis's point, and the same made by a script.
	"lathe_point_axis": func(t *testing.T, ui, _ *sketch.Sketch) {
		g := ui.Group(ui.Active)
		if g.Type != sketch.GroupLathe || g.Problem.Msg != "" || g.Predef.Origin.V == 0 {
			t.Errorf("the lathe: type %d, problem %q", g.Type, g.Problem.Msg)
		}
	},
	// The drawing's views chosen and placed on its screen, with a section
	// placed, and the same set by a script.
	"drawing_views": func(t *testing.T, ui, _ *sketch.Sketch) {
		d := ui.Drawing
		if d == nil || len(d.Views) != 4 || d.ViewIndex("back") < 0 || d.ViewIndex("iso") >= 0 ||
			len(d.Sections) != 1 || d.Sections[0].X != 300 {
			t.Errorf("the drawing: %+v", d)
		}
	},
	// What a feature is built from changed on its screen, and by a
	// script (ROADMAP F7): a fillet's and a chamfer's edges, a shell's
	// faces, a sweep's and a pattern's path, a loft's profiles and a
	// mirror's plane.
	"fillet_edges": func(t *testing.T, ui, _ *sketch.Sketch) {
		g := ui.Group(ui.Active)
		if g.Type != sketch.GroupFillet || g.Problem.Msg != "" || len(g.Edges) != 1 || g.Edges[0].V != 0x8003001f {
			t.Errorf("the fillet: type %d, problem %q, edges %v", g.Type, g.Problem.Msg, g.Edges)
		}
	},
	"chamfer_edges": func(t *testing.T, ui, _ *sketch.Sketch) {
		g := ui.Group(ui.Active)
		if g.Type != sketch.GroupChamfer || g.Problem.Msg != "" || len(g.Edges) != 1 || g.Edges[0].V != 0x8003001f {
			t.Errorf("the chamfer: type %d, problem %q, edges %v", g.Type, g.Problem.Msg, g.Edges)
		}
	},
	"shell_faces": func(t *testing.T, ui, _ *sketch.Sketch) {
		g := ui.Group(ui.Active)
		if g.Type != sketch.GroupShell || g.Problem.Msg != "" || len(g.SolidFaces) != 1 || g.SolidFaces[0] != 0x80030019 {
			t.Errorf("the shell: type %d, problem %q, faces %x", g.Type, g.Problem.Msg, g.SolidFaces)
		}
	},
	"sweep_path": func(t *testing.T, ui, _ *sketch.Sketch) {
		g := ui.Group(ui.Active)
		if g.Type != sketch.GroupSweep || g.Problem.Msg != "" || len(g.Edges) != 2 {
			t.Errorf("the sweep: type %d, problem %q, %d edges", g.Type, g.Problem.Msg, len(g.Edges))
		}
	},
	"pattern_path_changed": func(t *testing.T, ui, _ *sketch.Sketch) {
		g := ui.Group(ui.Active)
		if g.Type != sketch.GroupPathPattern || g.Problem.Msg != "" || len(g.Edges) != 1 || g.Edges[0].V != 0x80030016 {
			t.Errorf("the pattern: type %d, problem %q, edges %v", g.Type, g.Problem.Msg, g.Edges)
		}
	},
	"loft_profiles": func(t *testing.T, ui, _ *sketch.Sketch) {
		g := ui.Group(ui.Active)
		if g.Type != sketch.GroupLoft || g.Problem.Msg != "" || len(g.LoftThrough) != 0 ||
			ui.Group(g.OpA).Name != "bottom" || ui.Group(g.OpB).Name != "middle" {
			t.Errorf("the loft: type %d, problem %q, from %s to %s through %d", g.Type, g.Problem.Msg,
				ui.GroupName(g.OpA), ui.GroupName(g.OpB), len(g.LoftThrough))
		}
	},
	"mirror_plane": func(t *testing.T, ui, _ *sketch.Sketch) {
		g := ui.Group(ui.Active)
		if g.Type != sketch.GroupMirror || g.Problem.Msg != "" || g.Predef.EntityB.V != 0x20000 {
			t.Errorf("the mirror: type %d, problem %q, plane %x", g.Type, g.Problem.Msg, g.Predef.EntityB.V)
		}
	},
}

// TestPathEquivalence runs every path pair of testdata/ui/scripts, and
// fails on a pair pathEquivalences has no entry for or an entry with no
// pair. path_script_form pairs with an example, not a script of its own,
// and has TestPathEquivalenceScriptForm.
func TestPathEquivalence(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob(filepath.Join("..", "..", "testdata", "ui", "scripts", "path_*.script"))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, f := range files {
		name := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(f), "path_"), ".script")
		if name == "script_form" {
			continue
		}
		check, ok := pathEquivalences[name]
		if !ok {
			t.Errorf("%s has no entry in pathEquivalences", f)
			continue
		}
		seen[name] = true
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fromUI, fromScript := requireSamePaths(t, "path_"+name, "path_"+name)
			if check != nil {
				check(t, fromUI, fromScript)
			}
		})
	}
	for _, name := range slices.Sorted(maps.Keys(pathEquivalences)) {
		if !seen[name] {
			t.Errorf("pathEquivalences has %s, but there is no path_%s.script", name, name)
		}
	}
}

// The washer of examples/scripts/form, its form filled in the dialog
// Script > Run File shows, and the same values given as telesapr-cli
// run --set gives them.
func TestPathEquivalenceScriptForm(t *testing.T) {
	t.Parallel()
	var r *uiScriptRunner
	runUIScriptWith(t, "path_script_form", func(rr *uiScriptRunner) { r = rr })
	savedUI := tlspr.Encode(r.ss.SK, tlspr.Options{})

	path := filepath.Join("..", "..", "examples", "scripts", "form", "form.go")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	doc := api.New()
	opts := script.Options{Host: doc.Host(), Stdout: io.Discard, Stderr: io.Discard,
		Form: map[string]string{"outer": "40", "slots": "6", "finish": "plastic", "save": "false"}}
	if err := script.RunSource(context.Background(), opts, path, string(src)); err != nil {
		t.Fatal(err)
	}
	savedScript := tlspr.Encode(doc.Host().Sketch(), tlspr.Options{})
	if string(savedUI) != string(savedScript) {
		t.Fatalf("the dialog and --set save different files, at %s", golden.FirstDifference(savedScript, savedUI))
	}
	if !strings.Contains(string(savedScript), "plastic") {
		t.Error("the finish chosen is not in the file")
	}
}
