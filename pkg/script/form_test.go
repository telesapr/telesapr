package script

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

const formProgram = `package main

import (
	"fmt"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// The form, with a comment } and a brace in a string.
var Form = api.Form{
	Title: "Gear {blank}",
	Fields: []api.Field{
		{Name: "teeth", Label: "Teeth", Value: 24},
		{Name: "module", Label: "Module (mm)", Value: 2.0},
		{Name: "offset", Value: -1.5},
		{Name: "name", Value: "gear"},
		{Name: "hub", Label: "Hub", Value: true},
		{Name: "material", Value: "steel", Choices: []string{"steel", "brass"}},
	},
}

func main() {
	fmt.Println(Form.Int("teeth"), Form.Float("module"), Form.Float("offset"), Form.Text("name"),
		Form.Bool("hub"), Form.Text("material"), Form.Float("teeth"))
}
`

func TestReadForm(t *testing.T) {
	t.Parallel()
	f, err := ReadForm(formProgram)
	if err != nil {
		t.Fatal(err)
	}
	if f.Title != "Gear {blank}" {
		t.Errorf("title %q", f.Title)
	}
	type row struct {
		Name, Label string
		Kind        FieldKind
		Value       string
		Choices     []string
	}
	var got []row
	for _, fd := range f.Fields {
		got = append(got, row{fd.Name, fd.Label, fd.Kind, fd.Value, fd.Choices})
	}
	want := []row{
		{"teeth", "Teeth", FieldInt, "24", nil},
		{"module", "Module (mm)", FieldFloat, "2", nil},
		{"offset", "offset", FieldFloat, "-1.5", nil},
		{"name", "name", FieldText, "gear", nil},
		{"hub", "Hub", FieldBool, "true", nil},
		{"material", "material", FieldChoice, "steel", []string{"steel", "brass"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("fields\n%v\nwant\n%v", got, want)
	}
	if f.Field("none") != nil {
		t.Error("a field that isn't there was found")
	}
	if none, err := ReadForm("x := 1\n"); none != nil || err != nil {
		t.Errorf("a script without a form gave %v, %v", none, err)
	}
}

// The values given run the script, which reads them as it reads the
// ones it was written with; its lines don't move.
func TestApplyFormRuns(t *testing.T) {
	t.Parallel()
	run := func(values map[string]string) string {
		t.Helper()
		src, err := ApplyForm(formProgram, values)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Count(src, "\n") != strings.Count(formProgram, "\n") {
			t.Errorf("the script has other lines with the values")
		}
		var out bytes.Buffer
		if err := RunSource(context.Background(), Options{Host: newHost(), Stdout: &out, Stderr: &out}, "form.go", src); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}
	if got := run(nil); got != "24 2 -1.5 gear true steel 24\n" {
		t.Errorf("as written the script printed %q", got)
	}
	got := run(map[string]string{"teeth": "30", "module": "3", "offset": " 0.25", "name": `a "b"`,
		"hub": "0", "material": "brass"})
	if got != "30 3 0.25 a \"b\" false brass 30\n" {
		t.Errorf("with values the script printed %q", got)
	}
}

// A script of statements declares its form without the package's
// name.
func TestApplyFormStatements(t *testing.T) {
	t.Parallel()
	src := "var form = Form{Fields: []Field{{Name: \"size\", Value: 10.0}}}\n\nfmt.Println(form.Float(\"size\"))\n"
	src, err := ApplyForm(src, map[string]string{"size": "12.5"})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := RunSource(context.Background(), Options{Host: newHost(), Stdout: &out}, "s.go", src); err != nil {
		t.Fatal(err)
	}
	if out.String() != "12.5\n" {
		t.Errorf("printed %q", out.String())
	}
}

func TestFormErrors(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ src, want string }{
		{"var f = Form{Fields: []Field{{Name: \"a\"}}}", `the field "a" has no Value`},
		{"var f = Form{Fields: []Field{{Value: 1}}}", "a field has no Name"},
		{"var f = Form{Fields: []Field{{Name: \"a\", Value: 1}, {Name: \"a\", Value: 2}}}", `two fields are named "a"`},
		{"var f = Form{Fields: []Field{{Name: \"a\", Value: x}}}", "not a literal"},
		{"var f = Form{Fields: []Field{{Name: \"a\", Value: -x}}}", "not a literal"},
		{"var f = Form{Fields: []Field{{Name: \"a\", Value: 'c'}}}", "not a number, a string"},
		{"var f = Form{Fields: []Field{{Name: \"a\", Value: 1, Choices: []string{\"1\"}}}}", "has a string for its Value"},
		{"var f = Form{Fields: []Field{{Name: \"a\", Value: \"z\", Choices: []string{\"y\"}}}}", "not one of its Choices"},
		{"var f = Form{Fields: []Field{{Name: \"a\", Value: \"z\", Choices: names}}}", "Choices are not a list"},
		{"var f = Form{Fields: []Field{{Name: \"a\", Value: \"z\", Choices: []string{y}}}}", "a choice is"},
		{"var f = Form{Fields: []Field{{Name: \"a\", Value: 1, Hint: \"\"}}}", "a Field has no Hint"},
		{"var f = Form{Fields: fields}", "Fields are not a list"},
		{"var f = Form{Size: 1}", "a Form has no Size"},
		{"var f = Form{Title: t}", "the Title"},
		{"var f = Form{Fields: []Field{{\"a\", \"\", 1, nil}}}", "name what they set"},
		{"var f = Form{Fields: []Field{x}}", "not written out"},
		{"var f = Form{Fields: []Field{{Name: \"a\", Value: 1}", "doesn't end"},
		{"var f = Form{Fields: []Field{{Name: \"a\", Value: 1}}} ]", ""},
		{"var f = Form{Fields: []Field{{Name: \"a\" Value: 1}}}", "in composite literal"},
	} {
		_, err := ReadForm(c.src)
		if c.want == "" {
			if err != nil {
				t.Errorf("%s: %v", c.src, err)
			}
			continue
		}
		if !errors.Is(err, ErrForm) || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want an error saying %q", c.src, err, c.want)
		}
	}

	for _, c := range []struct {
		values map[string]string
		want   string
	}{
		{map[string]string{"teeth": "2.5"}, "teeth is a whole number"},
		{map[string]string{"module": "x"}, "module is a number"},
		{map[string]string{"module": "Inf"}, "module is a number"},
		{map[string]string{"hub": "maybe"}, "hub is true or false"},
		{map[string]string{"material": "wood"}, "material is one of steel, brass"},
		{map[string]string{"colour": "red"}, `no field "colour"; it has teeth, module`},
	} {
		if _, err := ApplyForm(formProgram, c.values); !errors.Is(err, ErrForm) || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%v: %v, want an error saying %q", c.values, err, c.want)
		}
	}
	if _, err := ApplyForm("x := 1", map[string]string{"a": "1"}); !errors.Is(err, ErrForm) {
		t.Errorf("values for a script without a form gave %v", err)
	}
	if _, err := ApplyForm("var f = Form{Size: 1}", map[string]string{"a": "1"}); !errors.Is(err, ErrForm) {
		t.Errorf("values for a wrong form gave %v", err)
	}
}

// A script that reads a field it didn't declare, or as what it isn't,
// fails with what is wrong.
func TestFormMisuse(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ call, want string }{
		{`form.Int("none")`, `has no field "none"`},
		{`form.Int("f")`, "not a whole number"},
		{`form.Float("s")`, "not a number"},
		{`form.Text("i")`, "not a string"},
		{`form.Bool("i")`, "not true or false"},
	} {
		src := "var form = Form{Fields: []Field{{Name: \"i\", Value: 1}, {Name: \"f\", Value: 1.5}, {Name: \"s\", Value: \"x\"}}}\n" + c.call + "\n"
		err := RunSource(context.Background(), Options{Host: newHost()}, "m.go", src)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want an error saying %q", c.call, err, c.want)
		}
	}
}
