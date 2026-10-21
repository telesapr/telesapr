package api

import (
	"strings"
	"testing"
)

// A form's fields are read as what they hold; a whole number reads as
// a number too, and a number with nothing after the point as a whole
// one.
func TestForm(t *testing.T) {
	t.Parallel()
	f := Form{Title: "t", Fields: []Field{
		{Name: "n", Value: 3},
		{Name: "x", Value: 2.5},
		{Name: "w", Value: 4.0},
		{Name: "s", Value: "steel", Choices: []string{"steel"}},
		{Name: "b", Value: true},
	}}
	if f.Int("n") != 3 || f.Float("n") != 3 || f.Float("x") != 2.5 || f.Int("w") != 4 ||
		f.Text("s") != "steel" || !f.Bool("b") {
		t.Errorf("the fields read as %d %g %g %d %q %v", f.Int("n"), f.Float("n"), f.Float("x"), f.Int("w"), f.Text("s"), f.Bool("b"))
	}
}

// Asking a form for a field it hasn't, or as what it isn't, panics
// with what is wrong, as a script's mistake does.
func TestFormMisuse(t *testing.T) {
	t.Parallel()
	f := Form{Fields: []Field{{Name: "x", Value: 2.5}, {Name: "s", Value: "a"}}}
	for _, c := range []struct {
		call func()
		want string
	}{
		{func() { f.Int("none") }, `no field "none"`},
		{func() { f.Int("x") }, "not a whole number"},
		{func() { f.Float("s") }, "not a number"},
		{func() { f.Text("x") }, "not a string"},
		{func() { f.Bool("x") }, "not true or false"},
	} {
		func() {
			defer func() {
				err, _ := recover().(error)
				if err == nil || !strings.Contains(err.Error(), c.want) {
					t.Errorf("panicked with %v, want an error saying %q", err, c.want)
				}
			}()
			c.call()
		}()
	}
}
