package api

import (
	"fmt"
	"math"
)

// Form declares the values a script asks for when it is run, as a
// variable at the top level of the script:
//
//	var Form = api.Form{
//		Title: "Gear blank",
//		Fields: []api.Field{
//			{Name: "teeth", Label: "Teeth", Value: 24},
//			{Name: "module", Label: "Module (mm)", Value: 2.0},
//			{Name: "hub", Label: "Hub", Value: true},
//			{Name: "material", Value: "steel", Choices: []string{"steel", "brass"}},
//		},
//	}
//
// (Form{...} without the package's name in a script of statements.)
// Script > Run File shows the fields in a dialog before the script
// runs, telesapr-cli run takes them as --set teeth=30, and an AI
// program's run_script as its form argument; what is given replaces
// the field's Value in the script's text, so the script reads the
// values with Int, Float, Text and Bool whichever way it was run. A
// script run with nothing given has the Values as written.
//
// Each Value is a literal: a whole number, a number with a point, a
// quoted string or true or false, which says what the field takes. A
// field with Choices takes one of them, and its Value is the first
// taken.
type Form struct {
	// Title is the dialog's title; the script's file name if empty.
	Title  string
	Fields []Field
}

// Field is one value a Form asks for.
type Field struct {
	// Name is what the script and --set call the field.
	Name string
	// Label is what the dialog shows beside it; the Name if empty.
	Label string
	// Value is the field's value: as written in the script, or as
	// given when it was run.
	Value any
	// Choices, if any, are the strings the field may take.
	Choices []string
}

// field is the field of the form with the name; it panics if there is
// none, as a script that asks for one it didn't declare is wrong.
func (f Form) field(name string) Field {
	for _, fd := range f.Fields {
		if fd.Name == name {
			return fd
		}
	}
	panic(fmt.Errorf("the form has no field %q", name))
}

// Float is the value of the field name, a number.
func (f Form) Float(name string) float64 {
	switch v := f.field(name).Value.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	}
	panic(fmt.Errorf("the form's field %q is not a number", name))
}

// Int is the value of the field name, a whole number.
func (f Form) Int(name string) int {
	switch v := f.field(name).Value.(type) {
	case int:
		return v
	case float64:
		if v == math.Trunc(v) {
			return int(v)
		}
	}
	panic(fmt.Errorf("the form's field %q is not a whole number", name))
}

// Text is the value of the field name, a string or one of its
// choices.
func (f Form) Text(name string) string {
	if v, ok := f.field(name).Value.(string); ok {
		return v
	}
	panic(fmt.Errorf("the form's field %q is not a string", name))
}

// Bool is the value of the field name, true or false.
func (f Form) Bool(name string) bool {
	if v, ok := f.field(name).Value.(bool); ok {
		return v
	}
	panic(fmt.Errorf("the form's field %q is not true or false", name))
}
