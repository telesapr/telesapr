package examples

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Every directory of examples/scripts is listed, with a title: a script
// added without a comment that says what it makes fails here.
func TestListHasEveryExample(t *testing.T) {
	t.Parallel()
	dirs, err := os.ReadDir("scripts")
	if err != nil {
		t.Fatal(err)
	}
	list, err := List()
	if err != nil {
		t.Fatal(err)
	}
	titles := map[string]string{}
	for _, e := range list {
		titles[e.Name] = e.Title
	}
	n := 0
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		n++
		title, ok := titles[d.Name()]
		switch {
		case !ok:
			t.Errorf("example %s is not listed", d.Name())
		case !strings.HasSuffix(title, "."):
			t.Errorf("example %s: title %q is not a sentence", d.Name(), title)
		}
	}
	if len(list) != n {
		t.Errorf("%d examples listed, %d directories", len(list), n)
	}
	for i := 1; i < len(list); i++ {
		if list[i-1].Name >= list[i].Name {
			t.Errorf("%s listed before %s", list[i-1].Name, list[i].Name)
		}
	}
}

func TestFilesReadBackEqual(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"bolt", "library"} {
		got, err := Files(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range got {
			want, err := os.ReadFile(filepath.Join("scripts", name, filepath.FromSlash(f.Path)))
			if err != nil {
				t.Fatal(err)
			}
			if f.Text != string(want) {
				t.Errorf("%s/%s differs from the file", name, f.Path)
			}
		}
		if got[0].Path != name+".go" {
			t.Errorf("%s: first file %s", name, got[0].Path)
		}
	}
	lib, _ := Files("library")
	if len(lib) != 2 || lib[1].Path != "lib/gears/gears.go" {
		t.Errorf("library's files: %v", lib)
	}
}

func TestFilesUnknown(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"", "nosuch", "../embed", "bolt/bolt.go", "library/lib"} {
		if _, err := Files(name); !errors.Is(err, ErrUnknown) {
			t.Errorf("Files(%q): %v, not ErrUnknown", name, err)
		}
	}
}

func TestNearest(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]string{"bolts": "bolt", "gear": "library", "Thread": "threadchamfer", "drawing sheet": "drawing"} {
		if got := Nearest(name); !slices.Contains(got, want) || len(got) > 8 {
			t.Errorf("Nearest(%q) = %v, want %s among at most 8", name, got, want)
		}
	}
	if got := Nearest("zzzzzzz"); len(got) != 0 {
		t.Errorf("Nearest of nothing like an example: %v", got)
	}
}

func TestTitle(t *testing.T) {
	t.Parallel()
	for src, want := range map[string]string{
		"// A bolt, made by a script.\npackage main":                  "A bolt.",
		"//go:build telesapr\n\n// A gear blank drawn.\n//\n// More.": "A gear blank drawn.",
		"// A plate\n// with holes. And more.\n":                      "A plate with holes.",
		"// No full stop\n\npackage main":                             "No full stop",
		"// Fit. Then more.":                                          "Fit.",
		"package main\n// late":                                       "",
	} {
		if got := Title(src); got != want {
			t.Errorf("Title(%q) = %q, want %q", src, got, want)
		}
	}
}
