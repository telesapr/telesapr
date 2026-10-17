// Package headless is a platform without a display, for tests: it
// ports platform/guinone.cpp, and records what the application asks of
// its windows, menus and timers, and answers dialogs from scripts.
package headless

import (
	"image"
	"strconv"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/render/soft"
)

// Platform is a headless platform.Platform.
type Platform struct {
	Store       core.Settings
	Timers      []*Timer
	Windows     []*Window
	MainMenuBar *MenuBar
	Menus       []*Menu
	// Consoles are the consoles made, see console.go.
	Consoles []*Console
	// DialogResponses answer message dialogs, in order; DialogsShown
	// records them.
	DialogResponses []platform.Response
	DialogsShown    []*MessageDialog
	// FilePaths answer file dialogs, in order: "" cancels.
	FilePaths   []string
	FileDialogs []*FileDialog
	// FormResponses answer form dialogs, in order; FormsShown records
	// them.
	FormResponses []FormResponse
	FormsShown    []*FormDialog
	Fonts         []string
	OpenedURLs    []string
	ExitRequests  int

	// PopUpHook, when set, picks the item a context menu without its own
	// PopUpChoice triggers; nil triggers none.
	PopUpHook func(m *Menu) *MenuItem
	// OnMessageDialog and OnFileDialog, when set, see each dialog as it
	// is run, before it is answered.
	OnMessageDialog func(d *MessageDialog)
	OnFileDialog    func(d *FileDialog)
	OnFormDialog    func(d *FormDialog)
}

var _ platform.Platform = (*Platform)(nil)

// New returns a headless platform with in-memory settings.
func New() *Platform {
	return &Platform{Store: platform.NewMemorySettings()}
}

// Settings returns the settings.
func (p *Platform) Settings() core.Settings { return p.Store }

// CreateTimer returns a timer RunTimers fires.
func (p *Platform) CreateTimer() platform.Timer {
	t := &Timer{}
	p.Timers = append(p.Timers, t)
	return t
}

// RunTimers fires every running timer once, in creation order, until
// none are left running; it gives up after limit rounds and returns
// how many timeouts ran.
func (p *Platform) RunTimers(limit int) int {
	n := 0
	for range limit {
		fired := false
		for _, t := range p.Timers {
			if t.Running {
				t.Running = false
				fired = true
				n++
				if t.onTimeout != nil {
					t.onTimeout()
				}
			}
		}
		if !fired {
			break
		}
	}
	return n
}

// RunDueTimers is RunTimers for the timers due within a second: those
// an event loop fires while a script runs, not autosave.
func (p *Platform) RunDueTimers(limit int) int {
	n := 0
	for range limit {
		fired := false
		for _, t := range p.Timers {
			if t.Running && t.DelayMs < 1000 {
				t.Running = false
				fired = true
				n++
				if t.onTimeout != nil {
					t.onTimeout()
				}
			}
		}
		if !fired {
			break
		}
	}
	return n
}

// CreateMenu returns a recording menu.
func (p *Platform) CreateMenu() platform.Menu {
	m := &Menu{platform: p}
	p.Menus = append(p.Menus, m)
	return m
}

// MainMenu returns the recording menu bar.
func (p *Platform) MainMenu() (platform.MenuBar, bool) {
	if p.MainMenuBar == nil {
		p.MainMenuBar = &MenuBar{platform: p}
	}
	return p.MainMenuBar, false
}

// CreateWindow returns a recording window of 600x600 pixels.
func (p *Platform) CreateWindow(kind platform.WindowKind, parent platform.Window) platform.Window {
	w := &Window{Kind: kind, Width: 600, Height: 600, DPI: 96, PixelRatio: 1}
	if pw, ok := parent.(*Window); ok {
		w.Parent = pw
	}
	p.Windows = append(p.Windows, w)
	return w
}

// CreateMessageDialog returns a dialog answered from DialogResponses.
func (p *Platform) CreateMessageDialog(platform.Window) platform.MessageDialog {
	d := &MessageDialog{platform: p}
	p.DialogsShown = append(p.DialogsShown, d)
	return d
}

