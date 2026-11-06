package sketch

import (
	"errors"
	"slices"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
)

// damagedSketch is a loaded sketch whose drawing group holds a request
// and a constraint left without what they use: a request in group 9,
// which isn't there, and a distance to a point of request 7, which
// isn't either.
func damagedSketch(t *testing.T) *Sketch {
	t.Helper()
	sk := NewSketch()
	sk.AddReferences()
	hg := sk.AddGroup(Group{Type: GroupDrawing3D, Order: 1, Scale: 1, Name: "drawing"})
	sk.GroupOrder = []core.HGroup{{V: core.HGroupReferences}, hg}
	sk.Requests = append(sk.Requests,
		Request{H: core.HRequest{V: 4}, Type: RequestDatumPoint, Group: hg, Workplane: core.FreeIn3D},
		Request{H: core.HRequest{V: 6}, Type: RequestDatumPoint, Group: core.HGroup{V: 9}, Workplane: core.FreeIn3D})
	var c core.Constraint
	c.H = core.HConstraint{V: 5}
	c.Group = hg
	c.Type = core.ConstraintPtPtDistance
	c.PtA = core.HRequest{V: 4}.Entity(0)
	c.PtB = core.HRequest{V: 7}.Entity(0)
	c.ValA = 10
	sk.Constraints = append(sk.Constraints, c)
	sk.Params = append(sk.Params,
		core.Param{H: core.HRequest{V: 6}.Param(0), Val: 3},
		core.Param{H: core.HRequest{V: 4}.Param(0), Val: 1})
	sk.UpgradeLegacyData()
	return sk
}

// The load keeps what it leaves out, with its values, named for the
// user; nothing in the model holds it.
func TestLeftOutKept(t *testing.T) {
	t.Parallel()
	sk := damagedSketch(t)
	want := []string{"r006-datum-point of g009", "c005-pt-pt-distance of g002-drawing"}
	if got := sk.LeftOutNames(); !slices.Equal(got, want) {
		t.Fatalf("left out %q, want %q", got, want)
	}
	if sk.Request(core.HRequest{V: 6}) != nil || sk.Constraint(core.HConstraint{V: 5}) != nil {
		t.Error("what was left out is still in the model")
	}
	if p := sk.LeftOut.Params; len(p) != 1 || p[0].H != (core.HRequest{V: 6}.Param(0)) || p[0].Val != 3 {
		t.Errorf("left-out values %v, want request 6's", p)
	}
}

// A new item takes no handle of what was left out, nor of the group a
// left-out request was in, which would bring it back.
func TestLeftOutHandles(t *testing.T) {
	t.Parallel()
	sk := damagedSketch(t)
	if h := sk.AddRequest(Request{Group: core.HGroup{V: 2}}); h.V != 7 {
		t.Errorf("new request %x, want 7", h.V)
	}
	if h := sk.AddConstraint(core.Constraint{}); h.V != 6 {
		t.Errorf("new constraint %x, want 6", h.V)
	}
	if h := sk.AddGroup(Group{}); h.V != 10 {
		t.Errorf("new group %x, want 10, past the left-out request's", h.V)
	}
}

// Discarding what was left out is one undo step, and comes back on
// redo as it was.
func TestDiscardLeftOutUndo(t *testing.T) {
	t.Parallel()
	sk := damagedSketch(t)
	kept := sk.LeftOut
	if err := sk.UndoRememberFor(sk.DiscardLeftOut); err != nil {
		t.Fatal(err)
	}
	if sk.LeftOut != nil {
		t.Fatal("still kept after a discard")
	}
	if err := sk.UndoRememberFor(sk.DiscardLeftOut); !errors.Is(err, ErrNothingLeftOut) {
		t.Errorf("a second discard: %v", err)
	}
	if !sk.UndoUndo() || sk.LeftOut != kept {
		t.Fatal("undo didn't bring it back")
	}
	if !sk.UndoRedo() || sk.LeftOut != nil {
		t.Error("redo didn't discard it again")
	}
	sk.Clear()
	if sk.LeftOut != nil {
		t.Error("a cleared sketch keeps it")
	}
}

