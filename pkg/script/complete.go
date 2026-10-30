package script

import (
	"go/token"
	"reflect"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/telesapr/telesapr/pkg/script/symbols"
)

// This file is what a console's Tab offers: the names that the word
// being typed can become.

// builtins are the names of Go a console offers that no package has.
var builtins = []string{
	"append(", "cap(", "clear(", "close(", "complex(", "copy(", "delete(", "imag(",
	"len(", "make(", "max(", "min(", "new(", "panic(", "print(", "println(", "real(", "recover(",
	"bool", "byte", "complex128", "error", "false", "float32", "float64", "int", "int32",
	"int64", "iota", "nil", "rune", "string", "true", "uint", "uint32", "uint64", "any",
	"help(",
}

// Complete says what the word that ends at cursor in src can become:
// the names it can be completed to, sorted, and where in src the word
// starts; candidates replace src[start:cursor]. After a dot they are
// what the value or package before it has, worked out from types alone:
// nothing is run, so "doc.Active()." offers a group's methods without
// calling Active. A function's or method's name ends in "(". Nothing
// is offered while the engine runs.
func (e *Engine) Complete(src string, cursor int) (start int, candidates []string) {
	cursor = min(max(cursor, 0), len(src))
	before := src[:cursor]
	start = identStart(before)
	word := before[start:]
	if e.Running() || e.i == nil {
		return start, nil
	}
	var names []string
	if start > 0 && before[start-1] == '.' {
		names = e.members(before[:start-1])
	} else {
		names = e.topNames()
	}
	for _, n := range names {
		if strings.HasPrefix(n, word) && !strings.HasPrefix(n, "telesaprConsole") {
			candidates = append(candidates, n)
		}
	}
	slices.Sort(candidates)
	return start, slices.Compact(candidates)
}

// isIdentRune reports whether r can be in a Go identifier.
func isIdentRune(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) }

// identStart is where the identifier that s ends in starts, len(s) if
// it ends in none.
func identStart(s string) int {
	i := len(s)
	for i > 0 {
		r, n := utf8.DecodeLastRuneInString(s[:i])
		if !isIdentRune(r) {
			break
		}
		i -= n
	}
	return i
}

// topNames are what a word with no dot before it can be: the console's
// variables, the names of package api, the packages and Go's own.
func (e *Engine) topNames() []string {
	names := slices.Clone(builtins)
	for name := range e.i.Globals() {
		names = append(names, name)
	}
	for name, v := range symbols.Symbols[apiPath+"/api"] {
		names = append(names, symbolName(name, v))
	}
	names = append(names, packageNames()...)
	for tok := token.BREAK; tok <= token.VAR; tok++ {
		names = append(names, tok.String())
	}
	return names
}

// symbolName is how a symbol of a table is offered: with "(" if it is
// a function.
func symbolName(name string, v reflect.Value) string {
	if v.Kind() == reflect.Func {
		return name + "("
	}
	return name
}

// packageNames are the names the console's packages are known by, but
// api's, whose names need none.
func packageNames() []string {
	var names []string
	for key := range symbols.Symbols {
		path, name := splitKey(key)
		if path == apiPath || path == showPath {
			continue
		}
		names = append(names, name)
	}
	return names
}

// splitKey splits a key of the symbol tables, "math/rand/rand", into
// the package's path and its name.
func splitKey(key string) (path, name string) {
	i := strings.LastIndexByte(key, '/')
	return key[:i], key[i+1:]
}

// pkgSymbols are the symbols of the package a console knows by name,
// the one of the shortest path if several are called that.
func pkgSymbols(name string) map[string]reflect.Value {
	best := ""
	for key := range symbols.Symbols {
		path, n := splitKey(key)
		if n == name && path != apiPath && path != showPath && (best == "" || len(key) < len(best)) {
			best = key
		}
	}
	if best == "" {
		return nil
	}
	return symbols.Symbols[best]
}

// segment is a part of a selector chain: a name, a call or an index.
type segment struct {
	name  string
	call  bool
	index bool
}

// chain splits the expression that s ends in, as "doc.Groups()[0]",
// into its parts, the first a name; ok is false if s doesn't end in
// one.
func chain(s string) (parts []segment, ok bool) {
	i := len(s)
	for i > 0 {
		switch s[i-1] {
		case ')', ']':
			open := matching(s[:i])
			if open < 0 {
				return nil, false
			}
			parts = append(parts, segment{call: s[i-1] == ')', index: s[i-1] == ']'})
			i = open
			continue
		}
		start := identStart(s[:i])
		if start == i {
			return nil, false
		}
		parts = append(parts, segment{name: s[start:i]})
		i = start
		if i == 0 || s[i-1] != '.' {
			break
		}
		i--
	}
	slices.Reverse(parts)
	return parts, len(parts) > 0 && parts[0].name != ""
}

