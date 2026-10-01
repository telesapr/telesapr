package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spacesolve/spacesolve/pkg/platform"
	"github.com/spacesolve/spacesolve/pkg/platform/headless"
)

// shownConsole opens the console from the menu and returns its
// window.
func shownConsole(t *testing.T, ss *SolveSpaceUI, p *headless.Platform) *headless.Console {
	t.Helper()
	item := p.MainMenu.Find("Script", "Show Console")
	if item == nil {
		t.Fatal("no Script > Show Console in the menu")
	}
	item.OnTrigger()
	if len(p.Consoles) != 1 || !p.Consoles[0].Visible {
		t.Fatalf("the menu made %d consoles", len(p.Consoles))
	}
	return p.Consoles[0]
}

// after is what the console showed after the first n texts.
func after(w *headless.Console, n int) string {
	var b strings.Builder
	for _, text := range w.Texts[n:] {
		b.WriteString(text.Text)
	}
	return b.String()
}

// The console opens from the menu; what is typed draws on the sketch
// and shows its values, a statement over several lines waits for its
// end, and an error is shown without a dialog.
func TestScriptConsole(t *testing.T) {
	ss, p := newTestApp(t)
	w := shownConsole(t, ss, p)
	if w.Prompt != ">>> " || !strings.HasPrefix(w.Text(), "Go on the sketch") {
		t.Fatalf("the console starts with the prompt %q and %q", w.Prompt, w.Text())
	}
	if item := p.MainMenu.Find("Script", "Show Console"); !item.Active {
		t.Error("the menu item isn't checked while the console shows")
	}
	n := len(w.Texts)

	w.Submit(`c := doc.Circle(V(5, 5, 0), 2)`)
	w.Submit(`c.Radius()`)
	w.Submit(`for i := 0; i < 2; i++ {`)
	if w.Prompt != "... " {
		t.Errorf("the prompt in the middle of a statement is %q", w.Prompt)
	}
	w.Submit(`	fmt.Println("i =", i)`)
	w.Submit(`}`)
	if w.Prompt != ">>> " {
		t.Errorf("the prompt after the statement is %q", w.Prompt)
	}
	w.Submit(`c.Pos()`)
	w.Submit(`doc.Export("/nowhere/x.stl")`)
	want := ">>> c := doc.Circle(V(5, 5, 0), 2)\n" +
		">>> c.Radius()\n2\n" +
		">>> for i := 0; i < 2; i++ {\n" +
		"... \tfmt.Println(\"i =\", i)\n" +
		"... }\ni = 0\ni = 1\n" +
		">>> c.Pos()\nerror: circle r004-circle, radius 2.000 isn't a point; see Start, End, Center and Points\n" +
		">>> doc.Export(\"/nowhere/x.stl\")\nActive group mesh is empty; nothing to export.\n"
	if got := after(w, n); got != want {
		t.Errorf("the console showed:\n%s\nwant:\n%s", got, want)
	}
	kinds := map[platform.ConsoleKind]bool{}
	for _, text := range w.Texts[n:] {
		kinds[text.Kind] = true
	}
	for _, kind := range []platform.ConsoleKind{platform.ConsoleInput, platform.ConsoleValue,
		platform.ConsoleOutput, platform.ConsoleError} {
		if !kinds[kind] {
			t.Errorf("nothing was shown as kind %d", kind)
		}
	}
	if w.Busy || ss.Console().Busy() {
		t.Error("the console is still busy")
	}
	if got := len(ss.SK.Requests); got != 4 {
		t.Errorf("the sketch has %d requests, want the references and the circle", got)
	}

	// An interrupt drops the statement that is half typed.
	n = len(w.Texts)
	w.Submit(`if true {`)
	w.Interrupt()
	w.Submit(`1 + 1`)
	if got, want := after(w, n), ">>> if true {\nDropped.\n>>> 1 + 1\n2\n"; got != want {
		t.Errorf("after an interrupt the console showed %q, want %q", got, want)
	}

	// Several lines entered at once run one after another, a function
	// and its use among them, until one fails.
	n = len(w.Texts)
	w.Submit("func twice(v float64) float64 {\n\treturn 2 * v\n}\ntwice(c.Radius())\nnope()\nfmt.Println(\"not reached\")")
	if got := after(w, n+1); got != "4\nerror: undefined: nope\n" {
		t.Errorf("several lines showed %q", got)
	}

	// Closing the window hides it and unchecks the menu item.
	w.Events().OnClose()
	if w.Visible || p.MainMenu.Find("Script", "Show Console").Active {
		t.Error("the console is still shown, or checked, after closing")
	}
}

