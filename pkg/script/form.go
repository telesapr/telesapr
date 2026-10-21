package script

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// A script declares what it asks for with an api.Form at its top
// level (see api.Form). This file reads that declaration from the
// script's text without running it, so that the window can show a
// dialog first, and writes the values given back into the text, so
// that the script, run as it is, has them.

// FieldKind is what a form's field takes.
type FieldKind int

const (
	// FieldInt is a whole number.
	FieldInt FieldKind = iota
	// FieldFloat is a number.
	FieldFloat
	// FieldText is a string.
	FieldText
	// FieldBool is true or false.
	FieldBool
	// FieldChoice is one of the field's Choices.
	FieldChoice
)

// Form is the form a script declares, as read from its text.
type Form struct {
	// Title is the form's, "" if it has none.
	Title  string
	Fields []FormField
}

// FormField is a field of a script's form.
type FormField struct {
	Name, Label string
	Kind        FieldKind
	// Value is the value as written in the script, in the form
	// FormValues take it.
	Value   string
	Choices []string
	// start and end are where the field's value is in the script.
	start, end int
}

// ErrForm is the error of a form that is declared wrongly, or of a
// value that doesn't fit its field.
var ErrForm = errors.New("script form")

// formDecl finds a form's declaration: a variable at the top level,
// in column 1, whose value is a Form of package api or, in a script of
// statements, of the dot-imported package.
var formDecl = regexp.MustCompile(`(?m)^var\s+\w+\s*=\s*((?:api\.)?Form\s*\{)`)

// ReadForm reads the form a script declares; nil if it declares none.
func ReadForm(src string) (*Form, error) {
	m := formDecl.FindStringSubmatchIndex(src)
	if m == nil {
		return nil, nil
	}
	start := m[2]
	end, err := closingBrace(src, m[3]-1)
	if err != nil {
		return nil, err
	}
	expr, err := parser.ParseExpr(src[start:end])
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrForm, err)
	}
	lit := expr.(*ast.CompositeLit)
	at := func(p token.Pos) int { return start + int(p) - 1 }
	f := &Form{}
	for _, el := range lit.Elts {
		key, value, err := keyed(el)
		if err != nil {
			return nil, err
		}
		switch key {
		case "Title":
			if f.Title, err = stringLit(value); err != nil {
				return nil, fmt.Errorf("%w: the Title: %v", ErrForm, err)
			}
		case "Fields":
			fields, ok := value.(*ast.CompositeLit)
			if !ok {
				return nil, fmt.Errorf("%w: the Fields are not a list written out", ErrForm)
			}
			for _, el := range fields.Elts {
				fd, err := readField(el, at)
				if err != nil {
					return nil, err
				}
				if slices.ContainsFunc(f.Fields, func(o FormField) bool { return o.Name == fd.Name }) {
					return nil, fmt.Errorf("%w: two fields are named %q", ErrForm, fd.Name)
				}
				f.Fields = append(f.Fields, fd)
			}
		default:
			return nil, fmt.Errorf("%w: a Form has no %s", ErrForm, key)
		}
	}
	return f, nil
}

// closingBrace is where the brace at open is closed, past it.
func closingBrace(src string, open int) (int, error) {
	var s scanner.Scanner
	fset := token.NewFileSet()
	file := fset.AddFile("", -1, len(src)-open)
	s.Init(file, []byte(src[open:]), nil, 0)
	depth := 0
	for {
		pos, tok, _ := s.Scan()
		switch tok {
		case token.EOF:
			return 0, fmt.Errorf("%w: the form's declaration doesn't end", ErrForm)
		case token.LBRACE:
			depth++
		case token.RBRACE:
			if depth--; depth == 0 {
				return open + file.Offset(pos) + 1, nil
			}
		}
	}
}

