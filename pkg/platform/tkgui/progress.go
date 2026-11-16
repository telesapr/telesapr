package tkgui

import (
	"fmt"
	"time"

	"github.com/takigo/takigo/busy"
	"github.com/takigo/takigo/cursor"
	"github.com/takigo/takigo/event"
	"github.com/takigo/takigo/geometry/pack"
	"github.com/takigo/takigo/option"
	"github.com/takigo/takigo/screenunit"
	"github.com/takigo/takigo/ttk"
	"github.com/takigo/takigo/widget"
	"github.com/takigo/takigo/widget/toplevel"
	"github.com/takigo/takigo/window"

	"github.com/telesapr/telesapr/pkg/platform"
)

// A file is opened and exported on the loop's goroutine, which then
// draws nothing: the dialog that asked for the file stays painted over
// the main window and the program looks hung. The progress window lets
// the loop run a moment now and then, in a nested loop as a modal
// dialog does, so that the windows repaint. Only that may run there:
// the application's timers and the work posted to the loop, which would
// come into the half-loaded model, wait until the window closes
// (Platform.runOrHold), and so do its renders once the work has begun;
// the window's grab keeps the user's input from the other windows.

var _ platform.ProgressShower = (*Platform)(nil)

const (
	// progressChars is how many characters of the label's font a line
	// of the window holds, and progressLines how many lines its text
	// has room for: the window's size is fixed, so that it doesn't jump
	// with the length of its text, and made from the font, which a
	// desktop may make large.
	progressChars = 56
	progressLines = 2
	// progressPad is the margin round the text and the bar.
	progressPad = 14
	// progressEvery is how often an Update repaints: seldom enough that
	// the nested loops cost little of the work.
	progressEvery = 100 * time.Millisecond
	// progressSpell is how long one repaint runs the loop.
	progressSpell = 5 * time.Millisecond
	// progressMapWait bounds how long showing the window waits for the
	// window manager to map it, and progressSettle is how long the loop
	// then runs for it to draw: the work may not update it for a while.
	progressMapWait = 300 * time.Millisecond
	progressSettle  = 30 * time.Millisecond
)

// progressWindow is a Progress: a toplevel over the parent with a line
// of text over a bar.
type progressWindow struct {
	p      *Platform
	top    *toplevel.Toplevel
	text   *ttk.Label
	bar    *ttk.Progressbar
	covers []*busy.BusyWin
	mapped bool
	closed bool
	// working is set once ShowProgress has returned and the work has
	// begun: before, the model is whole and a render may run, as the
	// exposure of what the dialog that closed covered needs with GL,
	// which keeps no frame to copy back.
	working bool
	// What the last repaint showed, and when it was.
	shownText string
	shownFrac float64
	last      time.Time
	// release gives back the grab and the focus the window took.
	release func()
}

