// Package script runs Go scripts on a sketch with the yaegi
// interpreter: a line at a time for a console, or a file. What a
// script sees of the sketch is package api; this package is the
// interpreter around it.
package script

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/constant"
	"go/parser"
	"go/scanner"
	"go/token"
	"io"
	"log"
	"os"
	"path"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/traefik/yaegi/interp"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/script/symbols"
)

// apiPath is the import path of the package scripts use.
const apiPath = "github.com/telesapr/telesapr/pkg/script/api"

// showPath is the import path of a package that only the consoles of
// this package use.
const showPath = "github.com/telesapr/telesapr/pkg/script/console"

// Options say what an Engine runs scripts on.
type Options struct {
	// Host owns the sketch that is the scripts' document.
	Host api.Host
	// Stdout and Stderr are where a script's output goes, by default
	// the program's; Stdin is what it reads, by default nothing.
	Stdin          io.Reader
	Stdout, Stderr io.Writer
	// Args is the script's os.Args.
	Args []string
	// Open, if set, opens another sketch for a script that asks for
	// one; without it such a sketch has no application around it, and
	// can be exported as a mesh only.
	Open func(path string) (api.Host, error)
	// New, if set, likewise makes a new sketch.
	New func() api.Host
}

// evalHost is a host that is told where each step of a script starts
// and ends, to make it one step of undo and to show what it did.
type evalHost interface {
	BeginEval()
	EndEval()
}

// Engine is an interpreter with a document. It runs one thing at a
// time; only Cancel and Running may be called while it does.
type Engine struct {
	opts Options
	doc  *api.Doc
	i    *interp.Interpreter

	mu      sync.Mutex
	cancel  context.CancelFunc
	running bool

	// pending is the lines of a statement that isn't complete yet.
	pending []string

	// panicAt is where the interpreter said the last panic was.
	panicAt struct{ line, col int }

	// values are what the last expression of a console gave.
	values []any
	// ran is whether the last source evaluated got to run, rather than
	// failing before.
	ran bool
}

// prelude is evaluated before a console's first line, and before a
// script that isn't a whole program: doc is the document, and the
// names of package api need no prefix.
const prelude = `import . "` + apiPath + `"
import telesaprConsole "` + showPath + `"
var doc = Current()
func help(x ...any) { telesaprConsole.Help(x...) }
`

// helpText is what help() prints.
const helpText = `doc is the sketch. Lengths are in mm, angles in degrees.

Look around
    doc.Groups()  doc.Active()  doc.Group("extrude")  doc.Selection()
    g.Requests()  g.Entities()  g.Constraints()  g.Solved()  g.Dof()
    e.Kind()  e.Pos()  e.Start()  e.End()  e.Center()  e.Length()  e.Radius()
Draw, in the active group and workplane
    doc.Point(p)  doc.Line(a, b)  doc.Circle(c, r)  doc.Arc(c, start, end)
    doc.Rect(a, b)  doc.Polyline(closed, p...)  doc.Cubic(p0, p1, p2, p3)
    doc.Text(s, "", at, height)  doc.Image(path, at, height)
    doc.TangentArc(corner, radius, trim)  doc.Split(a, b, near)
    V(x, y, z) is a point; doc.UV(u, v) one in the active workplane
Constrain
    doc.Coincident(p, q)  doc.Distance(a, b, mm)  doc.Length(line, mm)
    doc.Diameter(c, mm)  doc.Radius(c, mm)  doc.Angle(a, b, degrees)
    doc.Horizontal(l)  doc.Vertical(l)  doc.Parallel(a, b)  doc.Perpendicular(a, b)
    doc.Equal(a, b)  doc.Tangent(a, b)  doc.On(p, e)  doc.Midpoint(p, l)
    doc.Symmetric(p, q, about)  doc.Lock(p)  c.Set(value)  c.Delete()
Make solids, each a new group
    doc.Extrude(depth, name)  doc.Lathe(axis, name)  doc.Revolve(axis, degrees, name)
    doc.Helix(axis, degrees, length, name)
    doc.Translate(n, V(dx, dy, dz), name)  doc.Rotate(n, center, axis, degrees, name)
    doc.SketchIn(doc.Workplane("XY"), name)  doc.Sketch3D(name)  g.SetCombine(Difference)
    g.Volume()  g.Bounds()  g.Check() says whether the solid is sound
Files
    doc.Save("part.tlspr")  doc.Export("part.stl")  .step .svg .dxf .pdf .png
    doc.Undo()  doc.Redo()

help(x) says what a value is and what it can do: help(doc), help(doc.Active()).
help("name") says what a package, a type, a function or a method is:
help("api"), help("Group"), help("Group.Bounds"), help("strings.ToUpper").

The packages fmt, math, strings, os and others of Go are there without an
import. The whole of it is package github.com/telesapr/telesapr/pkg/script/api.
`

