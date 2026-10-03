package tkgui

import (
	"strings"

	"github.com/msorc/takigo/bind"
	"github.com/msorc/takigo/geometry/pack"
	"github.com/msorc/takigo/option"
	tkplatform "github.com/msorc/takigo/platform"
	"github.com/msorc/takigo/widget"
	"github.com/msorc/takigo/widget/frame"
	"github.com/msorc/takigo/widget/label"
	"github.com/msorc/takigo/widget/scrollbar"
	"github.com/msorc/takigo/widget/text"
	"github.com/msorc/takigo/widget/toplevel"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/platform"
)

// This file is the window scripts are typed in, which C++ SolveSpace
// doesn't have, and the way into the event loop for the goroutines
// scripts run on.

var (
	_ platform.LoopPoster  = (*Platform)(nil)
	_ platform.ConsoleHost = (*Platform)(nil)
	_ platform.Console     = (*Console)(nil)
)

// PostToLoop has fn run on the event loop's goroutine; it may be
// called from any goroutine.
func (p *Platform) PostToLoop(fn func()) { p.app.RunOnMain(fn) }

// LoopDone is closed once the event loop was told to stop.
func (p *Platform) LoopDone() <-chan struct{} { return p.app.Done() }

// Console is a window with what the scripts printed, which can be
// scrolled and copied from, over a box where the next lines are typed.
// Return enters them and Shift+Return starts another line; Up and Down
// bring back what was entered before; Control+C or Escape interrupts.
type Console struct {
	p      *Platform
	events platform.ConsoleEvents

	top *toplevel.Toplevel
	// frame is the toplevel as a Window, for saving and restoring its
	// place as the other windows do.
	frame Window

	log    *text.TextWidget
	scroll *scrollbar.Scrollbar
	prompt *label.Label
	input  *text.TextWidget

	busy       bool
	promptText string

	// history is what was entered, oldest first; at is where Up and
	// Down are in it, len(history) when at what is being typed, which
	// draft keeps.
	history []string
	at      int
	draft   string
}

// The colours of the console: dark, like the other windows.
const (
	consoleBg      = "#000000"
	consoleInputBg = "#101018"
	consoleFg      = "#d0d0d0"
	consoleFont    = "{monospace} -13"
)

// consoleTags are the tags of the kinds of text, and their colours.
var consoleTags = map[platform.ConsoleKind]struct{ tag, color string }{
	platform.ConsoleOutput: {"output", "#d0d0d0"},
	platform.ConsoleValue:  {"value", "#70d0ff"},
	platform.ConsoleError:  {"error", "#ff6060"},
	platform.ConsoleInfo:   {"info", "#909090"},
	platform.ConsoleInput:  {"input", "#80e080"},
}

