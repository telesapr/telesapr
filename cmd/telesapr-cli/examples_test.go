package main

import (
	"path/filepath"
	"testing"
)

// Every example under examples/scripts runs with telesapr-cli run,
// with no arguments, in a directory of its own (doc/done-audit.md,
// "Definition of done": an example for every feature, executed by a
// test).
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
			t.Chdir(t.TempDir())
			status, _, stderr := runScripting(t, "", "run", src)
			if status != 0 {
				t.Fatalf("exit status %d:\n%s", status, stderr)
			}
		})
	}
}