// New makes an interpreter for a console or for a script given as
// statements. It imports the packages a script is likely to use, so
// that fmt.Println and math.Pi work without an import.
func New(opts Options) (*Engine, error) {
	e, err := newEngine(opts)
	if err != nil {
		return nil, err
	}
	e.i.ImportUsed()
	if _, err := e.i.Eval(prelude); err != nil {
		return nil, fmt.Errorf("script: prelude: %w", err)
	}
	return e, nil
}

func newEngine(opts Options) (*Engine, error) {
	if opts.Host == nil {
		return nil, errors.New("script: no host")
	}
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	if opts.Stdin == nil {
		opts.Stdin = strings.NewReader("")
	}
	if opts.Args == nil {
		opts.Args = []string{"script"}
	}
	e := &Engine{opts: opts, doc: api.NewDoc(opts.Host)}
	e.i = interp.New(interp.Options{
		Stdin:  opts.Stdin,
		Stdout: opts.Stdout,
		Stderr: &panicFilter{e: e, w: opts.Stderr},
		Args:   opts.Args,
		Env:    os.Environ(),
	})
	if err := e.i.Use(e.exports()); err != nil {
		return nil, fmt.Errorf("script: %w", err)
	}
	return e, nil
}

// exports is what the interpreter can import: the generated tables,
// with package api's Current, Open and New bound to this engine.
func (e *Engine) exports() interp.Exports {
	exports := make(interp.Exports, len(symbols.Symbols))
	for path, pkg := range symbols.Symbols {
		exports[path] = typedFloats(pkg)
	}
	key := apiPath + "/api"
	pkg := make(map[string]reflect.Value, len(symbols.Symbols[key]))
	for name, v := range exports[key] {
		pkg[name] = v
	}
	pkg["Current"] = reflect.ValueOf(func() *api.Doc { return e.doc })
	if e.opts.Open != nil {
		pkg["Open"] = reflect.ValueOf(func(path string) (*api.Doc, error) {
			h, err := e.opts.Open(path)
			if err != nil {
				return nil, err
			}
			return api.NewDoc(h), nil
		})
	}
	if e.opts.New != nil {
		pkg["New"] = reflect.ValueOf(func() *api.Doc { return api.NewDoc(e.opts.New()) })
	}
	exports[key] = pkg

	// The interpreter gives a script its own fmt.Print, but os.Stdout
	// and the others only if they are files; here they are whatever
	// the script's output is, a console's window for one.
	std := make(map[string]reflect.Value, len(symbols.Symbols["os/os"]))
	for name, v := range symbols.Symbols["os/os"] {
		std[name] = v
	}
	stdin, stdout, stderr := e.opts.Stdin, e.opts.Stdout, e.opts.Stderr
	if _, ok := stdin.(*os.File); !ok {
		std["Stdin"] = reflect.ValueOf(&stdin).Elem()
	}
	if _, ok := stdout.(*os.File); !ok {
		std["Stdout"] = reflect.ValueOf(&stdout).Elem()
	}
	if _, ok := stderr.(*os.File); !ok {
		std["Stderr"] = reflect.ValueOf(&stderr).Elem()
	}
	exports["os/os"] = std

	// What a console wraps an expression in to get all of its values.
	exports[showPath+"/console"] = map[string]reflect.Value{
		"Show": reflect.ValueOf(func(values ...any) { e.values = values }),
		"Take": reflect.ValueOf(func(i int) any { return e.values[i] }),
		"Help": reflect.ValueOf(e.help),
	}
	return exports
}

