package check

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompareBaseline(t *testing.T) {
	t.Parallel()
	base := "a.slvs g4 tri=12 faces=6 naked=0 inter=0 failed=false vol=24 com=1,2,3 min=0,0,0 max=2,3,4"
	for _, c := range []struct {
		got  string
		same bool
	}{
		{base, true},
		{strings.Replace(base, "vol=24", "vol=24.000000000001", 1), true},
		{strings.Replace(base, "vol=24", "vol=24.0001", 1), false},
		{strings.Replace(base, "com=1,2,3", "com=1,2,3.1", 1), false},
		{strings.Replace(base, "com=1,2,3", "com=1,2", 1), false},
		{strings.Replace(base, "max=2,3,4", "max=2,x,4", 1), false},
		{strings.Replace(base, "naked=0", "naked=1", 1), false},
		{strings.Replace(base, "failed=false", "failed=true", 1), false},
		{"a.slvs g4 tri=0 faces=0 naked=0 inter=0 failed=false", false},
		{"lonely", false},
	} {
		if msg := CompareBaseline(c.got, base, 1e-9); (msg == "") != c.same {
			t.Errorf("compare(%q) = %q, want same=%t", c.got, msg, c.same)
		}
	}
	if k := BaselineKey("lonely"); k != "lonely" {
		t.Errorf("BaselineKey = %q", k)
	}
}

func TestReadBaseline(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "b.txt")
	if _, _, err := ReadBaseline(path); err == nil {
		t.Error("read a missing file")
	}
	data := "# comment\na.slvs g1 tri=0\n\nb.slvs g2 tri=4\n"
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	lines, order, err := ReadBaseline(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 2 || order[0] != "a.slvs g1" || lines["b.slvs g2"] != "b.slvs g2 tri=4" {
		t.Errorf("got %v %v", lines, order)
	}
}
