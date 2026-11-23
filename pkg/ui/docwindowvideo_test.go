package ui

import (
	"bytes"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/telesapr/telesapr/pkg/platform/tkgui"
)

// The guide's videos of the real window, the ones that show a menu or
// a dialog, which the headless scenarios' frames don't draw. They are
// made only when TELESAPR_DOC_WINDOW_VIDEOS names a directory
// (scripts/regen-doc-window-images.sh videos), under the Xvfb that
// script starts.
//
// The screen is not filmed as it runs. Each step is done, the screen
// is waited for until it has stayed the same for settleSame looks and
// differs from the step before's (a busy machine opens a menu late,
// or drops a key: the step is then done again), and that one frame is
// held for the step's time. How slowly the machine ran shows in how
// long the test takes, not in the video, which is made of the same
// frames each time. The pointer is not on the screen's pixels and the
// steps are keys, so each frame names the key in the caption band's
// corner, as the headless videos do.

// docWindowVideoStep is one step of a video: what to do, what the
// frame it leaves says, and how long it is shown.
type docWindowVideoStep struct {
	caption string
	key     string // the key the step types, shown on its frame
	do      func(s *docWindowPlatform)
	hold    float64 // seconds
}

// docWindowVideo is a video of the real window, a step after another.
// The first step has no action: it is the window as it opens.
type docWindowVideo struct {
	name  string
	steps []docWindowVideoStep
}

// docWindowVideos are the videos, in the order made. Each ends with
// the window as it began, so that the next starts from it.
var docWindowVideos = []docWindowVideo{
	{name: "export-mesh", steps: []docWindowVideoStep{
		{caption: "A rounded block, ready to export.", hold: 2.4},
		{caption: "Open the File menu.", key: "Alt+F", hold: 2.2,
			do: func(s *docWindowPlatform) {
				bar, _ := s.MainMenu()
				bar.(*tkgui.MenuBar).OpenMenu(0)
			}},
		{caption: "Export has an entry for each kind of file.", key: "E", hold: 2.6,
			do: func(s *docWindowPlatform) {
				bar, _ := s.MainMenu()
				bar.(*tkgui.MenuBar).OpenSubMenu(0, "Export")
			}},
		{caption: "Triangle Mesh asks how fine the mesh is: quality, chord tolerance, segments.", key: "M", hold: 4.5,
			do: func(s *docWindowPlatform) {
				// The dialog is modal: the key returns once the next
				// step has closed it.
				bar, _ := s.MainMenu()
				bar.(*tkgui.MenuBar).ChooseInSubMenu(0, "Export", 'm')
			}},
		{caption: "Cancel writes nothing; Export... asks where to save the file.", key: "Esc", hold: 2.6,
			do: func(s *docWindowPlatform) { s.CloseDialogs() }},
	}},
}

// TestDocWindowVideos records docWindowVideos from the real window.
func TestDocWindowVideos(t *testing.T) {
	dir := os.Getenv("TELESAPR_DOC_WINDOW_VIDEOS")
	if dir == "" || os.Getenv("DISPLAY") == "" {
		t.Skip("set TELESAPR_DOC_WINDOW_VIDEOS and DISPLAY (scripts/regen-doc-window-images.sh videos)")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Fatal("ffmpeg is needed")
	}
	dir = mustAbs(dir)
	s, p := newDocWindow(t)
	rect := p.ScreenBounds()

	vi, si := 0, 0 // the step being waited for
	var v docVideo
	var last, prev []byte // the frame before's screen, and the last look's
	same, stalled := 0, 0
	deadline := time.Now().Add(settleMost)
	next := func() {
		stalled, same, prev, deadline = 0, 0, nil, time.Now().Add(settleMost)
	}
	startVideo := func() {
		frames, err := os.MkdirTemp("", "telesapr-video-")
		if err != nil {
			t.Fatal(err)
		}
		v = docVideo{dir: dir, name: docWindowVideos[vi].name, frames: frames}
		last = nil
	}
	fail := func(format string, args ...any) {
		t.Errorf(format, args...)
		p.Exit()
	}
	var look func()
	look = func() {
		img := p.ReadScreen(rect)
		if img == nil {
			fail("the screen can't be read")
			return
		}
		step := docWindowVideos[vi].steps[si]
		switch {
		case last != nil && bytes.Equal(img.Pix, last):
			// The step has not changed the screen yet. Past
			// settleRetry looks its action is taken to have been lost
			// (a key the busy server dropped) and is done again.
			same = 0
			if stalled++; stalled%settleRetry == 0 && step.do != nil {
				p.App().After(0, func() { step.do(s) })
			}
		case prev != nil && bytes.Equal(img.Pix, prev):
			same++
		default:
			same = 0
		}
		prev = img.Pix
		if same < settleSame {
			if time.Now().After(deadline) {
				fail("%s step %d: the screen did not settle in %v", docWindowVideos[vi].name, si, settleMost)
				return
			}
			p.App().After(settlePoll, look)
			return
		}
		v.caption, v.key = step.caption, step.key
		frame := v.withCaption(img)
		v.keyBadge(frame)
		if err := v.add(frame, step.hold); err != nil {
			fail("%v", err)
			return
		}
		last = img.Pix
		if si++; si == len(docWindowVideos[vi].steps) {
			if err := v.finish(); err != nil {
				fail("%v", err)
				return
			}
			if vi++; vi == len(docWindowVideos) {
				// Let what the last step closed go before quitting.
				p.App().After(10*settlePoll, p.Exit)
				return
			}
			si = 0
			startVideo()
		}
		next()
		p.App().After(settlePoll, look)
		// A step's action may open a modal dialog, whose loop runs the
		// looks until the next step closes it.
		if do := docWindowVideos[vi].steps[si].do; do != nil {
			p.App().After(0, func() { do(s) })
		}
	}
	startVideo()
	p.App().After(settlePoll, look)
	p.Run()
	if vi != len(docWindowVideos) {
		t.Errorf("%d of %d videos made", vi, len(docWindowVideos))
	}
}
