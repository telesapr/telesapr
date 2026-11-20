// Package examples is the example scripts of examples/scripts, built
// into the program so that help can show them where the files are not
// at hand: to an AI model through the MCP server's api_help, and in the
// script console. It knows nothing of the kernel or of scripting; the
// files are text to it.
package examples

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
	"sync"
	"unicode"
)

// The scripts, each examples/scripts/<name>/<name>.go with, for some, a
// script library in lib beside it.
//
//go:embed scripts
var files embed.FS

// ErrUnknown is what Files returns for a name no example has.
var ErrUnknown = errors.New("no such example")

// Example is one example script: its directory's name and the first
// sentence of the comment it starts with, which says what it makes.
type Example struct {
	Name  string
	Title string
}

// File is one file of an example: its path from the example's
// directory, as "bolt.go" or "lib/gears/gears.go", and what is in it.
type File struct {
	Path string
	Text string
}

// list is every example, by name, read once.
var list = sync.OnceValues(func() ([]Example, error) {
	dirs, err := fs.ReadDir(files, "scripts")
	if err != nil {
		return nil, fmt.Errorf("listing the examples: %w", err)
	}
	var out []Example
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		name := d.Name()
		text, err := fs.ReadFile(files, path.Join("scripts", name, name+".go"))
		if err != nil {
			return nil, fmt.Errorf("reading example %s: %w", name, err)
		}
		title := Title(string(text))
		if title == "" {
			return nil, fmt.Errorf("example %s: no comment says what it makes", name)
		}
		out = append(out, Example{Name: name, Title: title})
	}
	return out, nil
})

// List is every example, in the order of their names. It fails only if
// an example has no comment to take its title from, which the package's
// tests rule out.
func List() ([]Example, error) {
	l, err := list()
	return slices.Clone(l), err
}

// Files is the files of the example called name: its script first, then
// those of its script library, if it has one, in the order of their
// paths. An unknown name is ErrUnknown.
func Files(name string) ([]File, error) {
	if name == "" || strings.ContainsAny(name, `/\.`) {
		return nil, fmt.Errorf("%w: %q", ErrUnknown, name)
	}
	dir := path.Join("scripts", name)
	main, err := fs.ReadFile(files, path.Join(dir, name+".go"))
	if err != nil {
		return nil, fmt.Errorf("%w: %q", ErrUnknown, name)
	}
	out := []File{{Path: name + ".go", Text: string(main)}}
	lib := path.Join(dir, "lib")
	err = fs.WalkDir(files, lib, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		text, err := fs.ReadFile(files, p)
		if err != nil {
			return err
		}
		out = append(out, File{Path: strings.TrimPrefix(p, dir+"/"), Text: string(text)})
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("reading the library of example %s: %w", name, err)
	}
	return out, nil
}

// Nearest is the names of the examples most like name, best first and at
// most a few: those whose names are a letter or two from it or hold it,
// then those whose titles have it as a word, so that "thread" finds the
// threaded ones and "gear" the library's gear blank.
func Nearest(name string) []string {
	const most = 8
	name = strings.ToLower(strings.TrimSpace(name))
	l, _ := list()
	if name == "" {
		return nil
	}
	var near, about []string
	for _, e := range l {
		switch {
		case strings.Contains(e.Name, name) || strings.Contains(name, e.Name) || distance(e.Name, name) <= 2:
			near = append(near, e.Name)
		case slices.ContainsFunc(strings.FieldsFunc(strings.ToLower(e.Title), notLetter), func(w string) bool {
			return strings.HasPrefix(w, name)
		}):
			about = append(about, e.Name)
		}
	}
	slices.SortStableFunc(near, func(a, b string) int { return distance(a, name) - distance(b, name) })
	out := slices.Concat(near, about)
	return out[:min(len(out), most)]
}

// notLetter splits a title into its words.
func notLetter(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }

// distance is the number of letters to add, take away or change to make
// a into b.
func distance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	row := make([]int, len(rb)+1)
	for j := range row {
		row[j] = j
	}
	for i := range ra {
		prev := row[0]
		row[0] = i + 1
		for j := range rb {
			cost := 1
			if ra[i] == rb[j] {
				cost = 0
			}
			prev, row[j+1] = row[j+1], min(row[j+1]+1, row[j]+1, prev+cost)
		}
	}
	return row[len(rb)]
}

// Title is the first sentence of the comment a script starts with,
// after any build constraint, on one line and without a closing ", made
// by a script"; "" if it starts with no comment.
func Title(src string) string {
	var words []string
	for line := range strings.Lines(src) {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "//go:build"):
			continue
		case line == "" && len(words) == 0:
			continue
		case !strings.HasPrefix(line, "//"):
			return sentence(words)
		}
		text := strings.TrimSpace(strings.TrimPrefix(line, "//"))
		if text == "" {
			// The end of the comment's first paragraph.
			return sentence(words)
		}
		words = append(words, strings.Fields(text)...)
		if strings.HasSuffix(text, ".") {
			return sentence(words)
		}
	}
	return sentence(words)
}

// sentence is words up to the first that ends one, without the ", made
// by a script" nearly every example says, which tells a list of them
// nothing.
func sentence(words []string) string {
	s := strings.Join(words, " ")
	for i, w := range words {
		if strings.HasSuffix(w, ".") {
			s = strings.Join(words[:i+1], " ")
			break
		}
	}
	for _, said := range []string{", made by a script.", " made by a script."} {
		if t, ok := strings.CutSuffix(s, said); ok {
			return t + "."
		}
	}
	return s
}