// CreateFormDialog returns a dialog answered from FormResponses.
func (p *Platform) CreateFormDialog(platform.Window) platform.FormDialog {
	d := &FormDialog{platform: p}
	p.FormsShown = append(p.FormsShown, d)
	return d
}

// CreateOpenFileDialog returns a dialog answered from FilePaths.
func (p *Platform) CreateOpenFileDialog(platform.Window) platform.FileDialog {
	d := &FileDialog{platform: p}
	p.FileDialogs = append(p.FileDialogs, d)
	return d
}

// CreateSaveFileDialog returns a dialog answered from FilePaths.
func (p *Platform) CreateSaveFileDialog(platform.Window) platform.FileDialog {
	d := &FileDialog{platform: p, Save: true}
	p.FileDialogs = append(p.FileDialogs, d)
	return d
}

// FontFiles returns FontFiles.
func (p *Platform) FontFiles() []string { return p.Fonts }

// OpenInBrowser records the URL.
func (p *Platform) OpenInBrowser(url string) { p.OpenedURLs = append(p.OpenedURLs, url) }

// Run returns at once.
func (p *Platform) Run() {}

// Exit records the request.
func (p *Platform) Exit() { p.ExitRequests++ }

// Timer is a timer fired by Platform.RunTimers.
type Timer struct {
	Running   bool
	DelayMs   int
	onTimeout func()
}

func (t *Timer) SetOnTimeout(fn func())    { t.onTimeout = fn }
func (t *Timer) RunAfter(ms int)           { t.Running, t.DelayMs = true, ms }
func (t *Timer) RunAfterNextFrame()        { t.RunAfter(1) }
func (t *Timer) RunAfterProcessingEvents() { t.RunAfter(0) }

// MenuItem is a recorded menu entry: a separator, a command, or a
// submenu.
type MenuItem struct {
	Label string
	// RawLabel is the label as given, with its '&' mnemonic markers.
	RawLabel  string
	Accel     string
	Indicator platform.Indicator
	Enabled   bool
	Active    bool
	Separator bool
	SubMenu   *Menu
	OnTrigger func()
}

func (i *MenuItem) SetAccelerator(accel platform.KeyboardEvent) {
	i.Accel = platform.AcceleratorDescription(accel)
}
func (i *MenuItem) SetIndicator(ind platform.Indicator) { i.Indicator = ind }
func (i *MenuItem) SetEnabled(e bool)                   { i.Enabled = e }
func (i *MenuItem) SetActive(a bool)                    { i.Active = a }

// Menu is a recorded menu.
type Menu struct {
	platform *Platform
	Items    []*MenuItem
	// PopUpChoice, when set, picks the item PopUp triggers.
	PopUpChoice func(m *Menu) *MenuItem
	PoppedUp    int
}

func (m *Menu) AddItem(label string, onTrigger func(), mnemonics bool) platform.MenuItem {
	raw := label
	if mnemonics {
		label = strings.ReplaceAll(label, "&", "")
	}
	it := &MenuItem{Label: label, RawLabel: raw, OnTrigger: onTrigger, Enabled: true}
	m.Items = append(m.Items, it)
	return it
}

func (m *Menu) AddSubMenu(label string) platform.Menu {
	sub := &Menu{platform: m.platform}
	m.Items = append(m.Items, &MenuItem{Label: strings.ReplaceAll(label, "&", ""), RawLabel: label,
		SubMenu: sub, Enabled: true})
	return sub
}

func (m *Menu) AddSeparator() { m.Items = append(m.Items, &MenuItem{Separator: true}) }

// PopUp triggers the item PopUpChoice (else the platform's PopUpHook)
// picks, if any.
func (m *Menu) PopUp() {
	m.PoppedUp++
	choose := m.PopUpChoice
	if choose == nil && m.platform != nil {
		choose = m.platform.PopUpHook
	}
	if choose == nil {
		return
	}
	if it := choose(m); it != nil && it.OnTrigger != nil {
		it.OnTrigger()
	}
}

func (m *Menu) Clear() { m.Items = nil }

