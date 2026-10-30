package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"testing"

	"github.com/reeflective/readline"

	"github.com/telesapr/telesapr/pkg/script"
)

// newEngine is an engine on a new sketch, as repl makes one, that
// shows on out.
func newEngine(t *testing.T, out io.Writer) *script.Engine {
	t.Helper()
	host, err := scriptSession("", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	e, err := script.New(script.Options{Host: host, Stdout: out, Stderr: out})
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// TestEnter runs what was entered at the terminal: several lines as
// one entry, values shown, an error stopping the rest, an exit ending
// the console.
func TestEnter(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	e := newEngine(t, &out)
	ctx := context.Background()
	for _, text := range []string{"x := 2", "for i := 0; i < x; i++ {\n\tfmt.Println(i)\n}", "x * 3", "nope\nx", "help()"} {
		if err := enter(ctx, e, &out, text); err != nil {
			t.Fatalf("%q: %v", text, err)
		}
	}
	if got, want := out.String()[:len("0\n1\n6\nerror: undefined: nope\ndoc is the sketch.")], "0\n1\n6\nerror: undefined: nope\ndoc is the sketch."; got != want {
		t.Errorf("the console showed %q, want %q", got, want)
	}
	var exit script.Exit
	if err := enter(ctx, e, &out, "os.Exit(4)"); !errors.As(err, &exit) || exit != 4 {
		t.Errorf("an exit gave %v", err)
	}
}

// TestCompletions offers names as readline replaces them: the whole
// word before the cursor, up to space.
func TestCompletions(t *testing.T) {
	t.Parallel()
	e := newEngine(t, io.Discard)
	c := completions(e, "x := fmt.Println(doc.Ske", len("x := fmt.Println(doc.Ske"))
	if c.PREFIX != "fmt.Println(doc.Ske" {
		t.Errorf("the prefix is %q", c.PREFIX)
	}
	found := false
	c.EachValue(func(v readline.Completion) readline.Completion {
		if v.Value == "fmt.Println(doc.Sketch" && v.Display == "Sketch" {
			found = true
		}
		return v
	})
	if !found {
		t.Error("doc.Ske doesn't complete to doc.Sketch")
	}
}

// TestHistorySource keeps readline's history in the console's file.
func TestHistorySource(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "history")
	h, err := script.LoadHistory(path)
	if err != nil {
		t.Fatal(err)
	}
	s := historySource{h}
	if n, err := s.Write("a := 1"); n != 1 || err != nil {
		t.Errorf("writing gave %d, %v", n, err)
	}
	if line, err := s.GetLine(0); line != "a := 1" || err != nil || s.Len() != 1 {
		t.Errorf("line 0 is %q, %v", line, err)
	}
	if _, err := s.GetLine(1); err == nil {
		t.Error("a line past the end")
	}
	if entries, ok := s.Dump().([]string); !ok || len(entries) != 1 {
		t.Errorf("dumped %v", s.Dump())
	}
	again, _ := script.LoadHistory(path)
	if len(again.Entries()) != 1 {
		t.Errorf("the file has %q", again.Entries())
	}
}

// TestShellKeys binds Alt+Backspace as a terminal sends it, Escape and
// DEL, which the library's own binding misses.
func TestShellKeys(t *testing.T) {
	t.Parallel()
	rl := newShell(newEngine(t, io.Discard))
	if got := rl.Config.Binds["emacs"]["\x1b\x7f"].Action; got != "backward-kill-word" {
		t.Errorf("Escape DEL is %q", got)
	}
	if !rl.AcceptMultiline([]rune("x := 1")) || rl.AcceptMultiline([]rune("if true {")) {
		t.Error("an unfinished statement isn't told from a finished one")
	}
}
