package ui

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/platform"
	"github.com/spacesolve/spacesolve/pkg/script"
	"github.com/spacesolve/spacesolve/pkg/script/agent"
)

// ScriptConsole is the application's console for scripts: the window
// where Go is typed a line at a time and run on the sketch, and where
// script files show what they print. Each line, or file, runs on a
// goroutine of its own and reaches the sketch through the event loop,
// so the windows stay alive under a script that takes long; one runs
// at a time.
type ScriptConsole struct {
	ss     *SolveSpaceUI
	window platform.Console
	poster platform.LoopPoster
	engine *script.Engine

	// busy is whether something runs, and cancel stops it.
	busy   bool
	cancel context.CancelFunc

	// session runs the scripts of a program that draws by writing
	// them, in the console's engine; capture keeps what such a script
	// writes, to return it to the program.
	session *agent.Session
	capture *agent.Capture
}

const (
	consolePrompt     = ">>> "
	consoleContPrompt = "... "
)

const consoleGreeting = `Go on the sketch: doc is the drawing. Try
    doc.Groups()                              the groups
    l := doc.Line(V(0, 0, 0), V(40, 0, 0))    draw, in mm
    doc.Length(l, 50)                         constrain
    doc.Selection()                           what is selected
help() says more.
`

// Console is the application's console for scripts, made when first
// asked for. Its window is nil on a platform without consoles.
func (ss *SolveSpaceUI) Console() *ScriptConsole {
	if ss.console != nil {
		return ss.console
	}
	c := &ScriptConsole{ss: ss, capture: &agent.Capture{}}
	ss.console = c
	c.poster, _ = ss.Platform.(platform.LoopPoster)
	if host, ok := ss.Platform.(platform.ConsoleHost); ok {
		c.window = host.CreateConsole(ss.GW.window)
		ev := c.window.Events()
		ev.OnSubmit = c.Submit
		ev.OnInterrupt = c.Stop
		ev.OnClose = func() { c.SetVisible(false) }
		c.window.ThawPosition(ss.Settings, "ScriptConsole")
		c.window.Print(platform.ConsoleInfo, consoleGreeting)
		c.window.SetPrompt(consolePrompt)
	}
	return c
}

// consoleWriter shows what is written to it in the console; a script
// writes from its own goroutine.
type consoleWriter struct {
	c    *ScriptConsole
	kind platform.ConsoleKind
}

func (w consoleWriter) Write(p []byte) (int, error) {
	_, _ = w.c.capture.Write(p)
	text := string(p)
	w.c.onLoop(func() { w.c.print(w.kind, text) })
	return len(p), nil
}

// onLoop runs fn on the event loop's goroutine, later if there is a
// loop and now if not.
func (c *ScriptConsole) onLoop(fn func()) {
	if c.poster != nil {
		c.poster.PostToLoop(fn)
		return
	}
	fn()
}

func (c *ScriptConsole) print(kind platform.ConsoleKind, text string) {
	if c.window != nil {
		c.window.Print(kind, text)
	}
}

// options are what scripts of the console run with; args is a script
// file's os.Args.
func (c *ScriptConsole) options(args []string) script.Options {
	host := c.ss.ScriptHost()
	host.Messages = consoleWriter{c, platform.ConsoleInfo}
	return script.Options{
		Host:   host,
		Stdout: consoleWriter{c, platform.ConsoleOutput},
		Stderr: consoleWriter{c, platform.ConsoleError},
		Args:   args,
	}
}

// Visible reports whether the console's window is shown.
func (c *ScriptConsole) Visible() bool {
	return c.window != nil && c.window.IsVisible()
}

// SetVisible shows or hides the console's window.
func (c *ScriptConsole) SetVisible(visible bool) {
	if c.window == nil {
		if visible {
			c.ss.Error("There is no script console on this platform.")
		}
		return
	}
	c.window.SetVisible(visible)
	if visible {
		c.window.Focus()
	}
	if item := c.ss.GW.scriptConsoleMenuItem; item != nil {
		item.SetActive(visible)
	}
}

// Busy reports whether a script is running.
func (c *ScriptConsole) Busy() bool { return c.busy }