// Find returns the item with that label, looking into submenus along
// a path such as "File", "Save".
func (m *Menu) Find(path ...string) *MenuItem {
	cur := m
	var it *MenuItem
	for _, label := range path {
		if cur == nil {
			return nil
		}
		it = nil
		for _, c := range cur.Items {
			if c.Label == label {
				it = c
				break
			}
		}
		if it == nil {
			return nil
		}
		cur = it.SubMenu
	}
	return it
}

// MenuBar is a recorded menu bar.
type MenuBar struct {
	platform *Platform
	Menus    []*MenuItem
}

func (b *MenuBar) AddSubMenu(label string) platform.Menu {
	sub := &Menu{platform: b.platform}
	b.Menus = append(b.Menus, &MenuItem{Label: strings.ReplaceAll(label, "&", ""), RawLabel: label,
		SubMenu: sub, Enabled: true})
	return sub
}

func (b *MenuBar) Clear() { b.Menus = nil }

// Find returns the item along a path from the bar, such as "File",
// "Save".
func (b *MenuBar) Find(path ...string) *MenuItem {
	if len(path) == 0 {
		return nil
	}
	for _, m := range b.Menus {
		if m.Label == path[0] {
			if len(path) == 1 {
				return m
			}
			return m.SubMenu.Find(path[1:]...)
		}
	}
	return nil
}

// Window records a window's state.
type Window struct {
	events platform.WindowEvents

	Kind          platform.WindowKind
	Parent        *Window
	Width, Height float64
	MinWidth      float64
	MinHeight     float64
	DPI           float64
	PixelRatio    float64

	Visible    bool
	Focused    bool
	FullScreen bool
	Title      string
	MenuBar    platform.MenuBar
	Cursor     platform.Cursor

	TooltipText string
	TooltipRect [4]float64

	EditorVisible   bool
	EditorText      string
	EditorX         float64
	EditorY         float64
	EditorMinWidth  float64
	EditorMonospace bool

	ScrollbarVisible bool
	ScrollMin        float64
	ScrollMax        float64
	ScrollPage       float64
	ScrollPos        float64

	Invalidated int
	Frame       *image.RGBA
	// NoFrame keeps PresentFrame from reading the frame back, for
	// benchmarks that time drawing alone.
	NoFrame bool
}

var _ platform.Window = (*Window)(nil)

func (w *Window) Events() *platform.WindowEvents { return &w.events }
func (w *Window) PixelDensity() float64          { return w.DPI }
func (w *Window) DevicePixelRatio() float64      { return w.PixelRatio }
func (w *Window) DeviceFontScale() float64 {
	return w.DPI / w.PixelRatio / 96.0
}
func (w *Window) IsVisible() bool      { return w.Visible }
func (w *Window) SetVisible(v bool)    { w.Visible = v }
func (w *Window) Focus()               { w.Focused = true }
func (w *Window) IsFullScreen() bool   { return w.FullScreen }
func (w *Window) SetFullScreen(f bool) { w.FullScreen = f }
func (w *Window) SetTitle(t string)    { w.Title = t }
func (w *Window) SetTitleForFilename(string) bool {
	return false
}
func (w *Window) SetMenuBar(mb platform.MenuBar) { w.MenuBar = mb }
func (w *Window) ContentSize() (float64, float64) {
	return w.Width, w.Height
}
func (w *Window) SetMinContentSize(width, height float64) {
	w.MinWidth, w.MinHeight = width, height
}
func (w *Window) FreezePosition(core.Settings, string) {}
func (w *Window) ThawPosition(core.Settings, string)   {}
func (w *Window) SetCursor(c platform.Cursor)          { w.Cursor = c }
func (w *Window) SetTooltip(text string, x, y, width, height float64) {
	w.TooltipText = text
	w.TooltipRect = [4]float64{x, y, width, height}
}
func (w *Window) IsEditorVisible() bool { return w.EditorVisible }
func (w *Window) ShowEditor(x, y, fontHeight, minWidth float64, isMonospace bool, text string) {
	w.EditorVisible = true
	w.EditorX, w.EditorY, w.EditorMinWidth = x, y, minWidth
	w.EditorMonospace = isMonospace
	w.EditorText = text
}
func (w *Window) HideEditor()                { w.EditorVisible = false }
func (w *Window) SetScrollbarVisible(v bool) { w.ScrollbarVisible = v }
func (w *Window) ConfigureScrollbar(lo, hi, pageSize float64) {
	w.ScrollMin, w.ScrollMax, w.ScrollPage = lo, hi, pageSize
}
func (w *Window) ScrollbarPosition() float64     { return w.ScrollPos }
func (w *Window) SetScrollbarPosition(p float64) { w.ScrollPos = p }
func (w *Window) Invalidate()                    { w.Invalidated++ }

