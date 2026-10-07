package ui

import (
	"bufio"
	"fmt"
	"hash/maphash"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/cfmt"
	"github.com/telesapr/telesapr/pkg/internal/golden"
	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/platform/headless"
	"github.com/telesapr/telesapr/pkg/ttf"
)

// uiScriptRunner runs the scripts in testdata/ui/scripts on the headless
// platform, and prints what the C++ harness that recorded the first
// goldens printed.
type uiScriptRunner struct {
	t        testing.TB
	p        *headless.Platform
	ss       *Application
	b        strings.Builder
	dir      string // the directory script paths are relative to
	tmp      string // a temporary directory, $tmp in a script's paths
	savePath string

	popupChoices []string

	// frames, when not nil, collects a hash of each frame the graphics
	// window presents.
	frames *[]uint64
}

func g17(v float64) string { return cfmt.Sprintf("%.17g", v) }

func v17(v geom.Vector) string { return g17(v.X) + " " + g17(v.Y) + " " + g17(v.Z) }

func escLine(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, "\n", `\n`)
}

// runUIScript runs a script from testdata/ui/scripts and returns its
// output.
func runUIScript(t testing.TB, name string) string {
	return runUIScriptWith(t, name, nil)
}

// runUIScriptWith is runUIScript, first letting setup adjust the
// runner.
func runUIScriptWith(t testing.TB, name string, setup func(r *uiScriptRunner)) string {
	r := newUIScriptRunner(t)
	if setup != nil {
		setup(r)
	}
	f, err := os.Open(filepath.Join(r.dir, "ui", "scripts", name+".script"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || line[0] == '#' {
			continue
		}
		if !r.step(line) {
			break
		}
		r.settle()
	}
	return r.b.String()
}

// newUIScriptRunner starts the application, as the harness does.
func newUIScriptRunner(t testing.TB) *uiScriptRunner {
	root := filepath.Join("..", "..", "testdata")
	ttf.SetFontFiles([]string{filepath.Join(root, "Gentium-R.ttf")})

	// The save goes where the harness's mktemp -d puts its own, one
	// folder under the temporary directory: a linked file is saved
	// relative to it.
	saveDir, err := os.MkdirTemp("", "telesapr-ui-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(saveDir) })
	r := &uiScriptRunner{t: t, dir: root, tmp: saveDir, savePath: filepath.Join(saveDir, "save.tlspr")}
	r.p = headless.New()
	r.p.FontFiles = []string{filepath.Join(root, "Gentium-R.ttf")}
	r.p.PopUpHook = r.popUp
	r.p.OnMessageDialog = func(d *headless.MessageDialog) {
		fmt.Fprintf(&r.b, "dialog %d %s|%s\n", d.Type, escLine(r.unexpand(d.Message)), escLine(r.unexpand(d.Description)))
	}
	r.p.OnFileDialog = func(d *headless.FileDialog) {
		kind := "open"
		if d.Save {
			kind = "save"
		}
		fmt.Fprintf(&r.b, "filedialog %s\n", kind)
	}
	r.ss = New(r.p)
	r.ss.Init()
	// A script that allows AI connections listens here, not where the
	// user's own TeleSAPR does.
	r.ss.AgentSocketDir = filepath.Join(saveDir, "agent")
	t.Cleanup(func() { _ = r.ss.AllowAgent(false) })
	// Animations jump to their end, so they don't depend on the clock.
	r.ss.AnimationSpeed = 0
	r.settle()
	return r
}

// expand replaces $tmp/ and $testdata/ at the start of a script's path
// by the runner's temporary directory and testdata.
func (r *uiScriptRunner) expand(path string) string {
	for prefix, dir := range map[string]string{"$tmp/": r.tmp, "$testdata/": r.dir} {
		if rest, ok := strings.CutPrefix(path, prefix); ok {
			abs, err := filepath.Abs(dir)
			if err != nil {
				r.t.Fatal(err)
			}
			return filepath.Join(abs, filepath.FromSlash(rest))
		}
	}
	return path
}

// unexpand writes the runner's temporary directory and testdata in s
// as $tmp and $testdata, so output doesn't depend on the machine.
func (r *uiScriptRunner) unexpand(s string) string {
	for _, d := range []struct{ dir, name string }{{r.tmp, "$tmp"}, {r.dir, "$testdata"}} {
		if abs, err := filepath.Abs(d.dir); err == nil {
			s = strings.ReplaceAll(s, abs, d.name)
		}
	}
	return s
}

func (r *uiScriptRunner) gw() *headless.Window { return r.ss.GW.window.(*headless.Window) }
func (r *uiScriptRunner) tw() *headless.Window { return r.ss.TW.window.(*headless.Window) }

// frameSeed hashes the frames settle records: one seed, so the hashes
// of two runs compare.
var frameSeed = maphash.MakeSeed()

// settle does the event loop's idle work: timers, then repaints, until
// quiet.
func (r *uiScriptRunner) settle() {
	for range 100 {
		busy := r.p.RunDueTimers(1000) > 0
		if w := r.gw(); w.Invalidated > 0 {
			busy = true
			w.Render()
			if r.frames != nil && w.Frame != nil {
				*r.frames = append(*r.frames, maphash.Bytes(frameSeed, w.Frame.Pix))
			}
		}
		if w := r.tw(); w.Invalidated > 0 {
			busy = true
			w.Render()
		}
		if !busy {
			return
		}
	}
}

func (r *uiScriptRunner) popUp(m *headless.Menu) *headless.MenuItem {
	fmt.Fprintf(&r.b, "popup\n")
	dumpMenuItems(&r.b, m.Items, 1)
	if len(r.popupChoices) == 0 {
		fmt.Fprintf(&r.b, "popup none\n")
		return nil
	}
	choice := r.popupChoices[0]
	r.popupChoices = r.popupChoices[1:]
	for _, it := range m.Items {
		if !it.Separator && it.Label == choice {
			fmt.Fprintf(&r.b, "popup pick %s\n", choice)
			return it
		}
	}
	fmt.Fprintf(&r.b, "popup missing %s\n", choice)
	return nil
}

func dumpMenuItems(b *strings.Builder, items []*headless.MenuItem, depth int) {
	indent := strings.Repeat("  ", depth)
	for _, it := range items {
		if it.Separator {
			fmt.Fprintf(b, "%s---\n", indent)
			continue
		}
		fmt.Fprintf(b, "%s%s", indent, it.RawLabel)
		if it.Accel != "" {
			fmt.Fprintf(b, " [%s]", it.Accel)
		}
		b2i := func(v bool) int {
			if v {
				return 1
			}
			return 0
		}
		fmt.Fprintf(b, " ind=%d en=%d act=%d\n", it.Indicator, b2i(it.Enabled), b2i(it.Active))
		if it.SubMenu != nil {
			dumpMenuItems(b, it.SubMenu.Items, depth+1)
		}
	}
}

func (r *uiScriptRunner) dumpHover() {
	h := &r.ss.GW.hover
	fmt.Fprintf(&r.b, "hover %x %x %d", h.Entity.V, h.Constraint.V, b2i(h.Emphasized))
	if h.Edge.A != 0 {
		fmt.Fprintf(&r.b, " %s", h.Edge)
	}
	fmt.Fprintf(&r.b, "\n")
}

func b2i(v bool) int {
	if v {
		return 1
	}
	return 0
}

func (r *uiScriptRunner) dump() {
	gw := r.ss.GW
	sk := r.ss.SK
	pd := &gw.pending
	fmt.Fprintf(&r.b, "pending %d %d \"%s\" %x %x %d %d\n", pd.operation, pd.command, pd.description,
		pd.request.V, pd.point.V, b2i(pd.hasSuggestion), pd.suggestion.Type)
	r.dumpHover()
	fmt.Fprintf(&r.b, "selection %d", len(gw.selection))
	for _, s := range gw.selection {
		fmt.Fprintf(&r.b, " %x/%x", s.Entity.V, s.Constraint.V)
		if s.Edge.A != 0 {
			fmt.Fprintf(&r.b, "/%s", strings.ReplaceAll(s.Edge.String(), " ", "-"))
		}
	}
	fmt.Fprintf(&r.b, "\n")
	fmt.Fprintf(&r.b, "active %x %x\n", gw.activeGroupH().V, gw.ActiveWorkplane().V)
	v := &gw.view
	fmt.Fprintf(&r.b, "camera %s %s %s %s\n", g17(v.Scale), v17(v.Offset), v17(v.ProjRight), v17(v.ProjUp))
	fmt.Fprintf(&r.b, "counts %d %d %d %d %d\n", len(sk.GroupOrder), len(sk.Requests), len(sk.Entities),
		len(sk.Constraints), len(sk.Params))
	g, t := r.gw(), r.tw()
	fmt.Fprintf(&r.b, "editor %d \"%s\" %d \"%s\"\n", b2i(g.EditorVisible), escLine(g.EditorText),
		b2i(t.EditorVisible), escLine(t.EditorText))
	fmt.Fprintf(&r.b, "tw %d unsaved %d cursor %d tooltip \"%s\"\n", r.ss.TW.shown.screen,
		b2i(sk.Unsaved), g.Cursor, escLine(g.TooltipText))
}

func parseButton(s string) platform.MouseButton {
	switch s {
	case "L":
		return platform.ButtonLeft
	case "M":
		return platform.ButtonMiddle
	case "R":
		return platform.ButtonRight
	}
	return platform.ButtonNone
}

func modifiers(t []string, from int) (shift, ctrl bool) {
	for _, s := range t[min(from, len(t)):] {
		shift = shift || s == "shift"
		ctrl = ctrl || s == "ctrl"
	}
	return shift, ctrl
}

func scriptFloat(s string) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		panic(err)
	}
	return v
}

