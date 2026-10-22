// Package agent is what a program that draws by writing scripts needs
// of TeleSAPR, such as an AI model behind an MCP client: it runs a
// script and says what came of it, describes the drawing in words,
// renders a picture of it, measures it, exports and opens files, lists
// a directory, and gives the scripting interface's help. A Backend
// does those things; a Session does them on a script
// engine, Serve and Dial pass them through a socket to the program
// that has the drawing, and package mcpserver offers them as MCP
// tools.
package agent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/telesapr/telesapr/pkg/script"
	"github.com/telesapr/telesapr/pkg/script/api"
)

// Result is what came of running a script.
type Result struct {
	// Output is what the script printed, and what the program had to
	// say while it ran.
	Output string `json:"output,omitempty"`
	// Value is the value of the script's last statement, if that is an
	// expression, as a console shows it.
	Value string `json:"value,omitempty"`
	// Error is why the script stopped, with the line and column it
	// stopped at; "" if it ran to its end.
	Error string `json:"error,omitempty"`
	// RolledBack is whether what the script changed before it stopped
	// was taken back: the drawing is then as it was before the script.
	RolledBack bool `json:"rolledBack,omitempty"`
	// Status is how the drawing is after the script: the active group
	// with how it solved, and every group that is in trouble.
	Status string `json:"status,omitempty"`
	// Groups are the groups Status names, active first, for a program
	// that would rather not read the text.
	Groups []GroupStatus `json:"groups,omitempty"`
	// ExpressionErrors are the parameters and dimensions whose
	// expressions can't be worked out.
	ExpressionErrors []string `json:"expressionErrors,omitempty"`
}

// GroupStatus is how a group came out of a script.
type GroupStatus struct {
	// Name is the group's name, as Doc.Group takes it.
	Name string `json:"name"`
	// Kind is what the group is, as Group.Kind says it.
	Kind string `json:"kind"`
	// Active is whether the group is the one being worked in.
	Active bool `json:"active,omitempty"`
	// Solved is whether the group's sketch solved.
	Solved bool `json:"solved"`
	// DOF is the degrees of freedom the group's sketch has left.
	DOF int `json:"dof"`
	// Problem is why the group made no solid, or "".
	Problem string `json:"problem,omitempty"`
}

// Backend is a drawing as a program that writes scripts sees it. Its
// errors are of what couldn't be done at all, as when another script
// runs or the user refused; a script that ran and failed is a Result.
type Backend interface {
	// Run runs code, Go statements or a whole program, on the drawing
	// as one step of undo.
	Run(ctx context.Context, code string) (Result, error)
	// Describe is the drawing in words (api.Doc.Describe), or the
	// group called group if that isn't "" (api.Group.Describe).
	Describe(ctx context.Context, group string) (string, error)
	// Render is a PNG picture of the drawing, as p asks for it.
	Render(ctx context.Context, p Picture) ([]byte, error)
	// Measure is the solid of the model up to m.Group (the active group
	// for "") as Analyze > Check Solid reports it, with the gaps and
	// walls m asks about; with m.Selected, it is instead what the user
	// has selected, measured as Analyze > Measure does.
	Measure(ctx context.Context, m Measuring) (string, error)
	// Export writes the drawing to path, in the format its extension
	// names (api.FileMenu.Export), and says what it wrote. A relative
	// path is taken from the directory the program with the drawing
	// works in.
	Export(ctx context.Context, path string) (string, error)
	// Open replaces the drawing by the file at path (api.FileMenu.Load),
	// and describes the drawing it opened; a drawing with changes that
	// aren't saved is not replaced (ErrUnsaved).
	Open(ctx context.Context, path string) (string, error)
	// Files lists the directory dir, the working directory for "".
	Files(ctx context.Context, dir string) (Listing, error)
	// Help is what the scripting interface's help says of name, as
	// "Features.Extrude", "Group" or "Features > Extrude"; of "", the Primer.
	Help(ctx context.Context, name string) (string, error)
}