// CreateRenderer returns the software renderer: a headless window has
// no GL, so goldens and tests never see a driver's pixels.
func (w *Window) CreateRenderer(bool) render.ViewportCanvas { return soft.New() }

// PresentFrame keeps the frame canvas finished as Frame, unless
// NoFrame is set.
func (w *Window) PresentFrame(canvas render.ViewportCanvas) {
	if !w.NoFrame {
		w.Frame = canvas.ReadFrame()
	}
}

// Render runs the window's OnRender now, as the event loop would after
// Invalidate.
func (w *Window) Render() {
	w.Invalidated = 0
	if w.events.OnRender != nil {
		w.events.OnRender()
	}
}

// Mouse delivers a mouse event.
func (w *Window) Mouse(ev platform.MouseEvent) bool {
	if w.events.OnMouseEvent == nil {
		return false
	}
	return w.events.OnMouseEvent(ev)
}

// Key delivers a key press and its release.
func (w *Window) Key(ev platform.KeyboardEvent) bool {
	if w.events.OnKeyboardEvent == nil {
		return false
	}
	ev.Type = platform.KeyPress
	handled := w.events.OnKeyboardEvent(ev)
	ev.Type = platform.KeyRelease
	w.events.OnKeyboardEvent(ev)
	return handled
}

// FinishEditing closes the editor with text, as pressing Enter does.
func (w *Window) FinishEditing(text string) {
	if w.events.OnEditingDone != nil {
		w.events.OnEditingDone(text)
	}
}

// MessageDialog is a recorded dialog.
type MessageDialog struct {
	platform    *Platform
	Type        platform.DialogType
	Title       string
	Message     string
	Description string
	Buttons     []platform.Response
	Labels      []string
	Response    platform.Response
}

func (d *MessageDialog) SetType(t platform.DialogType) { d.Type = t }
func (d *MessageDialog) SetTitle(s string)             { d.Title = s }
func (d *MessageDialog) SetMessage(s string)           { d.Message = s }
func (d *MessageDialog) SetDescription(s string)       { d.Description = s }
func (d *MessageDialog) AddButton(label string, r platform.Response, _ bool) {
	d.Labels = append(d.Labels, label)
	d.Buttons = append(d.Buttons, r)
}

// RunModal answers with the next scripted response; it panics without
// one, so tests say what every dialog gets.
func (d *MessageDialog) RunModal() platform.Response {
	if d.platform.OnMessageDialog != nil {
		d.platform.OnMessageDialog(d)
	}
	if len(d.platform.DialogResponses) == 0 {
		panic("headless: unexpected message dialog: " + d.Message)
	}
	d.Response = d.platform.DialogResponses[0]
	d.platform.DialogResponses = d.platform.DialogResponses[1:]
	return d.Response
}

// FileDialog is a recorded file dialog.
type FileDialog struct {
	platform *Platform
	Save     bool
	Title    string
	Path     string
	Filters  []platform.FileFilter
}

func (d *FileDialog) SetTitle(s string)       { d.Title = s }
func (d *FileDialog) SetCurrentName(s string) { d.Path = s }
func (d *FileDialog) Filename() string        { return d.Path }
func (d *FileDialog) SetFilename(s string)    { d.Path = s }
func (d *FileDialog) SuggestFilename(s string) {
	d.Path = s
}
func (d *FileDialog) AddFilter(f platform.FileFilter)     { d.Filters = append(d.Filters, f) }
func (d *FileDialog) AddFilters(fs []platform.FileFilter) { d.Filters = append(d.Filters, fs...) }
func (d *FileDialog) FreezeChoices(core.Settings, string) {}
func (d *FileDialog) ThawChoices(core.Settings, string)   {}

