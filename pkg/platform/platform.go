// Package platform is the abstraction of platform-dependent GUI
// functionality the user interface is written against: windows with an
// editor overlay and a scrollbar, menus, timers, settings, and message
// and file dialogs. It ports platform/gui.h and gui.cpp; backends are
// pkg/platform/headless and pkg/platform/takigo.
package platform

import (
	"fmt"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/render"
)

// MouseEventType is what a mouse event reports (C++ MouseEvent::Type).
type MouseEventType int

const (
	MouseMotion MouseEventType = iota
	MousePress
	MouseDblPress
	MouseRelease
	MouseScrollVert
	MouseLeave
)

// MouseButton is a mouse button (C++ MouseEvent::Button).
type MouseButton int

const (
	ButtonNone MouseButton = iota
	ButtonLeft
	ButtonMiddle
	ButtonRight
)

// MouseEvent is a mouse input event; X and Y are in the window's
// content, y down. Button is set for motion, press and release;
// ScrollDelta for scrolling, positive up (C++ MouseEvent).
type MouseEvent struct {
	Type        MouseEventType
	X, Y        float64
	ShiftDown   bool
	ControlDown bool
	Button      MouseButton
	ScrollDelta float64
}

// KeyboardEventType is a key's press or release.
type KeyboardEventType int

const (
	KeyPress KeyboardEventType = iota
	KeyRelease
)

// KeyKind says whether a key is a character or a function key.
type KeyKind int

const (
	KeyCharacter KeyKind = iota
	KeyFunction
)

// KeyboardEvent is a keyboard input event: Chr for a character, Num
// for function key F<Num> (C++ KeyboardEvent).
type KeyboardEvent struct {
	Type        KeyboardEventType
	Key         KeyKind
	Chr         rune
	Num         int
	ShiftDown   bool
	ControlDown bool
}

// Equals compares two events as accelerators do.
func (k KeyboardEvent) Equals(o KeyboardEvent) bool {
	return k.Type == o.Type && k.Key == o.Key &&
		k.ShiftDown == o.ShiftDown && k.ControlDown == o.ControlDown &&
		((k.Key == KeyCharacter && k.Chr == o.Chr) ||
			(k.Key == KeyFunction && k.Num == o.Num))
}

// AcceleratorDescription is the accelerator's label for a menu, like
// "Ctrl+Shift+S" (C++ AcceleratorDescription).
func AcceleratorDescription(accel KeyboardEvent) string {
	var b strings.Builder
	if accel.ControlDown {
		b.WriteString("Ctrl+")
	}
	if accel.ShiftDown {
		b.WriteString("Shift+")
	}
	switch accel.Key {
	case KeyFunction:
		fmt.Fprintf(&b, "F%d", accel.Num)
	case KeyCharacter:
		switch {
		case accel.Chr == '\t':
			b.WriteString("Tab")
		case accel.Chr == ' ':
			b.WriteString("Space")
		case accel.Chr == '\x1b':
			b.WriteString("Esc")
		case accel.Chr == '\x7f':
			b.WriteString("Del")
		case accel.Chr != 0:
			c := byte(accel.Chr & 0xff)
			if c >= 'a' && c <= 'z' {
				c -= 'a' - 'A'
			}
			b.WriteByte(c)
		}
	}
	return b.String()
}

// Timer is a single-shot timer (C++ Timer).
type Timer interface {
	// SetOnTimeout sets what runs when the timer fires.
	SetOnTimeout(fn func())
	// RunAfter (re)starts the timer.
	RunAfter(ms int)
	RunAfterNextFrame()
	RunAfterProcessingEvents()
}

// Indicator is a menu item's check or radio mark (C++
// MenuItem::Indicator).
type Indicator int

const (
	IndicatorNone Indicator = iota
	IndicatorCheckMark
	IndicatorRadioMark
)

// MenuItem is a menu entry (C++ MenuItem).
type MenuItem interface {
	SetAccelerator(accel KeyboardEvent)
	SetIndicator(Indicator)
	SetEnabled(bool)
	SetActive(bool)
}

// Menu is a menu or submenu (C++ Menu). A label's '&' marks the
// mnemonic when mnemonics is set.
type Menu interface {
	AddItem(label string, onTrigger func(), mnemonics bool) MenuItem
	AddSubMenu(label string) Menu
	AddSeparator()
	// PopUp shows the menu at the pointer and returns when it is
	// dismissed, after running the chosen item's trigger.
	PopUp()
	Clear()
}

// MenuBar is a window's menu bar (C++ MenuBar).
type MenuBar interface {
	AddSubMenu(label string) Menu
	Clear()
}

// WindowKind is a toplevel window or a tool window that stays with its
// parent (C++ Window::Kind).
type WindowKind int

const (
	WindowToplevel WindowKind = iota
	WindowTool
)

