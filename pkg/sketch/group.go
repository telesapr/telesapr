package sketch

import (
	"fmt"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/srf"
	"github.com/spacesolve/spacesolve/pkg/topo"
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

// Groups of this program's own, which SolveSpace has no counterpart
// for.
const (
	// GroupMirror reflects its source group (OpA), the entities and
	// the solid, in a plane: Predef.EntityB, a workplane or a planar
	// face. SkipFirst leaves the original out of the solid.
	GroupMirror GroupType = 5400 + iota
	// GroupChamfer cuts the edges Edges of the solid so far, line
	// segments between two planar faces, at the distance ValB.
	GroupChamfer
	// GroupSweep moves the loops of its profile group (OpA) along the
	// path Edges, a chain of line segments and arcs meeting tangent.
	GroupSweep
	// GroupFillet rounds its edges of the solid so far, Edges and
	// SolidEdges, with the radius ValB, editing a copy of that solid.
	GroupFillet
	// GroupLoft joins the closed loops of two sketches in workplanes,
	// OpA and OpB, by ruled patches, the second turned by ValA curves.
	GroupLoft
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

	Type     GroupType
	Order    int
	OpA      core.HGroup
	OpB      core.HGroup
	Visible  bool
	Suppress bool
	// SuppressAll leaves the whole group out of the model: its sketch,
	// its solve and its solid, where Suppress leaves out the solid
	// only. The groups that use it are left out with it.
	SuppressAll            bool
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
	// ExprA, ExprB, ExprC and ExprScale are the expressions ValA,
	// ValB, ValC and Scale are evaluated from at each regeneration,
	// or "" for a plain number.
	ExprA, ExprB, ExprC, ExprScale string

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

	// SuppressedBy is set by each regeneration: the group whose
	// SuppressAll leaves this one out, this group itself or one it
	// uses, directly or not; zero for a group that is generated. A
	// group left out has no entities, no solve and no solid of its own,
	// and keeps its parameter values for when it comes back.
	SuppressedBy core.HGroup

	// SketchDiagnostic captures errors from the last GenerateAll.
	// Cleared at the start of each GenerateAll invocation.
	SketchDiagnostic *SketchDiagnostic
	MeshCombine      CombineAs
	ForceToMesh      bool

	Remap    core.EntityMap
	LinkFile string
	// LinkInfo describes the linked file as its loader read it, a line
	// each, such as a STEP file's unit and how many faces it has; and
	// LinkProblem says what of the file could not be used. Neither is
	// saved; reloading the links fills them.
	LinkInfo    []string
	LinkProblem string

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

	// Edges are the entities a chamfer cuts, or the path a sweep
	// follows, in the order the user gave them.
	Edges []core.HEntity
	// SolidEdges are the edges of the solid a chamfer cuts, by name:
	// any edge of the solid so far, as the topology names it.
	SolidEdges []topo.Name

	// topology caches the topology of RunningShell, built on first use
	// after a regeneration (Topology).
	topology    *topo.Topology
	topologyErr error
	topologySet bool

	// Problem is what the last regeneration found wrong with the
	// group's input, such as an edge a chamfer can't cut; the group
	// then adds no solid. It is not saved.
	Problem GroupProblem

	Name string
}

// GroupProblem is a diagnostic on a group: a message and, when the
// problem has a place, the point it is at.
type GroupProblem struct {
	Msg      string
	At       geom.Vector
	HasPoint bool
}

// setProblem records a problem on the group; the first one found is
// kept.
func (g *Group) setProblem(at *geom.Vector, format string, args ...any) {
	if g.Problem.Msg != "" {
		return
	}
	g.Problem.Msg = fmt.Sprintf(format, args...)
	if at != nil {
		g.Problem.At, g.Problem.HasPoint = *at, true
	}
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
	g.forgetTopology()
	// remap is the only one that doesn't get recreated when we regen
	g.Remap = nil
}

// forgetTopology drops the cached topology, when the shell changes.
func (g *Group) forgetTopology() {
	g.topology, g.topologyErr, g.topologySet = nil, nil, false
}

// Topology is the topology of the solid so far at this group, its
// running shell, derived on first use after a regeneration: the
// faces, edges and vertices that edge names refer to. A group forced
// to a triangle mesh has none.
func (g *Group) Topology() (*topo.Topology, error) {
	if !g.topologySet {
		g.topology, g.topologyErr = topo.Build(&g.RunningShell)
		g.topologySet = true
	}
	return g.topology, g.topologyErr
}

func (g *Group) IsSolvedOkay() bool {
	return g.Solved.How == core.SolveOkay ||
		(g.AllowRedundant && g.Solved.How == core.SolveRedundantOkay)
}

func (g *Group) IsVisible() bool {
	return g.Visible && !g.Suppress && g.SuppressedBy.V == 0
}

func (g *Group) IsMeshGroup() bool {
	return g.Type == GroupExtrude ||
		g.Type == GroupLathe ||
		g.Type == GroupRevolve ||
		g.Type == GroupHelix ||
		g.Type == GroupRotate ||
		g.Type == GroupTranslate ||
		g.Type == GroupChamfer ||
		g.Type == GroupSweep ||
		g.Type == GroupFillet ||
		g.Type == GroupLoft
}

// IsCopyGroup reports whether the group copies its source group,
// entities and solid, and combines the copies with what came before
// the source: step and repeat, and mirror.
func (g *Group) IsCopyGroup() bool {
	return g.Type == GroupRotate || g.Type == GroupTranslate || g.Type == GroupMirror
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
	case GroupRotate, GroupTranslate, GroupMirror:
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
