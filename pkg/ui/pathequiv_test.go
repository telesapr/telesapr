package ui

import (
	"context"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/persist/tlspr"
	"github.com/telesapr/telesapr/pkg/script"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// requireSamePaths builds a model twice, through the interface and
// through a script, and requires the same saved file from both: the
// interface script testdata/ui/scripts/<uiScript>.script runs on the
// headless platform, and the Go script testdata/ui/scripts/<goScript>.go
// runs on a new sketch with no interface, as telesapr-cli run does.
// Both paths must reach the same engine entry points, so neither can
// drift from the other (ROADMAP-JUMP.md, "Definition of done").
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
			uiScript, goScript, firstDiffLine(string(savedUI), string(savedScript)))
	}
	return r.ss.SK, doc.Host().Sketch()
}

// A rectangle drawn with the mouse and extruded, and the same made by
// a script.
func TestPathEquivalenceExtrude(t *testing.T) {
	requireSamePaths(t, "path_extrude", "path_extrude")
}

// A screw thread made through its screen, and by a script.
func TestPathEquivalenceThread(t *testing.T) {
	requireSamePaths(t, "path_thread", "path_thread")
}

// A rectangle whose sides come from parameters, made through the
// parameters screen and the dimension editor, and by a script.
func TestPathEquivalenceParameters(t *testing.T) {
	requireSamePaths(t, "path_parameters", "path_parameters")
}

// A sketch in 3d moved earlier in the history through the Edit menu,
// and by a script.
func TestPathEquivalenceMoveGroup(t *testing.T) {
	requireSamePaths(t, "path_move_group", "path_move_group")
}

// A sketch suppressed whole through its group screen, and by a script.
func TestPathEquivalenceSuppressAll(t *testing.T) {
	requireSamePaths(t, "path_suppress_all", "path_suppress_all")
}

// An inch document made through View > Dimension Units, and by a
// script: the file's units line comes from either.
func TestPathEquivalenceUnits(t *testing.T) {
	ui, _ := requireSamePaths(t, "path_units", "path_units")
	if ui.Units != core.UnitInches {
		t.Errorf("the interface's document is in %v", ui.Units)
	}
}

// An extrusion deleted from its group screen, and by a script.
func TestPathEquivalenceDeleteGroup(t *testing.T) {
	requireSamePaths(t, "path_delete_group", "path_delete_group")
}

// A section view cut parallel to a face picked with the mouse, its
// offset typed and its side flipped on its screen, and the same set by
// a script. The section isn't saved, so its state and where it cuts
// are compared too.
func TestPathEquivalenceSection(t *testing.T) {
	ui, script := requireSamePaths(t, "path_section", "path_section")
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
}

// A circle swept along a path picked with the mouse, and by a script.
func TestPathEquivalenceSweep(t *testing.T) {
	requireSamePaths(t, "path_sweep", "path_sweep")
}

// An extrusion mirrored in a face picked with the mouse, and by a
// script.
func TestPathEquivalenceMirror(t *testing.T) {
	requireSamePaths(t, "path_mirror", "path_mirror")
}

// A mirror set on its screen to reflect the whole solid so far, and by
// a script (roadmap 15.5).
func TestPathEquivalenceMirrorSolid(t *testing.T) {
	ui, _ := requireSamePaths(t, "path_mirror_solid", "path_mirror_solid")
	if g := ui.GetGroup(ui.Active); g.Type != sketch.GroupMirror || !g.MirrorSolid || g.Problem.Msg != "" {
		t.Errorf("the mirror: type %d, whole %v, problem %q", g.Type, g.MirrorSolid, g.Problem.Msg)
	}
}

// A post repeated along a construction line picked with the mouse, and
// by a script (roadmap 15.6).
func TestPathEquivalencePathPattern(t *testing.T) {
	ui, _ := requireSamePaths(t, "path_pattern_path", "path_pattern_path")
	if g := ui.GetGroup(ui.Active); g.Type != sketch.GroupPathPattern || g.Problem.Msg != "" || g.ValA != 3 {
		t.Errorf("the pattern: type %d, problem %q, copies %g", g.Type, g.Problem.Msg, g.ValA)
	}
}

// Two edges picked with the mouse and chamfered, and by a script.
func TestPathEquivalenceChamfer(t *testing.T) {
	requireSamePaths(t, "path_chamfer", "path_chamfer")
}

