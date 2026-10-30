package tkgui

import (
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/takigo/takigo/bind"
	"github.com/takigo/takigo/geometry/grid"
	"github.com/takigo/takigo/geometry/pack"
	"github.com/takigo/takigo/option"
	tkplatform "github.com/takigo/takigo/platform"
	"github.com/takigo/takigo/screenunit"
	"github.com/takigo/takigo/ttk"
	"github.com/takigo/takigo/widget"
	"github.com/takigo/takigo/widget/text"
	"github.com/takigo/takigo/widget/toplevel"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/platform/lineedit"
)

// This file is the window scripts are typed in, and the way into the
// event loop for the goroutines scripts run on.

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
// Return enters them, or starts another line in a statement that isn't
// finished, as Shift+Return always does; Up and Down bring back what
// was entered before; Control+C or Escape interrupts. The box's other
// keys are readline's (keys).
type Console struct {
	p      *Platform
	events platform.ConsoleEvents

	top *toplevel.Toplevel
	// frame is the toplevel as a Window, for saving and restoring its
	// place as the other windows do.
	frame Window

	log    *text.TextWidget
	scroll *ttk.Scrollbar
	prompt *ttk.Label
	input  *text.TextWidget
	// status says whether something runs, beside the buttons.
	status *ttk.Label
	// atEnd is whether the text shown ends at its bottom, which it
	// keeps when the window changes size.
	atEnd bool

	busy       bool
	promptText string

	// edit is what readline's keys keep: the history, what was
	// killed, a search.
	edit lineedit.Editor
}

// The shares of the panes' height: what is typed has a fifth of it.
// The lines are what the texts ask for, in the same proportion.
const (
	consoleShownShare = 4
	consoleTypedShare = 1
	consoleShownLines = 20
	consoleTypedLines = 5
)

// consoleFont is the font of what is typed and shown: code lines up in
// columns.
const consoleFont = "TkFixedFont"

// consoleTag is the tag of a kind of text, and its colour on a light
// field and on a dark one; output keeps the text's own colour.
type consoleTag struct{ tag, light, dark string }

// consoleTags are the tags of the kinds of text.
var consoleTags = map[platform.ConsoleKind]consoleTag{
	platform.ConsoleOutput: {"output", "", ""},
	platform.ConsoleValue:  {"value", "#0050a0", "#70c0ff"},
	platform.ConsoleError:  {"error", "#c00000", "#ff7070"},
	platform.ConsoleInfo:   {"info", "#707070", "#a0a0a0"},
	platform.ConsoleInput:  {"input", "#006000", "#80e080"},
}

