package agent

import (
	"bytes"
	"context"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/telesapr/telesapr/pkg/script"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/script/symbols"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// newSession is a session on a new sketch with no program around it.
func newSession(t *testing.T) *Session {
	t.Helper()
	sk := sketch.NewSketch()
	sk.NewFile()
	s, _, err := Open(script.Options{Host: api.NewSketchHost(sk, "")})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func run(t *testing.T, s *Session, code string) Result {
	t.Helper()
	res, err := s.Run(context.Background(), code)
	if err != nil {
		t.Fatalf("running %q: %v", code, err)
	}
	return res
}

// primerDocs is what is known of package api's declarations.
func primerDocs(t *testing.T) *symbols.PackageDoc {
	t.Helper()
	p := symbols.Docs()[apiPath]
	if p == nil {
		t.Fatalf("nothing is known of %s", apiPath)
	}
	return p
}

// saved is the drawing as a file would hold it.
func saved(t *testing.T, s *Session) []byte {
	t.Helper()
	file := filepath.Join(t.TempDir(), "state.tlspr")
	if err := s.Engine().Doc().File.Save(file); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// The script the primer shows runs as it stands, and makes what it
// says it makes.
func TestPrimerExample(t *testing.T) {
	t.Parallel()
	s := newSession(t)
	res := run(t, s, primerExample)
	if res.Error != "" {
		t.Fatalf("the primer's example fails: %s\n%s", res.Error, res.Output)
	}
	lines := strings.Split(strings.TrimSpace(res.Output), "\n")
	if len(lines) != 4 || lines[0] != "solved: true DOF: 0" ||
		!strings.HasPrefix(lines[1], "sound: true volume: ") || lines[2] != "sound: true edges rounded: 4" ||
		!strings.HasPrefix(lines[3], "sound: true volume: ") {
		t.Fatalf("the example printed\n%s", res.Output)
	}
	doc := s.Engine().Doc()
	plate, pocket := doc.Group("plate").Analyze().Volume(), doc.Group("pocket").Analyze().Volume()
	wantPlate := (60*20 - 3*math.Pi*2.5*2.5) * 4
	wantPocket := wantPlate - math.Pi*3*3*2
	if math.Abs(plate-wantPlate) > wantPlate*0.01 || math.Abs(pocket-wantPocket) > wantPocket*0.01 {
		t.Errorf("the plate's volume is %g and with the pocket %g, want about %g and %g", plate, pocket, wantPlate, wantPocket)
	}
	if g := doc.Group("pocket-outline"); g.Dof() != 0 {
		t.Errorf("the pocket's outline has %d degrees of freedom, want 0", g.Dof())
	}
	if !strings.HasPrefix(res.Status, "active group g006-pocket, extrude: solved, ") || strings.Contains(res.Status, "in trouble") {
		t.Errorf("the status after the example is\n%s", res.Status)
	}
}

// The lines the primer shows for checking a result run.
func TestPrimerChecks(t *testing.T) {
	t.Parallel()
	s := newSession(t)
	if res := run(t, s, primerExample); res.Error != "" {
		t.Fatalf("the primer's example fails: %s", res.Error)
	}
	if res := run(t, s, primerChecks); res.Error != "" {
		t.Fatalf("the primer's checks fail: %s", res.Error)
	}
}

func TestPrimerAndInstructions(t *testing.T) {
	t.Parallel()
	if n := len(Instructions); n > MaxInstructions {
		t.Errorf("the instructions are %d bytes, over the %d clients keep", n, MaxInstructions)
	}
	primer := Primer()
	for _, part := range []string{
		"# Scripting TeleSAPR",
		primerExample,
		"doc finds what the drawing holds:\n    Activate Active ",
		"    doc.Features   BrokenReferences Chamfer ",
		"    doc.Sketch     ActiveWorkplane Anywhere3D Arc ",
		" SetCombine ",
		"    g.AsFillet()   EdgeRadius Radius ",
		"    SolidEdge: ",
		"\nFunctions: ",
		" V\n",
	} {
		if !strings.Contains(primer, part) {
			t.Errorf("the primer lacks %q", part)
		}
	}
	// The types it lists are all there are with methods, but for the
	// ones only the program uses.
	p := primerDocs(t)
	listed := map[string]bool{"Doc": true, "Group": true, "SketchHost": true}
	for f := range reflect.TypeFor[api.Doc]().Fields() {
		listed[f.Type.Name()] = true
	}
	for _, v := range groupViews() {
		listed[v[1]] = true
	}
	for _, name := range primerTypes {
		if p.Types[name] == nil {
			t.Errorf("the primer lists %s, which package api doesn't have", name)
		}
		listed[name] = true
	}
	for name, td := range p.Types {
		if len(td.Methods) > 0 && !listed[name] && td.Kind != "interface" {
			t.Errorf("package api's type %s has methods the primer doesn't list", name)
		}
	}
}

// What a script prints and the value it ends with come back, and what
// it defines stays for the next.
func TestSessionRun(t *testing.T) {
	t.Parallel()
	s := newSession(t)
	res := run(t, s, "l := doc.Sketch.Line(V(0, 0, 0), V(30, 40, 0))\nfmt.Println(\"drawn\")\nl.Length()")
	if res.Output != "drawn\n" || res.Value != "50" || res.Error != "" || res.RolledBack {
		t.Errorf("the run gave %+v", res)
	}
	if res.Status != "active group g002-sketch-in-plane, sketch-in-plane: solved, 4 DOF" {
		t.Errorf("the status is %q", res.Status)
	}
	if res = run(t, s, "doc.Constrain.Length(l, 60)\nl.Length()"); res.Value != "60" {
		t.Errorf("the line defined before is %s long, want 60; %+v", res.Value, res)
	}

	// Trouble in a group that isn't active shows in the status.
	res = run(t, s, "doc.Constrain.Length(l, 70).SetReference(false)\ndoc.Features.Sketch3D(\"more\")")
	if !strings.Contains(res.Status, "active group g003-more, sketch-in-3d: solved") ||
		!strings.Contains(res.Status, "\nin trouble: g002-sketch-in-plane, sketch-in-plane: NOT SOLVED") {
		t.Errorf("after two lengths on a line, the status is\n%s", res.Status)
	}

	// A whole program runs too, and a script that exits has ended.
	res = run(t, s, `package main

import (
	"fmt"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() { fmt.Println(len(api.Current().Groups())) }
`)
	if res.Output != "2\n" || res.Error != "" {
		t.Errorf("the program gave %+v", res)
	}
	if res = run(t, s, "fmt.Println(\"bye\")\nos.Exit(0)\nfmt.Println(\"never\")"); res.Output != "bye\n" || res.Error != "" {
		t.Errorf("a script that exits gave %+v", res)
	}
	if res = run(t, s, "os.Exit(3)"); res.Error != "exit status 3" {
		t.Errorf("a script that exits with 3 gave %+v", res)
	}

	if _, err := s.Run(context.Background(), "  \n"); err == nil {
		t.Errorf("running nothing is no error")
	}
}

// A script that fails leaves the drawing as it was, with nothing to
// redo, and says where it failed.
func TestSessionRollback(t *testing.T) {
	t.Parallel()
	s := newSession(t)
	run(t, s, "doc.Sketch.Rect(V(0, 0, 0), V(40, 20, 0))")
	before := saved(t, s)

	res := run(t, s, "c := doc.Sketch.Circle(V(10, 10, 0), 3)\ndoc.Features.Extrude(5, \"plate\")\nc.Length()\ndoc.Sketch.Line(V(0, 0, 0), V(1, 1, 0)).Radius()")
	if !res.RolledBack || !strings.HasPrefix(res.Error, "script:4:") || !strings.Contains(res.Error, "isn't a group to draw in") {
		t.Fatalf("the failing script gave %+v", res)
	}
	if !strings.HasPrefix(res.Status, "active group g002-sketch-in-plane, ") {
		t.Errorf("the status after the rollback is %q", res.Status)
	}
	doc := s.Engine().Doc()
	if after := saved(t, s); !bytes.Equal(before, after) {
		t.Errorf("the drawing changed over a script that failed:\n%s\nwas\n%s", after, before)
	}
	if doc.Edit.Redo() {
		t.Errorf("what the failed script did can be redone")
	}
	// The rectangle is still one step of undo away.
	if !doc.Edit.Undo() || len(doc.Active().Requests()) != 0 {
		t.Errorf("undo after the rollback leaves %d requests, want none", len(doc.Active().Requests()))
	}

	// A script that fails before it changes anything rolls nothing
	// back.
	res = run(t, s, "doc.Group(\"nothing\").Name()")
	if res.RolledBack || res.Error == "" {
		t.Errorf("a script that only failed gave %+v", res)
	}
	if res = run(t, s, "undefined(3)"); res.RolledBack || res.Error == "" {
		t.Errorf("a script that doesn't compile gave %+v", res)
	}
}

// A failed script is taken back whole though it called what remembers
// its own step of undo, as TangentArc and Split do, before or after
// its first other change.
func TestSessionRollbackOwnUndoSteps(t *testing.T) {
	t.Parallel()
	for name, code := range map[string]string{
		"after a change":  "ls := doc.Sketch.Polyline(false, V(0, 0, 0), V(20, 0, 0), V(20, 20, 0))\ndoc.Sketch.TangentArc(ls[0].End(), 5, true)\nls[0].Radius()",
		"first change":    "doc.Sketch.TangentArc(corner[0].End(), 5, true)\ndoc.Sketch.Circle(V(0, 0, 0), 1)\ndoc.Group(\"nothing\").Name()",
		"only change":     "doc.Sketch.TangentArc(corner[0].End(), 5, false)\ndoc.Group(\"nothing\").Name()",
		"twice":           "ls := doc.Sketch.Polyline(false, V(0, 0, 0), V(20, 0, 0), V(20, 20, 0), V(0, 20, 0))\ndoc.Sketch.TangentArc(ls[0].End(), 5, true)\ndoc.Sketch.TangentArc(ls[1].End(), 5, true)\ndoc.Group(\"nothing\").Name()",
		"split then fail": "a := doc.Sketch.Line(V(0, -5, 0), V(0, 5, 0))\nb := doc.Sketch.Line(V(-5, 0, 0), V(5, 0, 0))\ndoc.Sketch.Split(a, b, V(0, 0, 0))\ndoc.Group(\"nothing\").Name()",
	} {
		t.Run(name, func(t *testing.T) {
			s := newSession(t)
			run(t, s, "corner := doc.Sketch.Polyline(false, V(0, 0, 0), V(20, 0, 0), V(20, 20, 0))")
			before := saved(t, s)
			res := run(t, s, code)
			if !res.RolledBack || res.Error == "" {
				t.Fatalf("the failing script gave %+v", res)
			}
			if after := saved(t, s); !bytes.Equal(before, after) {
				t.Errorf("the drawing changed over a script that failed:\n%s\nwas\n%s", after, before)
			}
			doc := s.Engine().Doc()
			if doc.Edit.Redo() {
				t.Errorf("what the failed script did can be redone")
			}
			// What was drawn before is one step of undo away.
			if !doc.Edit.Undo() || len(doc.Active().Requests()) != 0 {
				t.Errorf("undo after the rollback leaves %d requests, want none", len(doc.Active().Requests()))
			}
		})
	}

	// A script that succeeds is one step of undo, whatever it called.
	s := newSession(t)
	if res := run(t, s, "ls := doc.Sketch.Polyline(false, V(0, 0, 0), V(20, 0, 0), V(20, 20, 0))\ndoc.Sketch.TangentArc(ls[0].End(), 5, true)"); res.Error != "" {
		t.Fatalf("the script gave %+v", res)
	}
	doc := s.Engine().Doc()
	if !doc.Edit.Undo() || len(doc.Active().Requests()) != 0 {
		t.Errorf("one undo after the script leaves %d requests, want none", len(doc.Active().Requests()))
	}
}

// A script that is stopped is rolled back like one that failed; one
// that runs elsewhere in the engine is in the way.
func TestSessionStopAndBusy(t *testing.T) {
	t.Parallel()
	s := newSession(t)
	ctx, cancel := context.WithCancel(context.Background())
	// The script says when it has drawn by making a file.
	drawn := filepath.Join(t.TempDir(), "drawn")
	go func() {
		for {
			if _, err := os.Stat(drawn); err == nil {
				break
			}
			time.Sleep(time.Millisecond)
		}
		cancel()
	}()
	res, err := s.Run(ctx, "doc.Sketch.Circle(V(0, 0, 0), 1)\nos.WriteFile("+strconv.Quote(drawn)+", nil, 0o644)\nfor {\n\ttime.Sleep(time.Millisecond)\n}")
	if err != nil || res.Error != "the script was stopped" || !res.RolledBack {
		t.Errorf("the stopped script gave %+v, %v", res, err)
	}
	if n := len(s.Engine().Doc().Active().Requests()); n != 0 {
		t.Errorf("%d requests are left of the stopped script", n)
	}

	done := make(chan struct{})
	ctx, cancel = context.WithCancel(context.Background())
	go func() {
		defer close(done)
		_, _ = s.Engine().Run(ctx, "", "for {\n\ttime.Sleep(time.Millisecond)\n}")
	}()
	for !s.Engine().Running() {
		time.Sleep(time.Millisecond)
	}
	if _, err := s.Run(context.Background(), "1 + 1"); err == nil || !strings.Contains(err.Error(), "another script is running") {
		t.Errorf("a run while the engine runs gave %v", err)
	}
	cancel()
	<-done
}

// After a stop the interpreter starts afresh, and the run says so.
func TestSessionRestartedAfterStop(t *testing.T) {
	t.Parallel()
	s := newSession(t)
	if res := run(t, s, "kept := 7"); res.Restarted {
		t.Errorf("the first run reports a restart: %+v", res)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	res, err := s.Run(ctx, "for {\n\ttime.Sleep(time.Millisecond)\n}")
	if err != nil || res.Error != "the script was stopped" || res.Restarted {
		t.Fatalf("the stopped script gave %+v, %v", res, err)
	}
	res = run(t, s, "kept")
	if !res.Restarted || res.Error == "" {
		t.Errorf("after the stop, the variable is %+v; want a restart and an error", res)
	}
	if res = run(t, s, "kept := 8"); res.Restarted {
		t.Errorf("the run after that reports a restart again: %+v", res)
	}
}

func TestSessionDescribeHelpRender(t *testing.T) {
	t.Parallel()
	s := newSession(t)
	ctx := context.Background()
	run(t, s, "doc.Sketch.Circle(V(0, 0, 0), 5)\ndoc.Features.Extrude(3, \"disc\")")

	text, err := describeText(s.Describe(ctx, Describing{}))
	if err != nil || text != s.Engine().Doc().Describe() || !strings.Contains(text, "g003-disc, extrude") {
		t.Errorf("the drawing is described as %q, %v", text, err)
	}
	if text, err = describeText(s.Describe(ctx, Describing{Script: true})); err != nil || !strings.Contains(text, "package main") || !strings.Contains(text, `"disc"`) {
		t.Errorf("the drawing as a script is %q, %v", text, err)
	}
	if text, err = describeText(s.Describe(ctx, Describing{Group: "g002"})); err != nil || !strings.Contains(text, "r004-circle: circle at (0, 0, 0), radius 5") {
		t.Errorf("the sketch is described as %q, %v", text, err)
	}
	if _, err = s.Describe(ctx, Describing{Group: "disk"}); err == nil || !strings.Contains(err.Error(), "g003-disc") {
		t.Errorf("describing a group that isn't there: %v, want the groups named", err)
	}
	// The solid at the active group, and at a group by name; the script
	// wins over it.
	if text, err = describeText(s.Describe(ctx, Describing{Solid: true})); err != nil || !strings.HasPrefix(text, "solid at g003-disc: 3 faces, 2 edges, ") ||
		!strings.Contains(text, ": circle about (0, 0, 3), radius 5, 31.416 mm, closed") {
		t.Errorf("the solid is described as %q, %v", text, err)
	}
	if text, err = describeText(s.Describe(ctx, Describing{Solid: true, Group: "g002"})); err != nil || text != "no solid yet at g002-sketch-in-plane\n" {
		t.Errorf("the solid at the sketch is described as %q, %v", text, err)
	}
	if text, err = describeText(s.Describe(ctx, Describing{Solid: true, Script: true})); err != nil || !strings.Contains(text, "package main") {
		t.Errorf("the script with solid is %q, %v", text, err)
	}
	if _, err = s.Describe(ctx, Describing{Solid: true, Group: "disk"}); err == nil || !strings.Contains(err.Error(), "g003-disc") {
		t.Errorf("the solid at a group that isn't there: %v", err)
	}

	if text, err = s.Help(ctx, ""); err != nil || text != Primer() {
		t.Errorf("help of no name isn't the primer: %v", err)
	}
	if text, err = s.Help(ctx, " Features.Extrude "); err != nil || !strings.Contains(text, "func (m FeaturesMenu) Extrude(depth float64, name string) Group") {
		t.Errorf("help of Features.Extrude is %q, %v", text, err)
	}
	if _, err = s.Help(ctx, "Features.Extrud"); err == nil {
		t.Errorf("help of a name nothing has is no error")
	}

	// A sketch with no program around it has no pictures; the sizes
	// are checked before that is found out.
	if _, err = s.Render(ctx, Picture{}); err == nil || !strings.Contains(err.Error(), "not available") {
		t.Errorf("a picture without the application: %v", err)
	}
	if _, err = s.Render(ctx, Picture{View: "front", Width: MaxSide + 1, Height: 10}); err == nil || !strings.Contains(err.Error(), "each side") {
		t.Errorf("a picture too wide: %v", err)
	}
	if _, err = s.Render(ctx, Picture{View: "sideways", Width: 10, Height: 10}); err == nil || !strings.Contains(err.Error(), "no view") {
		t.Errorf("a picture from nowhere: %v", err)
	}
}

// One side of a picture given, the other follows at 4:3.
func TestPictureSize(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ w, h, wantW, wantH int }{
		{0, 0, DefaultWidth, DefaultHeight},
		{400, 0, 400, 300},
		{0, 300, 400, 300},
		{101, 0, 101, 76},
		{1, 0, 1, 1},
		{0, 1, 1, 1},
		{640, 100, 640, 100},
	} {
		p := Picture{Width: c.w, Height: c.h}
		p.size()
		if p.Width != c.wantW || p.Height != c.wantH {
			t.Errorf("%d by %d became %d by %d, want %d by %d", c.w, c.h, p.Width, p.Height, c.wantW, c.wantH)
		}
	}
	s := newSession(t)
	if _, err := s.Render(context.Background(), Picture{View: "front", Width: MaxSide + 1}); err == nil || !strings.Contains(err.Error(), "each side") {
		t.Errorf("a picture too wide, with the height derived, gave %v", err)
	}
}

// A caption names the view, the size, the group and the solid's bounds.
func TestRenderCaption(t *testing.T) {
	t.Parallel()
	s := newSession(t)
	doc := s.Engine().Doc()
	if got := caption(doc, Picture{View: "top", Width: 80, Height: 60}); !strings.HasPrefix(got, "top view, 80 by 60 pixels, up to g002") || !strings.HasSuffix(got, "no solid yet") {
		t.Errorf("an empty drawing's caption is %q", got)
	}
	run(t, s, "doc.Sketch.Rect(V(0, 0, 0), V(60, 20, 0))\ndoc.Features.Extrude(4, \"plate\")")
	got := caption(doc, Picture{View: "isometric", Width: 800, Height: 600, Group: "g003", Section: "XY", SectionOffset: "2", SectionFlip: true})
	want := "isometric view, 800 by 600 pixels, up to g003-plate, cut by a plane parallel to XY at 2 with the other side kept; solid bounds (0, 0, 0) to (60, 20, 4) mm"
	if got != want {
		t.Errorf("the caption is\n%q\nwant\n%q", got, want)
	}
}
