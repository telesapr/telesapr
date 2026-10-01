package script

import (
	"fmt"
	"io"
	"path"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/spacesolve/spacesolve/pkg/script/symbols"
)

// This file is help(x) of a console: what a value is and what can be
// done with it, from the value's type and from the declarations and
// comments of the packages a script can import (symbols.Docs).

// helpOrder is where a name without a package is looked for first.
var helpOrder = []string{
	apiPath,
	"github.com/spacesolve/spacesolve/pkg/geom",
	"github.com/spacesolve/spacesolve/pkg/core",
	"github.com/spacesolve/spacesolve/pkg/sketch",
	"github.com/spacesolve/spacesolve/pkg/srf",
	"github.com/spacesolve/spacesolve/pkg/persist/slvs",
}

// helpPackages are the import paths help knows, helpOrder first.
var helpPackages = sync.OnceValue(func() []string {
	paths := append([]string{}, helpOrder...)
	var rest []string
	for p := range symbols.Docs() {
		known := false
		for _, o := range helpOrder {
			known = known || o == p
		}
		if !known {
			rest = append(rest, p)
		}
	}
	sort.Strings(rest)
	return append(paths, rest...)
})

// funcNames names the functions of the packages by where their code
// is, to tell which one a function value is.
var funcNames = sync.OnceValue(func() map[uintptr][2]string {
	names := map[uintptr][2]string{}
	for key, pkg := range symbols.Symbols {
		for name, v := range pkg {
			if v.Kind() == reflect.Func && !v.IsNil() {
				names[v.Pointer()] = [2]string{path.Dir(key), name}
			}
		}
	}
	return names
})

// help writes what each of values is; with none, the summary.
func (e *Engine) help(values ...any) {
	w := e.opts.Stdout
	if len(values) == 0 {
		fmt.Fprint(w, helpText)
		return
	}
	for i, v := range values {
		if i > 0 {
			fmt.Fprintln(w)
		}
		if name, ok := v.(string); ok {
			if !helpName(w, name) {
				fmt.Fprintf(w, "%q is a string, and nothing help knows is called that.\n", name)
				fmt.Fprintln(w, `Names are like "api", "Group", "Group.Bounds" or "strings.ToUpper".`)
			}
			continue
		}
		helpValue(w, v)
	}
}

// synopsis is the first sentence of a comment, on one line.
func synopsis(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	if i := strings.Index(text, ". "); i >= 0 {
		text = text[:i+1]
	}
	// One line of a list; the whole of it is a help away.
	const most = 72
	if r := []rune(text); len(r) > most {
		text = strings.TrimRight(string(r[:most-1]), " ,;:") + "…"
	}
	return text
}

// indent starts every line of text with four spaces.
func indent(text string) string {
	return "    " + strings.ReplaceAll(text, "\n", "\n    ")
}

// signature is a function's declaration without "func" and, for a
// method, its receiver: "Bounds() (min, max Vec)".
func signature(decl string) string {
	decl = strings.TrimPrefix(decl, "func ")
	if strings.HasPrefix(decl, "(") {
		if i := strings.Index(decl, ") "); i >= 0 {
			decl = decl[i+2:]
		}
	}
	return decl
}

// listed writes names with what each is, in columns.
func listed(w io.Writer, title string, docs map[string]symbols.Doc, names []string) {
	if len(names) == 0 {
		return
	}
	sort.Strings(names)
	fmt.Fprintf(w, "\n%s:\n", title)
	for _, name := range names {
		d := docs[name]
		// What it is goes beside a short declaration, under a long
		// one.
		const column = 44
		line, about := signature(d.Decl), synopsis(d.Text)
		switch {
		case about == "":
		case len(line) <= column:
			line = fmt.Sprintf("%-*s %s", column, line, about)
		default:
			line += "\n" + strings.Repeat(" ", 4+column+1) + about
		}
		fmt.Fprintln(w, "    "+line)
	}
}

func keys[V any](m map[string]V) []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	return names
}