// typedFloats returns a package's symbols with its untyped float
// constants, such as math.Pi, made float64 values. The interpreter
// gives such a constant the type "untyped float64", which it won't mix
// with a literal: 2.0 * math.Pi fails to compile. As float64 it mixes
// with literals; the interpreter still takes it in a const declaration
// and in float32(math.Pi), but no longer in var r float32 = math.Pi.
func typedFloats(pkg map[string]reflect.Value) map[string]reflect.Value {
	var out map[string]reflect.Value
	for name, v := range pkg {
		c, ok := v.Interface().(constant.Value)
		if !ok || c.Kind() != constant.Float {
			continue
		}
		if out == nil {
			out = make(map[string]reflect.Value, len(pkg))
			for name, v := range pkg {
				out[name] = v
			}
		}
		f, _ := constant.Float64Val(c)
		out[name] = reflect.ValueOf(f)
	}
	if out == nil {
		return pkg
	}
	return out
}

// Doc is the document scripts run on.
func (e *Engine) Doc() *api.Doc { return e.doc }

// Running reports whether a script is running.
func (e *Engine) Running() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.running
}

// Cancel stops the script that is running, at its next statement.
func (e *Engine) Cancel() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cancel != nil {
		e.cancel()
	}
}

// ErrBusy is the error of running a script while another runs.
var ErrBusy = errors.New("a script is running")

// step runs fn as one step: one undo step, shown when it is done.
func (e *Engine) step(ctx context.Context, fn func(ctx context.Context) (reflect.Value, error)) (reflect.Value, error) {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return reflect.Value{}, ErrBusy
	}
	ctx, cancel := context.WithCancel(ctx)
	e.running, e.cancel = true, cancel
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		e.running, e.cancel = false, nil
		e.mu.Unlock()
		cancel()
	}()

	host := e.opts.Host
	if eh, ok := host.(evalHost); ok {
		host.Run(eh.BeginEval)
		defer host.Run(eh.EndEval)
	}
	return fn(ctx)
}

// Eval runs source: statements, declarations or an expression, whose
// value it returns.
func (e *Engine) Eval(ctx context.Context, src string) (reflect.Value, error) {
	return e.step(ctx, func(ctx context.Context) (reflect.Value, error) {
		return e.eval(ctx, src)
	})
}

// eval runs source in the interpreter and says where an error is.
func (e *Engine) eval(ctx context.Context, src string) (reflect.Value, error) {
	e.panicAt.line = 0
	// The interpreter logs some of what it can't compile, besides
	// returning the error.
	logged := log.Writer()
	log.SetOutput(io.Discard)
	v, err := e.i.EvalWithContext(ctx, src)
	log.SetOutput(logged)
	// The interpreter says where a panic of the script was; one without
	// a place is its own, from before the script ran.
	var p interp.Panic
	e.ran = err == nil || ctx.Err() != nil || (errors.As(err, &p) && e.panicAt.line != 0)
	return v, e.locate(err, src)
}

// definition is a statement that defines variables from one
// expression: "a, b := f()" or "var a = f()", with a type or not.
type definition struct {
	names []string
	typ   string
	value string
	// call is whether the value is one call, which may have several
	// values.
	call bool
	// line and col are where the expression starts in the statement,
	// from 1.
	line, col int
}

// parseDefinition reads src as a definition, if it is one.
func parseDefinition(src string) (d definition, ok bool) {
	const prefix = "package p\nfunc _() {\n"
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", prefix+src+"\n}", 0)
	if err != nil {
		return d, false
	}
	body := f.Decls[0].(*ast.FuncDecl).Body.List
	if len(body) != 1 {
		return d, false
	}
	text := func(n ast.Node) string {
		return src[fset.Position(n.Pos()).Offset-len(prefix) : fset.Position(n.End()).Offset-len(prefix)]
	}
	// values is the text of the expressions, one or one to a name, as
	// "a, b := s[0], s[1]" has.
	values := func(xs []ast.Expr) string {
		return src[fset.Position(xs[0].Pos()).Offset-len(prefix) : fset.Position(xs[len(xs)-1].End()).Offset-len(prefix)]
	}
	var lhs []ast.Expr
	switch stmt := body[0].(type) {
	case *ast.AssignStmt:
		if stmt.Tok != token.DEFINE || (len(stmt.Rhs) != 1 && len(stmt.Rhs) != len(stmt.Lhs)) {
			return d, false
		}
		lhs = stmt.Lhs
		d.value = values(stmt.Rhs)
		_, d.call = stmt.Rhs[0].(*ast.CallExpr)
		d.call = d.call && len(stmt.Rhs) == 1
	case *ast.DeclStmt:
		decl, isGen := stmt.Decl.(*ast.GenDecl)
		if !isGen || decl.Tok != token.VAR || len(decl.Specs) != 1 {
			return d, false
		}
		spec := decl.Specs[0].(*ast.ValueSpec)
		if len(spec.Values) != 1 && (len(spec.Values) != len(spec.Names) || spec.Type != nil) {
			return d, false
		}
		for _, name := range spec.Names {
			lhs = append(lhs, name)
		}
		if spec.Type != nil {
			d.typ = text(spec.Type)
		}
		d.value = values(spec.Values)
		_, d.call = spec.Values[0].(*ast.CallExpr)
		d.call = d.call && len(spec.Values) == 1
	default:
		return d, false
	}
	at := strings.LastIndex(src, d.value)
	d.line = 1 + strings.Count(src[:at], "\n")
	d.col = at - strings.LastIndex(src[:at], "\n")
	for _, x := range lhs {
		name, isIdent := x.(*ast.Ident)
		if !isIdent {
			return d, false
		}
		d.names = append(d.names, name.Name)
	}
	return d, true
}

