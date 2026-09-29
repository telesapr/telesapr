package sketch

import (
	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/srf"
)

type GroupType int

const (
	GroupDrawing3D GroupType = 5000 + iota
	GroupDrawingWorkplane
)

const (
	GroupExtrude GroupType = 5100 + iota
	GroupLathe
	GroupRevolve
	GroupHelix
)

const (
	GroupRotate GroupType = 5200 + iota
	GroupTranslate
)

const (
	GroupLinked GroupType = 5300
)

type GroupSubtype int

const (
	GroupWorkplaneByPointOrtho GroupSubtype = 6000 + iota
	GroupWorkplaneByLineSegments
	GroupWorkplaneByPointNormal
)

const (
	GroupOneSided GroupSubtype = 7000 + iota
	GroupTwoSided
	_
	_
	GroupOneSkewed
	GroupTwoSkewed
)

type CombineAs int

const (
	CombineUnion CombineAs = iota
	CombineDifference
	CombineAssemble
	CombineIntersection
)

type Group struct {
	Tag int
	H   core.HGroup

	Type                   GroupType
	Order                  int
	OpA                    core.HGroup
	OpB                    core.HGroup
	Visible                bool
	Suppress               bool
	RelaxConstraints       bool
	AllowRedundant         bool
	SuppressDofCalculation bool
	AllDimsReference       bool
	Scale                  float64

	Clean           bool
	DofCheckOk      bool
	ActiveWorkplane core.HEntity
	ValA            float64
	ValB            float64
	ValC            float64

	Solved struct {
		How              core.SolveResult
		Dof              int
		FindToFixTimeout int
		Timeout          bool
		Remove           []core.HConstraint
	}

	Subtype   GroupSubtype
	SkipFirst bool

	BooleanFailed bool

	// SketchDiagnostic captures errors from the last GenerateAll.
	// Cleared at the start of each GenerateAll invocation.
	SketchDiagnostic *SketchDiagnostic
	MeshCombine      CombineAs
	ForceToMesh      bool

	Remap    core.EntityMap
	LinkFile string

	Color core.RgbaColor

	Predef struct {
		Q       geom.Quaternion
		Origin  core.HEntity
		EntityB core.HEntity
		EntityC core.HEntity
		SwapUV  bool
		NegateU bool
		NegateV bool
	}

	// Phase 5 additions: trim loops and the SShell/SMesh pipeline.
	PolyLoops   srf.SPolygon
	BezierLoops srf.SBezierLoopSetSet
	BezierOpens []srf.SBezierLoop
	PolyError   struct {
		How        int
		NotClosed  srf.SEdge
		ErrorPoint geom.Vector
	}

	ThisShell    srf.SShell
	RunningShell srf.SShell
	ThisMesh     srf.SMesh

	RunningMesh     srf.SMesh
	DisplayMesh     srf.SMesh
	DisplayOutlines srf.SOutlineList
	// DisplayDirty is set when the shell or mesh changed, so the display
	// items need making again (C++ Group::displayDirty).
	DisplayDirty bool

	ImpMesh   srf.SMesh
	ImpShell  srf.SShell
	ImpEntity []core.Entity

	Name string
}

// Clear drops everything regeneration makes, and the remap table (C++
// Group::Clear).
func (g *Group) Clear() {
	g.Solved.Remove = nil
	g.PolyLoops.Clear()
	g.BezierLoops.Clear()
	g.BezierOpens = nil
	g.ThisMesh.Clear()
	g.RunningMesh.Clear()
	g.ThisShell.Clear()
	g.RunningShell.Clear()
	g.DisplayMesh.Clear()
	g.DisplayOutlines.Clear()
	g.ImpMesh.Clear()
	g.ImpShell.Clear()
	g.ImpEntity = nil
	// remap is the only one that doesn't get recreated when we regen
	g.Remap = nil
}

func (g *Group) IsSolvedOkay() bool {
	return g.Solved.How == core.SolveOkay ||
		(g.AllowRedundant && g.Solved.How == core.SolveRedundantOkay)
}

func (g *Group) IsVisible() bool {
	return g.Visible && !g.Suppress
}

func (g *Group) IsMeshGroup() bool {
	return g.Type == GroupExtrude ||
		g.Type == GroupLathe ||
		g.Type == GroupRevolve ||
		g.Type == GroupHelix ||
		g.Type == GroupRotate ||
		g.Type == GroupTranslate
}

func (g *Group) PreviousGroup(sk *Sketch) *Group {
	if g == nil || sk == nil {
		return nil
	}

	var prev *Group
	for _, gh := range sk.GroupOrder {
		current := sk.GetGroup(gh)
		if current == nil {
			continue
		}
		if current.H == g.H {
			return prev
		}
		prev = current
	}
	return nil
}

func (g *Group) RunningMeshGroup(sk *Sketch) *Group {
	if g == nil || sk == nil {
		return nil
	}

	switch g.Type {
	case GroupRotate, GroupTranslate:
		opA := sk.GetGroup(g.OpA)
		if opA == nil {
			return nil
		}
		return opA.RunningMeshGroup(sk)
	default:
		return g.PreviousGroup(sk)
	}
}

type GroupList []Group

// SketchDiagnostic captures transient per-group errors during
// Sketch::GenerateAll. Cleared on each invocation.
type SketchDiagnostic struct {
	AddConstraintErrors []string
}
