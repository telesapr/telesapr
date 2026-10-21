package main

import (
	"os"
	"path/filepath"
	"testing"
)

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
			t.Chdir(t.TempDir())
			status, _, stderr := runScripting(t, "", append(args, src)...)
			if status != 0 {
				t.Fatalf("exit status %d:\n%s", status, stderr)
			}
		})
	}
}

// isDir reports whether path is a directory.
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
