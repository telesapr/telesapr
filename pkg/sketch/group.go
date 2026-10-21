package sketch

import (
	"fmt"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
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
	// face. SkipFirst leaves the original out of the solid; MirrorSolid
	// reflects the whole solid so far, up to the source group.
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
	// OpA and OpB, by ruled patches, the second turned by ValA curves;
	// or, through the sketches LoftThrough between them, by one smooth
	// surface, held to the guide curves Edges if it has any.
	GroupLoft
	// GroupShell hollows the solid so far to walls ValB thick, open
	// where its faces SolidFaces are taken away.
	GroupShell
	// GroupPathPattern repeats the solid of its source group (OpA) ValA
	// times along the path Edges, as a sweep's (pattern.go).
	GroupPathPattern
)

type GroupSubtype int

const (
	GroupWorkplaneByPointOrtho GroupSubtype = 6000 + iota
	GroupWorkplaneByLineSegments
	GroupWorkplaneByPointNormal
	// GroupWorkplaneByFace lies in a flat face of the solid so far,
	// SolidFaces[0], and follows it at each regeneration
	// (topo.FaceFrame); Predef.Q and Predef.At hold where it was last
	// found.
	GroupWorkplaneByFace
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
	// MirrorSolid has a mirror reflect the whole solid so far, up to
	// and with its source group, in place of the source group's own
	// solid; its result is then the solid so far, the reflection with
	// it or alone (roadmap 15.5).
	MirrorSolid bool
	// DraftFaces are the curves of an extrusion's sketch (OpA) whose
	// sides its draft leans, line segments, arcs and circles; none for
	// every side (roadmap 28.2).
	DraftFaces []core.HEntity

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
	// LinkAsSolid reads a linked STEP file as a solid, its faces exact,
	// rather than as a reference mesh (roadmap D3); a file that can't be
	// read as one links as a mesh, with the reason in LinkProblem.
	LinkAsSolid bool
	// LinkInfo describes the linked file as its loader read it, a line
	// each, such as a STEP file's unit and how many faces it has; and
	// LinkProblem says what of the file could not be used. Neither is
	// saved; reloading the links fills them.
	LinkInfo    []string
	LinkProblem string
	// LinkParts is the bill of materials of a linked model: the parts
	// it places by linking files in turn, each File named relative to
	// the file holding this sketch (roadmap 29.3). Not saved; reloading
	// the links fills it.
	LinkParts []BOMItem

	Color core.RGBA

	Predef struct {
		Q      geom.Quaternion
		Origin core.HEntity
		// At is the origin of a workplane in a face, where the last
		// regeneration found it; it holds the workplane when the face
		// can't be found, as before the solid is made on loading.
		At      geom.Vector
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
	// DisplayDirty is set when the shell or mesh changed, so the
	// display items need making again.
	DisplayDirty bool

	ImpMesh   srf.SMesh
	ImpShell  srf.SShell
	ImpEntity []core.Entity

	// ShellsStale is set when a regeneration with DeferShells left the
	// group's solid as it was (shelljob.go).
	ShellsStale bool

	// solidTol is the tolerance the group's solid was last made at,
	// which a regeneration that leaves the group alone keeps.
	solidTol srf.Tolerance

	// Appearance is how the group's solid takes light when the window
	// shades (roadmap 19.2): matte, plastic, glossy or metal, "" for
	// plastic; FaceAppearances give single faces their own.
	Appearance      string
	FaceAppearances []FaceAppearance

	// Density is the density of the group's solid in g/cm³, for its
	// mass (mass.go); 0 when none is set.
	Density float64

	// Edges are the entities a chamfer cuts, the path a sweep
	// follows, or a loft's guide curves, in the order the user gave
	// them.
	Edges []core.HEntity
	// SolidEdges are the edges of the solid a chamfer cuts, by name:
	// any edge of the solid so far, as the topology names it.
	SolidEdges []topo.Name
	// SweepUp, when not zero, holds a sweep's frames to this direction
	// rather than minimising their rotation (topo.PathSweepOptions.Up).
	SweepUp geom.Vector
	// fittedPath is a sweep's path when its surface is fitted, made
	// with the group's shell.
	fittedPath *topo.SweepPath
	// LoftThrough are the profiles a smooth loft goes through between
	// OpA and OpB, in order (roadmap E8).
	LoftThrough []core.HGroup
	// SolidFaces are the faces of the solid a shell takes away, or the
	// face a sketch on a face lies in, by identity (srf.FaceID).
	SolidFaces []srf.FaceID
	// faceProblem is why the face of a workplane in a face was not
	// found when the group's entities were made, which the solid's
	// regeneration, clearing Problem, then reports (face_workplane.go).
	faceProblem string
	// EdgeRadii are a fillet's edges that have a radius of their own,
	// rather than ValB, in the order of Edges and then SolidEdges.
	EdgeRadii []EdgeRadius

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

// Clear drops everything regeneration makes, and the remap table.
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
		g.Type == GroupLoft ||
		g.Type == GroupShell ||
		g.Type == GroupPathPattern
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
		current := sk.Group(gh)
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
	case GroupRotate, GroupTranslate, GroupMirror, GroupPathPattern:
		opA := sk.Group(g.OpA)
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

// FaceAppearance is a face's own appearance, by the face's identity.
type FaceAppearance struct {
	Face       srf.FaceID
	Appearance string
}

// SetFaceAppearance gives the face id of the model an appearance of its
// own, or with "" takes it away, so that it has its group's again. The
// face is kept by its identity, in the group that made it.
func (sk *Sketch) SetFaceAppearance(id srf.FaceID, name string) error {
	g := sk.Group(faceGroup(id))
	if g == nil {
		return fmt.Errorf("no group made face %x", uint32(id))
	}
	kept := g.FaceAppearances[:0]
	for _, fa := range g.FaceAppearances {
		if fa.Face != id {
			kept = append(kept, fa)
		}
	}
	g.FaceAppearances = kept
	if name != "" {
		g.FaceAppearances = append(g.FaceAppearances, FaceAppearance{Face: id, Appearance: name})
	}
	sk.Unsaved = true
	return nil
}

// AppearanceOf is the appearance of the face id of the model: its own,
// or its group's, or "" for the default.
func (sk *Sketch) AppearanceOf(id srf.FaceID) string {
	g := sk.Group(faceGroup(id))
	if g == nil {
		return ""
	}
	for _, fa := range g.FaceAppearances {
		if fa.Face == id {
			return fa.Appearance
		}
	}
	return g.Appearance
}
