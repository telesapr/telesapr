package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/reeflective/readline"

	"github.com/telesapr/telesapr/pkg/script"
)

// This file is repl at a terminal: what is typed is edited as readline
// does it, as in the window's console.

// interactive runs a console on the terminal: it runs help(), then
// reads what is typed, with readline's keys, Tab completing names and
// the history kept in historyFile, until the input ends. Return on a
// statement that isn't finished goes on to another line.
func interactive(ctx context.Context, e *script.Engine, out io.Writer, historyFile string) error {
	rl := newShell(e)
	history, err := script.LoadHistory(historyFile)
	if err != nil {
		_, _ = fmt.Fprintln(out, "error:", err)
	}
	rl.History.Add("script", historySource{history})

	_, _ = fmt.Fprintln(out, ">>> help()")
	if err := enter(ctx, e, out, "help()"); err != nil {
		return err
	}
	for {
		text, err := rl.Readline()
		switch {
		case errors.Is(err, readline.ErrInterrupt):
			// Control+C drops what is typed, and a statement half
			// entered.
			e.Reset()
			continue
		case errors.Is(err, io.EOF):
			_, _ = fmt.Fprintln(out)
			return nil
		case err != nil:
			return fmt.Errorf("reading the terminal: %w", err)
		}
		if err := enter(ctx, e, out, text); err != nil {
			return err
		}
	}
}

// newShell is the line editor of a console on e: its prompts, where a
// statement goes on to another line, and Tab.
func newShell(e *script.Engine) *readline.Shell {
	rl := readline.NewShell()
	// A terminal sends Escape and DEL for Alt+Backspace, which the
	// library's own binding misses: it reads its \M-\C-? as Escape and
	// 0x1f.
	_ = rl.Config.Bind("emacs", "\x1b\x7f", "backward-kill-word", false)
	rl.Prompt.Primary(func() string {
		if e.Pending() {
			return "... "
		}
		return ">>> "
	})
	rl.Prompt.Secondary(func() string { return "... " })
	rl.AcceptMultiline = func(line []rune) bool { return !e.Unfinished(string(line)) }
	rl.Completer = func(line []rune, cursor int) readline.Completions {
		return completions(e, string(line), len(string(line[:cursor])))
	}
	return rl
}

// enter runs what was entered, one line or several, as REPL runs its
// lines: values and errors are shown, the first error stops the rest,
// and only a script's exit ends the console, with its status.
func enter(ctx context.Context, e *script.Engine, out io.Writer, text string) error {
	for line := range strings.SplitSeq(text, "\n") {
		shown, _, err := e.Feed(ctx, line)
		var exit script.Exit
		switch {
		case errors.As(err, &exit):
			return exit
		case err != nil:
			_, _ = fmt.Fprintln(out, "error:", err)
			return nil
		case shown != "":
			_, _ = fmt.Fprintln(out, shown)
		}
	}
	return nil
}

// completions are what Tab offers for the name that ends at cursor.
// readline replaces the word of everything but space before the
// cursor, so each is that word with the name completed; it lists the
// names alone.
func completions(e *script.Engine, text string, cursor int) readline.Completions {
	start, names := e.Complete(text, cursor)
	before := text[:cursor]
	word := before[strings.LastIndexFunc(before, unicode.IsSpace)+1:]
	head := word[:len(word)-(cursor-start)]
	values := make([]readline.Completion, 0, len(names))
	for _, name := range names {
		values = append(values, readline.Completion{Value: head + name, Display: name})
	}
	c := readline.CompleteRaw(values)
	c.PREFIX = word
	return c
}

// historySource keeps readline's history in a script.History, the
// file the window's console keeps its own in.
type historySource struct{ h *script.History }

func (s historySource) Write(line string) (int, error) {
	err := s.h.Add(line)
	return len(s.h.Entries()), err
}

func (s historySource) GetLine(pos int) (string, error) {
	entries := s.h.Entries()
	if pos < 0 || pos >= len(entries) {
		return "", fmt.Errorf("no history entry %d", pos)
	}
	return entries[pos], nil
}

func (s historySource) Len() int  { return len(s.h.Entries()) }
func (s historySource) Dump() any { return s.h.Entries() }