// CreateConsole makes a console, hidden. It is a window of its own
// made of takigo's themed widgets, in the application's palette: what
// was shown above, what is typed below in a fifth of the window's
// height, and buttons to run, stop and clear.
func (p *Platform) CreateConsole(platform.Window) platform.Console {
	c := &Console{p: p}
	c.top = toplevel.New(p.app, p.newName("console"), toplevel.Title("Script Console"))
	c.top.Hide()
	c.frame = Window{p: p, top: c.top, topWin: c.top.Window(), wmInfo: c.top.WmInfo, toolTop: c.top}
	setWindowIcon(c.frame.topWin)
	p.app.RegisterCloseHandler(c.frame.topWin.PlatformID, func() {
		if c.events.OnClose != nil {
			c.events.OnClose()
		}
	})
	dark := widget.PaletteFor(p.app).Dark

	body := ttk.NewFrame(c.top, p.newName("body"), ttk.FramePadding(ttk.Padding{Left: 6, Top: 6, Right: 6, Bottom: 6}))
	_ = pack.Pack(body, pack.FillOpt(pack.FillBoth), pack.Expand(true))

	buttons := ttk.NewFrame(body, p.newName("buttons"))
	_ = pack.Pack(buttons, pack.SideOpt(pack.Bottom), pack.FillOpt(pack.FillX), pack.PadY(screenunit.Px(4)))
	for _, b := range []struct {
		text string
		do   func()
	}{{"Clear", c.Clear}, {"Stop", c.interrupt}, {"Run", c.Enter}} {
		button := ttk.NewButton(buttons, p.newName("button"), ttk.ButtonText(b.text), ttk.ButtonCommand(b.do))
		_ = pack.Pack(button, pack.SideOpt(pack.Right), pack.PadX(screenunit.Px(2)))
	}
	c.status = ttk.NewLabel(buttons, p.newName("status"), ttk.LabelText(""))
	_ = pack.Pack(c.status, pack.SideOpt(pack.Left))

	// The two panes are rows of one uniform group, which grid keeps in
	// the proportion of their weights whatever the window's height.
	panes := ttk.NewFrame(body, p.newName("panes"))
	_ = pack.Pack(panes, pack.SideOpt(pack.Top), pack.FillOpt(pack.FillBoth), pack.Expand(true))
	grid.ColumnConfigure(panes, 0, grid.Weight(1))
	grid.RowConfigure(panes, 0, grid.Weight(consoleShownShare), grid.Uniform("panes"))
	grid.RowConfigure(panes, 1, grid.Weight(consoleTypedShare), grid.Uniform("panes"))

	// Above, everything so far.
	shown := ttk.NewFrame(panes, p.newName("shown"))
	c.scroll = ttk.NewScrollbar(shown, p.newName("scroll"), ttk.ScrollbarOrient(ttk.Vertical),
		ttk.ScrollbarCommand(c.scrolled))
	_ = pack.Pack(c.scroll, pack.SideOpt(pack.Right), pack.FillOpt(pack.FillY))
	c.log = text.New(shown, p.newName("log"), text.Width(80), text.Height(consoleShownLines),
		text.FontOpt(consoleFont), text.WrapModeOpt(text.WrapChar),
		text.UndoOpt(false), text.ReadOnly(true),
		text.YScrollCommand(func(first, last float64) {
			c.scroll.Set(first, last)
			c.atEnd = last >= 1
		}))
	_ = pack.Pack(c.log, pack.SideOpt(pack.Left), pack.FillOpt(pack.FillBoth), pack.Expand(true))
	for _, t := range consoleTags {
		if color := tagColor(t, dark); color != "" {
			c.log.TagConfigure(t.tag, text.TagForeground(color))
		}
	}
	_ = grid.Grid(shown, grid.Row(0), grid.Column(0), grid.Sticky(option.StickN|option.StickS|option.StickE|option.StickW))

	// Below, what is being typed, after its prompt.
	typed := ttk.NewFrame(panes, p.newName("typed"))
	c.prompt = ttk.NewLabel(typed, p.newName("prompt"), ttk.LabelText(""), ttk.LabelFont(consoleFont),
		ttk.LabelAnchor(option.AnchorNW), ttk.LabelPadding("0 3 0 0"))
	_ = pack.Pack(c.prompt, pack.SideOpt(pack.Left), pack.FillOpt(pack.FillY))
	inputScroll := ttk.NewScrollbar(typed, p.newName("inputscroll"), ttk.ScrollbarOrient(ttk.Vertical))
	_ = pack.Pack(inputScroll, pack.SideOpt(pack.Right), pack.FillOpt(pack.FillY))
	c.input = text.New(typed, p.newName("input"), text.Width(80), text.Height(consoleTypedLines),
		text.FontOpt(consoleFont), text.WrapModeOpt(text.WrapChar), text.UndoOpt(true),
		text.YScrollCommand(func(first, last float64) { inputScroll.Set(first, last) }))
	_ = inputScroll.Configure(ttk.ScrollbarCommand(widget.ScrollY(c.input)))
	_ = pack.Pack(c.input, pack.SideOpt(pack.Left), pack.FillOpt(pack.FillBoth), pack.Expand(true))
	_ = grid.Grid(typed, grid.Row(1), grid.Column(0), grid.Sticky(option.StickN|option.StickS|option.StickE|option.StickW),
		grid.PadYPair(screenunit.Px(4), screenunit.Px(0)))

	// A text is scrolled by lines from its top, so one that was showing
	// its end, as when it was filled while hidden, shows less of it as
	// it grows: it is scrolled to its end again once it has its size.
	c.atEnd = true
	_ = p.app.Bind().Bind(c.log.Window().PathName, "<Configure>", func(*bind.EventData) bool {
		if c.atEnd {
			c.p.app.After(consoleLayoutDelay, c.showEnd)
		}
		return false
	})

	// This comes before the text's own bindings, and replaces them
	// when it says so.
	_ = p.app.Bind().Bind(c.input.Window().PathName, "<KeyPress>", c.onKey)
	return c
}

