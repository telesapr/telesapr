//go:build js && wasm

package web

import (
	"strings"
	"syscall/js"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/platform"
)

// Console is the script console: a panel under the windows with what
// the scripts printed over a box where the next lines are typed. Return
// enters them, or starts another line in a statement that isn't
// finished, as Shift+Return always does; Up and Down bring back what was
// entered before; Tab completes; Escape or Control+C interrupts.
type Console struct {
	p      *Platform
	events platform.ConsoleEvents
	js     js.Value

	prompt string
	busy   bool
}

var _ platform.Console = (*Console)(nil)

// CreateConsole makes a console, hidden.
func (p *Platform) CreateConsole(platform.Window) platform.Console {
	c := &Console{p: p, prompt: ">>> "}
	cb := js.Global().Get("Object").New()
	cb.Set("enter", p.fn(func(a []js.Value) any {
		text, newline := a[0].String(), a[1].Bool()
		p.q.post(func() { c.enter(text, newline) })
		return nil
	}))
	cb.Set("tab", p.fn(func(a []js.Value) any {
		text, cursor := a[0].String(), a[1].Int()
		p.q.post(func() { c.tab(text, cursor) })
		return nil
	}))
	cb.Set("interrupt", p.fn(func([]js.Value) any {
		p.q.post(func() {
			if c.events.OnInterrupt != nil {
				c.events.OnInterrupt()
			}
		})
		return nil
	}))
	cb.Set("close", p.fn(func([]js.Value) any {
		p.q.post(func() {
			if c.events.OnClose != nil {
				c.events.OnClose()
			}
		})
		return nil
	}))
	c.js = p.w.Call("console", cb)
	return c
}

// enter is Return in the input box, with the text there: the line is
// shown after the prompt and given to the console's owner, unless the
// console is busy or the statement isn't finished.
func (c *Console) enter(text string, newline bool) {
	if c.busy {
		return
	}
	if newline || (c.events.IsUnfinished != nil && c.events.IsUnfinished(text)) {
		c.js.Call("insertNewline")
		return
	}
	c.js.Call("clearInput")
	c.Print(platform.ConsoleInput, c.prompt+strings.ReplaceAll(text, "\n", "\n... ")+"\n")
	if strings.TrimSpace(text) != "" {
		c.js.Call("addHistory", text)
	}
	if c.events.OnSubmit != nil {
		c.events.OnSubmit(text)
	}
}

// tab completes the word before the cursor: as far as the candidates
// agree, and with several shows them.
func (c *Console) tab(text string, cursor int) {
	if c.busy || c.events.OnComplete == nil {
		return
	}
	// The browser counts in UTF-16 units, the completer in bytes.
	byteCursor := utf16ToByteOffset(text, cursor)
	start, candidates := c.events.OnComplete(text, byteCursor)
	if len(candidates) == 0 || start < 0 || start > byteCursor {
		return
	}
	common := candidates[0]
	for _, s := range candidates[1:] {
		common = commonPrefix(common, s)
	}
	c.js.Call("replace", byteToUTF16Offset(text, start), cursor, common)
	if len(candidates) > 1 {
		c.Print(platform.ConsoleInfo, strings.Join(candidates, "  ")+"\n")
	}
}

func (c *Console) Events() *platform.ConsoleEvents { return &c.events }

// Print adds text above the prompt.
func (c *Console) Print(kind platform.ConsoleKind, text string) {
	c.js.Call("print", int(kind), text)
}

// SetPrompt sets what stands before the line being typed.
func (c *Console) SetPrompt(prompt string) {
	c.prompt = prompt
	c.js.Call("setPrompt", prompt)
}

// SetBusy says that something runs.
func (c *Console) SetBusy(busy bool) {
	c.busy = busy
	c.js.Call("setBusy", busy)
}

// Clear drops the text.
func (c *Console) Clear() { c.js.Call("clear") }

// SetHistory sets what was entered before, oldest first.
func (c *Console) SetHistory(entries []string) {
	arr := js.Global().Get("Array").New(len(entries))
	for i, e := range entries {
		arr.SetIndex(i, e)
	}
	c.js.Call("setHistory", arr)
}

func (c *Console) SetVisible(visible bool) { c.js.Call("setVisible", visible) }
func (c *Console) IsVisible() bool         { return c.js.Get("visible").Bool() }
func (c *Console) Focus()                  { c.js.Call("focus") }

// FreezePosition and ThawPosition keep nothing: the console is a panel
// under the page, and its height is the browser's to remember.
func (c *Console) FreezePosition(core.Settings, string) {}
func (c *Console) ThawPosition(core.Settings, string)   {}