// keyed is the key and value of an element of a composite literal
// that names its fields.
func keyed(el ast.Expr) (string, ast.Expr, error) {
	kv, ok := el.(*ast.KeyValueExpr)
	if !ok {
		return "", nil, fmt.Errorf("%w: a Form and its Fields name what they set, as Name: \"teeth\"", ErrForm)
	}
	key, ok := kv.Key.(*ast.Ident)
	if !ok {
		return "", nil, fmt.Errorf("%w: a Form and its Fields name what they set", ErrForm)
	}
	return key.Name, kv.Value, nil
}

// stringLit is the value of a string literal.
func stringLit(e ast.Expr) (string, error) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", errors.New("not a string written out")
	}
	return strconv.Unquote(lit.Value)
}

// readField reads a field of a form; at says where in the script a
// position of the parsed form is.
func readField(el ast.Expr, at func(token.Pos) int) (FormField, error) {
	lit, ok := el.(*ast.CompositeLit)
	if !ok {
		return FormField{}, fmt.Errorf("%w: a field is not written out, as {Name: \"teeth\", Value: 24}", ErrForm)
	}
	var fd FormField
	var value ast.Expr
	for _, el := range lit.Elts {
		key, v, err := keyed(el)
		if err != nil {
			return FormField{}, err
		}
		switch key {
		case "Name":
			fd.Name, err = stringLit(v)
		case "Label":
			fd.Label, err = stringLit(v)
		case "Value":
			value = v
		case "Choices":
			list, ok := v.(*ast.CompositeLit)
			if !ok {
				return FormField{}, fmt.Errorf("%w: the Choices are not a list written out", ErrForm)
			}
			for _, c := range list.Elts {
				s, err := stringLit(c)
				if err != nil {
					return FormField{}, fmt.Errorf("%w: a choice is %v", ErrForm, err)
				}
				fd.Choices = append(fd.Choices, s)
			}
		default:
			err = fmt.Errorf("a Field has no %s", key)
		}
		if err != nil {
			return FormField{}, fmt.Errorf("%w: %v", ErrForm, err)
		}
	}
	if fd.Name == "" {
		return FormField{}, fmt.Errorf("%w: a field has no Name", ErrForm)
	}
	if value == nil {
		return FormField{}, fmt.Errorf("%w: the field %q has no Value", ErrForm, fd.Name)
	}
	if fd.Label == "" {
		fd.Label = fd.Name
	}
	fd.start, fd.end = at(value.Pos()), at(value.End())
	if err := fd.kindOf(value); err != nil {
		return FormField{}, fmt.Errorf("%w: the field %q: %v", ErrForm, fd.Name, err)
	}
	return fd, nil
}

// kindOf takes the field's kind and value from the literal its Value
// is.
func (fd *FormField) kindOf(value ast.Expr) error {
	neg := false
	if u, ok := value.(*ast.UnaryExpr); ok && u.Op == token.SUB {
		neg, value = true, u.X
	}
	switch v := value.(type) {
	case *ast.BasicLit:
		text := v.Value
		if neg {
			text = "-" + text
		}
		switch v.Kind {
		case token.INT:
			n, err := strconv.Atoi(text)
			if err != nil {
				return err
			}
			fd.Kind, fd.Value = FieldInt, strconv.Itoa(n)
		case token.FLOAT:
			x, err := strconv.ParseFloat(text, 64)
			if err != nil {
				return err
			}
			fd.Kind, fd.Value = FieldFloat, strconv.FormatFloat(x, 'g', -1, 64)
		case token.STRING:
			if neg {
				return errors.New("its Value is not a literal")
			}
			s, err := strconv.Unquote(text)
			if err != nil {
				return err
			}
			fd.Kind, fd.Value = FieldText, s
		default:
			return errors.New("its Value is not a number, a string, true or false")
		}
	case *ast.Ident:
		if neg || (v.Name != "true" && v.Name != "false") {
			return errors.New("its Value is not a literal")
		}
		fd.Kind, fd.Value = FieldBool, v.Name
	default:
		return errors.New("its Value is not a literal")
	}
	if len(fd.Choices) == 0 {
		return nil
	}
	if fd.Kind != FieldText {
		return errors.New("a field with Choices has a string for its Value")
	}
	if !slices.Contains(fd.Choices, fd.Value) {
		return fmt.Errorf("its Value %q is not one of its Choices", fd.Value)
	}
	fd.Kind = FieldChoice
	return nil
}