// typeName spells a type as a script does, if a script can: its named
// types must be ones a script can import.
func typeName(t reflect.Type) (string, bool) {
	if t == nil {
		return "", false
	}
	if t.Name() != "" {
		if t.PkgPath() == "" {
			return t.Name(), true
		}
		pkg := path.Base(t.PkgPath())
		if _, known := symbols.Symbols[t.PkgPath()+"/"+pkg][t.Name()]; !known {
			return "", false
		}
		return pkg + "." + t.Name(), true
	}
	switch t.Kind() {
	case reflect.Slice, reflect.Pointer, reflect.Array:
		elem, ok := typeName(t.Elem())
		switch t.Kind() {
		case reflect.Slice:
			return "[]" + elem, ok
		case reflect.Pointer:
			return "*" + elem, ok
		}
		return fmt.Sprintf("[%d]%s", t.Len(), elem), ok
	case reflect.Map:
		key, ok1 := typeName(t.Key())
		elem, ok2 := typeName(t.Elem())
		return "map[" + key + "]" + elem, ok1 && ok2
	}
	return "", false
}

// run runs a statement or a declaration. The interpreter works out
// the type of a variable defined outside a function before it runs
// anything, and can't for some values of this program's types, such as
// "g := doc.Groups()[0]"; such a definition is then made from the
// value itself: the expression is evaluated, the variable declared
// with the type the value has, and the value assigned.
func (e *Engine) run(ctx context.Context, src string) error {
	if inBlocks(src) {
		return e.inFunc(ctx, src)
	}
	if d, ok := parseDefinition(src); ok && d.call {
		if err := e.tooMany(d); err != nil {
			return err
		}
	}
	_, err := e.eval(ctx, src)
	if err == nil || e.ran || Incomplete(err, src) {
		return err
	}
	if parts, ok := split(src, err); ok {
		for _, p := range parts {
			if err := e.run(ctx, p.src); err != nil {
				return p.place(err)
			}
		}
		return nil
	}
	d, ok := parseDefinition(src)
	if !ok {
		return err
	}
	if d.typ != "" {
		// "var a T = x" is "var a T" and "a = x".
		if len(d.names) != 1 {
			return err
		}
		if _, derr := e.eval(ctx, "var "+d.names[0]+" "+d.typ); derr != nil {
			return err
		}
		_, derr := e.eval(ctx, d.names[0]+" = "+d.value)
		return derr
	}

	e.values = nil
	const wrapper = "telesaprConsole.Show("
	if _, derr := e.eval(ctx, wrapper+d.value+")"); derr != nil {
		// What is wrong with the expression says more than that the
		// definition couldn't be made; its place is counted from the
		// statement's start.
		var serr *Error
		if errors.As(derr, &serr) && serr.Line > 0 {
			if serr.Line == 1 {
				serr.Col += d.col - 1 - len(wrapper)
			}
			serr.Line += d.line - 1
		}
		internal := strings.Contains(derr.Error(), "definition loop") || strings.Contains(derr.Error(), "CFG post-order panic")
		if e.ran || !internal {
			return derr
		}
		return err
	}
	values := e.values
	if len(values) != len(d.names) {
		return fmt.Errorf("assignment mismatch: %d variables but %d values", len(d.names), len(values))
	}
	for i, name := range d.names {
		if name == "_" {
			continue
		}
		typ, ok := typeName(reflect.TypeOf(values[i]))
		if !ok {
			return fmt.Errorf("can't tell the type of %s from its value; declare it first: var %s T", name, name)
		}
		e.values = values
		if _, derr := e.eval(ctx, "var "+name+" "+typ); derr != nil {
			return derr
		}
		if _, derr := e.eval(ctx, fmt.Sprintf("%s = telesaprConsole.Take(%d).(%s)", name, i, typ)); derr != nil {
			return derr
		}
	}
	return nil
}

