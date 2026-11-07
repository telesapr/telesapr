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
	"cmp"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
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
	// Restarted is whether the interpreter was started afresh for this
	// script, as it is after one was stopped: the variables of earlier
	// scripts are gone.
	Restarted bool `json:"restarted,omitempty"`
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
	// group called d.Group if that isn't "" (api.Group.Describe); with
	// d.Solid, the faces and edges of the solid at that group or the
	// active one (api.Group.DescribeSolid); with d.Script, the drawing
	// as the script that builds it (api.Doc.Script). The text comes
	// with the same as data (Description).
	Describe(ctx context.Context, d Describing) (Described, error)
	// Render is a PNG picture of the drawing, as p asks for it, with a
	// caption that says what it shows.
	Render(ctx context.Context, p Picture) (Rendered, error)
	// Measure is the solid of the model up to m.Group (the active group
	// for "") as Analyze > Check Solid reports it, with the gaps and
	// walls m asks about; with m.Selected, it is instead what the user
	// has selected, measured as Analyze > Measure does. The text comes
	// with the same as data (Measurement).
	Measure(ctx context.Context, m Measuring) (Measured, error)
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

// Describing is what Describe tells of.
type Describing struct {
	// Group, if not "", is the one group whose requests and constraints
	// are listed, instead of the active group's.
	Group string `json:"group,omitempty"`
	// Script has the whole drawing returned as the Go program that
	// builds it again, as File > Export > Script writes it; Group and
	// Solid are then not used.
	Script bool `json:"script,omitempty"`
	// Solid has the faces and edges of the solid at Group, or at the
	// active group for "", listed by the names a script picks them by
	// (api.Group.DescribeSolid) instead of the groups.
	Solid bool `json:"solid,omitempty"`
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

// Rendered is a picture and what it shows.
type Rendered struct {
	// PNG is the picture.
	PNG []byte `json:"png,omitempty"`
	// Caption says in a line or two what was drawn: the view, the size,
	// the group, the section and the bounds of the solid in mm.
	Caption string `json:"caption,omitempty"`
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
	// line, if not nil, is told each whole line kept (watchLines), and
	// partial is what came of a line not yet ended.
	line    func(string)
	partial string
}

func (c *Capture) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.on {
		c.buf.Write(p)
		c.passLines(p)
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

var (
	_ Backend        = (*Session)(nil)
	_ ProgressRunner = (*Session)(nil)
)

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
	return s.RunWithProgress(ctx, code, nil)
}

// RunWithProgress is Run that reports, if report is not nil, each line
// the script prints and, with a host that tells it
// (RegenerationWatcher), each group regenerated, at most a few times a
// second; the last report is sent before it returns.
func (s *Session) RunWithProgress(ctx context.Context, code string, report func(Progress)) (Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(code) == "" {
		return Result{}, errors.New("no code to run")
	}

	s.out.Start()
	if report != nil {
		defer s.watch(report)()
	}
	was := s.engine.Generation()
	shown, err := s.engine.Run(ctx, "script", code)
	res := Result{Value: shown, Restarted: s.engine.Generation() != was}
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

// watch passes on to report, paced, what the script prints and the
// groups the host regenerates, until the stop it returns.
func (s *Session) watch(report func(Progress)) (stop func()) {
	p := newPacer(report, reportEvery)
	stopLines := s.out.watchLines(func(line string) {
		p.add(Progress{Message: line, Output: line + "\n"})
	})
	stopRegen := func() {}
	if w, ok := s.engine.Host().(RegenerationWatcher); ok {
		stopRegen = w.WatchRegeneration(func(group string, before, total int) {
			p.add(Progress{Message: regenerationMessage(group, before, total), Done: before + 1, Total: total})
		})
	}
	return func() {
		stopRegen()
		stopLines()
		p.close()
	}
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

// Describe is the drawing, or one group of it, in words, or the
// solid at a group, or the whole drawing as a script.
func (s *Session) Describe(_ context.Context, d Describing) (out Described, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	doc := s.engine.Doc()
	err = try(func() {
		switch {
		case d.Script:
			out.Text, _ = doc.Script()
			out.Data.Script = out.Text
		case d.Solid:
			g := doc.Active()
			if d.Group != "" {
				g = groupNamed(doc, d.Group)
			}
			out.Text, out.Data = g.DescribeSolid(), g.SolidDescription()
		case d.Group == "":
			out.Text, out.Data = doc.Describe(), doc.Description()
		default:
			g := groupNamed(doc, d.Group)
			out.Text, out.Data = g.Describe(), g.Description()
		}
	})
	return out, err
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

// size fills in the picture's size: the default when neither side is
// given, and the other side at 4:3 when only one is.
func (p *Picture) size() {
	switch {
	case p.Width == 0 && p.Height == 0:
		p.Width, p.Height = DefaultWidth, DefaultHeight
	case p.Height == 0 && p.Width > 0:
		p.Height = max(1, (p.Width*3+2)/4)
	case p.Width == 0 && p.Height > 0:
		p.Width = max(1, (p.Height*4+1)/3)
	}
}

// Render is a picture of the drawing.
func (s *Session) Render(_ context.Context, p Picture) (out Rendered, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p.View == "" {
		p.View = DefaultView
	}
	p.size()
	if p.Width < 1 || p.Height < 1 || p.Width > MaxSide || p.Height > MaxSide {
		return Rendered{}, fmt.Errorf("a picture of %d by %d pixels; each side is from 1 to %d", p.Width, p.Height, MaxSide)
	}
	dir, err := os.MkdirTemp("", "telesapr-render")
	if err != nil {
		return Rendered{}, err
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
		out.Caption = caption(doc, p)
	}); err != nil {
		return Rendered{}, err
	}
	if rerr != nil {
		return Rendered{}, rerr
	}
	out.PNG, err = os.ReadFile(file)
	return out, err
}

// caption says what a picture of doc, drawn as p asks and with the
// group p names active, shows.
func caption(doc *api.Doc, p Picture) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s view, %d by %d pixels", p.View, p.Width, p.Height)
	g := doc.Active()
	if p.Group != "" {
		g = doc.Group(p.Group)
	}
	fmt.Fprintf(&b, ", up to %s", g)
	if p.Section != "" {
		fmt.Fprintf(&b, ", cut by a plane parallel to %s at %s", p.Section, cmp.Or(p.SectionOffset, "0"))
		if p.SectionFlip {
			b.WriteString(" with the other side kept")
		}
	}
	if a := g.Analyze(); a.Triangles() > 0 {
		lo, hi := a.Bounds()
		fmt.Fprintf(&b, "; solid bounds %s to %s mm", shortPoint(lo), shortPoint(hi))
	} else {
		b.WriteString("; no solid yet")
	}
	return b.String()
}

// shortPoint is a point as "(x, y, z)", each to three decimals and with no
// "-0", as get_drawing writes one: a bound of -5e-16 is 0.
func shortPoint(p api.Vec) string {
	num := func(v float64) string {
		v = math.Round(v*1000) / 1000
		if v == 0 {
			v = 0
		}
		return strconv.FormatFloat(v, 'g', -1, 64)
	}
	return "(" + num(p.X) + ", " + num(p.Y) + ", " + num(p.Z) + ")"
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
