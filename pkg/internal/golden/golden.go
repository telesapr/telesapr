// Package golden compares test output with recorded goldens. Most of
// them were first recorded by the C++ SolveSpace harnesses this project
// grew out of; since the C++ comparison was frozen (roadmap package
// 5.1), Go writes them: TELESAPR_REGEN_GOLDEN=1, which
// scripts/regen-golden.sh sets, makes Check, CheckDump and CheckDir
// rewrite the golden instead of comparing with it.
package golden

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// Regen reports whether goldens are being rewritten.
func Regen() bool { return os.Getenv("TELESAPR_REGEN_GOLDEN") != "" }

// write rewrites a golden file, making its directory.
func write(t testing.TB, path string, got []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, got, 0o644); err != nil {
		t.Fatal(err)
	}
}

// read returns a golden file, or fails the test with a hint on how to
// record it.
func read(t testing.TB, path string) []byte {
	t.Helper()
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("golden: %v (record it with scripts/regen-golden.sh)", err)
	}
	return want
}

// Check requires got to be the golden at path, byte for byte.
func Check(t testing.TB, path string, got []byte) {
	t.Helper()
	if Regen() {
		write(t, path, got)
		return
	}
	if want := read(t, path); !bytes.Equal(want, got) {
		t.Errorf("differs from %s: %s", path, FirstDifference(want, got))
	}
}

// CheckDump requires got to be the golden dump at path, as CompareDump
// compares them.
func CheckDump(t testing.TB, path, got string) {
	t.Helper()
	if Regen() {
		write(t, path, []byte(got))
		return
	}
	CompareDump(t, string(read(t, path)), got)
}

// CheckDir requires the files of gotDir to be those of the golden
// directory wantDir, by name and byte for byte. Regenerating replaces
// wantDir's files.
func CheckDir(t testing.TB, wantDir, gotDir string) {
	t.Helper()
	got := dirFiles(t, gotDir)
	if Regen() {
		if err := os.RemoveAll(wantDir); err != nil {
			t.Fatal(err)
		}
		for _, name := range got {
			data, err := os.ReadFile(filepath.Join(gotDir, name))
			if err != nil {
				t.Fatal(err)
			}
			write(t, filepath.Join(wantDir, name), data)
		}
		return
	}
	want := dirFiles(t, wantDir)
	if strings.Join(want, " ") != strings.Join(got, " ") {
		t.Errorf("files: golden %v, got %v", want, got)
	}
	for _, name := range want {
		w := read(t, filepath.Join(wantDir, name))
		g, err := os.ReadFile(filepath.Join(gotDir, name))
		if err != nil {
			continue
		}
		if !bytes.Equal(w, g) {
			t.Errorf("%s differs: %s", name, FirstDifference(w, g))
		}
	}
}

func dirFiles(t testing.TB, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	slices.Sort(names)
	return names
}

// FirstDifference describes the first line where two files differ.
func FirstDifference(want, got []byte) string {
	wl := strings.Split(string(want), "\n")
	gl := strings.Split(string(got), "\n")
	for i := 0; i < len(wl) || i < len(gl); i++ {
		var w, g string
		if i < len(wl) {
			w = wl[i]
		}
		if i < len(gl) {
			g = gl[i]
		}
		if w != g {
			return fmt.Sprintf("line %d: want %q, got %q", i+1, w, g)
		}
	}
	return "lengths"
}

// CompareDump compares two dumps line by line and field by field; a
// field that differs as text is read as a float, since C's %a, in which
// the C++ harnesses wrote the first goldens, and Go's %x spell hex
// floats' exponents differently.
func CompareDump(t testing.TB, want, got string) {
	t.Helper()
	ws := bufio.NewScanner(strings.NewReader(want))
	gs := bufio.NewScanner(strings.NewReader(got))
	ws.Buffer(nil, 1<<24)
	gs.Buffer(nil, 1<<24)
	for line := 1; ; line++ {
		wok, gok := ws.Scan(), gs.Scan()
		if !wok || !gok {
			if wok != gok {
				t.Fatalf("line %d: dump lengths differ (the golden has more: %v)", line, wok)
			}
			return
		}
		wf, gf := strings.Fields(ws.Text()), strings.Fields(gs.Text())
		if len(wf) != len(gf) {
			t.Fatalf("line %d:\n want: %s\n  got: %s", line, ws.Text(), gs.Text())
		}
		for i := range wf {
			if wf[i] == gf[i] {
				continue
			}
			wv, werr := strconv.ParseFloat(wf[i], 64)
			gv, gerr := strconv.ParseFloat(gf[i], 64)
			if werr != nil || gerr != nil || wv != gv {
				t.Fatalf("line %d field %d:\n want: %s\n  got: %s", line, i, ws.Text(), gs.Text())
			}
		}
	}
}