// CreateConsole makes a console, hidden, that stays over parent.
func (p *Platform) CreateConsole(parent platform.Window) platform.Console {
	c := &Console{p: p}
	opts := []toplevel.ToplevelOption{toplevel.Title("Script Console"), toplevel.Background(consoleBg)}
	if pw, ok := parent.(*Window); ok && pw != nil {
		opts = append(opts, toplevel.TransientFor(pw.topWin))
	}
	c.top = toplevel.New(p.app, p.newName("console"), opts...)
	c.top.Hide()
	c.frame = Window{p: p, top: c.top, topWin: c.top.Window(), wmInfo: c.top.WmInfo, toolTop: c.top}
	setWindowIcon(c.frame.topWin)
	p.app.RegisterCloseHandler(c.frame.topWin.PlatformID, func() {
		if c.events.OnClose != nil {
			c.events.OnClose()
		}
	})

	// The line being typed, at the bottom, after its prompt.
	bottom := frame.New(c.top, p.newName("entry"), frame.Background(consoleInputBg))
	_ = pack.Pack(bottom, pack.SideOpt(pack.Bottom), pack.FillOpt(pack.FillX))
	c.prompt = label.New(bottom, p.newName("prompt"), label.Text(""),
		label.Background(consoleInputBg), label.Foreground(consoleTags[platform.ConsoleInput].color),
		label.FontOpt(consoleFont), label.Anchor(option.AnchorNW), label.BorderWidth(0),
		label.PadX(0), label.PadY(0))
	_ = pack.Pack(c.prompt, pack.SideOpt(pack.Left), pack.FillOpt(pack.FillY))
	c.input = text.New(bottom, p.newName("input"), text.Width(80), text.Height(3),
		text.Background(consoleInputBg), text.Foreground("#ffffff"), text.FontOpt(consoleFont),
		text.BorderWidthOpt(0), text.HighlightThickness(0), text.WrapModeOpt(text.WrapChar),
		text.PadXOpt(0), text.PadYOpt(0))
	_ = pack.Pack(c.input, pack.SideOpt(pack.Left), pack.FillOpt(pack.FillBoth), pack.Expand(true))

	// Above it, everything so far.
	c.scroll = scrollbar.New(c.top, p.newName("scroll"), scrollbar.OrientOpt(scrollbar.Vertical),
		scrollbar.CommandOpt(c.scrolled))
	_ = pack.Pack(c.scroll, pack.SideOpt(pack.Right), pack.FillOpt(pack.FillY))
	c.log = text.New(c.top, p.newName("log"), text.Width(80), text.Height(20),
		text.Background(consoleBg), text.Foreground(consoleFg), text.FontOpt(consoleFont),
		text.BorderWidthOpt(0), text.HighlightThickness(0), text.WrapModeOpt(text.WrapChar),
		text.UndoOpt(false), text.ReadOnly(true),
		text.YScrollCommand(func(first, last float64) { c.scroll.Set(first, last) }))
	_ = pack.Pack(c.log, pack.SideOpt(pack.Top), pack.FillOpt(pack.FillBoth), pack.Expand(true))
	for _, t := range consoleTags {
		c.log.TagConfigure(t.tag, text.TagForeground(t.color))
	}

	// These come before the text's own bindings, and replace them
	// when they say so.
	path := c.input.Window().PathName
	eng := p.app.Bind()
	_ = eng.Bind(path, "<KeyPress-Return>", c.onReturn)
	_ = eng.Bind(path, "<KeyPress-KP_Enter>", c.onReturn)
	_ = eng.Bind(path, "<KeyPress-Up>", func(*bind.EventData) bool { return c.recall(-1) })
	_ = eng.Bind(path, "<KeyPress-Down>", func(*bind.EventData) bool { return c.recall(1) })
	_ = eng.Bind(path, "<KeyPress-Escape>", func(*bind.EventData) bool {
		c.interrupt()
		return true
	})
	_ = eng.Bind(path, "<Control-KeyPress-c>", func(*bind.EventData) bool {
		// With text selected in the box, this copies it.
		if c.input.HasSelection() {
			return false
		}
		c.interrupt()
		return true
	})
	return c
}

// scrolled is the scrollbar moving the text.
func (c *Console) scrolled(r widget.ScrollRequest) {
	if r.MoveTo {
		c.log.YViewMoveTo(r.Fraction)
		return
	}
	c.log.YViewScroll(r.Count, r.Pages)
}

// Events returns the console's callbacks.
func (c *Console) Events() *platform.ConsoleEvents { return &c.events }

// typed is what is in the box.
func (c *Console) typed() string {
	return strings.TrimSuffix(c.input.Get("1.0", c.input.EndIndex()), "\n")
}

// setTyped puts s in the box, with the cursor after it.
func (c *Console) setTyped(s string) {
	_ = c.input.Delete("1.0", c.input.EndIndex())
	_ = c.input.Insert("1.0", s)
	_ = c.input.SetInsertPos(c.input.EndIndex())
	_ = c.input.See(c.input.EndIndex())
}