// Each kind of entity a mend may need takes an entity of that kind and
// refuses one of another.
func TestMendRoleFits(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		role      mendRole
		fits, not core.EntityType
	}{
		{mendPoint, core.EntityPointIn3D, core.EntityLineSegment},
		{mendWorkplane, core.EntityWorkplane, core.EntityPointIn3D},
		{mendCurve, core.EntityCubic, core.EntityPointIn3D},
		{mendLine, core.EntityLineSegment, core.EntityCircle},
		{mendCircle, core.EntityArcOfCircle, core.EntityLineSegment},
		{mendArc, core.EntityArcOfCircle, core.EntityCircle},
		{mendCubic, core.EntityCubicPeriodic, core.EntityArcOfCircle},
		{mendFace, core.EntityFaceNormalPt, core.EntityWorkplane},
		{mendNormal, core.EntityNormalIn3D, core.EntityLineSegment},
		{mendVector, core.EntityNormalIn2D, core.EntityCircle},
		{mendArcOrCubic, core.EntityCubic, core.EntityLineSegment},
		{mendPlane, core.EntityFaceNormalPt, core.EntityNormalIn3D},
		{mendPlane, core.EntityWorkplane, core.EntityLineSegment},
	} {
		if !tc.role.fits(&core.Entity{EntityBase: core.EntityBase{Type: tc.fits}}) ||
			tc.role.fits(&core.Entity{EntityBase: core.EntityBase{Type: tc.not}}) {
			t.Errorf("%s: fits wrong", tc.role)
		}
	}
	if !mendAny.fits(&core.Entity{}) {
		t.Error("an entity of any kind doesn't fit")
	}
}

// The kind a constraint's entity must be follows from the constraint.
func TestConstraintRoles(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		c    core.ConstraintType
		a, b mendRole
	}{
		{core.ConstraintPtPlaneDistance, mendWorkplane, mendAny},
		{core.ConstraintPtLineDistance, mendLine, mendAny},
		{core.ConstraintPtOnFace, mendFace, mendAny},
		{core.ConstraintProjPtDistance, mendVector, mendAny},
		{core.ConstraintEqualLengthLines, mendLine, mendLine},
		{core.ConstraintEqualLineArcLen, mendLine, mendArc},
		{core.ConstraintArcArcDifference, mendArc, mendArc},
		{core.ConstraintAtMidpoint, mendLine, mendWorkplane},
		{core.ConstraintDiameter, mendCircle, mendAny},
		{core.ConstraintEqualRadius, mendCircle, mendCircle},
		{core.ConstraintSameOrientation, mendNormal, mendNormal},
		{core.ConstraintParallel, mendVector, mendVector},
		{core.ConstraintEqualAngle, mendVector, mendVector},
		{core.ConstraintAxisAngle, mendAxis, mendAny},
		{core.ConstraintArcLineTangent, mendArc, mendLine},
		{core.ConstraintCubicLineTangent, mendCubic, mendLine},
		{core.ConstraintCurveCurveTangent, mendArcOrCubic, mendArcOrCubic},
		{core.ConstraintPtPtDistance, mendAny, mendAny},
	} {
		if r := constraintRoles(tc.c); r[0] != tc.a || r[1] != tc.b {
			t.Errorf("constraint %d: roles %v", tc.c, r)
		}
	}
}

// An entity the file lacks is named for its kind: the point of a
// point-to-point distance.
func TestLeftOutMissingKind(t *testing.T) {
	t.Parallel()
	sk := NewSketch()
	if sk.LeftOutMissingKind(core.HEntity{V: 0x70000}) != "" {
		t.Error("a kind with nothing left out")
	}
	sk = damagedSketch(t)
	if got := sk.LeftOutMissingKind(core.HRequest{V: 7}.Entity(0)); got != "a point" {
		t.Errorf("kind %q", got)
	}
}

// The kind a group's axis, direction or plane must be follows from the
// group: a lathe's axis is a line or a normal, an extrusion's plane a
// workplane, a mirror's a workplane or a face.
func TestPredefRoles(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		g    Group
		b, c mendRole
	}{
		{Group{Type: GroupDrawingWorkplane, Subtype: GroupWorkplaneByLineSegments}, mendVector, mendVector},
		{Group{Type: GroupDrawingWorkplane, Subtype: GroupWorkplaneByPointNormal}, mendNormal, mendAny},
		{Group{Type: GroupDrawingWorkplane, Subtype: GroupWorkplaneByPointOrtho}, mendAny, mendAny},
		{Group{Type: GroupLathe}, mendVector, mendAny},
		{Group{Type: GroupRevolve}, mendVector, mendAny},
		{Group{Type: GroupHelix}, mendVector, mendAny},
		{Group{Type: GroupRotate}, mendVector, mendAny},
		{Group{Type: GroupExtrude}, mendWorkplane, mendAny},
		{Group{Type: GroupTranslate}, mendWorkplane, mendAny},
		{Group{Type: GroupMirror}, mendPlane, mendAny},
		{Group{Type: GroupLinked}, mendAny, mendAny},
	} {
		if b, c := tc.g.predefRoles(); b != tc.b || c != tc.c {
			t.Errorf("group %d/%d: roles %s, %s", tc.g.Type, tc.g.Subtype, b, c)
		}
	}
}
