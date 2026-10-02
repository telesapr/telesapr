package slvs_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/persist/slvs"
	"github.com/spacesolve/spacesolve/pkg/sketch"
)

func fixturesRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	dir := filepath.Dir(file)          // .../go/pkg/persist/slvs
	persistDir := filepath.Dir(dir)    // .../go/pkg/persist
	pkgDir := filepath.Dir(persistDir) // .../go/pkg
	goRoot := filepath.Dir(pkgDir)     // .../go
	root := filepath.Join(goRoot, "testdata")
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("fixtures not found at %s: %v", root, err)
	}
	return root
}

func TestSLVSLoaderCapturesRequestTypes(t *testing.T) {
	root := fixturesRoot(t)
	reqRoot := filepath.Join(root, "request")
	cases := map[string]sketch.RequestType{
		"workplane":      sketch.RequestWorkplane,
		"datum_point":    sketch.RequestDatumPoint,
		"line_segment":   sketch.RequestLineSegment,
		"cubic":          sketch.RequestCubic,
		"cubic_periodic": sketch.RequestCubicPeriodic,
		"circle":         sketch.RequestCircle,
		"arc_of_circle":  sketch.RequestArcOfCircle,
		"ttf_text":       sketch.RequestTTFText,
		"image":          sketch.RequestImage,
	}

	for name, reqType := range cases {
		path := filepath.Join(reqRoot, name, "normal.slvs")
		doc, err := slvs.LoadFile(path)
		if err != nil {
			t.Fatalf("load %s: %v", path, err)
		}

		found := false
		for _, req := range doc.Sketch.Requests {
			if req.Type == reqType {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("request type %d not found in %s", reqType, path)
		}
	}
}

func TestSLVSRejectsLegacyFixtures(t *testing.T) {
	tmp := t.TempDir()
	legacy := `SolveSpaceREVa

Group.h.v=00000001
Group.type=5000
Group.name=#references
AddGroup
`
	path := filepath.Join(tmp, "legacy.slvs")
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatalf("write legacy fixture: %v", err)
	}
	if _, err := slvs.LoadFile(path); err == nil {
		t.Fatalf("expected legacy loader failure for %s", path)
	}
}

func TestConstraintFixturesContainExpectedType(t *testing.T) {
	root := filepath.Join(fixturesRoot(t), "constraint")
	expected := map[string]core.ConstraintType{
		"angle":               core.ConstraintAngle,
		"arc_line_tangent":    core.ConstraintArcLineTangent,
		"at_midpoint":         core.ConstraintAtMidpoint,
		"comment":             core.ConstraintComment,
		"cubic_line_tangent":  core.ConstraintCubicLineTangent,
		"curve_curve_tangent": core.ConstraintCurveCurveTangent,
		"diameter":            core.ConstraintDiameter,
		"eq_len_pt_line_d":    core.ConstraintEqLenPtLineD,
		"eq_pt_ln_distances":  core.ConstraintEqPtLnDistances,
		"equal_angle":         core.ConstraintEqualAngle,
		"equal_length_lines":  core.ConstraintEqualLengthLines,
		"equal_line_arc_len":  core.ConstraintEqualLineArcLen,
		"equal_radius":        core.ConstraintEqualRadius,
		"horizontal":          core.ConstraintHorizontal,
		"length_difference":   core.ConstraintLengthDifference,
		"length_ratio":        core.ConstraintLengthRatio,
		"parallel":            core.ConstraintParallel,
		"perpendicular":       core.ConstraintPerpendicular,
		"points_coincident":   core.ConstraintPointsCoincident,
		"proj_pt_distance":    core.ConstraintProjPtDistance,
		"pt_face_distance":    core.ConstraintPtFaceDistance,
		"pt_in_plane":         core.ConstraintPtInPlane,
		"pt_line_distance":    core.ConstraintPtLineDistance,
		"pt_on_circle":        core.ConstraintPtOnCircle,
		"pt_on_face":          core.ConstraintPtOnFace,
		"pt_on_line":          core.ConstraintPtOnLine,
		"pt_plane_distance":   core.ConstraintPtPlaneDistance,
		"pt_pt_distance":      core.ConstraintPtPtDistance,
		"same_orientation":    core.ConstraintSameOrientation,
		"symmetric":           core.ConstraintSymmetric,
		"symmetric_horiz":     core.ConstraintSymmetricHoriz,
		"symmetric_line":      core.ConstraintSymmetricLine,
		"symmetric_vert":      core.ConstraintSymmetricVert,
		"vertical":            core.ConstraintVertical,
		"where_dragged":       core.ConstraintWhereDragged,
	}

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".slvs" {
			return nil
		}

		dirName := filepath.Base(filepath.Dir(path))
		expectedType, ok := expected[dirName]
		if !ok {
			return nil
		}

		doc, err := slvs.LoadFile(path)
		if err != nil {
			t.Fatalf("load %s: %v", path, err)
		}

		found := false
		for _, c := range doc.Sketch.Constraints {
			if c.Type == expectedType {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected constraint type %d in %s", expectedType, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk constraint fixtures: %v", err)
	}
}
