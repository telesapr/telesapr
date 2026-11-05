package ui

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/platform/tkgui"
)

// The guide's pictures of what the headless platform draws none of, the
// menus, popup menus and dialogs, which are the X server's own
// toplevels, are taken from the real window under Xvfb, read back from
// the screen. They are made only when TELESAPR_DOC_WINDOW_SHOTS names a
// directory (scripts/regen-doc-window-images.sh, which starts an Xvfb
// of its own), never by make test: its Xvfb is shared, and the
// pictures depend on the screen and fonts that script gives them.

// docWindowShot is one picture of the real window: what to do before
// it, run once the picture before is taken, and what to do after it.
type docWindowShot struct {
	name   string
	before func(s *docWindowPlatform)
	after  func(s *docWindowPlatform)
}

// docWindowPlatform is the window's platform, keeping the windows the
// application makes so that a shot can click in them.
type docWindowPlatform struct {
	*tkgui.Platform
	app     *Application
	windows []*tkgui.Window
}

func (s *docWindowPlatform) CreateWindow(kind platform.WindowKind, parent platform.Window) platform.Window {
	w := s.Platform.CreateWindow(kind, parent)
	s.windows = append(s.windows, w.(*tkgui.Window))
	return w
}

// A shot is taken once the screen has stayed the same for settleSame
// looks settlePoll apart, and fails when that takes over settleMost.
const (
	settlePoll = 100 * time.Millisecond
	settleSame = 5
	settleMost = 60 * time.Second
)

// docWindowShots are the pictures, in the order taken.
var docWindowShots = []docWindowShot{
	{name: "interface-7-file-menu",
		before: func(s *docWindowPlatform) {
			bar, _ := s.MainMenu()
			bar.(*tkgui.MenuBar).OpenMenu(0)
		},
		after: func(s *docWindowPlatform) { s.CloseMenus() }},
	{name: "interface-8-context-menu",
		// A right click on the block's top back edge, which opens the
		// menu of what applies to an edge.
		before: func(s *docWindowPlatform) { s.windows[0].Click(400, 150, 3) },
		after:  func(s *docWindowPlatform) { s.CloseMenus() }},
	{name: "export-import-1-mesh-dialog",
		// The dialog is modal: the command returns once after has
		// closed it.
		before: func(s *docWindowPlatform) {
			s.app.GW.ClearSelection()
			_ = s.app.RunCommand("File > Export > Triangle Mesh...")
		},
		after: func(s *docWindowPlatform) { s.CloseDialogs() }},
	{name: "export-import-2-export-menu",
		// File > Export open, typed as Alt+F, E would: the cascade's
		// mnemonic once the File menu has its first entry active, which
		// OpenMenu does keyPostTimeout (300 ms) after posting it.
		before: func(s *docWindowPlatform) {
			bar, _ := s.MainMenu()
			mb := bar.(*tkgui.MenuBar)
			mb.OpenMenu(0)
			s.App().After(docSubMenuWait, func() { mb.OpenSubMenu(0, "Export") })
		},
		after: func(s *docWindowPlatform) { s.CloseMenus() }},
	{name: "fillet-7-selection-message",
		// Fillet Edges with nothing selected says what it takes.
		before: func(s *docWindowPlatform) {
			s.app.GW.ClearSelection()
			_ = s.app.RunCommand("Features > Fillet Edges")
		},
		after: func(s *docWindowPlatform) { s.CloseDialogs() }},
	{name: "threads-11-selection-message",
		before: func(s *docWindowPlatform) {
			s.app.GW.ClearSelection()
			_ = s.app.RunCommand("Features > Screw Thread...")
		},
		after: func(s *docWindowPlatform) { s.CloseDialogs() }},
}

// docSubMenuWait is how long after opening a menu its cascade is
// opened: past the 300 ms in which the menu's first entry is made
// active, and short of the half second the screen must stay the same
// for the shot to be taken.
const docSubMenuWait = 400 * time.Millisecond

// The windows are placed side by side, since no window manager places
// them under Xvfb: the graphics window docWindowGW wide, the property
// browser docWindowTW, both docWindowH high.
const docWindowGW, docWindowTW, docWindowH = 800, 440, 600

// docWindowRepo is the repository, fixed before the test changes
// directory.
var docWindowRepo = mustAbs(filepath.Join("..", ".."))

func TestDocWindowShots(t *testing.T) {
	dir := os.Getenv("TELESAPR_DOC_WINDOW_SHOTS")
	if dir == "" || os.Getenv("DISPLAY") == "" {
		t.Skip("set TELESAPR_DOC_WINDOW_SHOTS and DISPLAY (scripts/regen-doc-window-images.sh)")
	}
	dir = mustAbs(dir)
	t.Chdir(t.TempDir())
	p, err := tkgui.New("")
	if err != nil {
		t.Fatal(err)
	}
	for key, g := range map[string][4]uint32{
		"GraphicsWindow": {0, 0, docWindowGW, docWindowH},
		"TextWindow":     {docWindowGW, 0, docWindowTW, docWindowH},
	} {
		p.Settings().FreezeInt(key+"_Left", g[0])
		p.Settings().FreezeInt(key+"_Top", g[1])
		p.Settings().FreezeInt(key+"_Width", g[2])
		p.Settings().FreezeInt(key+"_Height", g[3])
	}
	s := &docWindowPlatform{Platform: p}
	app := New(s)
	s.app = app
	app.BackgroundShells = false
	app.RendererOverride = "software"
	app.Init()
	// The model of interface.md's pictures: examples/scripts/fillet, in
	// the nearest isometric view.
	app.Console().RunFile(filepath.Join(docWindowRepo, "examples", "scripts", "fillet", "fillet.go"))
	for _, c := range []string{"View > Nearest Isometric View", "View > Zoom To Fit"} {
		if err := app.RunCommand(c); err != nil {
			t.Fatal(err)
		}
	}

	i := -1 // the shot being waited for; -1 is the model drawn
	var prev []byte
	same := 0
	deadline := time.Now().Add(settleMost)
	var look func()
	look = func() {
		img := p.ReadScreen(p.ScreenBounds())
		if img == nil {
			t.Error("the screen can't be read")
			p.Exit()
			return
		}
		if prev != nil && bytes.Equal(img.Pix, prev) {
			same++
		} else {
			same = 0
		}
		prev = img.Pix
		if same < settleSame {
			if time.Now().After(deadline) {
				t.Errorf("shot %d: the screen did not settle in %v", i, settleMost)
				p.Exit()
				return
			}
			p.App().After(settlePoll, look)
			return
		}
		if i >= 0 {
			sh := docWindowShots[i]
			if err := writeDocPng(img, filepath.Join(dir, sh.name+".png")); err != nil {
				t.Error(err)
			}
			if sh.after != nil {
				sh.after(s)
			}
		}
		i++
		if i == len(docWindowShots) {
			// Let what after closed go before quitting.
			p.App().After(10*settlePoll, p.Exit)
			return
		}
		prev, same, deadline = nil, 0, time.Now().Add(settleMost)
		p.App().After(settlePoll, look)
		// A shot's action may open a modal dialog, whose loop runs the
		// looks until the picture is taken and after closes it.
		if b := docWindowShots[i].before; b != nil {
			p.App().After(0, func() { b(s) })
		}
	}
	p.App().After(settlePoll, look)
	p.Run()
	if i != len(docWindowShots) {
		t.Errorf("%d of %d shots taken", max(i, 0), len(docWindowShots))
	}
}