func (r *uiScriptRunner) mouse(w *headless.Window, t []string) {
	var ev platform.MouseEvent
	op := strings.TrimPrefix(t[0], "tw")
	var mods int
	switch op {
	case "move":
		ev.Type = platform.MouseMotion
		ev.X, ev.Y = scriptFloat(t[1]), scriptFloat(t[2])
		if len(t) > 3 {
			ev.Button = parseButton(t[3])
		}
		mods = 3
	case "scroll":
		ev.Type = platform.MouseScrollVert
		ev.X, ev.Y = scriptFloat(t[1]), scriptFloat(t[2])
		ev.ScrollDelta = scriptFloat(t[3])
		mods = 4
	case "leave":
		ev.Type = platform.MouseLeave
		mods = 1
	default:
		switch op {
		case "down":
			ev.Type = platform.MousePress
		case "up":
			ev.Type = platform.MouseRelease
		default:
			ev.Type = platform.MouseDblPress
		}
		ev.Button = parseButton(t[1])
		ev.X, ev.Y = scriptFloat(t[2]), scriptFloat(t[3])
		mods = 4
	}
	ev.ShiftDown, ev.ControlDown = modifiers(t, mods)
	w.Mouse(ev)
}

func (r *uiScriptRunner) key(t []string) {
	ev := platform.KeyboardEvent{Key: platform.KeyCharacter}
	switch k := t[1]; {
	case k == "esc":
		ev.Chr = '\x1b'
	case k == "del":
		ev.Chr = '\x7f'
	case k == "tab":
		ev.Chr = '\t'
	case k == "bs":
		ev.Chr = '\b'
	case k == "ret":
		ev.Chr = '\r'
	case k == "space":
		ev.Chr = ' '
	case len(k) > 1 && k[0] == 'F':
		ev.Key = platform.KeyFunction
		ev.Num, _ = strconv.Atoi(k[1:])
	default:
		ev.Chr = rune(k[0])
	}
	ev.ShiftDown, ev.ControlDown = modifiers(t, 2)
	r.gw().Key(ev)
}

