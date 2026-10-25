package fixtures

import (
	"os"
	"slices"
	"testing"
)

func TestPathAndWalk(t *testing.T) {
	t.Parallel()
	if _, err := os.Stat(Path("Gentium-R.ttf")); err != nil {
		t.Fatal(err)
	}
	files, err := Walk(".tlspr")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(files, "thread/screw_m8.tlspr") || !slices.IsSorted(files) {
		t.Errorf("walked %d files, sorted %t", len(files), slices.IsSorted(files))
	}
	if _, err := os.Stat(Path(files[0])); err != nil {
		t.Error(err)
	}
}

func TestSample(t *testing.T) {
	t.Parallel()
	xs := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	got := Sample(xs)
	want := xs
	if RaceDetector {
		want = []int{0, 5, 10}
	}
	if !slices.Equal(got, want) {
		t.Errorf("sample %v, want %v", got, want)
	}
	if got := sample(xs, true); !slices.Equal(got, []int{0, 5, 10}) {
		t.Errorf("thinned %v", got)
	}
	if got := sample(xs, false); !slices.Equal(got, xs) {
		t.Errorf("not thinned %v", got)
	}
}
