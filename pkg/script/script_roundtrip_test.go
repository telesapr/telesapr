package script

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// scriptRoundTripKnown are the fixtures whose script doesn't build the
// model again, with why. The list may only shrink.
var scriptRoundTripKnown = map[string]string{
	// A file of every field set, whose references lead to entities it
	// hasn't: it doesn't open as a model, so there is none to write.
	"tlspr/v1/fields.tlspr": "every field set, not a model that regenerates",
}

// handlePrefix is the handle part of a group's name in Describe, which
// a model made again numbers afresh.
var handlePrefix = regexp.MustCompile(`\bg[0-9a-f]{3,}-`)

// volume is a volume in Describe, compared to four figures: a solid
// made again from the same sketch is the same to the tolerance its
// curves are followed at, not to the last bit.
var volume = regexp.MustCompile(`volume ([0-9.e+-]+) mm³`)

func roundVolume(s string) string {
	return volume.ReplaceAllStringFunc(s, func(m string) string {
		v, err := strconv.ParseFloat(volume.FindStringSubmatch(m)[1], 64)
		if err != nil {
			return m
		}
		return "volume " + strconv.FormatFloat(v, 'g', 4, 64) + " mm³"
	})
}

// modelSummary is what the round trip compares: each group's kind,
// name, requests and constraints counted, and how it solved with its
// DOF, problem, volume and bounds as Describe says them.
func modelSummary(d *api.Doc) []string {
	var out []string
	for _, g := range d.Groups() {
		out = append(out, fmt.Sprintf("%s %q: %d requests, %d constraints",
			g.Kind(), g.Name(), len(g.Requests()), len(g.Constraints())))
	}
	desc := d.Describe()
	if i := strings.Index(desc, "groups, in the order"); i >= 0 {
		desc = desc[i:]
	}
	if i := strings.Index(desc, "active group "); i >= 0 {
		desc = desc[:i]
	}
	for l := range strings.SplitSeq(desc, "\n") {
		l = strings.TrimSuffix(handlePrefix.ReplaceAllString(l, "g-"), "; active")
		l = roundVolume(l)
		out = append(out, l)
	}
	return out
}

// roundTrip writes the fixture's script, runs it on a new document and
// says how the models differ, "" when they don't.
func roundTrip(t *testing.T, path string) (string, []string) {
	t.Helper()
	orig, err := api.Open(path)
	if err != nil {
		return "opening: " + err.Error(), nil
	}
	src, notes := orig.Script()
	sk := sketch.NewSketch()
	sk.NewFile()
	host := api.NewSketchHost(sk, "")
	if err := RunSource(context.Background(), Options{Host: host}, "model.go", src); err != nil {
		if dir := os.Getenv("TELESAPR_SCRIPT_OUT"); dir != "" {
			_ = os.WriteFile(filepath.Join(dir, strings.ReplaceAll(strings.TrimPrefix(path, "../../testdata/"), "/", "_")+".go"), []byte(src), 0o644)
		}
		return "running the script: " + err.Error(), notes
	}
	made := api.NewDoc(host)
	want, got := modelSummary(orig), modelSummary(made)
	if slices.Equal(want, got) {
		return "", notes
	}
	if dir := os.Getenv("TELESAPR_SCRIPT_OUT"); dir != "" {
		_ = os.WriteFile(filepath.Join(dir, strings.ReplaceAll(strings.TrimPrefix(path, "../../testdata/"), "/", "_")+".go"), []byte(src), 0o644)
	}
	for i := range max(len(want), len(got)) {
		var w, g string
		if i < len(want) {
			w = want[i]
		}
		if i < len(got) {
			g = got[i]
		}
		if w != g {
			return fmt.Sprintf("differs at line %d:\n want %s\n  got %s", i, w, g), notes
		}
	}
	return "differs", notes
}

// Every fixture's script builds its model again, but those listed in
// scriptRoundTripKnown.
func TestScriptRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("writes and runs a script for every .tlspr fixture, about a minute")
	}
	var paths []string
	err := filepath.WalkDir("../../testdata", func(p string, de os.DirEntry, err error) error {
		if err == nil && !de.IsDir() && strings.HasSuffix(p, ".tlspr") {
			paths = append(paths, p)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(paths)
	type outcome struct{ path, diff string }
	results := make([]outcome, len(paths))
	t.Run("fixtures", func(t *testing.T) {
		for i, p := range paths {
			t.Run(strings.TrimPrefix(p, "../../testdata/"), func(t *testing.T) {
				t.Parallel()
				diff, _ := roundTrip(t, p)
				results[i] = outcome{p, diff}
			})
		}
	})
	ok := 0
	for _, r := range results {
		name := strings.TrimPrefix(r.path, "../../testdata/")
		_, known := scriptRoundTripKnown[name]
		switch {
		case r.diff == "":
			ok++
			if known {
				t.Errorf("%s comes back now: take it off scriptRoundTripKnown", name)
			}
		case !known:
			t.Errorf("%s: %s", name, r.diff)
		}
	}
	t.Logf("%d of %d fixtures come back from their scripts", ok, len(paths))
}
