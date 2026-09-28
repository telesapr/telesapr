package ttf

import (
	"os"
	"strings"
	"testing"
)

// TestDumpVsCppFile compares dump with a C++ harness dump given by
// TTF_CPP_DUMP, for the fonts listed (one per line) in TTF_FONTS and
// the strings in TTF_STRS; see scripts/ttfcheck.sh.
func TestDumpVsCppFile(t *testing.T) {
	dumpFile := os.Getenv("TTF_CPP_DUMP")
	if dumpFile == "" {
		t.Skip("TTF_CPP_DUMP not set")
	}
	want, err := os.ReadFile(dumpFile)
	if err != nil {
		t.Fatal(err)
	}
	fonts, err := os.ReadFile(os.Getenv("TTF_FONTS"))
	if err != nil {
		t.Fatal(err)
	}
	strs, err := os.ReadFile(os.Getenv("TTF_STRS"))
	if err != nil {
		t.Fatal(err)
	}
	got := dump(lines(string(fonts)), lines(string(strs)))
	if got != string(want) {
		os.WriteFile(dumpFile+".go", []byte(got), 0o644)
		wl, gl := strings.Split(string(want), "\n"), strings.Split(got, "\n")
		for i := 0; i < len(wl) && i < len(gl); i++ {
			if wl[i] != gl[i] {
				t.Fatalf("line %d differs:\nC++: %s\nGo:  %s", i+1, wl[i], gl[i])
			}
		}
		t.Fatalf("lengths differ: C++ %d lines, Go %d", len(wl), len(gl))
	}
}

func lines(s string) []string {
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}
