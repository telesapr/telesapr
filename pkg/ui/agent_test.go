package ui

import (
	"bytes"
	"context"
	"github.com/telesapr/telesapr/pkg/core"
	"image/png"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/platform/headless"
	"github.com/telesapr/telesapr/pkg/script/agent"
)

// agentApp is an application with an event loop on another goroutine
// that lets programs connect, and such a program's end of it.
type agentApp struct {
	t  *testing.T
	p  *loopPlatform
	ss *Application
	w  *headless.Console
	r  *agent.Remote
}

// on runs fn on the event loop and waits for it.
func (a *agentApp) on(fn func()) {
	a.t.Helper()
	ran := make(chan struct{})
	a.p.PostToLoop(func() {
		fn()
		close(ran)
	})
	<-ran
}

// shown is what the console showed after its first n texts.
func (a *agentApp) shown(n int) (text string) {
	a.on(func() { text = after(a.w, n) })
	return text
}

func newAgentApp(t *testing.T) *agentApp {
	t.Helper()
	// A socket's name must be short; a test's own directory may not
	// be.
	dir, err := os.MkdirTemp("", "ss")
	if err != nil {
		t.Fatal(err)
	}
	a := &agentApp{t: t, p: &loopPlatform{Platform: headless.New(), posted: make(chan func(), 64), done: make(chan struct{})}}
	go func() {
		for fn := range a.p.posted {
			fn()
		}
	}()
	a.on(func() {
		a.ss = New(a.p)
		a.ss.Init()
		a.ss.AgentSocketDir = dir
		a.ss.Console().SetVisible(true)
		a.w = a.p.Consoles[0]
		a.p.MainMenu.Find("Script", "Allow AI Connections").OnTrigger()
	})
	a.r = &agent.Remote{Connect: func() (net.Conn, error) { return agent.Connect(dir) }, CancelWait: 10 * time.Second}
	t.Cleanup(func() {
		a.r.Close()
		a.on(func() { _ = a.ss.AllowAgent(false) })
		close(a.p.posted)
		_ = os.RemoveAll(dir)
	})
	return a
}

