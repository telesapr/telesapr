package platform

import (
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/platform/lineedit"
)

// ConsoleKind is what a piece of a console's text is, which decides
// how it looks.
type ConsoleKind int

const (
	// ConsoleOutput is what a script printed.
	ConsoleOutput ConsoleKind = iota
	// ConsoleValue is the value of what was typed.
	ConsoleValue
	// ConsoleError is an error.
	ConsoleError
	// ConsoleInfo is what the console itself has to say.
	ConsoleInfo
	// ConsoleInput is a line that was typed, or that is shown as if
	// it had been.
	ConsoleInput
)

// ConsoleEvents are what the user does in a console.
type ConsoleEvents struct {
	// OnSubmit is given a line the user entered.
	OnSubmit func(line string)
	// OnInterrupt is called when the user asks to stop what runs, or
	// to drop a half-typed statement.
	OnInterrupt func()
	// OnClose is called when the user closes the console's window,
	// which hides it.
	OnClose func()
	// IsUnfinished, if set, says whether what is typed stops in the
	// middle of a statement: Return then starts another line rather
	// than entering it.
	IsUnfinished func(text string) bool
	// OnComplete, if set, gives what the name before the cursor can
	// be completed to, for Tab.
	OnComplete lineedit.Completer
}

// Console is a window with lines of text that grow at the bottom,
// where the user types after a prompt: a terminal for a script
// interpreter. What is typed is edited with readline's keys, and what
// was typed can be got back with the up key or searched for.
type Console interface {
	Events() *ConsoleEvents
	// Print adds text above the prompt; it should end with a new
	// line.
	Print(kind ConsoleKind, text string)
	// SetPrompt sets what stands before the line being typed.
	SetPrompt(prompt string)
	// SetBusy says that something runs: until it is over nothing can
	// be entered, only interrupted.
	SetBusy(busy bool)
	// Clear drops the text.
	Clear()
	// SetHistory sets what was entered before, oldest first, for the
	// up key and searching; what is entered is added to it.
	SetHistory(entries []string)
	SetVisible(visible bool)
	IsVisible() bool
	Focus()
	FreezePosition(s core.Settings, key string)
	ThawPosition(s core.Settings, key string)
}

// ConsoleHost is a Platform that has consoles.
type ConsoleHost interface {
	CreateConsole(parent Window) Console
}