// ShowProgress shows a progress window over parent and lets the loop
// run until it is on screen, so that what was over parent before, the
// dialog that chose the file, is gone from it.
func (p *Platform) ShowProgress(parent platform.Window, title string) platform.Progress {
	if p.progress != nil {
		// One at a time: the one up shows the inner operation's
		// progress too, and goes when the outer one ends.
		return nestedProgress{p.progress}
	}
	pw := &progressWindow{p: p, shownFrac: -2}
	opts := []toplevel.ToplevelOption{toplevel.Title(prepareTitle(title))}
	var parentWin *window.Window
	if tw, ok := parent.(*Window); ok && tw != nil {
		parentWin = tw.topWin
		opts = append(opts, toplevel.TransientFor(parentWin))
	}
	pw.top = toplevel.New(p.app, p.newName("progress"), opts...)
	tw := pw.top.Window()
	setWindowIcon(tw)
	// Closing it would not stop the work, so the window manager's close
	// button does nothing.
	pw.top.WmInfo.OnDeleteWindow(func() {})

	body := ttk.NewFrame(pw.top, "body", ttk.FramePadding(ttk.Padding{
		Left: progressPad, Top: progressPad, Right: progressPad, Bottom: progressPad}))
	_ = pack.Pack(body, pack.FillOpt(pack.FillBoth), pack.Expand(true))
	pw.text = ttk.NewLabel(body, "text", ttk.LabelText(title), ttk.LabelAnchor(option.AnchorNW),
		ttk.LabelJustify(option.JustifyLeft))
	inner, textHeight := progressTextSize(pw.text)
	_ = pw.text.Configure(ttk.LabelWrapLength(screenunit.Px(float64(inner))))
	pw.bar = ttk.NewProgressbar(body, "bar", ttk.ProgressbarLength(screenunit.Px(float64(inner))))
	// The bar at the foot and the text over it: a second line of text
	// doesn't move the bar.
	_ = pack.Pack(pw.bar, pack.SideOpt(pack.Bottom), pack.FillOpt(pack.FillX), pack.PadYPair(10, 0))
	_ = pack.Pack(pw.text, pack.SideOpt(pack.Top), pack.FillOpt(pack.FillBoth), pack.Expand(true))
	width := inner + 2*progressPad
	height := textHeight + 10 + pw.bar.Window().ReqHeight + 2*progressPad
	_ = pw.top.WmInfo.SetGeometry(fmt.Sprintf("%dx%d", width, height))
	pw.top.WmInfo.SetResizable(false, false)
	if parentWin != nil {
		centerOver(tw, parentWin, width, height)
	}

	p.app.Dispatcher().Bind(tw.PlatformID, event.StructureNotifyMask, func(ev *event.Event) {
		if ev.Type == event.MapType {
			pw.mapped = true
		}
	})
	pw.top.Show()
	p.progress = pw

	// As a modal dialog does: a local grab, so that the other windows
	// take no input, and the focus, so that no keys reach them either.
	gm := p.app.GrabManager()
	prevGrab := gm.Current()
	gm.Set(tw, false)
	oldFocus := widget.FocusWindow(p.app)
	widget.Focus(p.app, tw)
	// The watch over every window of the program, which says why none
	// answers, and over this one.
	for _, w := range p.toplevels() {
		if w != tw {
			pw.covers = append(pw.covers, busy.Hold(p.app, w))
		}
	}
	p.app.Server().SetCursorShape(tw.PlatformID, cursor.Watch)
	pw.release = func() {
		gm.Release()
		if prevGrab != nil && !prevGrab.IsDestroyed() {
			gm.Set(prevGrab, false)
		}
		if oldFocus != nil && !oldFocus.IsDestroyed() {
			widget.Focus(p.app, oldFocus)
		}
	}

	// The GL windows render once more, with the model still whole, so
	// that they have a frame to repaint from during the work.
	for _, w := range p.windows {
		if w.canvas.glr != nil && w.visible {
			w.canvas.invalidate()
		}
	}
	pw.waitMapped()
	pw.working = true
	pw.last = time.Now()
	return pw
}

// Update shows text and fraction, and lets the windows repaint, every
// progressEvery.
func (pw *progressWindow) Update(text string, fraction float64) {
	if pw.closed || time.Since(pw.last) < progressEvery {
		return
	}
	pw.last = time.Now()
	if text != pw.shownText {
		pw.shownText = text
		pw.text.SetText(text)
	}
	switch {
	case fraction < 0:
		if pw.bar.Mode != ttk.ProgressIndeterminate {
			pw.bar.Mode = ttk.ProgressIndeterminate
			pw.bar.Value = 0
		}
		pw.bar.Step(5)
	case fraction != pw.shownFrac:
		pw.bar.Mode = ttk.ProgressDeterminate
		pw.bar.SetValue(100 * min(fraction, 1))
	}
	pw.shownFrac = fraction
	pw.pump()
}