// Picture is what Render draws. A zero size, or "" for the view, is
// the default: 800 by 600, isometric.
type Picture struct {
	// View is "top", "bottom", "left", "right", "front", "back",
	// "isometric", or "window" for the view the window shows.
	View string `json:"view,omitempty"`
	// Group, if not "", has the drawing drawn as it is up to that group,
	// as if it were active.
	Group  string `json:"group,omitempty"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
	// Section, if not "", draws the solid cut by a plane parallel to
	// the reference workplane it names, "XY", "YZ" or "ZX", as View >
	// Section View does, for this picture only. SectionOffset is the
	// plane's distance from the workplane along its normal, a number or
	// an expression in the document's unit; SectionFlip keeps the side
	// the normal points to instead of the other.
	Section       string `json:"section,omitempty"`
	SectionOffset string `json:"sectionOffset,omitempty"`
	SectionFlip   bool   `json:"sectionFlip,omitempty"`
}

// The picture Render makes when it isn't told.
const (
	DefaultView   = "isometric"
	DefaultWidth  = 800
	DefaultHeight = 600
	// MaxSide is the most pixels a picture is wide or high.
	MaxSide = 2400
)

// Capture is a writer that keeps what is written to it between Start
// and Stop, and drops the rest. A script's output goes to one, for
// Session.Run to return.
type Capture struct {
	mu  sync.Mutex
	on  bool
	buf bytes.Buffer
}

func (c *Capture) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.on {
		c.buf.Write(p)
	}
	return len(p), nil
}

// Start begins keeping what is written.
func (c *Capture) Start() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.buf.Reset()
	c.on = true
}

// Stop ends it, and returns what was written since Start.
func (c *Capture) Stop() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.on = false
	return c.buf.String()
}

// Session is a Backend on a script engine: scripts run in the engine,
// one after the other, each seeing what those before it defined.
type Session struct {
	mu     sync.Mutex
	engine *script.Engine
	out    *Capture
}

var _ Backend = (*Session)(nil)

// NewSession is a session on an engine whose scripts write to out:
// the engine was made with out as its Stdout and Stderr, or with
// writers that pass what they get on to it.
func NewSession(engine *script.Engine, out *Capture) *Session {
	return &Session{engine: engine, out: out}
}

// Open makes an engine with opts for a session of its own. What the
// scripts write is the session's to return, so opts.Stdout and
// opts.Stderr are not used; what the host has to say while a script
// runs should go to the returned Capture too.
func Open(opts script.Options) (*Session, *Capture, error) {
	out := &Capture{}
	opts.Stdout, opts.Stderr = out, out
	engine, err := script.New(opts)
	if err != nil {
		return nil, nil, err
	}
	return NewSession(engine, out), out, nil
}

// Engine is the engine the session runs scripts in.
func (s *Session) Engine() *script.Engine { return s.engine }

// rollbacker is a host that can take back a script's step for good.
type rollbacker interface {
	Rollback() bool
}

// try runs fn, and returns as an error what it panics with: package
// api panics with an error when it can't do what it is asked.
func try(fn func()) (err error) {
	defer func() {
		switch r := recover().(type) {
		case nil:
		case error:
			err = r
		default:
			err = fmt.Errorf("%v", r)
		}
	}()
	fn()
	return nil
}

// Run runs code in the session's engine. A script that stops at an
// error has what it changed taken back, so that the script can be
// corrected and run again as a whole.
func (s *Session) Run(ctx context.Context, code string) (Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(code) == "" {
		return Result{}, errors.New("no code to run")
	}

	s.out.Start()
	shown, err := s.engine.Run(ctx, "script", code)
	res := Result{Value: shown}
	var exit script.Exit
	switch {
	case err == nil:
	case errors.Is(err, script.ErrBusy):
		s.out.Stop()
		return Result{}, errors.New("another script is running in TeleSAPR; wait for it to end, or stop it")
	case errors.As(err, &exit) && exit == 0:
		// A script that exits with success has just ended.
	default:
		res.Error = err.Error()
		if errors.Is(err, context.Canceled) {
			res.Error = "the script was stopped"
		}
		doc := s.engine.Doc()
		if rb, ok := s.engine.Host().(rollbacker); ok {
			if rerr := try(func() { doc.Batch(func() { res.RolledBack = rb.Rollback() }) }); rerr != nil {
				res.Error += "\n" + rerr.Error()
			}
		}
	}
	if serr := s.status(&res); serr != nil {
		res.Status = "the drawing's state couldn't be read: " + serr.Error()
	}
	// What the program said while the state was read is the script's
	// doing too.
	res.Output = s.out.Stop()
	return res, nil
}

// firstLine is text up to its first line break.
func firstLine(text string) string {
	line, _, _ := strings.Cut(text, "\n")
	return line
}

// status is the active group with how it solved, and every other
// group that didn't solve or has a problem.
func (s *Session) status(res *Result) error {
	doc := s.engine.Doc()
	return try(func() {
		doc.Batch(func() {
			active := doc.Active()
			lines := []string{"active group " + firstLine(active.Describe())}
			groups := []GroupStatus{groupStatus(active, true)}
			for _, g := range doc.Groups() {
				if g.H == active.H || g.LeftOut() != "" {
					continue
				}
				if !g.Solved() || g.Problem() != "" {
					lines = append(lines, "in trouble: "+firstLine(g.Describe()))
					groups = append(groups, groupStatus(g, false))
				}
			}
			errs := doc.Params.Errors()
			for _, e := range errs {
				lines = append(lines, "expression error: "+e)
			}
			res.Status = strings.Join(lines, "\n")
			res.Groups = groups
			res.ExpressionErrors = errs
		})
	})
}

// groupStatus is how g came out.
func groupStatus(g api.Group, active bool) GroupStatus {
	return GroupStatus{Name: g.Name(), Kind: g.Kind(), Active: active,
		Solved: g.Solved(), DOF: g.Dof(), Problem: g.Problem()}
}

// Describe is the drawing, or one group of it, in words.
func (s *Session) Describe(_ context.Context, group string) (text string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	doc := s.engine.Doc()
	err = try(func() {
		if group == "" {
			text = doc.Describe()
			return
		}
		text = groupNamed(doc, group).Describe()
	})
	return text, err
}

// groupNamed is the group called name, or a panic with an error that
// lists the groups there are.
func groupNamed(doc *api.Doc, name string) api.Group {
	g := doc.Group(name)
	if !g.Valid() {
		panic(fmt.Errorf("no group is called %q; the groups are %v", name, groupNames(doc)))
	}
	return g
}

// groupNames are the groups as Doc.Group takes them.
func groupNames(doc *api.Doc) []string {
	var names []string
	for _, g := range doc.Groups() {
		names = append(names, g.String())
	}
	return names
}

// Render is a picture of the drawing.
func (s *Session) Render(_ context.Context, p Picture) (png []byte, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p.View == "" {
		p.View = DefaultView
	}
	if p.Width == 0 && p.Height == 0 {
		p.Width, p.Height = DefaultWidth, DefaultHeight
	}
	if p.Width < 1 || p.Height < 1 || p.Width > MaxSide || p.Height > MaxSide {
		return nil, fmt.Errorf("a picture of %d by %d pixels; each side is from 1 to %d", p.Width, p.Height, MaxSide)
	}
	dir, err := os.MkdirTemp("", "telesapr-render")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	file := filepath.Join(dir, "view.png")
	var rerr error
	if err := try(func() {
		// The program's own document: the picture is the session's
		// file, not one a guarded script may write.
		doc := api.NewDoc(s.engine.Host())
		if p.Group != "" {
			g := groupNamed(doc, p.Group)
			// Drawn as if active, and the active group put back.
			was := doc.Active()
			doc.Activate(g)
			defer doc.Activate(was)
		}
		if p.Section != "" {
			// The user's own section view, if any, comes back after.
			was := doc.View.Section()
			doc.View.SetSection(doc.Workplane(p.Section), p.SectionOffset, p.SectionFlip)
			defer func() {
				if was.On {
					doc.View.SetSection(was.Plane, was.Offset, was.Flip)
				} else {
					doc.View.ClearSection()
				}
			}()
		}
		rerr = doc.File.Render(file, p.Width, p.Height, p.View)
	}); err != nil {
		return nil, err
	}
	if rerr != nil {
		return nil, rerr
	}
	return os.ReadFile(file)
}

// Help is the scripting interface's help for a name, or the Primer.
func (s *Session) Help(_ context.Context, name string) (string, error) {
	return Help(name)
}

// Help is what any Backend's Help gives: it needs no drawing.
func Help(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Primer(), nil
	}
	var b strings.Builder
	if !script.Help(&b, name) {
		return "", fmt.Errorf(`nothing is called %q; names are like "api", "Doc", "Features", "Features.Extrude", "Features > Extrude", "Group.Solved", "strings.ToUpper"`, name)
	}
	return b.String(), nil
}