// Cursor is a mouse cursor shape (C++ Window::Cursor).
type Cursor int

const (
	CursorPointer Cursor = iota
	CursorHand
)

// WindowEvents are the callbacks a window makes into the application
// (C++ Window's on* members). Unset ones are ignored.
type WindowEvents struct {
	OnClose             func()
	OnFullScreen        func(bool)
	OnMouseEvent        func(MouseEvent) bool
	OnKeyboardEvent     func(KeyboardEvent) bool
	OnEditingDone       func(string)
	OnScrollbarAdjusted func(float64)
	OnContextLost       func()
	// OnRender draws the window's content: the application renders a
	// frame and hands it to PresentFrame.
	OnRender func()
}

// Window is a top-level window with a drawing area, an editor overlay
// and a vertical scrollbar (C++ Window).
type Window interface {
	Events() *WindowEvents

	// PixelDensity is the physical DPI; GetDevicePixelRatio the size
	// of a logical pixel in physical ones.
	PixelDensity() float64
	DevicePixelRatio() float64
	DeviceFontScale() float64

	IsVisible() bool
	SetVisible(bool)
	Focus()

	IsFullScreen() bool
	SetFullScreen(bool)

	SetTitle(string)
	// SetTitleForFilename sets a title for the file where the platform
	// shows files specially; false if it doesn't.
	SetTitleForFilename(path string) bool

	SetMenuBar(MenuBar)

	// ContentSize is the drawing area's size in logical pixels.
	ContentSize() (width, height float64)
	SetMinContentSize(width, height float64)

	FreezePosition(s core.Settings, key string)
	ThawPosition(s core.Settings, key string)

	SetCursor(Cursor)
	// SetTooltip shows text while the pointer is in the given
	// rectangle; empty text hides it.
	SetTooltip(text string, x, y, width, height float64)

	IsEditorVisible() bool
	ShowEditor(x, y, fontHeight, minWidth float64, isMonospace bool, text string)
	HideEditor()

	SetScrollbarVisible(bool)
	ConfigureScrollbar(lo, hi, pageSize float64)
	ScrollbarPosition() float64
	SetScrollbarPosition(pos float64)

	// Invalidate asks for OnRender to be called soon.
	Invalidate()
	// CreateRenderer returns a renderer that draws the window's
	// content: with useGL, OpenGL where the platform has it, else the
	// software renderer (C++ CreateRenderer). A renderer made earlier
	// for the window must not be used after.
	CreateRenderer(useGL bool) render.ViewportCanvas
	// PresentFrame shows the frame canvas, which CreateRenderer made,
	// has just finished: of the content size times the device pixel
	// ratio. The frame stays on screen until the next one.
	PresentFrame(canvas render.ViewportCanvas)
}

// DialogType is a message dialog's icon (C++ MessageDialog::Type).
type DialogType int

const (
	DialogInformation DialogType = iota
	DialogQuestion
	DialogWarning
	DialogError
)

// Response is the button a dialog was closed with (C++
// MessageDialog::Response).
type Response int

const (
	ResponseNone Response = iota
	ResponseOK
	ResponseYes
	ResponseNo
	ResponseCancel
)

// MessageDialog asks for one choice out of several (C++
// MessageDialog).
type MessageDialog interface {
	SetType(DialogType)
	SetTitle(string)
	SetMessage(string)
	SetDescription(string)
	AddButton(label string, response Response, isDefault bool)
	RunModal() Response
}

// FileFilter is a named set of file extensions (C++ FileFilter).
type FileFilter struct {
	Name       string
	Extensions []string
}

// FileDialog asks for a file to open or save (C++ FileDialog).
type FileDialog interface {
	SetTitle(string)
	// SetCurrentName sets the file name, without its folder.
	SetCurrentName(string)
	Filename() string
	SetFilename(string)
	// SuggestFilename sets the folder and name, with the extension of
	// the first filter.
	SuggestFilename(string)
	AddFilter(FileFilter)
	AddFilters([]FileFilter)
	FreezeChoices(s core.Settings, key string)
	ThawChoices(s core.Settings, key string)
	RunModal() bool
}

// Platform creates the platform's objects and runs its event loop
// (the C++ free functions of gui.h).
type Platform interface {
	Settings() core.Settings
	CreateTimer() Timer
	CreateMenu() Menu
	// MainMenu returns the menu bar; unique is true on
	// platforms with one application-wide menu bar.
	MainMenu() (bar MenuBar, unique bool)
	CreateWindow(kind WindowKind, parent Window) Window
	CreateMessageDialog(parent Window) MessageDialog
	CreateOpenFileDialog(parent Window) FileDialog
	CreateSaveFileDialog(parent Window) FileDialog

	FontFiles() []string
	OpenInBrowser(url string)

	// Run runs the event loop until Exit.
	Run()
	Exit()
}