// onReturn enters what is in the box; with Shift it is left to the
// text, which starts a new line.
func (c *Console) onReturn(ev *bind.EventData) bool {
	if ev.Event != nil && ev.Event.State&tkplatform.ShiftMask != 0 {
		return false
	}
	c.Enter()
	return true
}

// Enter enters what is in the box, as Return does: it is shown above
// after the prompt, remembered, and given to OnSubmit. Nothing is
// entered while the console is busy.
func (c *Console) Enter() {
	if c.busy {
		return
	}
	line := c.typed()
	c.setTyped("")
	prompt := c.promptText
	c.Print(platform.ConsoleInput, prompt+strings.ReplaceAll(line, "\n", "\n"+strings.Repeat(" ", len(prompt)))+"\n")
	if strings.TrimSpace(line) != "" && (len(c.history) == 0 || c.history[len(c.history)-1] != line) {
		c.history = append(c.history, line)
	}
	c.at, c.draft = len(c.history), ""
	if c.events.OnSubmit != nil {
		c.events.OnSubmit(line)
	}
}

// recall puts what was entered before, or after, in the box. In a box
// of several lines the keys first move between the lines, which is
// left to the text: Up recalls from the first line and Down from the
// last.
func (c *Console) recall(dir int) bool {
	doc := c.input.Doc()
	if mark := doc.Marks["insert"]; mark != nil {
		if (dir < 0 && mark.Pos.Line > 1) || (dir > 0 && mark.Pos.Line < doc.LineCount()) {
			return false
		}
	}
	at := c.at + dir
	if at < 0 || at > len(c.history) {
		return true
	}
	if c.at == len(c.history) {
		c.draft = c.typed()
	}
	c.at = at
	if at == len(c.history) {
		c.setTyped(c.draft)
	} else {
		c.setTyped(c.history[at])
	}
	return true
}

func (c *Console) interrupt() {
	if c.events.OnInterrupt != nil {
		c.events.OnInterrupt()
	}
}

// Print adds text at the end of what the console shows, and shows the
// end.
func (c *Console) Print(kind platform.ConsoleKind, s string) {
	if s == "" {
		return
	}
	start := c.log.EndIndex()
	_ = c.log.Insert(start, s)
	_ = c.log.TagAdd(consoleTags[kind].tag, start, c.log.EndIndex())
	_ = c.log.See(c.log.EndIndex())
}

// Text is everything the console shows.
func (c *Console) Text() string { return c.log.Get("1.0", c.log.EndIndex()) }

// SetPrompt sets what stands before the box.
func (c *Console) SetPrompt(prompt string) {
	c.promptText = prompt
	_ = c.prompt.Configure(label.Text(prompt))
}

// SetBusy says whether something runs; then Return enters nothing.
func (c *Console) SetBusy(busy bool) {
	c.busy = busy
	title := "Script Console"
	if busy {
		title += " (running; Escape stops)"
	}
	c.frame.wmInfo.SetTitle(title)
}

// Clear drops the text.
func (c *Console) Clear() { _ = c.log.Delete("1.0", c.log.EndIndex()) }

// IsVisible reports whether the console is shown.
func (c *Console) IsVisible() bool { return c.frame.visible }

// SetVisible shows or hides the console.
func (c *Console) SetVisible(v bool) { c.frame.SetVisible(v) }

// Focus raises the console and gives its box the keyboard.
func (c *Console) Focus() {
	c.p.app.FocusManager().SetFocus(c.input.Window())
	if c.frame.visible {
		c.frame.present()
	}
}

// FreezePosition saves the console's place.
func (c *Console) FreezePosition(s core.Settings, key string) { c.frame.FreezePosition(s, key) }

// ThawPosition restores the place FreezePosition saved; a console that
// was never placed opens where the window manager puts it.
func (c *Console) ThawPosition(s core.Settings, key string) {
	if h, ok := s.(interface{ Has(string) bool }); ok && !h.Has(key+"_Width") {
		return
	}
	c.frame.ThawPosition(s, key)
}