// helpPackage writes what a package has.
func helpPackage(w io.Writer, importPath string, p *symbols.PackageDoc) {
	fmt.Fprintf(w, "package %s  // import %q\n", p.Name, importPath)
	if p.Text != "" {
		fmt.Fprintln(w, indent(p.Text))
	}
	if len(p.Types) > 0 {
		names := keys(p.Types)
		sort.Strings(names)
		fmt.Fprintln(w, "\nTypes:")
		for _, name := range names {
			fmt.Fprintf(w, "    %-20s %s\n", name, synopsis(p.Types[name].Text))
		}
	}
	listed(w, "Functions", p.Funcs, keys(p.Funcs))
	if len(p.Values) > 0 {
		names := keys(p.Values)
		sort.Strings(names)
		fmt.Fprintf(w, "\nConstants and variables:\n    %s\n", strings.Join(names, " "))
	}
	fmt.Fprintf(w, "\nhelp(\"%s.Name\") says more about one.\n", p.Name)
}

// helpType writes what a type is: its comment, its fields if t is
// known, what makes one and its methods.
func helpType(w io.Writer, p *symbols.PackageDoc, name string, td *symbols.TypeDoc, t reflect.Type) {
	fmt.Fprintf(w, "type %s.%s %s\n", p.Name, name, td.Kind)
	if td.Text != "" {
		fmt.Fprintln(w, indent(td.Text))
	}
	helpFields(w, t)
	listed(w, "Made by", p.Funcs, append([]string{}, td.Makers...))
	if len(td.Values) > 0 {
		values := append([]string{}, td.Values...)
		sort.Strings(values)
		fmt.Fprintf(w, "\nValues:\n    %s\n", strings.Join(values, " "))
	}
	listed(w, "Methods", td.Methods, keys(td.Methods))
	if len(td.Methods) > 0 {
		fmt.Fprintf(w, "\nhelp(\"%s.Method\") says more about one.\n", name)
	}
}

// helpFields writes the exported fields of a struct type.
func helpFields(w io.Writer, t reflect.Type) {
	if t == nil || t.Kind() != reflect.Struct {
		return
	}
	var lines []string
	for i := 0; i < t.NumField(); i++ {
		if f := t.Field(i); f.IsExported() {
			lines = append(lines, fmt.Sprintf("    %-20s %s", f.Name, f.Type))
		}
	}
	if len(lines) > 0 {
		fmt.Fprintf(w, "\nFields:\n%s\n", strings.Join(lines, "\n"))
	}
}

// helpDoc writes one function, method, constant or variable in full.
func helpDoc(w io.Writer, d symbols.Doc) {
	fmt.Fprintln(w, d.Decl)
	if d.Text != "" {
		fmt.Fprintln(w, indent(d.Text))
	}
}

// packagesNamed are the import paths of the packages called name, or
// with that import path.
func packagesNamed(name string) (paths []string) {
	docs := symbols.Docs()
	for _, p := range helpPackages() {
		if d := docs[p]; d != nil && (d.Name == name || p == name) {
			paths = append(paths, p)
		}
	}
	return paths
}

// helpName writes what the name of a package, a type, a function or
// a method says: "api", "Group", "Group.Bounds", "geom.Vector" or
// "geom.Vector.Plus". It is false if nothing is called that.
func helpName(w io.Writer, name string) bool {
	docs := symbols.Docs()
	parts := strings.Split(name, ".")
	if strings.Contains(name, "/") {
		// An import path, perhaps with a name after it.
		for i := len(parts); i > 0; i-- {
			if p := strings.Join(parts[:i], "."); docs[p] != nil {
				parts = append([]string{p}, parts[i:]...)
				break
			}
		}
	}
	// in looks a name, and a method of it, up in one package.
	in := func(importPath string, names []string) bool {
		p := docs[importPath]
		if p == nil {
			return false
		}
		switch len(names) {
		case 1:
			if td := p.Types[names[0]]; td != nil {
				helpType(w, p, names[0], td, nil)
				return true
			}
			if d, ok := p.Funcs[names[0]]; ok {
				helpDoc(w, d)
				return true
			}
			if d, ok := p.Values[names[0]]; ok {
				helpDoc(w, d)
				return true
			}
		case 2:
			if td := p.Types[names[0]]; td != nil {
				if d, ok := td.Methods[names[1]]; ok {
					helpDoc(w, d)
					return true
				}
			}
		}
		return false
	}

	if len(parts) == 1 {
		if paths := packagesNamed(parts[0]); len(paths) > 0 {
			helpPackage(w, paths[0], docs[paths[0]])
			return true
		}
	}
	// With a package first, then without.
	if len(parts) > 1 {
		for _, p := range packagesNamed(parts[0]) {
			if in(p, parts[1:]) {
				return true
			}
		}
	}
	for _, p := range helpPackages() {
		if in(p, parts) {
			return true
		}
	}
	return false
}