// Script > Run File runs a program on the sketch and shows its output
// in the console, which it opens.
func TestScriptConsoleRunFile(t *testing.T) {
	ss, p := newTestApp(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "disc.go")
	src := `package main

import (
	"fmt"

	"github.com/spacesolve/spacesolve/pkg/script/api"
)

func main() {
	doc := api.Current()
	c := doc.Circle(api.V(0, 0, 0), 10)
	doc.Diameter(c, 30)
	fmt.Println("radius", c.Radius())
	disc := doc.Extrude(2, "disc")
	fmt.Println(disc, "made")
}
`
	if err := os.WriteFile(file, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	p.FilePaths = append(p.FilePaths, file)
	p.MainMenu.Find("Script", "Run File...").OnTrigger()
	if len(p.Consoles) != 1 || !p.Consoles[0].Visible {
		t.Fatal("running a file didn't open the console")
	}
	w := p.Consoles[0]
	want := "run " + file + "\nradius 15\ng003-disc made\ndisc.go done.\n"
	if got := after(w, 1); got != want {
		t.Errorf("the console showed %q, want %q", got, want)
	}
	if got := ss.SK.Undo.Cnt; got != 1 {
		t.Errorf("the script is %d steps of undo, want 1", got)
	}

	// A file that fails says where.
	bad := filepath.Join(dir, "bad.go")
	if err := os.WriteFile(bad, []byte("a := 1\nb := a + c\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	n := len(w.Texts)
	ss.Console().RunFile(bad)
	if got, want := after(w, n), "run "+bad+"\nerror: "+bad+":2:10: undefined: c\n"; got != want {
		t.Errorf("the console showed %q, want %q", got, want)
	}
}

// With an event loop, what is entered runs on a goroutine of its own
// while the loop goes on; its calls, and what it shows, come to the
// loop, and a script that doesn't end can be stopped.
func TestScriptConsoleOnLoop(t *testing.T) {
	p := &loopPlatform{Platform: headless.New(), posted: make(chan func(), 64), done: make(chan struct{})}
	go func() {
		for fn := range p.posted {
			fn()
		}
	}()
	// on runs fn on the loop and waits for it.
	on := func(fn func()) {
		ran := make(chan struct{})
		p.PostToLoop(func() {
			fn()
			close(ran)
		})
		<-ran
	}
	var ss *SolveSpaceUI
	var w *headless.Console
	on(func() {
		ss = New(p)
		ss.Init()
		ss.Console().SetVisible(true)
		w = p.Consoles[0]
	})
	// idle waits for the script that runs to end.
	idle := func() {
		t.Helper()
		for range 5000 {
			busy := false
			on(func() { busy = ss.Console().Busy() })
			if !busy {
				return
			}
			time.Sleep(time.Millisecond)
		}
		t.Fatal("the script didn't end")
	}

	n := 0
	on(func() { n = len(w.Texts) })
	on(func() { w.Submit(`sides := doc.Rect(V(0, 0, 0), V(8, 6, 0))`) })
	idle()
	// A function of the script, run on the loop by Batch.
	on(func() {
		w.Submit("doc.Batch(func() {\n\tfor i := 0; i < 3; i++ {\n\t\tdoc.Point(V(float64(i), 1, 0))\n\t}\n\tfmt.Println(\"batched\")\n})")
	})
	idle()
	on(func() { w.Submit(`len(doc.Active().Requests())`) })
	idle()

	// One that goes on forever is busy until it is stopped; meanwhile
	// the console takes nothing else.
	on(func() { w.Submit("for {\n\ttime.Sleep(time.Millisecond)\n}") })
	busy, taken := false, true
	on(func() {
		busy = ss.Console().Busy() && w.Busy
		taken = w.Submit("1")
	})
	if !busy || taken {
		t.Errorf("while a script runs: busy %v, another line taken %v", busy, taken)
	}
	on(func() { w.Interrupt() })
	idle()

	var got string
	on(func() { got = after(w, n) })
	want := ">>> sides := doc.Rect(V(0, 0, 0), V(8, 6, 0))\n" +
		">>> doc.Batch(func() {\n\tfor i := 0; i < 3; i++ {\n\t\tdoc.Point(V(float64(i), 1, 0))\n\t}\n\tfmt.Println(\"batched\")\n})\n" +
		"batched\n" +
		">>> len(doc.Active().Requests())\n7\n" +
		">>> for {\n\ttime.Sleep(time.Millisecond)\n}\nStopped.\n"
	if got != want {
		t.Errorf("the console showed:\n%s\nwant:\n%s", got, want)
	}
	on(func() {
		if w.Busy || w.Prompt != ">>> " {
			t.Errorf("after the script was stopped the console is busy %v, with the prompt %q", w.Busy, w.Prompt)
		}
	})
	close(p.posted)
}