// tagColor is a tag's colour on a dark field or a light one.
func tagColor(t consoleTag, dark bool) string {
	if dark {
		return t.dark
	}
	return t.light
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

// Keysyms the keys use that takigo doesn't name.
const (
	keyKPEnter = tkplatform.KeySym(0xff8d)
	// The keys of the modifiers, Shift_L to Hyper_R, and AltGr.
	keyFirstModifier = tkplatform.KeySym(0xffe1)
	keyLastModifier  = tkplatform.KeySym(0xffee)
	keyLevel3Shift   = tkplatform.KeySym(0xfe03)
)

// keys are readline's keys in the box, which onKey takes before the
// text does.
//
//	Return            enter, or a new line if the statement goes on
//	Shift+Return      a new line
//	Tab, Shift+Tab    complete the name before the cursor; again, go
//	                  through the names it can be, forward, back
//	Up, Down          the entry before, after (Control+P, Control+N)
//	Control+R         search back through the entries
//	Control+K, U      kill to the line's end, start
//	Control+W         kill the word before the cursor, up to space
//	Alt+D, Backspace  kill the word after, before the cursor
//	Control+Y         yank what was killed
//	Alt+B, Alt+F      a word back, forward
//	Control+H, T      delete back, swap two characters
//	Control+L         clear what the console shows
//	Control+C, Escape stop what runs, or drop the half-typed statement
//
// Control+A, E, B, F and D are the text's own.
func (c *Console) onKey(ed *bind.EventData) bool {
	if ed.Event == nil {
		return false
	}
	// A key taken is marked handled too: the focus manager moves the
	// focus on Tab unless it is, after the bindings have run.
	taken := c.key(ed)
	ed.Event.Handled = ed.Event.Handled || taken
	return taken
}

// key is onKey for an event there is.
func (c *Console) key(ed *bind.EventData) bool {
	ev := ed.Event
	if c.edit.Searching() {
		return c.searchKey(ed)
	}
	ctrl := ev.State&tkplatform.ControlMask != 0
	alt := ev.State&tkplatform.Mod1Mask != 0
	sym := tkplatform.KeySym(unicode.ToLower(rune(ev.KeySym)))
	tab := (ev.KeySym == tkplatform.XK_Tab || ev.KeySym == tkplatform.XK_ISO_Left_Tab) && !ctrl && !alt
	if !tab && !isModifier(ev.KeySym) {
		c.endCompletion()
	}
	switch {
	case ev.KeySym == tkplatform.XK_Return || ev.KeySym == keyKPEnter:
		if ev.State&tkplatform.ShiftMask != 0 || c.unfinished() {
			c.apply(lineedit.Newline(c.state()))
			return true
		}
		c.Enter()
	case tab:
		c.complete(ev.KeySym == tkplatform.XK_ISO_Left_Tab || ev.State&tkplatform.ShiftMask != 0)
	case ev.KeySym == tkplatform.XK_Up:
		return c.recall(c.edit.Previous)
	case ev.KeySym == tkplatform.XK_Down:
		return c.recall(c.edit.Next)
	case ev.KeySym == tkplatform.XK_Escape:
		c.interrupt()
	case ctrl && !alt:
		return c.controlKey(sym)
	case alt && !ctrl:
		return c.altKey(sym)
	default:
		return false
	}
	return true
}

// controlKey is a key with Control.
func (c *Console) controlKey(sym tkplatform.KeySym) bool {
	switch sym {
	case 'c':
		// With text selected in the box, this copies it.
		if c.input.HasSelection() {
			return false
		}
		c.interrupt()
	case 'p':
		return c.recall(c.edit.Previous)
	case 'n':
		return c.recall(c.edit.Next)
	case 'r':
		c.showSearch(c.edit.StartSearch(c.state()))
	case 'k':
		c.apply(c.edit.KillToEnd(c.state()))
	case 'u':
		c.apply(c.edit.KillToStart(c.state()))
	case 'w':
		// With text selected, the text cuts it.
		if c.input.HasSelection() {
			return false
		}
		c.apply(c.edit.KillSpaceWord(c.state()))
	case 'y':
		c.apply(c.edit.Yank(c.state()))
	case 'h':
		c.apply(lineedit.DeleteBack(c.state()))
	case 't':
		c.apply(lineedit.Transpose(c.state()))
	case 'l':
		c.Clear()
	default:
		return false
	}
	return true
}

// altKey is a key with Alt.
func (c *Console) altKey(sym tkplatform.KeySym) bool {
	switch sym {
	case 'b':
		c.apply(lineedit.WordBack(c.state()))
	case 'f':
		c.apply(lineedit.WordForward(c.state()))
	case 'd':
		c.apply(c.edit.KillWord(c.state()))
	case tkplatform.XK_BackSpace:
		c.apply(c.edit.KillWordBack(c.state()))
	default:
		return false
	}
	return true
}

// searchKey is a key while the history is searched: what is typed is
// looked for, Control+R looks further back, Escape or Control+G puts
// back what was typed before, and any other key ends the search on
// what it found and then does what it does.
func (c *Console) searchKey(ed *bind.EventData) bool {
	ev := ed.Event
	ctrl := ev.State&tkplatform.ControlMask != 0
	alt := ev.State&tkplatform.Mod1Mask != 0
	sym := tkplatform.KeySym(unicode.ToLower(rune(ev.KeySym)))
	switch {
	case isModifier(ev.KeySym):
	case ctrl && sym == 'r':
		c.showSearch(c.edit.StartSearch(c.state()))
	case ev.KeySym == tkplatform.XK_Escape || ctrl && (sym == 'g' || sym == 'c'):
		c.endSearch(false)
	case ev.KeySym == tkplatform.XK_BackSpace && !ctrl && !alt:
		c.showSearch(c.edit.SearchDelete())
	case !ctrl && !alt && printable(ev.Str, ev.KeySym) != "":
		c.showSearch(c.edit.SearchType(printable(ev.Str, ev.KeySym)))
	default:
		c.endSearch(true)
		return c.onKey(ed)
	}
	return true
}

// printable is the text a key types, "" if it types none.
func printable(str string, sym tkplatform.KeySym) string {
	if str == "" {
		if r := tkplatform.KeySymToRune(sym); r > 0 {
			str = string(r)
		}
	}
	if r, _ := utf8.DecodeRuneInString(str); str == "" || !unicode.IsPrint(r) {
		return ""
	}
	return str
}

// showSearch shows what a search found, and the search in the
// prompt's place.
func (c *Console) showSearch(s lineedit.State) {
	c.apply(s)
	c.prompt.SetText(c.edit.SearchPrompt())
}

// endSearch ends a search, keeping what it found or not, and puts the
// prompt back.
func (c *Console) endSearch(keep bool) {
	c.apply(c.edit.EndSearch(keep))
	c.prompt.SetText(c.promptText)
}

// unfinished reports whether what is typed stops in the middle of a
// statement, which Return then goes on with on a new line.
func (c *Console) unfinished() bool {
	return c.events.IsUnfinished != nil && c.events.IsUnfinished(c.typed())
}

// isModifier reports whether a key is Shift, Control, Alt or another
// modifier, which alone does nothing in the box.
func isModifier(sym tkplatform.KeySym) bool {
	return sym >= keyFirstModifier && sym <= keyLastModifier || sym == keyLevel3Shift
}

// complete completes the name before the cursor; with several that it
// can be, they are shown above, and Tab again puts each in its place in
// turn, back with Shift+Tab, the status line saying which it is.
func (c *Console) complete(back bool) {
	complete := c.events.OnComplete
	if complete == nil {
		complete = func(string, int) (int, []string) { return 0, nil }
	}
	step := c.edit.Complete
	if back {
		step = c.edit.CompleteBack
	}
	s, list := step(c.state(), complete)
	c.apply(s)
	if len(list) > 0 {
		c.Print(platform.ConsoleInfo, lineedit.Columns(list, consoleColumns))
	}
	switch i, n, name := c.edit.Choice(); {
	case n == 0 || c.busy:
	case i == 0:
		c.status.SetText(fmt.Sprintf("%d names; Tab goes through them, Shift+Tab back.", n))
	default:
		c.status.SetText(fmt.Sprintf("%d of %d: %s", i, n, name))
	}
}

// endCompletion ends going through the names Tab found, and what the
// status line says of them.
func (c *Console) endCompletion() {
	if _, n, _ := c.edit.Choice(); n > 0 && !c.busy {
		c.status.SetText("")
	}
	c.edit.EndCompletion()
}

// consoleColumns is how wide the console's text is, in characters.
const consoleColumns = 80

// state is what is in the box and where its cursor is.
func (c *Console) state() lineedit.State {
	typed := c.typed()
	pos := text.Index{Line: 1}
	if mark := c.input.Doc().Marks["insert"]; mark != nil {
		pos = mark.Pos
	}
	return lineedit.State{Text: typed, Cursor: offsetOf(typed, pos)}
}

// apply makes the box show s.
func (c *Console) apply(s lineedit.State) {
	if s.Text != c.typed() {
		c.setTyped(s.Text)
	}
	_ = c.input.SetInsertPos(indexOf(s.Text, s.Cursor))
	_ = c.input.See("insert")
}

// offsetOf is the byte offset in s of a text's index in it.
func offsetOf(s string, at text.Index) int {
	off := 0
	for range at.Line - 1 {
		i := strings.IndexByte(s[off:], '\n')
		if i < 0 {
			return len(s)
		}
		off += i + 1
	}
	for range at.Char {
		if off >= len(s) || s[off] == '\n' {
			break
		}
		_, n := utf8.DecodeRuneInString(s[off:])
		off += n
	}
	return off
}

// indexOf is the text's index of a byte offset in s.
func indexOf(s string, off int) text.Index {
	before := s[:min(max(off, 0), len(s))]
	line := strings.Count(before, "\n")
	return text.Index{Line: line + 1, Char: utf8.RuneCountInString(before[strings.LastIndexByte(before, '\n')+1:])}
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
	c.edit.Add(line)
	if c.events.OnSubmit != nil {
		c.events.OnSubmit(line)
	}
}

// SetHistory sets what Up and Control+R bring back, oldest first.
func (c *Console) SetHistory(entries []string) { c.edit.SetHistory(entries) }

// recall shows the entry that step brings back; in a box of several
// lines it leaves the key to the text but on the first or the last.
func (c *Console) recall(step func(lineedit.State) (lineedit.State, bool)) bool {
	s, ok := step(c.state())
	if ok {
		c.apply(s)
	}
	return ok
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
	// A hidden text has no height to scroll in: SetVisible shows the end
	// once it is laid out.
	if c.frame.visible {
		c.showEnd()
	}
}

// showEnd scrolls the text shown to its end, at the bottom. The text's
// See would not do: a text seen while it had no height has the end at
// its top, and See leaves it there as the end is in view.
func (c *Console) showEnd() { c.log.YViewMoveTo(1) }

// Text is everything the console shows.
func (c *Console) Text() string { return c.log.Get("1.0", c.log.EndIndex()) }

// SetPrompt sets what stands before the box.
func (c *Console) SetPrompt(prompt string) {
	c.promptText = prompt
	c.prompt.SetText(prompt)
}

// SetBusy says whether something runs; then Return enters nothing.
func (c *Console) SetBusy(busy bool) {
	c.busy = busy
	title, status := "Script Console", ""
	if busy {
		title, status = title+" (running)", "Running; Escape or Stop stops it."
	}
	c.frame.wmInfo.SetTitle(title)
	c.status.SetText(status)
}

// Clear drops the text.
func (c *Console) Clear() { _ = c.log.Delete("1.0", c.log.EndIndex()) }

// IsVisible reports whether the console is shown.
func (c *Console) IsVisible() bool { return c.frame.visible }

// SetVisible shows or hides the console; shown, it shows the end of its
// text once the window is laid out.
func (c *Console) SetVisible(v bool) {
	c.frame.SetVisible(v)
	if v {
		c.p.app.After(consoleLayoutDelay, c.showEnd)
	}
}

// consoleLayoutDelay is how long a console shown is given to be laid
// out before it is scrolled to its end.
const consoleLayoutDelay = 50 * time.Millisecond

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