// A STEP file linked from the menu, and by a script.
func TestPathEquivalenceLinkStep(t *testing.T) {
	ui, _ := requireSamePaths(t, "path_link_step", "path_link_step")
	if g := ui.GetGroup(ui.Active); len(g.ImpMesh.Triangles) == 0 || len(g.LinkInfo) == 0 {
		t.Errorf("the link has %d triangles, info %q", len(g.ImpMesh.Triangles), g.LinkInfo)
	}
}

// An edge of the solid picked on the model and chamfered, and the same
// edge chamfered by its name in a script.
func TestPathEquivalenceChamferSolidEdge(t *testing.T) {
	ui, _ := requireSamePaths(t, "path_chamfer_solid_edge", "path_chamfer_solid_edge")
	if g := ui.GetGroup(ui.Active); len(g.SolidEdges) != 1 || g.Problem.Msg != "" {
		t.Errorf("the chamfer has %d solid edges, problem %q", len(g.SolidEdges), g.Problem.Msg)
	}
}

// An edge of an extrusion picked with the mouse and rounded, and the
// same made by a script.
func TestPathEquivalenceFillet(t *testing.T) {
	ui, _ := requireSamePaths(t, "path_fillet", "path_fillet")
	if g := ui.GetGroup(ui.Active); g.Type != sketch.GroupFillet || g.Problem.Msg != "" {
		t.Errorf("the fillet: type %d, problem %q", g.Type, g.Problem.Msg)
	}
}

// A loft from the active sketch to one an entity is picked from with
// the mouse, and the same made by a script.
func TestPathEquivalenceLoft(t *testing.T) {
	ui, _ := requireSamePaths(t, "path_loft", "path_loft")
	if g := ui.GetGroup(ui.Active); g.Type != sketch.GroupLoft || g.Problem.Msg != "" {
		t.Errorf("the loft: type %d, problem %q", g.Type, g.Problem.Msg)
	}
}

// A loft's side, a face no entity names, picked with the mouse on the
// model and taken away by Shell Solid, and the same face by a script
// (roadmap 14.1).
func TestPathEquivalenceShellLoftFace(t *testing.T) {
	ui, _ := requireSamePaths(t, "path_shell_loft", "path_shell_loft")
	g := ui.GetGroup(ui.Active)
	if g.Type != sketch.GroupShell || g.Problem.Msg != "" || len(g.SolidFaces) != 1 {
		t.Errorf("the shell: type %d, problem %q, faces %v", g.Type, g.Problem.Msg, g.SolidFaces)
	}
}

// A face of an extrusion picked with the mouse and the solid hollowed,
// and the same made by a script.
func TestPathEquivalenceShell(t *testing.T) {
	ui, _ := requireSamePaths(t, "path_shell", "path_shell")
	if g := ui.GetGroup(ui.Active); g.Type != sketch.GroupShell || g.Problem.Msg != "" {
		t.Errorf("the shell: type %d, problem %q", g.Type, g.Problem.Msg)
	}
}

// The top of an extruded circle picked with the mouse and the cylinder
// hollowed into a cup, and the same made by a script (roadmap 15.2).
func TestPathEquivalenceShellCup(t *testing.T) {
	ui, _ := requireSamePaths(t, "path_shell_cup", "path_shell_cup")
	if g := ui.GetGroup(ui.Active); g.Type != sketch.GroupShell || g.Problem.Msg != "" {
		t.Errorf("the shell: type %d, problem %q", g.Type, g.Problem.Msg)
	}
}

// An extrusion's draft set on its group screen, and the same set by a
// script (roadmap 15.4).
func TestPathEquivalenceDraft(t *testing.T) {
	ui, _ := requireSamePaths(t, "path_draft", "path_draft")
	if g := ui.GetGroup(ui.Active); g.Type != sketch.GroupExtrude || g.ValC != 5 || g.Problem.Msg != "" {
		t.Errorf("the extrusion: type %d, draft %g, problem %q", g.Type, g.ValC, g.Problem.Msg)
	}
}

// Two edges that meet at a corner picked with the mouse and rounded,
// mitred, and the same made by a script.
func TestPathEquivalenceFilletMitre(t *testing.T) {
	ui, _ := requireSamePaths(t, "path_fillet_mitre", "path_fillet_mitre")
	if g := ui.GetGroup(ui.Active); g.Type != sketch.GroupFillet || g.Problem.Msg != "" || len(g.Edges) != 2 {
		t.Errorf("the fillet: type %d, problem %q, %d edges", g.Type, g.Problem.Msg, len(g.Edges))
	}
}

