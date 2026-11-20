package script

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/telesapr/telesapr/examples"
)

// This file is help("examples") and help("example bolt"): the example
// scripts built into the program, listed and shown, so that a console
// or an AI model reading api_help has them where examples/scripts is
// not on disk.

// examplesName is the name help lists the examples for, and
// exampleWord the word before an example's name. A name with a space
// is no name of Go, and no menu is called Example, so neither can mean
// anything else.
const (
	examplesName = "examples"
	exampleWord  = "example"
)

// runScriptNote is how an example differs from what run_script and a
// console take.
const runScriptNote = `It is a whole program, as telesapr-cli run and Script > Run File take it: package main, its imports, and doc from api.Current(). In run_script or the console, doc is there already and nothing is imported: send the body of main as statements (api.V and V are the same there). Leave out the files it writes unless they are asked for.`

// helpExample writes what help says of name if it asks for the examples:
// ok is false if it doesn't, and then nothing was written.
func helpExample(w io.Writer, name string) (ok bool, err error) {
	word, rest, _ := strings.Cut(name, " ")
	switch {
	case strings.EqualFold(name, examplesName):
		return true, listExamples(w)
	case strings.EqualFold(word, exampleWord), strings.EqualFold(word, examplesName):
		return true, showExample(w, strings.TrimSpace(rest))
	}
	return false, nil
}

// listExamples writes every example's name and what it makes.
func listExamples(w io.Writer) error {
	list, err := examples.List()
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(w, "The example scripts, by name; %q shows one, as %q.\n\n", exampleWord+" <name>", exampleWord+" bolt")
	width := 0
	for _, e := range list {
		width = max(width, len(e.Name))
	}
	for _, e := range list {
		_, _ = fmt.Fprintf(w, "%-*s  %s\n", width, e.Name, e.Title)
	}
	return nil
}

// showExample writes the source of the example called name, with its
// script library's files after it.
func showExample(w io.Writer, name string) error {
	files, err := examples.Files(name)
	if errors.Is(err, examples.ErrUnknown) {
		if near := examples.Nearest(name); len(near) > 0 {
			return fmt.Errorf("no example is called %q; the nearest are %s; %q lists them all", name, quoted(near), examplesName)
		}
		return fmt.Errorf("no example is called %q; %q lists them", name, examplesName)
	}
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(w, "Example %s: %s\n%s\n", name, examples.Title(files[0].Text), runScriptNote)
	for _, f := range files {
		if len(files) > 1 {
			_, _ = fmt.Fprintf(w, "\n// ===== %s\n", f.Path)
		}
		_, _ = fmt.Fprintf(w, "\n%s", f.Text)
	}
	return nil
}

// quoted is names, each in quotes, separated by commas.
func quoted(names []string) string {
	q := make([]string, len(names))
	for i, n := range names {
		q[i] = fmt.Sprintf("%q", n)
	}
	return strings.Join(q, ", ")
}