// tooMany says when a definition from one call has fewer names than
// the call has values. The interpreter would quietly take the first
// values, as in "x := g.Bounds()", which Go refuses. A function that
// is never called is compiled, to see whether the call gives one value
// more than there are names; nothing is run.
func (e *Engine) tooMany(d definition) error {
	blanks := strings.Repeat("_, ", len(d.names)) + "_"
	logged := log.Writer()
	log.SetOutput(io.Discard)
	_, err := e.i.Eval("_ = func() { " + blanks + " = " + d.value + " }")
	log.SetOutput(logged)
	if err != nil {
		return nil
	}
	return &Error{Line: d.line, Col: d.col, Err: fmt.Errorf(
		"assignment mismatch: %d variable(s) but %s has more values; name each, as in \"lo, hi := g.Bounds()\"",
		len(d.names), d.value)}
}

// parseBody parses src as the statements of a function's body. at
// gives the offset in src of a position of the result.
func parseBody(src string) (body []ast.Stmt, at func(token.Pos) int, ok bool) {
	const prefix = "package p\nfunc _() {\n"
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", prefix+src+"\n}", 0)
	if err != nil {
		return nil, nil, false
	}
	at = func(p token.Pos) int { return fset.Position(p).Offset - len(prefix) }
	return f.Decls[0].(*ast.FuncDecl).Body.List, at, true
}

// inBlocks reports whether src is statements that only have blocks of
// their own, such as loops and ifs, and define nothing outside them.
func inBlocks(src string) bool {
	body, _, ok := parseBody(src)
	if !ok || len(body) == 0 {
		return false
	}
	for _, s := range body {
		if l, ok := s.(*ast.LabeledStmt); ok {
			s = l.Stmt
		}
		switch s.(type) {
		case *ast.BlockStmt, *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt,
			*ast.TypeSwitchStmt, *ast.SelectStmt:
		default:
			return false
		}
	}
	return true
}

// inFunc runs statements that only have blocks of their own in a
// function. Run where the interpreter runs a console's statements, a
// name defined in such a block is the variable of that name defined
// before, and it panics when the types differ: "p := 1.5" and then
// "for ... { p := "x" }" sets the first p. In a function the block's p
// is a variable of its own, as in Go. The source must not start with
// "func": the interpreter would take it for a declaration first, and
// then run the call again with each later source.
func (e *Engine) inFunc(ctx context.Context, src string) error {
	_, err := e.eval(ctx, "_ = 0; func() {\n"+src+"\n}()")
	var serr *Error
	if errors.As(err, &serr) && serr.Line > 0 {
		// The first line is the function's.
		if serr.Line--; serr.Line < 1 {
			serr.Line, serr.Col = 1, 1
		}
	}
	return err
}

// part is a statement of source of several.
type part struct {
	src string
	// line and col are where it starts in the source, from 1.
	line, col int
}

// place counts the place of an error of the part from the start of
// the source.
func (p part) place(err error) error {
	var serr *Error
	if errors.As(err, &serr) && serr.Line > 0 {
		if serr.Line == 1 {
			serr.Col += p.col - 1
		}
		serr.Line += p.line - 1
	}
	return err
}

// split divides source that failed with err into its statements, when
// the interpreter took it for declarations because it starts with one,
// as "const n = 3; fmt.Println(n)" does: the interpreter wants only
// declarations after a first one.
func split(src string, err error) ([]part, bool) {
	if err == nil || !strings.Contains(err.Error(), "expected declaration") {
		return nil, false
	}
	body, at, ok := parseBody(src)
	if !ok || len(body) < 2 {
		return nil, false
	}
	parts := make([]part, len(body))
	for i, s := range body {
		start := at(s.Pos())
		parts[i] = part{
			src:  src[start:at(s.End())],
			line: 1 + strings.Count(src[:start], "\n"),
			col:  start - strings.LastIndex(src[:start], "\n"),
		}
	}
	return parts, true
}

