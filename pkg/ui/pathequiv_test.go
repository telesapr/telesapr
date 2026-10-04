package ui

import (
	"context"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
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

// A post repeated round a circle picked with the mouse, and by a
// script (roadmap 28.5).
func TestPathEquivalencePatternRing(t *testing.T) {
	ui, _ := requireSamePaths(t, "path_pattern_ring", "path_pattern_ring")
	g := ui.GetGroup(ui.Active)
	if g.Type != sketch.GroupPathPattern || g.Problem.Msg != "" || g.ValA != 3 {
		t.Errorf("the pattern: type %d, problem %q, copies %g", g.Type, g.Problem.Msg, g.ValA)
	}
}

// A draft's sides chosen by the menu and its neutral plane set on its
// screen, and by a script (roadmap 28.2).
func TestPathEquivalenceDraftSides(t *testing.T) {
	ui, _ := requireSamePaths(t, "path_draft_sides", "path_draft_sides")
	if g := ui.GetGroup(ui.Active); len(g.DraftFaces) != 2 || g.ValB != 10 || g.Problem.Msg != "" {
		t.Errorf("the draft: %d sides, neutral %g, problem %q", len(g.DraftFaces), g.ValB, g.Problem.Msg)
	}
}

// A sweep round a corner, picked with the mouse, and by a script
// (roadmap 28.4).
func TestPathEquivalenceSweepCorner(t *testing.T) {
	ui, _ := requireSamePaths(t, "path_sweep_corner", "path_sweep_corner")
	if g := ui.GetGroup(ui.Active); g.Type != sketch.GroupSweep || g.Problem.Msg != "" || len(g.Edges) != 2 {
		t.Errorf("the sweep: type %d, problem %q, %d edges", g.Type, g.Problem.Msg, len(g.Edges))
	}
}

// Two edges picked with the mouse and chamfered, and by a script.
func TestPathEquivalenceChamfer(t *testing.T) {
	requireSamePaths(t, "path_chamfer", "path_chamfer")
}

// A chamfer given a second distance and swapped on its screen, and by
// a script (roadmap 28.1).
func TestPathEquivalenceChamferTwo(t *testing.T) {
	ui, _ := requireSamePaths(t, "path_chamfer_two", "path_chamfer_two")
	if g := ui.GetGroup(ui.Active); g.ValB != 2 || g.ValC != 1 {
		t.Errorf("the chamfer's distances are %g and %g, want 2 and 1", g.ValB, g.ValC)
	}
}

// A STEP file linked from the menu, and by a script.
func TestPathEquivalenceLinkStep(t *testing.T) {
	ui, _ := requireSamePaths(t, "path_link_step", "path_link_step")
	if g := ui.GetGroup(ui.Active); len(g.ImpMesh.Triangles) == 0 || len(g.LinkInfo) == 0 {
		t.Errorf("the link has %d triangles, info %q", len(g.ImpMesh.Triangles), g.LinkInfo)
	}
}

// A STEP file linked and read as a solid on its group screen, and the
// same linked as a solid by a script (roadmap 21.4).
func TestPathEquivalenceLinkStepSolid(t *testing.T) {
	ui, _ := requireSamePaths(t, "path_link_step_solid", "path_link_step_solid")
	g := ui.GetGroup(ui.Active)
	if !g.LinkAsSolid || len(g.ImpShell.Surface) == 0 || g.ForceToMesh {
		t.Errorf("the link: as solid %v, %d surfaces, forced to mesh %v", g.LinkAsSolid, len(g.ImpShell.Surface), g.ForceToMesh)
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

// A smooth loft from the active sketch through two picked with the
// mouse, and the same made by a script (roadmap 20).
func TestPathEquivalenceSmoothLoft(t *testing.T) {
	ui, _ := requireSamePaths(t, "path_loft_smooth", "path_loft_smooth")
	g := ui.GetGroup(ui.Active)
	if g.Type != sketch.GroupLoft || g.Problem.Msg != "" || len(g.LoftThrough) != 1 {
		t.Errorf("the loft: type %d, problem %q, through %v", g.Type, g.Problem.Msg, g.LoftThrough)
	}
}

// A sweep along a cubic picked with the mouse, twisted and held to an
// axis on the group screen, and the same made by a script (roadmap 20).
func TestPathEquivalenceSweepCubic(t *testing.T) {
	ui, _ := requireSamePaths(t, "path_sweep_cubic", "path_sweep_cubic")
	g := ui.GetGroup(ui.Active)
	if g.Type != sketch.GroupSweep || g.Problem.Msg != "" || g.ValC != 90 || g.SweepUp.X != 1 {
		t.Errorf("the sweep: type %d, problem %q, twist %g, up %v", g.Type, g.Problem.Msg, g.ValC, g.SweepUp)
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

// A group made with an earlier group active, from the mouse and from a
// script, goes into the history after that group, before the groups
// that follow it (roadmap 16.4).
func TestPathEquivalenceInsertGroup(t *testing.T) {
	ui, _ := requireSamePaths(t, "path_insert_group", "path_insert_group")
	var names []string
	for _, hg := range ui.GroupOrder {
		names = append(names, ui.GetGroup(hg).Name)
	}
	if got := strings.Join(names, " "); got != "#references sketch-in-plane extrude extrude" ||
		ui.GetGroup(ui.GroupOrder[2]).H.V < ui.GetGroup(ui.GroupOrder[3]).H.V {
		t.Errorf("the history: %s", got)
	}
}

// A dimension named in its label's editor and read by name in another
// one's, and the same made by a script (roadmap 16.1).
func TestPathEquivalenceDimensionName(t *testing.T) {
	ui, _ := requireSamePaths(t, "path_dimension_name", "path_dimension_name")
	if c := ui.NamedConstraint("width"); c == nil || c.ValA != 40 {
		t.Errorf("the dimension named width: %+v", c)
	}
}

// Configurations made and chosen on the parameters screen, and the
// same made by a script (roadmap 16.2).
func TestPathEquivalenceConfigurations(t *testing.T) {
	ui, _ := requireSamePaths(t, "path_configurations", "path_configurations")
	if ui.ActiveConfiguration != "small" || len(ui.Configurations) != 2 {
		t.Errorf("configurations %v, active %q", ui.Configurations, ui.ActiveConfiguration)
	}
}

// Two circles picked with the mouse and made a concentric mate, and the
// same made by a script (roadmap 16.9).
func TestPathEquivalenceMate(t *testing.T) {
	ui, _ := requireSamePaths(t, "path_mate", "path_mate")
	n := 0
	for _, c := range ui.Constraints {
		if c.Type == core.ConstraintPointsCoincident {
			n++
		}
	}
	if n != 1 {
		t.Errorf("%d coincident constraints", n)
	}
}

// A drawing set on its screen, and the same set by a script (roadmap
// 18.1).
func TestPathEquivalenceDrawing(t *testing.T) {
	ui, _ := requireSamePaths(t, "path_drawing", "path_drawing")
	if d := ui.Drawing; d == nil || d.Sheet != "A2" || d.Title != "PLATE" {
		t.Errorf("the drawing: %+v", d)
	}
}

// An appearance set on the group screen, and the same set by a script
// (roadmap 19.2).
func TestPathEquivalenceAppearance(t *testing.T) {
	ui, _ := requireSamePaths(t, "path_appearance", "path_appearance")
	if g := ui.GetGroup(ui.Active); g.Appearance != "metal" {
		t.Errorf("the extrusion's appearance %q", g.Appearance)
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
