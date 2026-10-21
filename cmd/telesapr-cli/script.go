package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/telesapr/telesapr/pkg/script"
	"github.com/telesapr/telesapr/pkg/script/api"
)

// This file is the tool's scripting: running Go scripts on a sketch.

const scriptUsage = `
Scripting commands:
    run [--lib <dir>] [<sketch.tlspr>] <script.go> [<argument>...]
        Runs a Go script on the sketch, or on a new sketch if none is given.
        The script is a program, with "package main" and a main function, or
        just statements. The arguments are the script's os.Args[1:]. Nothing
        is saved unless the script does it.
    repl [--lib <dir>] [<sketch.tlspr>]
        Reads Go statements and expressions from standard input and runs each
        on the sketch, printing the values of the expressions.
    eval [--lib <dir>] [<sketch.tlspr>] <code> [<code>...]
        Runs each <code> as a line given to repl.

    --lib <dir> is the script library: a script's import "gears" reads the
    package in <dir>/gears. Without it the library is
    telesapr/library in the user's configuration directory, as in the
    window.

    A script works on the sketch through the package
    github.com/telesapr/telesapr/pkg/script/api, whose Current function
    gives the sketch as a document. In repl and eval, and in a script given as
    statements, the document is "doc" and the package's names need no prefix:
        doc.Sketch.Line(V(0, 0, 0), V(10, 0, 0)).Length()
`

// Exit statuses of the scripting commands.
const (
	exitOK = iota
	exitUsage
	exitLoad
	exitScript
)

func isScriptCommand(cmd string) bool {
	return cmd == "run" || cmd == "repl" || cmd == "eval"
}

// isSketch reports whether a command-line argument names a sketch
// file rather than a script. A SolveSpace model counts, so that loading
// it says it must be converted first.
func isSketch(arg string) bool {
	ext := filepath.Ext(arg)
	return strings.EqualFold(ext, ".tlspr") || strings.EqualFold(ext, ".slvs")
}

// scriptSession is a session for scripts, with file loaded if one is
// named: what the application says goes to stderr.
func scriptSession(file string, stderr io.Writer) (api.Host, error) {
	s := newSession(stderr)
	host := s.ss.ScriptHost()
	host.Messages = stderr
	if file == "" {
		return host, nil
	}
	abs, err := filepath.Abs(file)
	if err != nil {
		abs = file
	}
	s.ss.SaveFile = abs
	if err := s.ss.LoadFromFile(abs); err != nil {
		return nil, fmt.Errorf("cannot load '%s': %w", file, err)
	}
	s.ss.AfterNewFile()
	return host, nil
}

// runScript performs the scripting command args[1], and returns the
// tool's exit status.
func runScript(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	rest := args[2:]
	library := script.DefaultLibrary()
	for len(rest) > 0 && rest[0] == "--lib" {
		if len(rest) < 2 {
			fmt.Fprintln(stderr, "--lib needs the library's folder.")
			return exitUsage
		}
		library, rest = rest[1], rest[2:]
	}
	sketchFile := ""
	if len(rest) > 0 && isSketch(rest[0]) {
		sketchFile, rest = rest[0], rest[1:]
	}
	switch {
	case args[1] == "run" && len(rest) == 0:
		fmt.Fprintln(stderr, "A script must be specified.")
		return exitUsage
	case args[1] == "eval" && len(rest) == 0:
		fmt.Fprintln(stderr, "Code to run must be specified.")
		return exitUsage
	case args[1] == "repl" && len(rest) != 0:
		fmt.Fprintf(stderr, "Unrecognized option '%s'.\n", rest[0])
		return exitUsage
	}

	host, err := scriptSession(sketchFile, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "%s\n", capitalized(err.Error()))
		return exitLoad
	}
	opts := script.Options{
		Host:    host,
		Stdin:   stdin,
		Stdout:  stdout,
		Stderr:  stderr,
		Library: library,
		Open:    func(path string) (api.Host, error) { return scriptSession(path, stderr) },
		New: func() api.Host {
			h, _ := scriptSession("", stderr)
			return h
		},
	}

	switch args[1] {
	case "run":
		opts.Args = rest
		err = script.RunFile(ctx, opts, rest[0])
	default:
		var e *script.Engine
		if e, err = script.New(opts); err != nil {
			break
		}
		if args[1] == "eval" {
			err = e.REPL(ctx, strings.NewReader(strings.Join(rest, "\n")), stdout, false)
			if err == nil && e.Pending() {
				err = errors.New("the code ends in the middle of a statement")
			}
			break
		}
		// At a terminal, an interrupt stops the script that runs, not
		// the tool.
		prompt := isTerminal(stdin)
		if prompt {
			interrupts := make(chan os.Signal, 1)
			signal.Notify(interrupts, os.Interrupt)
			defer signal.Stop(interrupts)
			go func() {
				for range interrupts {
					e.Cancel()
				}
			}()
			ctx = context.WithoutCancel(ctx)
		}
		err = e.REPL(ctx, stdin, stdout, prompt)
	}

	var exit script.Exit
	switch {
	case err == nil:
		return exitOK
	case errors.As(err, &exit):
		return int(exit)
	case errors.Is(err, context.Canceled):
		fmt.Fprintln(stderr, "Interrupted.")
		return exitScript
	}
	fmt.Fprintln(stderr, err)
	return exitScript
}

// isTerminal reports whether r is a terminal, where a prompt is
// wanted.
func isTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// capitalized is s with its first letter in upper case, as a message
// on its own line starts.
func capitalized(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
