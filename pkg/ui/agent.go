package ui

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/script/agent"
)

// This file lets a program that draws by writing scripts, such as an
// AI model's, work on the drawing on screen: Script > Allow AI
// Connections makes a socket that telesapr-cli mcp connects to, and
// what comes through it runs in the script console, where the user
// sees it.

// agentServer listens for such programs.
type agentServer struct {
	listener net.Listener
	// path is the socket's.
	path string
	// stop ends the connections that are open.
	stop context.CancelFunc
}

// AgentListening reports whether a program may connect now.
func (ss *Application) AgentListening() bool { return ss.agent != nil }

// AgentSocket is the socket programs connect to, "" when none may.
func (ss *Application) AgentSocket() string {
	if ss.agent == nil {
		return ""
	}
	return ss.agent.path
}

// AllowAgent lets programs that draw by writing scripts connect, or
// with false stops listening for them and ends the connections there
// are; a script of theirs that runs is stopped. It doesn't change the
// setting, which is the menu's to keep.
func (ss *Application) AllowAgent(allow bool) error {
	if allow == (ss.agent != nil) {
		return nil
	}
	defer func() {
		if item := ss.GW.scriptAgentMenuItem; item != nil {
			item.SetActive(ss.agent != nil)
		}
	}()
	if !allow {
		a := ss.agent
		ss.agent = nil
		if c := ss.console; c != nil && !c.busy {
			c.dropAgentSession()
		}
		_ = a.listener.Close()
		// The connections end on their own goroutines: one may wait
		// for this one, the event loop's, to run its script.
		a.stop()
		return nil
	}

	dir := ss.AgentSocketDir
	if dir == "" {
		var err error
		if dir, err = agent.SocketDir(); err != nil {
			return err
		}
	}
	l, path, err := agent.Listen(dir)
	if err != nil {
		return err
	}
	ctx, stop := context.WithCancel(context.Background())
	ss.agent = &agentServer{listener: l, path: path, stop: stop}
	go agent.Accept(ctx, l, consoleBackend{ss.Console()})
	return nil
}

// consoleBackend is the application as a program that writes scripts
// sees it: the scripts run in the script console. Its methods are
// called on the goroutines of the connections, and do their work on
// the event loop's.
type consoleBackend struct {
	c *ScriptConsole
}

var _ agent.Backend = consoleBackend{}

// errLoopEnded is the error of a request that came as the application
// closed.
var errLoopEnded = errors.New("TeleSAPR is closing")

// agentPrompt stands before a script of a program in the console, as
// the prompt does before what was typed.
const agentPrompt = "ai> "

// onLoop runs fn on the event loop's goroutine and waits for it; fn
// returns an error if what was asked can't be done.
func (b consoleBackend) onLoop(fn func() error) error {
	c := b.c
	if c.poster == nil {
		return fn()
	}
	done := make(chan error, 1)
	c.poster.PostToLoop(func() { done <- fn() })
	select {
	case err := <-done:
		return err
	case <-c.poster.LoopDone():
		return errLoopEnded
	}
}

// confirmed asks the user whether a program's script may run, if the
// setting says to ask.
func (b consoleBackend) confirmed(code string) bool {
	ss := b.c.ss
	if !ss.AgentConfirm {
		return true
	}
	// A long script is shown by its start; the console has all of it.
	const most = 30
	lines := strings.Split(strings.TrimRight(code, "\n"), "\n")
	shown := lines
	if len(lines) > most {
		shown = append(append([]string{}, lines[:most]...), fmt.Sprintf("… and %d more lines", len(lines)-most))
	}
	dialog := ss.Platform.CreateMessageDialog(ss.GW.window)
	dialog.SetType(platform.DialogQuestion)
	dialog.SetTitle("AI Script")
	dialog.SetMessage("An AI program wants to run this script on the drawing:")
	dialog.SetDescription(strings.Join(shown, "\n"))
	dialog.AddButton("&Run", platform.ResponseYes, true)
	dialog.AddButton("Do&n't Run", platform.ResponseNo, false)
	return dialog.RunModal() == platform.ResponseYes
}

