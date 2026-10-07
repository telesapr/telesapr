package slvs

import (
	"cmp"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
)

// TestParityConstraintFixtures walks every non-boolean constraint
// fixture through LoadFile + Sketch::GenerateAll and asserts that the
// load succeeds, every group reports Clean=true, and every group's
// Solved.How is either SolveOkay or SolveRedundantOkay. This is the
// parity gate for Phase E.
func TestParityConstraintFixtures(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(thisFile))))
	constraintsRoot := filepath.Join(root, "testdata/constraint")
	entries, err := os.ReadDir(constraintsRoot)
	if err != nil {
		t.Fatalf("readDir: %v", err)
	}
	slices.SortFunc(entries, func(a, b os.DirEntry) int { return cmp.Compare(a.Name(), b.Name()) })

	var solved, notSolved, skipped int
	var failing []string
	for _, dir := range entries {
		if !dir.IsDir() {
			continue
		}
		dirName := dir.Name()
		if dirName == "large_dimensions" {
			skipped++
			continue
		}
		fixturePath := filepath.Join(constraintsRoot, dirName, "normal.slvs")
		if _, err := os.Stat(fixturePath); err != nil {
			skipped++
			continue
		}
		doc, err := LoadFile(fixturePath)
		if err != nil {
			notSolved++
			failing = append(failing, dirName)
			continue
		}
		doc.Sketch.GenerateAll()
		allClean := true
		anySolveFailed := false
		for i := range doc.Sketch.Groups {
			g := &doc.Sketch.Groups[i]
			if !g.Clean {
				allClean = false
			}
			if g.Solved.How != core.SolveOkay && g.Solved.How != core.SolveRedundantOkay {
				anySolveFailed = true
			}
		}
		if allClean && !anySolveFailed {
			solved++
		} else {
			notSolved++
			failing = append(failing, dirName)
		}
	}
	t.Logf("constraint parity: solved=%d failed=%d skipped=%d", solved, notSolved, skipped)
	if notSolved > 0 {
		t.Errorf("constraint fixtures failed to solve: %v", failing)
	}
	if solved < 25 {
		t.Errorf("expected at least 25 constraint fixtures to solve, got %d", solved)
	}
}

// constraintFixtureStats is the recorded baseline of expected
// group/constraint/entity counts per constraint fixture. The
// values come from the Go port's first run after the SLVS parser
// was taught to associate entities with their parent group via the
// workplane reference (Phase E).
type constraintFixtureStats struct {
	Groups      int
	Entities    int
	Constraints int
}

var constraintFixtureGoldens = map[string]constraintFixtureStats{
	"angle":              {Groups: 2, Entities: 18, Constraints: 2},
	"arc_line_tangent":   {Groups: 2, Entities: 23, Constraints: 4},
	"comment":            {Groups: 2, Entities: 12, Constraints: 1},
	"cubic_line_tangent": {Groups: 2, Entities: 23, Constraints: 4},
	"diameter":           {Groups: 2, Entities: 16, Constraints: 1},
	"eq_len_pt_line_d":   {Groups: 2, Entities: 19, Constraints: 1},
	"eq_pt_ln_distances": {Groups: 2, Entities: 20, Constraints: 1},
	"equal_angle":        {Groups: 2, Entities: 24, Constraints: 3},
	"equal_length_lines": {Groups: 2, Entities: 18, Constraints: 1},
	"equal_line_arc_len": {Groups: 2, Entities: 20, Constraints: 1},
	"equal_radius":       {Groups: 2, Entities: 20, Constraints: 1},
	"length_difference":  {Groups: 2, Entities: 18, Constraints: 1},
	"length_ratio":       {Groups: 2, Entities: 18, Constraints: 1},
	"parallel":           {Groups: 2, Entities: 18, Constraints: 1},
	"perpendicular":      {Groups: 2, Entities: 18, Constraints: 1},
	"points_coincident":  {Groups: 2, Entities: 14, Constraints: 1},
	"proj_pt_distance":   {Groups: 2, Entities: 14, Constraints: 1},
	"pt_face_distance":   {Groups: 3, Entities: 33, Constraints: 2},
	"pt_in_plane":        {Groups: 2, Entities: 13, Constraints: 1},
	"pt_line_distance":   {Groups: 2, Entities: 16, Constraints: 1},
	"pt_on_circle":       {Groups: 2, Entities: 17, Constraints: 1},
	"pt_on_face":         {Groups: 3, Entities: 33, Constraints: 2},
	"pt_on_line":         {Groups: 2, Entities: 16, Constraints: 1},
	"pt_plane_distance":  {Groups: 2, Entities: 13, Constraints: 1},
	"pt_pt_distance":     {Groups: 2, Entities: 14, Constraints: 1},
	"same_orientation":   {Groups: 2, Entities: 16, Constraints: 1},
	"symmetric":          {Groups: 2, Entities: 14, Constraints: 1},
	"symmetric_horiz":    {Groups: 2, Entities: 14, Constraints: 1},
	"symmetric_line":     {Groups: 2, Entities: 17, Constraints: 1},
	"symmetric_vert":     {Groups: 2, Entities: 14, Constraints: 1},
	"where_dragged":      {Groups: 2, Entities: 13, Constraints: 1},
}

// TestParityConstraintCounts asserts each constraint fixture's
// group/entity/constraint counts match the recorded baseline. This
// is a structural parity check that catches regressions in the
// parser's entity-group association.
func TestParityConstraintCounts(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(thisFile))))
	for name, golden := range constraintFixtureGoldens {
		path := filepath.Join(root, "testdata/constraint", name, "normal.slvs")
		doc, err := LoadFile(path)
		if err != nil {
			t.Errorf("%s: load: %v", name, err)
			continue
		}
		doc.Sketch.GenerateAll()
		if got := len(doc.Sketch.Groups); got != golden.Groups {
			t.Errorf("%s: groups: got %d, want %d", name, got, golden.Groups)
		}
		if got := len(doc.Sketch.Entities); got != golden.Entities {
			t.Errorf("%s: entities: got %d, want %d", name, got, golden.Entities)
		}
		if got := len(doc.Sketch.Constraints); got != golden.Constraints {
			t.Errorf("%s: constraints: got %d, want %d", name, got, golden.Constraints)
		}
	}
}

// TestParityLinkAndTranslateFixtures loads and solves each link and
// translate fixture in testdata/group/. These exercise GroupLinked and
// GroupTranslate/GroupAsmToken/GroupNmToken group types.
func TestParityLinkAndTranslateFixtures(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(thisFile))))
	for _, dir := range []string{"link", "translate_asy", "translate_nd"} {
		path := filepath.Join(root, "testdata/group", dir, "normal.slvs")
		doc, err := LoadFile(path)
		if err != nil {
			t.Errorf("%s: load: %v", dir, err)
			continue
		}
		doc.Sketch.GenerateAll()
		clean := 0
		for _, g := range doc.Sketch.Groups {
			if g.Clean {
				clean++
			}
		}
		if clean != len(doc.Sketch.Groups) {
			t.Errorf("%s: not all groups clean (%d/%d)", dir, clean, len(doc.Sketch.Groups))
		}
	}
}
