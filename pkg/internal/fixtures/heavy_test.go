package fixtures

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// heavyNames is the HEAVY list of scripts/go-test.sh.
func heavyNames(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(Dir(), "..", "scripts", "go-test.sh"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?s)\nHEAVY=\$\{HEAVY:-(.*?)\}`).FindSubmatch(data)
	if m == nil {
		t.Fatal("no HEAVY list in scripts/go-test.sh")
	}
	return strings.Fields(strings.ReplaceAll(string(m[1]), "\\", " "))
}

// The HEAVY tests are matched by name in every package (go test -run
// and -skip take no package), so a name defined in two packages takes
// both out of make test, the light one too. Every name is defined once.
func TestHeavyNamesUnique(t *testing.T) {
	t.Parallel()
	heavy := map[string][]string{}
	for _, n := range heavyNames(t) {
		heavy[n] = nil
	}
	root := filepath.Join(Dir(), "..")
	def := regexp.MustCompile(`(?m)^func (Test\w+)\(`)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && path != root && skipDir(path, d.Name()) {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range def.FindAllSubmatch(data, -1) {
			if pkgs, ok := heavy[string(m[1])]; ok {
				rel, _ := filepath.Rel(root, filepath.Dir(path))
				heavy[string(m[1])] = append(pkgs, filepath.ToSlash(rel))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for n, pkgs := range heavy {
		switch {
		case len(pkgs) == 0:
			t.Errorf("HEAVY names %s, which no package defines", n)
		case len(pkgs) > 1:
			t.Errorf("HEAVY names %s, which %v each define: rename all but the heavy one", n, pkgs)
		}
	}
}

// skipDir reports whether the walk leaves out a directory: what go test
// ./... leaves out, hidden directories (.git, .gocache, worktrees under
// .claude), testdata and other modules (tools/lint), and node_modules.
func skipDir(path, name string) bool {
	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || name == "testdata" || name == "node_modules" {
		return true
	}
	_, err := os.Stat(filepath.Join(path, "go.mod"))
	return err == nil
}