// A block's foot edge on a block, which goes in, picked with the mouse
// in a turned view and filled, and the same made by a script; the fill
// adds to the solid.
func TestPathEquivalenceFilletConcave(t *testing.T) {
	ui, _ := requireSamePaths(t, "path_fillet_concave", "path_fillet_concave")
	g := ui.GetGroup(ui.Active)
	if g.Type != sketch.GroupFillet || g.Problem.Msg != "" || len(g.Edges) != 1 {
		t.Fatalf("the fillet: type %d, problem %q, %d edges", g.Type, g.Problem.Msg, len(g.Edges))
	}
	var before, after srf.SMesh
	g.PreviousGroup(ui).RunningShell.TriangulateInto(&before)
	g.RunningShell.TriangulateInto(&after)
	if gain := after.CalculateVolume() - before.CalculateVolume(); gain <= 0 {
		t.Errorf("the fill adds %g", gain)
	}
}

// A side of a rectangle with a corner rounded by a tangent arc, extruded,
// picked with the mouse and rounded on round the arc, and the same made
// by a script.
func TestPathEquivalenceFilletOutline(t *testing.T) {
	ui, _ := requireSamePaths(t, "path_fillet_outline", "path_fillet_outline")
	if g := ui.GetGroup(ui.Active); g.Type != sketch.GroupFillet || g.Problem.Msg != "" {
		t.Errorf("the fillet: type %d, problem %q", g.Type, g.Problem.Msg)
	}
}

// Two edges of a ramp picked with the mouse on the turned view and
// rounded, meeting in an uneven mitre, and the same made by a script.
func TestPathEquivalenceFilletUneven(t *testing.T) {
	ui, _ := requireSamePaths(t, "path_fillet_uneven", "path_fillet_uneven")
	if g := ui.GetGroup(ui.Active); g.Type != sketch.GroupFillet || g.Problem.Msg != "" || len(g.Edges) != 2 {
		t.Errorf("the fillet: type %d, problem %q, %d edges", g.Type, g.Problem.Msg, len(g.Edges))
	}
}

// The circle a lathed frustum's top rim traces, where the top meets the
// cone, picked with the mouse and rounded, and the same made by a
// script.
func TestPathEquivalenceFilletCone(t *testing.T) {
	ui, _ := requireSamePaths(t, "path_fillet_cone", "path_fillet_cone")
	if g := ui.GetGroup(ui.Active); g.Type != sketch.GroupFillet || g.Problem.Msg != "" {
		t.Errorf("the fillet: type %d, problem %q", g.Type, g.Problem.Msg)
	}
}

// Two edges picked with the mouse and rounded, the second given its own
// radius on the group's screen, and the same made by a script.
func TestPathEquivalenceFilletRadii(t *testing.T) {
	ui, _ := requireSamePaths(t, "path_fillet_radii", "path_fillet_radii")
	g := ui.GetGroup(ui.Active)
	if g.Type != sketch.GroupFillet || g.Problem.Msg != "" || len(g.EdgeRadii) != 1 || g.EdgeRadii[0].Val != 3 {
		t.Errorf("the fillet: type %d, problem %q, radii %+v", g.Type, g.Problem.Msg, g.EdgeRadii)
	}
}

// A circle of an extrusion picked with the mouse and rounded, and the
// same made by a script.
func TestPathEquivalenceFilletCircle(t *testing.T) {
	ui, _ := requireSamePaths(t, "path_fillet_circle", "path_fillet_circle")
	if g := ui.GetGroup(ui.Active); g.Type != sketch.GroupFillet || g.Problem.Msg != "" {
		t.Errorf("the fillet: type %d, problem %q", g.Type, g.Problem.Msg)
	}
}

// The circle at a boss's foot, one edge of four pieces, picked on a
// piece whose name is not the lowest and filled, and the same edge
// filled by a script: both name it by its lowest piece, and the fill
// goes all round.
func TestPathEquivalenceFilletRim(t *testing.T) {
	ui, _ := requireSamePaths(t, "path_fillet_rim", "path_fillet_rim")
	g := ui.GetGroup(ui.Active)
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
}