// typeDoc finds what is known about a named type.
func typeDoc(t reflect.Type) (*symbols.PackageDoc, *symbols.TypeDoc) {
	if t.Name() == "" || t.PkgPath() == "" {
		return nil, nil
	}
	p := symbols.Docs()[t.PkgPath()]
	if p == nil {
		return nil, nil
	}
	return p, p.Types[t.Name()]
}

// helpValue writes what a value is, from its type.
func helpValue(w io.Writer, value any) {
	if value == nil {
		fmt.Fprintln(w, "nil: a value that isn't there has no type to tell about.")
		fmt.Fprintln(w, `Name what you mean: help("Doc.Save"), help("Group"), help("api").`)
		return
	}
	v := reflect.ValueOf(value)
	t := v.Type()

	// What the value itself is, in a line.
	shown := Format(value)
	if strings.Contains(shown, "\n") || len(shown) > 60 {
		shown = ""
	}
	switch t.Kind() {
	case reflect.Func:
		if !v.IsNil() {
			if name, ok := funcNames()[v.Pointer()]; ok {
				if p := symbols.Docs()[name[0]]; p != nil {
					if d, ok := p.Funcs[name[1]]; ok {
						helpDoc(w, d)
						return
					}
				}
			}
		}
		fmt.Fprintf(w, "%s\n", t)
		fmt.Fprintln(w, `    A function, or a method of some value; which one can't be told from it.`)
		fmt.Fprintln(w, `    Name it for what it does: help("Doc.Line"), help("strings.ToUpper").`)
		return
	case reflect.Slice, reflect.Array, reflect.Map:
		what := "a list"
		if t.Kind() == reflect.Map {
			what = "a map"
		}
		fmt.Fprintf(w, "%s: %s of %d; index it, or range over it. Each is:\n\n", t, what, v.Len())
		helpOfType(w, t.Elem(), "")
		return
	}
	helpOfType(w, t, shown)
}

// helpOfType writes what a type is; shown is a value of it.
func helpOfType(w io.Writer, t reflect.Type, shown string) {
	named := t
	if t.Kind() == reflect.Pointer {
		named = t.Elem()
	}
	if p, td := typeDoc(named); td != nil {
		if shown != "" {
			fmt.Fprintf(w, "%s\n\n", shown)
		}
		helpType(w, p, named.Name(), td, named)
		return
	}

	// A type no comment is known of: a basic one, one of a package
	// without sources here, or one the script made itself.
	if shown != "" {
		fmt.Fprintf(w, "%s = %s\n", t, shown)
	} else {
		fmt.Fprintf(w, "%s\n", t)
	}
	helpFields(w, named)
	var methods []string
	for _, mt := range []reflect.Type{reflect.PointerTo(named), named} {
		for i := 0; i < mt.NumMethod(); i++ {
			m := mt.Method(i)
			sig := strings.TrimPrefix(m.Type.String(), "func")
			// Without the receiver, which is the first parameter.
			if i := strings.IndexAny(sig, ",)"); i >= 0 {
				sig = "(" + strings.TrimPrefix(strings.TrimPrefix(sig[i:], ","), " ")
			}
			methods = append(methods, "    "+m.Name+sig)
		}
		if len(methods) > 0 {
			break
		}
	}
	if len(methods) > 0 {
		fmt.Fprintf(w, "\nMethods:\n%s\n", strings.Join(methods, "\n"))
	} else if named.Kind() == reflect.Struct && named.Name() == "" {
		fmt.Fprintln(w, "\nA type made in a script; the methods a script gave it can't be listed.")
	}
}