// Run runs a program's script as the console runs what is typed: one
// at a time, shown in the console with what it prints, and stopped by
// Script > Stop.
func (b consoleBackend) Run(ctx context.Context, code string) (agent.Result, error) {
	c := b.c
	type outcome struct {
		res agent.Result
		err error
	}
	done := make(chan outcome, 1)
	err := b.onLoop(func() error {
		session, err := c.agentSession()
		if err != nil {
			return err
		}
		if c.busy {
			return errors.New("another script is running in TeleSAPR; wait for it to end, or ask the user to stop it with Script > Stop")
		}
		if !b.confirmed(code) {
			c.print(platform.ConsoleInfo, "A script of an AI program was refused.\n")
			return errors.New("the user refused to run the script; ask what to do instead")
		}
		shown := strings.ReplaceAll(strings.TrimRight(code, "\n"), "\n", "\n"+strings.Repeat(" ", len(agentPrompt)))
		c.print(platform.ConsoleInput, agentPrompt+shown+"\n")
		c.start(func(jobCtx context.Context) func() {
			// The program's request ending stops the script too.
			runCtx, cancel := context.WithCancel(jobCtx)
			stop := context.AfterFunc(ctx, cancel)
			res, err := session.Run(runCtx, code)
			stop()
			cancel()
			return func() {
				switch {
				case err != nil:
					c.print(platform.ConsoleError, "error: "+err.Error()+"\n")
				case res.Error != "":
					c.print(platform.ConsoleError, "error: "+res.Error+"\n")
					if res.RolledBack {
						c.print(platform.ConsoleInfo, "What the script changed was taken back.\n")
					}
				case res.Value != "":
					c.print(platform.ConsoleValue, res.Value+"\n")
				}
				done <- outcome{res, err}
			}
		})
		return nil
	})
	if err != nil {
		return agent.Result{}, err
	}
	var loopDone <-chan struct{}
	if c.poster != nil {
		loopDone = c.poster.LoopDone()
	}
	select {
	case o := <-done:
		return o.res, o.err
	case <-loopDone:
		return agent.Result{}, errLoopEnded
	}
}

// session is the session the program's scripts run in, made if need
// be.
func (b consoleBackend) session() (s *agent.Session, err error) {
	err = b.onLoop(func() (err error) {
		s, err = b.c.agentSession()
		return err
	})
	return s, err
}

func (b consoleBackend) Describe(ctx context.Context, group string) (string, error) {
	s, err := b.session()
	if err != nil {
		return "", err
	}
	return s.Describe(ctx, group)
}

func (b consoleBackend) Render(ctx context.Context, p agent.Picture) ([]byte, error) {
	s, err := b.session()
	if err != nil {
		return nil, err
	}
	return s.Render(ctx, p)
}

func (b consoleBackend) Measure(ctx context.Context, group string, selected bool) (string, error) {
	s, err := b.session()
	if err != nil {
		return "", err
	}
	return s.Measure(ctx, group, selected)
}

func (b consoleBackend) Files(ctx context.Context, dir string) (agent.Listing, error) {
	s, err := b.session()
	if err != nil {
		return agent.Listing{}, err
	}
	return s.Files(ctx, dir)
}

// Export writes a file the program asks for, as a script calling
// File.Export would, so the user is asked first as for a script, and
// the console shows it.
func (b consoleBackend) Export(ctx context.Context, path string) (string, error) {
	s, err := b.allowed("export to "+path, fmt.Sprintf("doc.File.Export(%q)", path))
	if err != nil {
		return "", err
	}
	return s.Export(ctx, path)
}

// Open replaces the drawing by a file, as File.Load would, but not a
// drawing with unsaved changes (agent.ErrUnsaved).
func (b consoleBackend) Open(ctx context.Context, path string) (string, error) {
	// Asked first, the user would be asked for what is then refused.
	s, err := b.session()
	if err != nil {
		return "", err
	}
	if s.Engine().Doc().File.Unsaved() {
		return "", agent.ErrUnsaved
	}
	if s, err = b.allowed("open "+path, fmt.Sprintf("doc.File.Load(%q)", path)); err != nil {
		return "", err
	}
	return s.Open(ctx, path)
}

// allowed asks the user, if the setting says to, whether a program may
// do what, shown as the script that would do the same, and puts that in
// the console; it gives the session to do it with.
func (b consoleBackend) allowed(what, asScript string) (*agent.Session, error) {
	c := b.c
	err := b.onLoop(func() error {
		if err := c.ensureEngine(); err != nil {
			return err
		}
		if c.busy {
			return errors.New("another script is running in TeleSAPR; wait for it to end, or ask the user to stop it with Script > Stop")
		}
		if !b.confirmed(asScript) {
			c.print(platform.ConsoleInfo, "An AI program asked to "+what+"; it was refused.\n")
			return errors.New("the user refused; ask what to do instead")
		}
		c.print(platform.ConsoleInput, agentPrompt+asScript+"\n")
		return nil
	})
	if err != nil {
		return nil, err
	}
	return c.session, nil
}

func (b consoleBackend) Help(_ context.Context, name string) (string, error) {
	return agent.Help(name)
}