// Field is the form's field with the name, nil if it has none.
func (f *Form) Field(name string) *FormField {
	for i := range f.Fields {
		if f.Fields[i].Name == name {
			return &f.Fields[i]
		}
	}
	return nil
}

// Names are the names of the form's fields, in order.
func (f *Form) Names() []string {
	names := make([]string, len(f.Fields))
	for i, fd := range f.Fields {
		names[i] = fd.Name
	}
	return names
}

// literal is the Go literal of a value given for the field, or an
// error if the value doesn't fit it.
func (fd *FormField) literal(value string) (string, error) {
	value = strings.TrimSpace(value)
	switch fd.Kind {
	case FieldInt:
		n, err := strconv.Atoi(value)
		if err != nil {
			return "", fmt.Errorf("%w: %s is a whole number, not %q", ErrForm, fd.Name, value)
		}
		return strconv.Itoa(n), nil
	case FieldFloat:
		x, err := strconv.ParseFloat(value, 64)
		if err != nil || math.IsInf(x, 0) || math.IsNaN(x) {
			return "", fmt.Errorf("%w: %s is a number, not %q", ErrForm, fd.Name, value)
		}
		lit := strconv.FormatFloat(x, 'g', -1, 64)
		if !strings.ContainsAny(lit, ".e") {
			// Written as 3 it would be an int in the field's any.
			lit += ".0"
		}
		return lit, nil
	case FieldBool:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return "", fmt.Errorf("%w: %s is true or false, not %q", ErrForm, fd.Name, value)
		}
		return strconv.FormatBool(b), nil
	case FieldChoice:
		if !slices.Contains(fd.Choices, value) {
			return "", fmt.Errorf("%w: %s is one of %s, not %q", ErrForm, fd.Name, strings.Join(fd.Choices, ", "), value)
		}
	}
	return strconv.Quote(value), nil
}

// Apply returns the script's text with the values given, by field
// name, in place of the Values written; a field not given keeps its
// Value. A name the form doesn't have, or a value that doesn't fit its
// field, is an error. The text keeps its lines, so that an error of
// the script names the line it has in the file.
func (f *Form) Apply(src string, values map[string]string) (string, error) {
	type edit struct {
		start, end int
		text       string
	}
	var edits []edit
	for name, value := range values {
		fd := f.Field(name)
		if fd == nil {
			return "", fmt.Errorf("%w: it has no field %q; it has %s", ErrForm, name, strings.Join(f.Names(), ", "))
		}
		lit, err := fd.literal(value)
		if err != nil {
			return "", err
		}
		if strings.Contains(src[fd.start:fd.end], "\n") {
			// A literal of several lines: keep the count.
			lit += strings.Repeat("\n", strings.Count(src[fd.start:fd.end], "\n"))
		}
		edits = append(edits, edit{fd.start, fd.end, lit})
	}
	slices.SortFunc(edits, func(a, b edit) int { return b.start - a.start })
	for _, e := range edits {
		src = src[:e.start] + e.text + src[e.end:]
	}
	return src, nil
}

// ApplyForm gives the form a script declares the values given, by
// field name (see Form.Apply). With no values it returns the script as
// it is; values for a script that declares no form are an error.
func ApplyForm(src string, values map[string]string) (string, error) {
	if len(values) == 0 {
		return src, nil
	}
	f, err := ReadForm(src)
	if err != nil {
		return "", err
	}
	if f == nil {
		return "", fmt.Errorf("%w: values were given, but the script declares no form", ErrForm)
	}
	return f.Apply(src, values)
}