func (r *uiScriptRunner) triggerMenu(path string) {
	bar := r.gw().MenuBar.(*headless.MenuBar)
	it := bar.Find(strings.Split(path, "|")...)
	if it == nil {
		fmt.Fprintf(&r.b, "menu missing %s\n", path)
		return
	}
	if it.OnTrigger != nil {
		it.OnTrigger()
	}
}

func (r *uiScriptRunner) setView(name string) {
	v := &r.ss.GW.view
	if name == "default" {
		v.Scale = 5
		v.Offset = geom.Vector{}
		v.ProjRight = geom.Vector{X: 1}
		v.ProjUp = geom.Vector{Y: 1}
	} else {
		v.Scale = 3.25
		v.Offset = geom.Vector{X: -4.5, Y: 2.25, Z: 1.5}
		v.ProjRight = geom.Vector{X: 0.8, Y: -0.5, Z: 0.2}
		v.ProjUp = geom.Vector{X: 0.3, Y: 0.6, Z: 0.75}
		if name == "under" {
			// The rotated view turned over: the viewer below.
			v.ProjUp = v.ProjUp.ScaledBy(-1)
		}
		r.ss.GW.NormalizeProjectionVectors()
	}
	r.ss.GW.Invalidate(false)
}

// step runs a script line; false stops the script, as the harness exits
// on an unanswered dialog.
func (r *uiScriptRunner) step(line string) (ok bool) {
	t := strings.Fields(line)
	if len(t) == 0 {
		return true
	}
	rest := ""
	if len(line) > len(t[0])+1 {
		rest = line[len(t[0])+1:]
	}
	defer func() {
		if e := recover(); e != nil {
			msg := fmt.Sprint(e)
			switch {
			case strings.HasPrefix(msg, "headless: unexpected message dialog"):
				fmt.Fprintf(&r.b, "dialog unanswered\n")
			case strings.HasPrefix(msg, "headless: unexpected file dialog"):
				fmt.Fprintf(&r.b, "filedialog unanswered\n")
			default:
				panic(e)
			}
			ok = false
		}
	}()
	switch op := t[0]; op {
	case "load":
		loadSlvs(r.t, r.ss, filepath.Join(r.dir, t[1]))
	case "view":
		r.setView(t[1])
	case "move", "down", "up", "dbl", "scroll", "leave":
		r.mouse(r.gw(), t)
	case "twmove", "twdown", "twup", "twdbl", "twscroll", "twleave":
		r.mouse(r.tw(), t)
	case "key":
		r.key(t)
	case "menu":
		r.triggerMenu(rest)
	case "popup":
		r.popupChoices = append(r.popupChoices, rest)
	case "answer":
		resp := map[string]platform.Response{"ok": platform.ResponseOK, "yes": platform.ResponseYes,
			"no": platform.ResponseNo}[t[1]]
		if resp == platform.ResponseNone {
			resp = platform.ResponseCancel
		}
		r.p.DialogResponses = append(r.p.DialogResponses, resp)
	case "file":
		f := r.expand(t[1])
		if f == "-" {
			f = ""
		}
		r.p.FilePaths = append(r.p.FilePaths, f)
	case "edit":
		switch {
		case r.gw().EditorVisible:
			r.gw().FinishEditing(rest)
		case r.tw().EditorVisible:
			r.tw().FinishEditing(rest)
		default:
			fmt.Fprintf(&r.b, "edit no editor\n")
		}
	case "dump":
		r.dump()
	case "hover":
		r.dumpHover()
		fmt.Fprintf(&r.b, "hoverlist %d\n", len(r.ss.GW.hoverList))
		for _, h := range r.ss.GW.hoverList {
			fmt.Fprintf(&r.b, "  %d %s %s %x %x\n", h.ZIndex, g17(h.Distance), g17(h.Depth),
				h.Selection.Entity.V, h.Selection.Constraint.V)
		}
	case "hovergrid":
		n, _ := strconv.Atoi(t[1])
		for i := range n {
			for j := range n {
				ev := platform.MouseEvent{Type: platform.MouseMotion,
					X: 600.0 * (float64(j) + 0.5) / float64(n), Y: 600.0 * (float64(i) + 0.5) / float64(n)}
				r.gw().Mouse(ev)
				h := &r.ss.GW.hover
				fmt.Fprintf(&r.b, "at %s %s %x %x %d", g17(ev.X), g17(ev.Y), h.Entity.V, h.Constraint.V,
					b2i(h.Emphasized))
				if h.Edge.A != 0 {
					fmt.Fprintf(&r.b, " %s", h.Edge)
				}
				fmt.Fprintf(&r.b, "\n")
			}
		}
	case "params":
		for _, p := range r.ss.SK.Params {
			fmt.Fprintf(&r.b, "param %x %s %d %d\n", p.H.V, g17(p.Val), b2i(p.Free), b2i(p.Known))
		}
		for _, hg := range r.ss.SK.GroupOrder {
			g := r.ss.SK.GetGroup(hg)
			fmt.Fprintf(&r.b, "group %x %d %d %d %d\n", hg.V, g.Solved.How, g.Solved.Dof, b2i(g.DofCheckOk),
				b2i(g.Clean))
		}
	case "freedom":
		// What colouring by constraint state gives each entity of the
		// active group, as drawEntity asks for it.
		sk := r.ss.SK
		for i := range sk.Entities {
			e := &sk.Entities[i]
			if e.Group != r.ss.GW.activeGroupH() || e.IsNormal() || e.Type == core.EntityWorkplane {
				continue
			}
			color := "-"
			if c, ok := r.ss.freedomColor(e); ok {
				color = fmt.Sprintf("%d,%d,%d", c.Red, c.Green, c.Blue)
			}
			fmt.Fprintf(&r.b, "freedom %x %s %s\n", e.H.V, sk.EntityFreedom(e.H), color)
		}
	case "polyerror":
		g := r.ss.SK.GetGroup(r.ss.GW.activeGroupH())
		fmt.Fprintf(&r.b, "polyerror %d %s %s %s\n", g.PolyError.How, v17(g.PolyError.ErrorPoint),
			v17(g.PolyError.NotClosed.A), v17(g.PolyError.NotClosed.B))
	case "menus":
		for _, m := range r.gw().MenuBar.(*headless.MenuBar).Menus {
			dumpMenuItems(&r.b, []*headless.MenuItem{m}, 0)
		}
	case "cat":
		// A file the script had written, as $tmp and $testdata.
		data, err := os.ReadFile(r.expand(t[1]))
		if err != nil {
			fmt.Fprintf(&r.b, "cat %s: no file\n", t[1])
			break
		}
		fmt.Fprintf(&r.b, "cat %s\n%s", t[1], r.unexpand(string(data)))
	case "tw":
		var rows strings.Builder
		r.ss.TW.dumpRows(&rows)
		r.b.WriteString(r.unexpand(rows.String()))
	case "save":
		if !r.ss.SaveToFile(r.savePath) {
			fmt.Fprintf(&r.b, "save failed\n")
		} else {
			data, err := os.ReadFile(r.savePath)
			if err != nil {
				r.t.Fatal(err)
			}
			fmt.Fprintf(&r.b, "save\n%send save\n", data)
		}
	default:
		r.t.Fatalf("unknown script line: %s", line)
	}
	return true
}