// RunModal answers with the next scripted path; "" cancels. It panics
// without one.
func (d *FileDialog) RunModal() bool {
	if d.platform.OnFileDialog != nil {
		d.platform.OnFileDialog(d)
	}
	if len(d.platform.FilePaths) == 0 {
		panic("headless: unexpected file dialog: " + d.Title)
	}
	path := d.platform.FilePaths[0]
	d.platform.FilePaths = d.platform.FilePaths[1:]
	if path == "" {
		return false
	}
	d.Path = path
	return true
}

// FormEdit is one change the user makes to a form's field.
type FormEdit struct{ Key, Value string }

// FormResponse is what the user does with a form: the changes they make,
// in order, and then accepting it, or cancelling it.
type FormResponse struct {
	Edits  []FormEdit
	Cancel bool
}

// FormField is a recorded field of a form.
type FormField struct {
	Key, Label string
	// Kind is "choice", "entry" or "check".
	Kind    string
	Choices []string
	Hint    string
	// Value is the field's value as platform.FormValues hold it, as the
	// form was shown and then as it was left.
	Value string
}

// FormDialog is a recorded form.
type FormDialog struct {
	platform *Platform
	Title    string
	Accept   string
	Cancel   string
	Fields   []*FormField
	onChange func(key, value string) platform.FormValues
	// Result is the values the form was accepted with.
	Result platform.FormValues
}

func (d *FormDialog) SetTitle(s string)                { d.Title = s }
func (d *FormDialog) SetButtons(accept, cancel string) { d.Accept, d.Cancel = accept, cancel }
func (d *FormDialog) AddChoice(key, label string, choices []string, selected int) {
	d.Fields = append(d.Fields, &FormField{Key: key, Label: label, Kind: "choice", Choices: choices,
		Value: strconv.Itoa(selected)})
}
func (d *FormDialog) AddEntry(key, label, text, hint string) {
	d.Fields = append(d.Fields, &FormField{Key: key, Label: label, Kind: "entry", Value: text, Hint: hint})
}
func (d *FormDialog) AddCheck(key, label string, on bool) {
	v := "0"
	if on {
		v = "1"
	}
	d.Fields = append(d.Fields, &FormField{Key: key, Label: label, Kind: "check", Value: v})
}
func (d *FormDialog) OnChange(fn func(key, value string) platform.FormValues) { d.onChange = fn }

// Field is the field with the key, nil if there is none.
func (d *FormDialog) Field(key string) *FormField {
	for _, f := range d.Fields {
		if f.Key == key {
			return f
		}
	}
	return nil
}

// RunModal makes the next scripted response's edits, as the window does
// (each is told to the form's change callback, whose answers are put
// into the other fields), and accepts the form or cancels it. It panics
// without a response, so tests say what every form gets.
func (d *FormDialog) RunModal() (platform.FormValues, bool) {
	if d.platform.OnFormDialog != nil {
		d.platform.OnFormDialog(d)
	}
	if len(d.platform.FormResponses) == 0 {
		panic("headless: unexpected form dialog: " + d.Title)
	}
	r := d.platform.FormResponses[0]
	d.platform.FormResponses = d.platform.FormResponses[1:]
	for _, e := range r.Edits {
		f := d.Field(e.Key)
		if f == nil {
			panic("headless: form dialog has no field " + e.Key)
		}
		f.Value = e.Value
		if d.onChange == nil {
			continue
		}
		for key, v := range d.onChange(e.Key, e.Value) {
			if g := d.Field(key); g != nil && g != f {
				g.Value = v
			}
		}
	}
	if r.Cancel {
		return nil, false
	}
	d.Result = make(platform.FormValues, len(d.Fields))
	for _, f := range d.Fields {
		d.Result[f.Key] = f.Value
	}
	return d.Result, true
}
