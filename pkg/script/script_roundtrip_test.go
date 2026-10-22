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
	// Drawn by the DXF import as no script can draw.
	"importdxf/golden/basic.3d.tlspr":                         dxf3dArc,
	"importdxf/golden/binary.3d.tlspr":                        dxf3dArc,
	"importdxf/golden/negative_extrusion.3d.tlspr":            dxf3dArc,
	"importdxf/golden/polylines.3d.tlspr":                     dxf3dArc,
	"importdxf/golden/blocks.tlspr":                           dxfWorkplane,
	"importdxf/golden/negative_extrusion.tlspr":               dxfWorkplane,
	"importdxf/golden/ezdxf/3dface.3d.tlspr":                  dxfTwice,
	"importdxf/golden/ezdxf/3dface.tlspr":                     dxfTwice,
	"importdxf/golden/ezdxf/colors.3d.tlspr":                  dxfTwice,
	"importdxf/golden/ezdxf/colors.tlspr":                     dxfTwice,
	"importdxf/golden/ezdxf/text_negative_extrusion.3d.tlspr": dxfTwice,
	"importdxf/golden/ezdxf/text_negative_extrusion.tlspr":    dxfTwice,
	// Not written yet (doc/notes/model-to-script.md).
	"tlspr/golden/constraint/equal_angle/normal.tlspr":             fourEntities,
	"tlspr/golden/constraint/equal_angle/normal_old_version.tlspr": fourEntities,
	"tlspr/golden/constraint/equal_angle/other.tlspr":              fourEntities,
	"tlspr/golden/constraint/pt_face_distance/normal.tlspr":        notSketch,
	"tlspr/golden/constraint/pt_face_distance/reference.tlspr":     notSketch,
	"tlspr/golden/constraint/pt_on_face/normal.tlspr":              notSketch,
	"tlspr/golden/request/cubic/normal.tlspr":                      spline,
	"tlspr/golden/request/cubic_periodic/normal.tlspr":             spline,
	"ui/spline_line.tlspr":                                         spline,
	"tlspr/golden/request/image/normal.tlspr":                      image,
	"tlspr/v1/image/image.tlspr":                                   image,
	// The font the fixture names is beside it, and a script's text is
	// looked for among the fonts installed.
	"tlspr/golden/request/ttf_text/extrude.tlspr":         font,
	"tlspr/golden/request/ttf_text/extrude_kerning.tlspr": font,
	"tlspr/golden/request/ttf_text/kerning.tlspr":         font,
	"tlspr/golden/request/ttf_text/normal.tlspr":          font,
	"tlspr/v1/text.tlspr":                                 font,
	// A file whose model doesn't regenerate.
	"tlspr/v1/fields.tlspr": "every field set, not a model that regenerates",
}

const (
	dxf3dArc     = "an arc drawn in 3d, which a script draws only in a workplane"
	dxfWorkplane = "a workplane drawn in a workplane, which a script draws in 3d"
	dxfTwice     = "the same constraint twice, which a script adds once"
	fourEntities = "a constraint on four entities, not written yet"
	notSketch    = "a request in a group that isn't a sketch, which a script can't draw"
	spline       = "a spline of more than four points or a periodic one, not written yet"
	image        = "an image, not written yet"
	font         = "a font beside the fixture, not installed"
)

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
