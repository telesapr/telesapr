package ui

import (
	"bytes"
	"fmt"
	"image"
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
// frames each time.
//
// The X pointer is not in the screen's pixels, so the pointer is drawn
// into the frames. A click step glides it, over the step before's
// screen, to what it clicks, tells the widget there that the pointer is
// on it, waits for the screen to settle (it need not change), shows
// that frame with a ring, and presses. A step of keys names the key in
// the caption band's corner, as the headless videos do.

// docWindowVideoStep is one step of a video: what to do, what the
// frame it leaves says, and how long it is shown. A step has do, or at
// and button, or neither (the first, the window as it opens).
type docWindowVideoStep struct {
	caption string
	key     string // what the step types or presses, shown on its frames
	do      func(s *docWindowPlatform)
	// at is what the step clicks, asked once the step before's screen
	// has settled; false when it is not there.
	at     func(s *docWindowPlatform) (tkgui.Spot, bool)
	button uint    // at's button: 1 is the left one, 3 the right
	hold   float64 // seconds
}

// docWindowVideo is a video of the real window, a step after another.
type docWindowVideo struct {
	name    string
	pointer image.Point // where the pointer rests as the video starts
	setup   func(s *docWindowPlatform)
	steps   []docWindowVideoStep
}

// hideSketches hides the groups between the references and the active
// one, as their "show entities from this group" boxes do. Their lines
// lie along the solid's edges, and a click there takes a line before an
// edge.
func hideSketches(s *docWindowPlatform) {
	for _, h := range s.app.SK.GroupOrder[1:] {
		if h != s.app.GW.activeGroupH() && s.app.SK.Group(h).Visible {
			s.app.TW.ScreenToggleGroupShown(0, h.V)
		}
	}
}

// showSketches shows again what hideSketches hid.
func showSketches(s *docWindowPlatform) {
	for _, h := range s.app.SK.GroupOrder[1:] {
		if !s.app.SK.Group(h).Visible {
			s.app.TW.ScreenToggleGroupShown(0, h.V)
		}
	}
}

// formWidget is a step's at for a widget of the open form dialog.
func formWidget(names ...string) func(s *docWindowPlatform) (tkgui.Spot, bool) {
	return func(s *docWindowPlatform) (tkgui.Spot, bool) { return s.FormSpot(names...) }
}

// docWindowVideos are the videos, in the order made. Each starts from
// the window as the one before left it, so one that changes the model
// goes after those that don't.
var docWindowVideos = []docWindowVideo{
	{name: "export-mesh", pointer: image.Pt(720, 470), steps: []docWindowVideoStep{
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
		{caption: "Triangle Mesh asks how fine the mesh is: quality, chord tolerance, segments.", key: "M", hold: 4,
			do: func(s *docWindowPlatform) {
				// The dialog is modal: the key returns once the last
				// step has closed it.
				bar, _ := s.MainMenu()
				bar.(*tkgui.MenuBar).ChooseInSubMenu(0, "Export", 'm')
			}},
		// The third choice of the first field, Quality.
		{caption: "Set Quality to Fine. The tolerances and the segments fill in.", hold: 3.5,
			at: formWidget("row0", "c2"), button: 1},
		// The fifth field, the strict check box.
		{caption: "Tick the box to refuse a mesh with holes or crossing triangles.", hold: 3,
			at: formWidget("row4", "check"), button: 1},
		{caption: "Export... would ask where to save the file. Cancel writes nothing.", hold: 2.6,
			at: formWidget("b1"), button: 1},
	}},
	{name: "context-menu", pointer: image.Pt(640, 330), setup: hideSketches, steps: []docWindowVideoStep{
		{caption: "A rounded block. The lines of its sketch and extrusion are hidden.", hold: 3},
		{caption: "Right-click an edge. The menu lists what applies to it.", key: "Right click", hold: 3.5,
			at: func(s *docWindowPlatform) (tkgui.Spot, bool) {
				// The middle of the front face's top edge.
				return s.windows[0].Spot(472, 203), true
			}, button: 3},
		{caption: "Choose Fillet Edges. The edge is rounded, with those it runs on into.", hold: 4.5,
			at:     func(s *docWindowPlatform) (tkgui.Spot, bool) { return s.menuEntry("Fillet Edges") },
			button: 1},
	}},
}

// docWindowRecorder records docWindowVideos, a look at the screen at a
// time.
type docWindowRecorder struct {
	t    *testing.T
	dir  string
	s    *docWindowPlatform
	p    *tkgui.Platform
	rect image.Rectangle

	vi, si int // the video and the step being waited for
	v      docVideo
	screen *image.RGBA // the screen of the step before's frame
	// shown is what the screen showed before the step's action, which
	// must change it; nil when it need not.
	shown   []byte
	prev    []byte // the last look's screen
	same    int    // looks in a row with the same screen
	stalled int    // looks since the action with the screen unchanged
	by      time.Time

	pointer image.Point
	spot    tkgui.Spot // where the pointer last clicked
	// hovering says the pointer has been put on a click step's spot, and
	// the press waits for the screen to settle.
	hovering bool
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
	r := &docWindowRecorder{t: t, dir: dir, s: s, p: p, rect: p.ScreenBounds()}
	r.startVideo()
	r.wait()
	p.App().After(settlePoll, r.look)
	p.Run()
	if r.vi != len(docWindowVideos) {
		t.Errorf("%d of %d videos made", r.vi, len(docWindowVideos))
	}
}

func (r *docWindowRecorder) fail(err error) {
	r.t.Errorf("%s step %d: %v", docWindowVideos[r.vi].name, r.si, err)
	r.p.Exit()
}

func (r *docWindowRecorder) step() docWindowVideoStep { return docWindowVideos[r.vi].steps[r.si] }

func (r *docWindowRecorder) startVideo() {
	frames, err := os.MkdirTemp("", "telesapr-video-")
	if err != nil {
		r.t.Fatal(err)
	}
	video := docWindowVideos[r.vi]
	r.v = docVideo{dir: r.dir, name: video.name, frames: frames}
	r.shown, r.pointer = nil, video.pointer
	if video.setup != nil {
		video.setup(r.s)
	}
}

// wait starts the wait for the screen to settle.
func (r *docWindowRecorder) wait() {
	r.stalled, r.same, r.prev, r.by = 0, 0, nil, time.Now().Add(settleMost)
}

// act does the step's action from the event loop. It may open a popup
// menu or a modal dialog, whose own loop runs the looks until a later
// step closes it.
func (r *docWindowRecorder) act() {
	step, spot := r.step(), r.spot
	switch {
	case step.at != nil:
		r.p.App().After(0, func() { spot.Press(step.button) })
	case step.do != nil:
		r.p.App().After(0, func() { step.do(r.s) })
	}
}

// look reads the screen and, once it has settled, goes on.
func (r *docWindowRecorder) look() {
	img := r.p.ReadScreen(r.rect)
	if img == nil {
		r.fail(fmt.Errorf("the screen can't be read"))
		return
	}
	switch {
	case r.shown != nil && bytes.Equal(img.Pix, r.shown):
		// The step has not changed the screen yet. Past settleRetry
		// looks its action is taken to have been lost (a key the busy
		// server dropped) and is done again.
		r.same = 0
		if r.stalled++; r.stalled%settleRetry == 0 {
			r.act()
		}
	case r.prev != nil && bytes.Equal(img.Pix, r.prev):
		r.same++
	default:
		r.same = 0
	}
	r.prev = img.Pix
	if r.same < settleSame {
		if time.Now().After(r.by) {
			r.fail(fmt.Errorf("the screen did not settle in %v", settleMost))
			return
		}
		r.p.App().After(settlePoll, r.look)
		return
	}
	if r.hovering {
		r.press(img)
		return
	}
	r.take(img)
}

// press adds the frame of the pointer on what it clicks, ringed, then
// clicks.
func (r *docWindowRecorder) press(img *image.RGBA) {
	if err := r.frame(img, videoPress, true); err != nil {
		r.fail(err)
		return
	}
	r.hovering, r.shown = false, img.Pix
	r.wait()
	r.p.App().After(settlePoll, r.look)
	r.act()
}

// take adds the step's frame, img, and starts the next step.
func (r *docWindowRecorder) take(img *image.RGBA) {
	if err := r.frame(img, r.step().hold, false); err != nil {
		r.fail(err)
		return
	}
	r.shown, r.screen = img.Pix, img
	if r.si++; r.si == len(docWindowVideos[r.vi].steps) {
		if err := r.v.finish(); err != nil {
			r.t.Error(err)
			r.p.Exit()
			return
		}
		if r.vi++; r.vi == len(docWindowVideos) {
			// Let what the last step closed go before quitting.
			r.p.App().After(10*settlePoll, r.p.Exit)
			return
		}
		r.si = 0
		r.startVideo()
	}
	r.wait()
	r.p.App().After(settlePoll, r.look)
	if r.step().at == nil {
		r.act()
		return
	}
	if err := r.aim(); err != nil {
		r.fail(err)
	}
}

// aim glides the pointer, over the screen of the step before, to what a
// click step clicks, and puts it there.
func (r *docWindowRecorder) aim() error {
	step := r.step()
	spot, ok := step.at(r.s)
	if !ok {
		return fmt.Errorf("what it clicks is not on the screen")
	}
	to := spot.Screen().Sub(r.rect.Min)
	r.v.caption = step.caption
	base := r.v.withCaption(r.screen)
	for _, p := range glidePoints(r.pointer, to) {
		img := cloneRGBA(base)
		drawCursor(img, p.X, p.Y)
		if err := r.v.add(img, videoFrame); err != nil {
			return err
		}
	}
	r.spot.Leave()
	r.spot, r.pointer, r.hovering, r.shown = spot, to, true, nil
	spot.Hover()
	return nil
}

// frame adds screen as a frame of the step, shown for dur seconds, with
// the pointer, ringed where it is about to press.
func (r *docWindowRecorder) frame(screen *image.RGBA, dur float64, press bool) error {
	r.v.caption, r.v.key = r.step().caption, r.step().key
	img := r.v.withCaption(screen)
	if press {
		drawRing(img, r.pointer, 14)
	}
	drawCursor(img, r.pointer.X, r.pointer.Y)
	r.v.keyBadge(img)
	return r.v.add(img, dur)
}