// uiScripts lists the scripts in testdata/ui/scripts with a prefix.
func uiScripts(t *testing.T, prefix string) []string {
	files, err := filepath.Glob(filepath.Join("..", "..", "testdata", "ui", "scripts", prefix+"*.script"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range files {
		names = append(names, strings.TrimSuffix(filepath.Base(f), ".script"))
	}
	return names
}

// linkRelToTestdata matches a saved link group's path up to testdata,
// in a .slvs and in a .tlspr file.
var linkRelToTestdata = regexp.MustCompile(`(?m)^(Group\.impFileRel=|  link ").*/testdata/`)

// linkPathsFromTestdata makes the paths of the linked files in a save
// start at testdata. A script saves into a temporary directory, so the
// path from there to a linked fixture depends on where the repository
// and the temporary directory are; the goldens hold what it was on the
// machine that recorded them.
func linkPathsFromTestdata(s string) string {
	return linkRelToTestdata.ReplaceAllString(s, "${1}testdata/")
}

// compareUIScripts runs the scripts with a prefix and compares each
// output with its golden, testdata/ui/golden/<name>.goui, reporting the
// first differing line. The goldens of the scripts that are not ext_
// were first recorded from C++. TELESAPR_REGEN_GOLDEN=1 rewrites them
// (scripts/regen-goui.sh); review the diff before committing.
func compareUIScripts(t *testing.T, prefix string) {
	names := uiScripts(t, prefix)
	if len(names) == 0 {
		t.Fatalf("no scripts %s*", prefix)
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("..", "..", "testdata", "ui", "golden", name+".goui")
			got := linkPathsFromTestdata(runUIScript(t, name))
			if golden.Regen() {
				golden.Check(t, path, []byte(got))
				return
			}
			wantB, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("%v (record it with scripts/regen-goui.sh %s)", err, name)
			}
			if want := linkPathsFromTestdata(string(wantB)); got != want {
				t.Fatalf("output differs from the golden at %s", firstDiffLine(got, want))
			}
		})
	}
}