// start runs job as the console's script, on its own goroutine if
// there is an event loop to keep alive; job returns what is to be
// done on the loop when it is over, such as showing its result. It is
// false if a script already runs.
func (c *ScriptConsole) start(job func(ctx context.Context) (done func())) bool {
	if c.busy {
		c.print(platform.ConsoleInfo, "A script is running; stop it first.\n")
		return false
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.busy, c.cancel = true, cancel
	if c.window != nil {
		c.window.SetBusy(true)
	}
	run := func() {
		done := job(ctx)
		c.onLoop(func() {
			if done != nil {
				done()
			}
			cancel()
			c.busy, c.cancel = false, nil
			if c.window != nil {
				c.window.SetBusy(false)
			}
		})
	}
	if c.poster != nil {
		go run()
	} else {
		run()
	}
	return true
}

// showError shows the error of a script, if it is one to show.
func (c *ScriptConsole) showError(err error) {
	var exit script.Exit
	switch {
	case err == nil:
	case errors.Is(err, context.Canceled):
		c.print(platform.ConsoleInfo, "Stopped.\n")
	case errors.As(err, &exit):
		// A script that exits has ended; the application goes on.
		c.print(platform.ConsoleInfo, exit.Error()+"\n")
	default:
		c.print(platform.ConsoleError, "error: "+err.Error()+"\n")
	}
}

// ensureEngine makes the console's interpreter when it is first
// needed.
func (c *ScriptConsole) ensureEngine() error {
	if c.engine != nil {
		return nil
	}
	e, err := script.New(c.options(nil))
	if err != nil {
		return err
	}
	c.engine = e
	c.session = agent.NewSession(e, c.capture)
	return nil
}

// Submit takes what was entered in the console, one line or several:
// each line is run once it, with the lines before it, makes something
// complete, and the first error stops the rest.
func (c *ScriptConsole) Submit(text string) {
	if err := c.ensureEngine(); err != nil {
		c.showError(err)
		return
	}
	c.start(func(ctx context.Context) func() {
		more := false
		for _, line := range strings.Split(text, "\n") {
			shown, m, err := c.engine.Feed(ctx, line)
			more = m
			// Shown now, in its place among what the script prints.
			c.onLoop(func() {
				if shown != "" {
					c.print(platform.ConsoleValue, shown+"\n")
				}
				c.showError(err)
			})
			if err != nil {
				break
			}
		}
		return func() {
			if c.window == nil {
				return
			}
			if more {
				c.window.SetPrompt(consoleContPrompt)
			} else {
				c.window.SetPrompt(consolePrompt)
			}
		}
	})
}

// RunFile runs a script file with arguments; what it prints shows in
// the console. A file is a program of its own: it doesn't see what
// was typed in the console, nor the console what it defines.
func (c *ScriptConsole) RunFile(path string, args ...string) {
	c.print(platform.ConsoleInput, "run "+strings.Join(append([]string{path}, args...), " ")+"\n")
	opts := c.options(append([]string{path}, args...))
	c.start(func(ctx context.Context) func() {
		err := script.RunFile(ctx, opts, path)
		return func() {
			c.showError(err)
			if err == nil {
				c.print(platform.ConsoleInfo, filepath.Base(path)+" done.\n")
			}
		}
	})
}

// Stop stops the script that runs, at its next statement; with none
// running it drops the statement that is half typed.
func (c *ScriptConsole) Stop() {
	switch {
	case c.busy:
		c.cancel()
	case c.engine != nil && c.engine.Pending():
		c.engine.Reset()
		c.print(platform.ConsoleInfo, "Dropped.\n")
		if c.window != nil {
			c.window.SetPrompt(consolePrompt)
		}
	}
}

// MenuScript runs a command of the Script menu.
func (ss *SolveSpaceUI) MenuScript(id Command) {
	c := ss.Console()
	switch id {
	case CmdScriptConsole:
		c.SetVisible(!c.Visible())

	case CmdScriptRun:
		dialog := ss.Platform.CreateOpenFileDialog(ss.GW.window)
		dialog.AddFilters(platform.ScriptFileFilters)
		dialog.ThawChoices(ss.Settings, "Script")
		if !dialog.RunModal() {
			break
		}
		dialog.FreezeChoices(ss.Settings, "Script")
		c.SetVisible(true)
		c.RunFile(dialog.GetFilename())

	case CmdScriptStop:
		c.Stop()

	case CmdScriptAgent:
		allow := !ss.AgentListening()
		if err := ss.AllowAgent(allow); err != nil {
			ss.Error("AI connections can't be allowed: %v", err)
			break
		}
		ss.AgentAllowed = allow
		core.FreezeBool(ss.Settings, "AgentAllowed", allow)

	default:
		panic("ui: unexpected menu ID")
	}
}
