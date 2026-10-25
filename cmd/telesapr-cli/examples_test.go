package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// asCLI is the environment variable that makes the test binary run as
// telesapr-cli (TestMain), so that an example can run in a process of
// its own, in its own working directory.
const asCLI = "TELESAPR_TEST_AS_CLI"

func TestMain(m *testing.M) {
	if os.Getenv(asCLI) == "1" {
		main()
	}
	os.Exit(m.Run())
}

// Every example under examples/scripts runs with telesapr-cli run,
// with no arguments, in a directory of its own (doc/done-audit.md,
// "Definition of done": an example for every feature, executed by a
// test). An example with a lib folder beside it has that as its script
// library.
func TestExamplesRun(t *testing.T) {
	if testing.Short() {
		t.Skip("every example script")
	}
	root, err := filepath.Abs(filepath.Join("..", "..", "examples", "scripts"))
	if err != nil {
		t.Fatal(err)
	}
	srcs, err := filepath.Glob(filepath.Join(root, "*", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	if len(srcs) == 0 {
		t.Fatal("no examples under examples/scripts")
	}
	for _, src := range srcs {
		rel, _ := filepath.Rel(root, src)
		t.Run(filepath.ToSlash(rel), func(t *testing.T) {
			args := []string{"run"}
			if lib := filepath.Join(filepath.Dir(src), "lib"); isDir(lib) {
				args = append(args, "--lib", lib)
			}
			args = append(args, src)
			// Under coverage the examples run in this process, one
			// after another, so that what they reach counts; else each
			// runs as telesapr-cli in a process of its own, side by side.
			if testing.CoverMode() != "" {
				t.Chdir(t.TempDir())
				status, _, stderr := runScripting(t, "", args...)
				if status != 0 {
					t.Fatalf("exit status %d:\n%s", status, stderr)
				}
				return
			}
			t.Parallel()
			cmd := exec.Command(os.Args[0], args...)
			cmd.Dir = t.TempDir()
			cmd.Env = append(os.Environ(), asCLI+"=1")
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil {
				if exit, ok := errors.AsType[*exec.ExitError](err); ok {
					t.Fatalf("exit status %d:\n%s", exit.ExitCode(), stderr.String())
				}
				t.Fatal(err)
			}
		})
	}
}

// isDir reports whether path is a directory.
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