// Close takes the window away, gives back the grab and the focus, and
// has what was held run once the operation has returned to the loop.
// What the window covered is repainted from the frame on screen first:
// what was held may well regenerate, and the next frame wait for it.
func (pw *progressWindow) Close() {
	if pw.closed {
		return
	}
	pw.closed = true
	p := pw.p
	pw.release()
	for _, c := range pw.covers {
		c.Release()
	}
	pw.top.Hide()
	pw.top.Destroy()
	pw.runLoop(progressSettle)
	p.progress = nil
	for _, w := range p.windows {
		if c := w.canvas; c.glr != nil {
			// The copy is stale; the next exposure renders anew.
			c.frame, c.shown = nil, nil
		}
	}
	held := p.held
	p.held = nil
	for _, fn := range held {
		p.app.RunOnMain(fn)
	}
}

// waitMapped runs the loop until the window manager has mapped the
// window, for at most progressMapWait, and once more for it to draw.
func (pw *progressWindow) waitMapped() {
	for start := time.Now(); !pw.mapped && time.Since(start) < progressMapWait; {
		pw.pump()
	}
	pw.runLoop(progressSettle)
}

// pump runs the loop for progressSpell.
func (pw *progressWindow) pump() { pw.runLoop(progressSpell) }

// runLoop runs the loop for d: the windows' exposures and the widgets'
// redraws, and the input the grab drops.
func (pw *progressWindow) runLoop(d time.Duration) {
	done := make(chan struct{})
	pw.p.app.After(d, func() { close(done) })
	pw.p.app.RunNestedLoop(done)
}

// progressAside hides the progress window, if one is up, while a dialog
// the work asks (a missing linked file, a warning) runs in its place,
// and returns what shows it again.
func (p *Platform) progressAside() (back func()) {
	pw := p.progress
	if pw == nil || pw.closed {
		return func() {}
	}
	pw.top.Hide()
	pw.mapped = false
	return func() {
		if pw.closed {
			return
		}
		pw.top.Show()
		pw.p.app.Server().RaiseWindow(pw.top.Window().PlatformID)
		pw.waitMapped()
	}
}

// nestedProgress is the Progress of an operation that runs inside one
// whose window is up: it shows there, and the outer one closes it.
type nestedProgress struct{ outer *progressWindow }

func (n nestedProgress) Update(text string, fraction float64) { n.outer.Update(text, fraction) }
func (nestedProgress) Close()                                 {}

// runOrHold runs fn now, or, while a progress window is up, once it
// has closed: fn is what comes into the application from the loop (a
// timer, work posted to it, a render), which must not see an operation
// half done.
func (p *Platform) runOrHold(fn func()) {
	if p.progress != nil {
		p.held = append(p.held, fn)
		return
	}
	fn()
}

// progressTextSize is the width of the progress window's text, the
// bar's with it, and the height of its lines, in the label's font.
func progressTextSize(l *ttk.Label) (width, height int) {
	const sample = "abcdefghijklmnopqrstuvwxyz"
	f := l.GetFont()
	if f == nil {
		return 420, 2 * 16
	}
	width = progressChars * f.MeasureString(sample) / len(sample)
	return width, progressLines * f.Metrics().Linespace()
}

// toplevels are the program's toplevel windows on screen: the main
// window, which is takigo's root, and the others, its children.
func (p *Platform) toplevels() []*window.Window {
	root := p.app.Window()
	out := []*window.Window{root}
	for _, c := range root.Children {
		if c.IsTopLevel() && c.IsMapped() {
			out = append(out, c)
		}
	}
	return out
}

// centerOver places the toplevel win of width by height over parent,
// on the screen.
func centerOver(win, parent *window.Window, width, height int) {
	x := parent.X + (parent.Width-width)/2
	y := parent.Y + (parent.Height-height)/2
	d := parent.Display.Server
	sw, sh := d.ScreenWidth(parent.Display.Screen), d.ScreenHeight(parent.Display.Screen)
	x = max(0, min(x, sw-width))
	y = max(0, min(y, sh-height))
	win.X, win.Y = x, y
	d.MoveWindow(win.PlatformID, x, y)
}
