package script

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/examples"
	"github.com/telesapr/telesapr/pkg/script/symbols"
)

// help("examples") lists every example by name with what it makes, in
// the console as in Help.
func TestHelpListsExamples(t *testing.T) {
	t.Parallel()
	list, err := examples.List()
	if err != nil {
		t.Fatal(err)
	}
	got := helped(t, `help("examples")`)
	mustContain(t, got, `"example <name>" shows one, as "example bolt".`)
	for _, e := range list {
		mustContain(t, got, e.Name+" ", e.Title+"\n")
	}
	var b strings.Builder
	if err := Help(&b, " Examples "); err != nil || b.String() != got {
		t.Errorf("Help(Examples): %v, and not what the console shows", err)
	}
}

// help("example bolt") is the script as the file has it, after a line on
// how it differs from statements; a library's files follow its script.
func TestHelpShowsExample(t *testing.T) {
	t.Parallel()
	want, err := os.ReadFile(filepath.Join("..", "..", "examples", "scripts", "bolt", "bolt.go"))
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	if err := Help(&b, "example bolt"); err != nil {
		t.Fatal(err)
	}
	head, src, ok := strings.Cut(b.String(), "\n\n")
	if !ok || src != string(want) {
		t.Errorf("help of example bolt is not the file:\n%s", b.String())
	}
	mustContain(t, head, "Example bolt: A hexagon head bolt with a screw thread.\n", "doc is there already and nothing is imported")

	b.Reset()
	if err := Help(&b, "example  library"); err != nil {
		t.Fatal(err)
	}
	mustContain(t, b.String(), "\n// ===== library.go\n\n//go:build telesapr", "\n// ===== lib/gears/gears.go\n\n// Package gears")
}

// An example that isn't there is an error naming the nearest, or
// pointing at the list.
func TestHelpUnknownExample(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]string{
		"example bolts":   `no example is called "bolts"; the nearest are "bolt"`,
		"example thread":  `"threadchamfer"`,
		"example":         `no example is called ""; "examples" lists them`,
		"example zzzzzzz": `no example is called "zzzzzzz"; "examples" lists them`,
	} {
		var b strings.Builder
		err := Help(&b, name)
		if err == nil || errors.Is(err, ErrNoHelp) || !strings.Contains(err.Error(), want) || b.Len() != 0 {
			t.Errorf("Help(%q): %v, wrote %q; want an error with %s", name, err, b.String(), want)
		}
	}
	mustContain(t, helped(t, `help("example bolts")`), `no example is called "bolts"; the nearest are "bolt"`)
}

// Nothing a script can import, nor a menu, is called "examples" or
// "example", so those names can mean the example scripts only.
func TestHelpExampleNamesAreFree(t *testing.T) {
	t.Parallel()
	for _, name := range []string{examplesName, exampleWord} {
		for p, d := range symbols.Docs() {
			if strings.EqualFold(d.Name, name) {
				t.Errorf("package %s is called %s", p, name)
			}
			for typ := range d.Types {
				if strings.EqualFold(typ, name) {
					t.Errorf("%s.%s is called %s", p, typ, name)
				}
			}
		}
		if _, ok := menuFields()[strings.ToUpper(name[:1])+name[1:]]; ok {
			t.Errorf("a menu is called %s", name)
		}
	}
}