// firstDiffLine reports the first line where two outputs differ.
func firstDiffLine(got, want string) string {
	gl, wl := strings.Split(got, "\n"), strings.Split(want, "\n")
	for i := range max(len(gl), len(wl)) {
		var g, w string
		if i < len(gl) {
			g = gl[i]
		}
		if i < len(wl) {
			w = wl[i]
		}
		if g != w {
			return fmt.Sprintf("line %d:\n got: %q\nwant: %q", i+1, g, w)
		}
	}
	return ""
}

// TestGoScripts replays the scripts of what only TeleSAPR has (ext_*).
func TestGoScripts(t *testing.T) { compareUIScripts(t, "ext_") }

// TestMenuTable replays the main menu, and its states.
func TestMenuTable(t *testing.T) { compareUIScripts(t, "menu_") }

// TestHover replays what the mouse hovers.
func TestHover(t *testing.T) { compareUIScripts(t, "hover_") }

// TestInteraction replays mouse, keyboard, menu and dialog input, and
// what the interface shows and saves.
func TestInteraction(t *testing.T) { compareUIScripts(t, "act_") }

// TestUIScript prints the output of the script TELESAPR_UI_SCRIPT
// names.
func TestUIScript(t *testing.T) {
	name := os.Getenv("TELESAPR_UI_SCRIPT")
	if name == "" {
		t.Skip("set TELESAPR_UI_SCRIPT to a script name")
	}
	fmt.Print(runUIScript(t, name))
}