// panicFilter takes the line the interpreter writes about a panic out
// of a script's errors, and keeps where it says the panic was.
type panicFilter struct {
	e *Engine
	w io.Writer
}

var panicLine = regexp.MustCompile(`^(\d+):(\d+): panic: [^\n]*\n?$`)

func (f *panicFilter) Write(p []byte) (int, error) {
	if m := panicLine.FindSubmatch(p); m != nil {
		f.e.panicAt.line, _ = strconv.Atoi(string(m[1]))
		f.e.panicAt.col, _ = strconv.Atoi(string(m[2]))
		return len(p), nil
	}
	return f.w.Write(p)
}

// wrapped is how many characters the interpreter puts before source
// that isn't a whole file, which its columns on the first line count.
func wrapped(src string) int {
	var s scanner.Scanner
	fset := token.NewFileSet()
	s.Init(fset.AddFile("", fset.Base(), len(src)), []byte(src), nil, 0)
	switch _, tok, _ := s.Scan(); tok {
	case token.PACKAGE:
		return 0
	case token.CONST, token.FUNC, token.IMPORT, token.TYPE, token.VAR:
		return len("package main;")
	}
	return len("package main; func main() {")
}

// Error is an error of a script, with where it is if that is known.
type Error struct {
	// File is the script's file, "" for a console.
	File string
	// Line and Col count from 1; Line is 0 if the place isn't known.
	Line, Col int
	Err       error
}

func (e *Error) Unwrap() error { return e.Err }

func (e *Error) Error() string {
	switch {
	case e.Line == 0 && e.File == "":
		return e.Err.Error()
	case e.Line == 0:
		return fmt.Sprintf("%s: %v", e.File, e.Err)
	case e.File == "":
		return fmt.Sprintf("%d:%d: %v", e.Line, e.Col, e.Err)
	}
	return fmt.Sprintf("%s:%d:%d: %v", e.File, e.Line, e.Col, e.Err)
}

var positioned = regexp.MustCompile(`^(\d+):(\d+): (.*)$`)

// severalValues is the interpreter's error for a call of several
// results passed beside other arguments; it names the call's type.
var severalValues = regexp.MustCompile(`^cannot use func\(.*?\) \((.+, .+)\) as type interface \{\}$`)