// A program that connected runs scripts on the drawing on screen: the
// console shows them, each is one step of undo, and a picture is the
// drawing's.
func TestAgentDraws(t *testing.T) {
	a := newAgentApp(t)
	ctx := context.Background()
	var listening, checked bool
	a.on(func() {
		listening = a.ss.AgentListening() && a.ss.AgentSocket() != ""
		checked = a.p.MainMenu.Find("Script", "Allow AI Connections").Active
	})
	if !listening || !checked || !a.ss.AgentAllowed {
		t.Fatalf("after the menu: listening %v, checked %v, the setting %v", listening, checked, a.ss.AgentAllowed)
	}
	n := 0
	a.on(func() { n = len(a.w.Texts) })

	res, err := a.r.Run(ctx, "sides := doc.Rect(V(0, 0, 0), V(40, 20, 0))\nfmt.Println(\"drawn\")\nlen(sides)")
	if err != nil {
		t.Fatal(err)
	}
	if res.Output != "drawn\n" || res.Value != "4" || res.Error != "" ||
		!strings.HasPrefix(res.Status, "active group g002-sketch-in-plane, sketch-in-plane: solved") {
		t.Errorf("the script gave %+v", res)
	}
	want := "ai> sides := doc.Rect(V(0, 0, 0), V(40, 20, 0))\n    fmt.Println(\"drawn\")\n    len(sides)\ndrawn\n4\n"
	if got := a.shown(n); got != want {
		t.Errorf("the console showed\n%s\nwant\n%s", got, want)
	}
	requests := func() (n int) {
		a.on(func() { n = len(a.ss.SK.Requests) })
		return n
	}
	if got := requests(); got != 7 {
		t.Fatalf("the sketch has %d requests, want the references and four sides", got)
	}

	// What the user types in the console sees what the program defined.
	a.on(func() { n = len(a.w.Texts) })
	a.on(func() { a.w.Submit("sides[0].Length()") })
	for range 5000 {
		busy := false
		a.on(func() { busy = a.ss.Console().Busy() })
		if !busy {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if got := a.shown(n); got != ">>> sides[0].Length()\n40\n" {
		t.Errorf("the console showed %q for what was typed", got)
	}

	// A script that fails is rolled back, and the console says so.
	a.on(func() { n = len(a.w.Texts) })
	res, err = a.r.Run(ctx, "c := doc.Circle(V(5, 5, 0), 2)\nc.Start()")
	if err != nil || !res.RolledBack || !strings.Contains(res.Error, "has no start") {
		t.Errorf("the failing script gave %+v, %v", res, err)
	}
	if got := a.shown(n); !strings.Contains(got, "\nerror: script:2:") || !strings.HasSuffix(got, "What the script changed was taken back.\n") {
		t.Errorf("the console showed %q for the failing script", got)
	}
	if got := requests(); got != 7 {
		t.Errorf("the sketch has %d requests after the failed script, want 7", got)
	}

	// The description and the picture are the drawing's.
	if res, err = a.r.Run(ctx, "doc.Extrude(5, \"plate\")"); err != nil || res.Error != "" {
		t.Fatalf("the extrusion gave %+v, %v", res, err)
	}
	text, err := a.r.Describe(ctx, "")
	if err != nil || !strings.Contains(text, "g003-plate, extrude: solved") {
		t.Errorf("the drawing is described as %q, %v", text, err)
	}
	var active core.HGroup
	a.on(func() { active = a.ss.SK.Active })
	// The fitted view, the window's view, and the drawing up to the
	// sketch, which leaves the plate active (roadmap 16.14).
	for _, c := range [][2]string{{"isometric", ""}, {"window", ""}, {"front", "g002-sketch-in-plane"}} {
		data, err := a.r.Render(ctx, c[0], c[1], 200, 150)
		if err != nil {
			t.Fatalf("%v: %v", c, err)
		}
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil || img.Bounds().Dx() != 200 || img.Bounds().Dy() != 150 {
			t.Errorf("%v: the picture doesn't decode as 200 by 150: %v", c, err)
		}
	}
	var after core.HGroup
	a.on(func() { after = a.ss.SK.Active })
	if after != active {
		t.Errorf("rendering a group left g%03x active, not g%03x", after.V, active.V)
	}
	if _, err := a.r.Render(ctx, "front", "g099", 20, 20); err == nil || !strings.Contains(err.Error(), "no group") {
		t.Errorf("a render of no group: %v", err)
	}
	// What the mouse is over is in the description.
	a.on(func() {
		for _, e := range a.ss.SK.Entities {
			if e.Type == core.EntityLineSegment && e.Group.V == 2 {
				a.ss.GW.hover.Entity = e.H
				break
			}
		}
	})
	if text, err := a.r.Describe(ctx, ""); err != nil || !strings.Contains(text, "under the mouse:\n    ") {
		t.Errorf("the hovered line isn't described: %v\n%s", err, text)
	}
	a.on(func() { a.ss.GW.hover.Clear() })

	// Each script was one step of undo: two take back the extrusion
	// and the rectangle, and the failed one left none.
	var undone int
	a.on(func() {
		a.ss.UndoUndo()
		a.ss.UndoUndo()
		undone = len(a.ss.SK.Requests)
	})
	if undone != 3 {
		t.Errorf("after two undos the sketch has %d requests, want the references alone", undone)
	}
}

// With the setting, the user is asked before a program's script runs,
// and one that is refused doesn't.
func TestAgentConfirm(t *testing.T) {
	a := newAgentApp(t)
	ctx := context.Background()
	script := "doc.Circle(V(0, 0, 0), 5)\n" + strings.Repeat("// more\n", 40)
	a.on(func() {
		a.ss.AgentConfirm = true
		a.p.DialogResponses = []platform.Response{platform.ResponseNo, platform.ResponseYes}
	})

	if _, err := a.r.Run(ctx, script); err == nil || !strings.Contains(err.Error(), "refused") {
		t.Errorf("a refused script: %v", err)
	}
	var requests int
	var dialog *headless.MessageDialog
	a.on(func() {
		requests = len(a.ss.SK.Requests)
		dialog = a.p.DialogsShown[len(a.p.DialogsShown)-1]
	})
	if requests != 3 {
		t.Errorf("the refused script left %d requests, want the references alone", requests)
	}
	if !strings.HasPrefix(dialog.Description, "doc.Circle(V(0, 0, 0), 5)\n// more\n") ||
		!strings.HasSuffix(dialog.Description, "… and 11 more lines") {
		t.Errorf("the user was asked with %q, %q", dialog.Message, dialog.Description)
	}

	res, err := a.r.Run(ctx, script)
	if err != nil || res.Error != "" {
		t.Errorf("an allowed script gave %+v, %v", res, err)
	}
	a.on(func() { requests = len(a.ss.SK.Requests) })
	if requests != 4 {
		t.Errorf("the allowed script left %d requests, want the circle too", requests)
	}

	// Reading the drawing asks nothing.
	if _, err := a.r.Describe(ctx, ""); err != nil {
		t.Errorf("describing with the setting on: %v", err)
	}
	var asked int
	a.on(func() { asked = len(a.p.DialogsShown) })
	if asked != 2 {
		t.Errorf("the user was asked %d times, want twice", asked)
	}
}

// While the user's script runs a program's doesn't; Script > Stop
// stops a program's script; and turning the menu item off ends the
// connection.
func TestAgentBusyStopAndOff(t *testing.T) {
	a := newAgentApp(t)
	ctx := context.Background()

	a.on(func() { a.w.Submit("for {\n\ttime.Sleep(time.Millisecond)\n}") })
	if _, err := a.r.Run(ctx, "1 + 1"); err == nil || !strings.Contains(err.Error(), "another script is running") {
		t.Errorf("a program's script while the user's runs: %v", err)
	}
	a.on(func() { a.w.Interrupt() })
	idle := func() {
		t.Helper()
		for range 5000 {
			busy := false
			a.on(func() { busy = a.ss.Console().Busy() })
			if !busy {
				return
			}
			time.Sleep(time.Millisecond)
		}
		t.Fatal("the script didn't end")
	}
	idle()

	// The program's own endless script, stopped from the menu.
	drawn := filepath.Join(a.ss.AgentSocketDir, "drawn")
	go func() {
		for {
			if _, err := os.Stat(drawn); err == nil {
				break
			}
			time.Sleep(time.Millisecond)
		}
		a.on(func() { a.p.MainMenu.Find("Script", "Stop").OnTrigger() })
	}()
	res, err := a.r.Run(ctx, "doc.Circle(V(0, 0, 0), 1)\nos.WriteFile(\""+filepath.ToSlash(drawn)+"\", nil, 0o644)\nfor {\n\ttime.Sleep(time.Millisecond)\n}")
	if err != nil || res.Error != "the script was stopped" || !res.RolledBack {
		t.Errorf("the stopped script gave %+v, %v", res, err)
	}
	idle()

	// Off: the socket is gone, and the program is told what to do.
	socket := ""
	a.on(func() {
		socket = a.ss.AgentSocket()
		a.p.MainMenu.Find("Script", "Allow AI Connections").OnTrigger()
	})
	var listening, checked bool
	a.on(func() {
		listening = a.ss.AgentListening()
		checked = a.p.MainMenu.Find("Script", "Allow AI Connections").Active
	})
	if listening || checked || a.ss.AgentAllowed {
		t.Errorf("after the menu again: listening %v, checked %v, the setting %v", listening, checked, a.ss.AgentAllowed)
	}
	if _, err := os.Stat(socket); !os.IsNotExist(err) {
		t.Errorf("the socket %s is still there: %v", socket, err)
	}
	if _, err := a.r.Describe(ctx, ""); err == nil || !strings.Contains(err.Error(), "Allow AI Connections") {
		t.Errorf("describing after connections were turned off: %v", err)
	}
}

// The settings are kept: a program that was allowed is listened for
// when the application starts again, but not by the command-line tool.
func TestAgentSettings(t *testing.T) {
	dir, err := os.MkdirTemp("", "ss")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	ss, p := newTestApp(t)
	ss.AgentSocketDir = dir
	p.MainMenu.Find("Script", "Allow AI Connections").OnTrigger()
	ss.TW.ScreenChangeAgentConfirm(0, 0)
	if !ss.AgentListening() || !ss.AgentConfirm {
		t.Fatalf("listening %v, confirm %v", ss.AgentListening(), ss.AgentConfirm)
	}
	ss.Exit()
	if ss.AgentListening() {
		t.Errorf("still listening after the exit")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("%d files are left in the sockets' directory after the exit", len(entries))
	}

	again := New(p)
	again.LoadSettings()
	if !again.AgentAllowed || !again.AgentConfirm {
		t.Errorf("the settings came back as allowed %v, confirm %v", again.AgentAllowed, again.AgentConfirm)
	}

	// A directory that can't be made is an error, and nothing listens.
	blocked := filepath.Join(dir, "file")
	if err := os.WriteFile(blocked, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	other, _ := newTestApp(t)
	other.AgentSocketDir = filepath.Join(blocked, "under")
	if err := other.AllowAgent(true); err == nil || other.AgentListening() {
		t.Errorf("listening in a directory that can't be made: %v", err)
	}
	if err := other.AllowAgent(false); err != nil {
		t.Errorf("turning off what is off: %v", err)
	}
}
