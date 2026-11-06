package topo_test

import (
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/fixtures"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// saddleRadii reads, from the fields of an OpenCASCADE saddle line, the
// radius or distance each of mixedSaddle's four edges is rounded at, ""
// for an edge left sharp or not picked: with the slopes, its first two
// fields in every file, the line's saddle.
type saddleRadii func(fs []string) [4]string

// saddleSharpRadii reads a line of fillet- and chamfer-saddle-sharp.txt:
// sx, sy, r, the edge left sharp, the volume.
func saddleSharpRadii(fs []string) [4]string {
	rs := [4]string{fs[2], fs[2], fs[2], fs[2]}
	rs[saddleEdge(fs[3])] = ""
	return rs
}

// saddleUnlikeRadii reads a line of the -sharp-unlike files: sx, sy, the
// four edges' radii (0 for the one left sharp), its number, the volume.
func saddleUnlikeRadii(fs []string) [4]string { return [4]string(fs[2:6]) }

// saddleEdgesRadii reads a line of the -one, -pair and -side files: sx,
// sy, r, the edges rounded, the volume, the count of faces.
func saddleEdgesRadii(fs []string) [4]string {
	var rs [4]string
	for _, e := range fs[3 : len(fs)-2] {
		rs[saddleEdge(e)] = fs[2]
	}
	return rs
}

// saddlePairUnlikeRadii reads a line of the -pair-unlike files: sx, sy,
// the two edges rounded, their radii, the volume, the count of faces.
func saddlePairUnlikeRadii(fs []string) [4]string {
	var rs [4]string
	rs[saddleEdge(fs[2])], rs[saddleEdge(fs[3])] = fs[4], fs[5]
	return rs
}

// saddleEdge is the number of an edge, as a line writes it.
func saddleEdge(f string) int {
	e, err := strconv.ParseFloat(f, 64)
	if err != nil || e < 0 || e > 3 || e != math.Trunc(e) {
		panic("saddle line: no edge " + f)
	}
	return int(e)
}

// saddleMirror is the key of a saddle line up to its mirror images: z =
// 10 + sx|x-10| - sy|y-10| is the same saddle mirrored in x = 10, which
// swaps its edges 0 and 1, and in y = 10, which swaps 2 and 3, whatever
// its slopes; so a line and its mirror images round the same solid but
// for the mirror, and OpenCASCADE's volumes for them agree to 0.014 mm³
// (but for occSaddleMirrors' line). The key is the least of the four.
func saddleMirror(fs []string, radii saddleRadii) string {
	rs := radii(fs)
	var keys []string
	for _, m := range [][4]int{{0, 1, 2, 3}, {1, 0, 2, 3}, {0, 1, 3, 2}, {1, 0, 3, 2}} {
		keys = append(keys, strings.Join([]string{fs[0], fs[1], rs[m[0]], rs[m[1]], rs[m[2]], rs[m[3]]}, " "))
	}
	return slices.Min(keys)
}

// saddleLines is the OpenCASCADE fixture lines a saddle test rounds: in
// the ordinary run the first line of each mirror image (saddleMirror),
// 430 of the twelve files' 968 lines, the side files' a fourth, since
// TestSaddleMirrorsAgree holds mirror images to one volume; every line
// under TELESAPR_SWEEP=1 (sweep.Full). Under -short and the race
// detector three of the fifth fixtures.Sample keeps: a saddle's corner
// is made on the same goroutines whatever its slopes, and the ten saddle
// tests took a fifth each of pkg/topo's time under the race detector,
// with the stage over its limit (2026-11-04).
func saddleLines(lines []string, radii saddleRadii) []string {
	if !sweep.Full() {
		seen := map[string]bool{}
		lines = slices.DeleteFunc(slices.Clone(lines), func(l string) bool {
			k := saddleMirror(strings.Fields(l), radii)
			if seen[k] {
				return true
			}
			seen[k] = true
			return false
		})
	}
	lines = fixtures.Sample(lines)
	if fixtures.RaceDetector || testing.Short() {
		lines = lines[:min(3, len(lines))]
	}
	return lines
}

// TestSaddleMirrorsAgree: a saddle rounded with a convex edge left sharp
// and its mirror image in x, and one with a concave edge left sharp and
// its mirror in y, give one volume each, which lets the oracle tests
// round one line of each mirror image (saddleLines); and the lines of
// every file fall into as many mirror images as counted there.
func TestSaddleMirrorsAgree(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		sx, sy, r float64
		sharp     [2]int
	}{
		{0.5, 0.2, 1, [2]int{0, 1}},
		{0.2, 0.8, 1, [2]int{2, 3}},
	} {
		var vols [2]float64
		for i, sharp := range c.sharp {
			sh, picks, err := saddleWithout(c.sx, c.sy, c.r, sharp, srf.Tol(0.002, 20))
			if err != nil {
				t.Fatal(err)
			}
			if err := topo.FilletPicks(sh, picks, c.r, 1<<30, false); err != nil {
				t.Fatalf("%+v sharp %d: %v", c, sharp, err)
			}
			if vols[i], err = soundVolume(sh); err != nil {
				t.Fatalf("%+v sharp %d: %v", c, sharp, err)
			}
		}
		if d := math.Abs(vols[0] - vols[1]); d > saddleMirrorAllow {
			t.Errorf("%+v: volumes %.9f and %.9f of mirror images, %.2g apart", c, vols[0], vols[1], d)
		}
	}
	for _, f := range []struct {
		name   string
		radii  saddleRadii
		groups int
	}{
		{"fillet-saddle-sharp.txt", saddleSharpRadii, 54},
		{"chamfer-saddle-sharp.txt", saddleSharpRadii, 54},
		{"fillet-saddle-sharp-unlike.txt", saddleUnlikeRadii, 42},
		{"chamfer-saddle-sharp-unlike.txt", saddleUnlikeRadii, 42},
		{"fillet-saddle-side.txt", saddleEdgesRadii, 21},
		{"chamfer-saddle-side.txt", saddleEdgesRadii, 21},
		{"fillet-saddle-one.txt", saddleEdgesRadii, 42},
		{"chamfer-saddle-one.txt", saddleEdgesRadii, 42},
		{"fillet-saddle-pair.txt", saddleEdgesRadii, 42},
		{"chamfer-saddle-pair.txt", saddleEdgesRadii, 42},
		{"fillet-saddle-pair-unlike.txt", saddlePairUnlikeRadii, 54},
		{"chamfer-saddle-pair-unlike.txt", saddlePairUnlikeRadii, 56},
	} {
		data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "topo", f.name))
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		keys := map[string]bool{}
		for _, l := range lines {
			keys[saddleMirror(strings.Fields(l), f.radii)] = true
		}
		if len(keys) != f.groups {
			t.Errorf("%s: %d lines in %d mirror images, want %d", f.name, len(lines), len(keys), f.groups)
		}
	}
}

// saddleMirrorAllow is how far a saddle's volume may lie from its mirror
// image's: the two are meshed apart, 6e-5 mm³ at the most of the pairs
// here (2026-11-06), a three-hundredth of saddleChords, which the
// oracle tests allow besides OpenCASCADE's differences.
const saddleMirrorAllow = 1e-3