// matching is where the bracket that closes s is opened, -1 if it
// isn't.
func matching(s string) int {
	closer := s[len(s)-1]
	opener := byte('(')
	if closer == ']' {
		opener = '['
	}
	depth := 0
	for i := len(s) - 1; i >= 0; i-- {
		switch s[i] {
		case closer:
			depth++
		case opener:
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// members are the names that can follow the expression that s ends in
// and a dot.
func (e *Engine) members(s string) []string {
	parts, ok := chain(s)
	if !ok {
		return nil
	}
	first, rest := parts[0], parts[1:]
	var t reflect.Type
	if v, ok := e.i.Globals()[first.name]; ok {
		t = valueType(v)
	} else if pkg := pkgSymbols(first.name); pkg != nil && !e.isGlobal(first.name) {
		if len(rest) == 0 {
			names := make([]string, 0, len(pkg))
			for name, v := range pkg {
				names = append(names, symbolName(name, v))
			}
			return names
		}
		v, ok := pkg[rest[0].name]
		if !ok {
			return nil
		}
		t, rest = exprType(v), rest[1:]
	} else if v, ok := symbols.Symbols[apiPath+"/api"][first.name]; ok {
		t = exprType(v)
	}
	for _, p := range rest {
		if t = follow(t, p); t == nil {
			return nil
		}
	}
	return typeMembers(t)
}

// isGlobal reports whether name is a variable of the console, which
// hides a package of that name.
func (e *Engine) isGlobal(name string) bool {
	_, ok := e.i.Globals()[name]
	return ok
}

// valueType is the type of a variable's value: the dynamic one if the
// variable is an interface that holds something.
func valueType(v reflect.Value) reflect.Type {
	if !v.IsValid() {
		return nil
	}
	if v.Kind() == reflect.Interface && !v.IsNil() {
		return v.Elem().Type()
	}
	return v.Type()
}

// exprType is the type of what a symbol of a table is, as used in an
// expression: a function's own type, a variable's, and nil for a type
// or a constant, after which there is nothing to offer.
func exprType(v reflect.Value) reflect.Type {
	if !v.IsValid() || v.Kind() == reflect.Pointer && v.IsNil() {
		return nil
	}
	if _, ok := reflect.TypeAssert[interface{ ExactString() string }](v); ok {
		return nil
	}
	return v.Type()
}

// follow is the type of a part of a chain applied to a value of type
// t, nil if that can't be told.
func follow(t reflect.Type, p segment) reflect.Type {
	if t == nil {
		return nil
	}
	switch {
	case p.call:
		if t.Kind() != reflect.Func || t.NumOut() == 0 {
			return nil
		}
		return t.Out(0)
	case p.index:
		for t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
		switch t.Kind() {
		case reflect.Slice, reflect.Array, reflect.Map:
			return t.Elem()
		}
		return nil
	}
	if m, ok := methodSet(t).MethodByName(p.name); ok {
		if t.Kind() == reflect.Interface {
			return m.Type
		}
		// A method of a concrete type takes its receiver first, which
		// a method value doesn't.
		in := slices.Collect(m.Type.Ins())[1:]
		return reflect.FuncOf(in, slices.Collect(m.Type.Outs()), m.Type.IsVariadic())
	}
	s := t
	for s.Kind() == reflect.Pointer {
		s = s.Elem()
	}
	if s.Kind() != reflect.Struct {
		return nil
	}
	f, ok := s.FieldByName(p.name)
	if !ok || !f.IsExported() {
		return nil
	}
	return f.Type
}

// methodSet is the type whose methods a value of type t can call: its
// pointer's, for a value that isn't one, since a variable can be
// addressed.
func methodSet(t reflect.Type) reflect.Type {
	if t.Kind() == reflect.Pointer || t.Kind() == reflect.Interface {
		return t
	}
	return reflect.PointerTo(t)
}

// typeMembers are the methods, with "(", and the fields of a value of
// type t.
func typeMembers(t reflect.Type) []string {
	if t == nil {
		return nil
	}
	var names []string
	ms := methodSet(t)
	for m := range ms.Methods() {
		if m.IsExported() {
			names = append(names, m.Name+"(")
		}
	}
	s := t
	for s.Kind() == reflect.Pointer {
		s = s.Elem()
	}
	if s.Kind() == reflect.Struct {
		for _, f := range reflect.VisibleFields(s) {
			if f.IsExported() {
				names = append(names, f.Name)
			}
		}
	}
	return names
}
