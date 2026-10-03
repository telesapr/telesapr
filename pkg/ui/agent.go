package ui

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/spacesolve/spacesolve/pkg/platform"
	"github.com/spacesolve/spacesolve/pkg/script/agent"
)

// This file lets a program that draws by writing scripts, such as an
// AI model's, work on the drawing on screen: Script > Allow AI
// Connections makes a socket that spacesolve-cli mcp connects to, and
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
func (ss *SolveSpaceUI) AgentListening() bool { return ss.agent != nil }

// AgentSocket is the socket programs connect to, "" when none may.
func (ss *SolveSpaceUI) AgentSocket() string {
	if ss.agent == nil {
		return ""
	}
	return ss.agent.path
}

// AllowAgent lets programs that draw by writing scripts connect, or
// with false stops listening for them and ends the connections there
// are; a script of theirs that runs is stopped. It doesn't change the
// setting, which is the menu's to keep.
func (ss *SolveSpaceUI) AllowAgent(allow bool) error {
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
var errLoopEnded = errors.New("spacesolve is closing")

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
		if err := c.ensureEngine(); err != nil {
			return err
		}
		if c.busy {
			return errors.New("another script is running in spacesolve; wait for it to end, or ask the user to stop it with Script > Stop")
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
			res, err := c.session.Run(runCtx, code)
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

// session is the console's session, made if need be.
func (b consoleBackend) session() (*agent.Session, error) {
	if err := b.onLoop(b.c.ensureEngine); err != nil {
		return nil, err
	}
	return b.c.session, nil
}

func (b consoleBackend) Describe(ctx context.Context, group string) (string, error) {
	s, err := b.session()
	if err != nil {
		return "", err
	}
	return s.Describe(ctx, group)
}

func (b consoleBackend) Render(ctx context.Context, view string, width, height int) ([]byte, error) {
	s, err := b.session()
	if err != nil {
		return nil, err
	}
	return s.Render(ctx, view, width, height)
}

func (b consoleBackend) Help(_ context.Context, name string) (string, error) {
	return agent.Help(name)
}