// locate makes what the interpreter returns for src an Error: a
// panic becomes what it carried, without the interpreter's own stack,
// and a place counts from the start of src.
func (e *Engine) locate(err error, src string) error {
	if err == nil || Incomplete(err, src) {
		return err
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	out := &Error{}
	var p interp.Panic
	var list scanner.ErrorList
	switch {
	case errors.As(err, &p):
		switch v := p.Value.(type) {
		case symbols.Exit:
			return Exit(v)
		case error:
			out.Err = v
		default:
			out.Err = fmt.Errorf("panic: %v", p.Value)
		}
		out.Line, out.Col = e.panicAt.line, e.panicAt.col
	case errors.As(err, &list) && len(list) > 0:
		out.Line, out.Col, out.Err = list[0].Pos.Line, list[0].Pos.Column, errors.New(list[0].Msg)
	default:
		out.Err = err
		if m := positioned.FindStringSubmatch(err.Error()); m != nil {
			out.Line, _ = strconv.Atoi(m[1])
			out.Col, _ = strconv.Atoi(m[2])
			out.Err = errors.New(m[3])
		}
	}
	if out.Line == 1 {
		if out.Col -= wrapped(src); out.Col < 1 {
			out.Col = 1
		}
	}
	if out.Err.Error() == "constant definition loop" {
		// What the interpreter says of a name it doesn't know in a
		// definition.
		out.Err = errors.New("constant definition loop, or a name that isn't defined")
	}
	if m := severalValues.FindStringSubmatch(out.Err.Error()); m != nil {
		// As fmt.Println("box", g.Bounds()).
		out.Err = fmt.Errorf("a call that gives several values (%s) can't stand beside other arguments; name them first, as lo, hi := g.Bounds()", m[1])
	}
	return out
}

// Incomplete reports whether err says that src ended in the middle of
// something: there is more to come.
func Incomplete(err error, src string) bool {
	var list scanner.ErrorList
	if !errors.As(err, &list) || len(list) == 0 {
		return false
	}
	msg := list[0].Msg
	return strings.HasSuffix(msg, "found 'EOF'") || msg == "raw string literal not terminated" ||
		msg == "comment not terminated" ||
		(strings.HasPrefix(msg, "expected operand, found '}'") && !strings.HasSuffix(strings.TrimSpace(src), "}"))
}

// unfinished reports whether src stops in the middle of something, as
// far as can be told without running it.
func unfinished(src string) bool {
	_, err := parser.ParseExpr(src)
	return Incomplete(err, src)
}

// Feed gives the engine the next line of a console. It runs what the
// lines so far make once that is complete; until then more is true
// and nothing ran. For an expression, shown is its value as a console
// shows it.
func (e *Engine) Feed(ctx context.Context, line string) (shown string, more bool, err error) {
	e.pending = append(e.pending, line)
	src := strings.Join(e.pending, "\n")
	if strings.TrimSpace(src) == "" {
		e.pending = nil
		return "", false, nil
	}
	if unfinished(src) {
		return "", true, nil
	}
	_, err = e.step(ctx, func(ctx context.Context) (reflect.Value, error) {
		shown, err = e.show(ctx, src)
		return reflect.Value{}, err
	})
	if Incomplete(err, src) {
		return "", true, nil
	}
	e.pending = nil
	var serr *Error
	if errors.As(err, &serr) && !strings.Contains(src, "\n") {
		// Where on the one line it is says little.
		err = serr.Err
	}
	return shown, false, err
}

// show runs src, and if it is an expression returns its values as a
// console shows them.
func (e *Engine) show(ctx context.Context, src string) (string, error) {
	if !isShown(src) {
		return "", e.run(ctx, src)
	}
	// A call can have several values, of which the interpreter returns
	// one; passed on to a function, they are all there. That doesn't
	// compile for a call without a value, which is then just run.
	e.values = nil
	e.panicAt.line = 0
	_, err := e.i.EvalWithContext(ctx, "telesaprConsole.Show("+src+")")
	var p interp.Panic
	switch {
	case err == nil:
		parts := make([]string, len(e.values))
		for i, v := range e.values {
			parts[i] = Format(v)
		}
		return strings.Join(parts, ", "), nil
	case errors.As(err, &p), ctx.Err() != nil:
		// It ran.
		return "", e.locate(err, src)
	}
	_, err = e.eval(ctx, src)
	return "", err
}

// Pending reports whether lines were fed that aren't complete yet.
func (e *Engine) Pending() bool { return len(e.pending) > 0 }

// Reset forgets the lines that aren't complete yet.
func (e *Engine) Reset() { e.pending = nil }

// isShown reports whether a console shows the value of src: it is an
// expression, and not a call that prints.
func isShown(src string) bool {
	x, err := parser.ParseExpr(src)
	if err != nil {
		return false
	}
	if call, ok := x.(*ast.CallExpr); ok {
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "fmt" && strings.Contains(sel.Sel.Name, "Print") {
				return false
			}
		}
	}
	return true
}

// Exit is the error of a script that called os.Exit.
type Exit int

func (e Exit) Error() string { return fmt.Sprintf("exit status %d", int(e)) }

// Format is a value as a console shows it.
func Format(value any) string {
	switch x := value.(type) {
	case nil:
		return "nil"
	case string:
		return strconv.Quote(x)
	case geom.Vector:
		return fmt.Sprintf("(%g, %g, %g)", x.X, x.Y, x.Z)
	case error:
		return x.Error()
	case fmt.Stringer:
		return x.String()
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Func:
		return v.Type().String()
	case reflect.Slice:
		if v.Len() == 0 {
			return "[]"
		}
		// A list of things that describe themselves, one to a line.
		switch v.Index(0).Interface().(type) {
		case fmt.Stringer, geom.Vector:
			lines := make([]string, v.Len())
			for i := range lines {
				lines[i] = fmt.Sprintf("%d: %s", i, Format(v.Index(i).Interface()))
			}
			return strings.Join(lines, "\n")
		}
	}
	return fmt.Sprintf("%+v", value)
}

