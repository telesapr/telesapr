package headless

import (
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/platform"
)

// ConsoleText is a piece of what a console shows.
type ConsoleText struct {
	Kind platform.ConsoleKind
	Text string
}

// Console records what a console is told to show; a test types into
// it with Submit and Interrupt.
type Console struct {
	events platform.ConsoleEvents

	Texts   []ConsoleText
	Prompt  string
	Busy    bool
	Visible bool
	Focused bool
	// Cleared counts the calls of Clear.
	Cleared int
}

var _ platform.Console = (*Console)(nil)
var _ platform.ConsoleHost = (*Platform)(nil)

// CreateConsole makes a console, kept in Consoles.
func (p *Platform) CreateConsole(platform.Window) platform.Console {
	c := &Console{}
	p.Consoles = append(p.Consoles, c)
	return c
}

func (c *Console) Events() *platform.ConsoleEvents { return &c.events }

func (c *Console) Print(kind platform.ConsoleKind, text string) {
	c.Texts = append(c.Texts, ConsoleText{kind, text})
}

func (c *Console) SetPrompt(prompt string)              { c.Prompt = prompt }
func (c *Console) SetBusy(busy bool)                    { c.Busy = busy }
func (c *Console) Clear()                               { c.Texts, c.Cleared = nil, c.Cleared+1 }
func (c *Console) SetVisible(visible bool)              { c.Visible = visible }
func (c *Console) IsVisible() bool                      { return c.Visible }
func (c *Console) Focus()                               { c.Focused = true }
func (c *Console) FreezePosition(core.Settings, string) {}
func (c *Console) ThawPosition(core.Settings, string)   {}

// Submit enters a line as the user does: it is shown after the
// prompt, and given to the console's owner unless the console is
// busy. It reports whether the line was taken.
func (c *Console) Submit(line string) bool {
	if c.Busy {
		return false
	}
	c.Print(platform.ConsoleInput, c.Prompt+line+"\n")
	if c.events.OnSubmit != nil {
		c.events.OnSubmit(line)
	}
	return true
}

// Interrupt asks to stop what runs.
func (c *Console) Interrupt() {
	if c.events.OnInterrupt != nil {
		c.events.OnInterrupt()
	}
}

// Text is everything the console shows.
func (c *Console) Text() string {
	var b strings.Builder
	for _, t := range c.Texts {
		b.WriteString(t.Text)
	}
	return b.String()
}