// RunSource runs a script: a whole program, with a package clause and
// a main function, or statements as a console takes them. name is the
// file's name, for the errors.
func RunSource(ctx context.Context, opts Options, name, src string) error {
	if isProgram(src) {
		e, err := newEngine(opts)
		if err != nil {
			return err
		}
		_, err = e.step(ctx, func(ctx context.Context) (reflect.Value, error) {
			return e.eval(ctx, src)
		})
		return named(name, 0, err)
	}

	e, err := New(opts)
	if err != nil {
		return err
	}
	// The whole script is one step, so each statement isn't one.
	_, err = e.step(ctx, func(ctx context.Context) (reflect.Value, error) {
		_, err := e.statements(ctx, name, src, false)
		return reflect.Value{}, err
	})
	return err
}

// statements runs source that is statements, one after the other, and
// stops at the first that fails. With show, an expression's values are
// taken as a console takes them, and those of the last statement
// returned as it shows them.
func (e *Engine) statements(ctx context.Context, name, src string, show bool) (shown string, err error) {
	var chunk []string
	start := 0
	lines := strings.Split(src, "\n")
	for n, line := range lines {
		if len(chunk) == 0 {
			if strings.TrimSpace(line) == "" || (n == 0 && strings.HasPrefix(line, "#!")) {
				continue
			}
			start = n
		}
		chunk = append(chunk, line)
		text := strings.Join(chunk, "\n")
		last := n == len(lines)-1
		if !last && unfinished(text) {
			continue
		}
		var err error
		if show {
			shown, err = e.show(ctx, text)
		} else {
			err = e.run(ctx, text)
		}
		if !last && Incomplete(err, text) {
			continue
		}
		if err != nil {
			return "", named(name, start, err)
		}
		chunk = nil
	}
	return shown, nil
}

// Run runs source of several statements as one step, and so one step
// of undo, on what the engine ran before: what was defined is there,
// and what the source defines stays, as in a console. shown is the
// value of its last statement, if that is an expression, as a console
// shows it. Source that is a whole program, with a package clause and
// a main function, runs as a file does: on the same sketch, but seeing
// and leaving nothing. name is the source's name, for the errors.
func (e *Engine) Run(ctx context.Context, name, src string) (shown string, err error) {
	if isProgram(src) {
		return "", RunSource(ctx, e.opts, name, src)
	}
	_, err = e.step(ctx, func(ctx context.Context) (reflect.Value, error) {
		shown, err = e.statements(ctx, name, src, true)
		return reflect.Value{}, err
	})
	return shown, err
}

// RunFile runs the script in a file.
func RunFile(ctx context.Context, opts Options, path string) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return RunSource(ctx, opts, path, string(src))
}

// isProgram reports whether source is a Go file, with a package
// clause, rather than statements.
func isProgram(src string) bool {
	_, err := parser.ParseFile(token.NewFileSet(), "", src, parser.PackageClauseOnly)
	return err == nil
}

// named puts a file's name on an error, and counts its line from the
// start of the file rather than of the statement it is in, which
// started offset lines down.
func named(name string, offset int, err error) error {
	var serr *Error
	switch {
	case err == nil:
		return nil
	case errors.As(err, &serr):
		if serr.Line == 0 {
			// At least the statement is known.
			serr.Line, serr.Col = 1, 1
		}
		serr.File = name
		serr.Line += offset
		return serr
	case Incomplete(err, ""):
		return &Error{File: name, Line: offset + 1, Col: 1, Err: errors.New("the script ends in the middle of a statement")}
	}
	return err
}

// REPL reads lines from in and runs them until it ends, showing
// values and errors on out, with a prompt if prompt is set.
func (e *Engine) REPL(ctx context.Context, in io.Reader, out io.Writer, prompt bool) error {
	show := func() {
		if !prompt {
			return
		}
		if e.Pending() {
			_, _ = fmt.Fprint(out, "... ")
		} else {
			_, _ = fmt.Fprint(out, ">>> ")
		}
	}
	scan := bufio.NewScanner(in)
	scan.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	show()
	for scan.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		shown, _, err := e.Feed(ctx, scan.Text())
		var exit Exit
		switch {
		case errors.As(err, &exit):
			return exit
		case err != nil:
			_, _ = fmt.Fprintln(out, "error:", err)
		case shown != "":
			_, _ = fmt.Fprintln(out, shown)
		}
		show()
	}
	if prompt {
		_, _ = fmt.Fprintln(out)
	}
	return scan.Err()
}
