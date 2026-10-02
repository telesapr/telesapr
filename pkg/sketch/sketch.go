package sketch

import (
	"fmt"
	"image"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/srf"
)

type Sketch struct {
	Groups      []Group
	GroupOrder  []core.HGroup
	Constraints []core.Constraint
	Requests    []Request
	Styles      []core.Style

	Entities []core.Entity
	Params   []core.Param

	// Units is the document's unit of length: what the values of
	// lengths that the user types, and that a file stores as text, are
	// in. The sketch itself is in millimetres.
	Units core.Unit

	// Parameters is the table of named values that expressions on
	// constraints and groups use, in the order the user made them.
	// Not to be confused with Params, the solver's unknowns.
	Parameters []Parameter
	// ExprErrors lists the stored expressions of constraints and
	// groups that the last regeneration could not evaluate.
	ExprErrors []ExprError

	// BrokenReferences lists what the last regeneration that deleted
	// something deleted, and why. It stays until another regeneration
	// deletes something, an undo or redo, or ClearBrokenReferences.
	BrokenReferences []BrokenReference
	// broken collects them during a regeneration; names are those of
	// the groups and requests at the end of the last one.
	broken []BrokenReference
	names  knownNames

	// Images are the loaded image files, by the file name image
	// requests give (C++ SS.images); nil for one that couldn't be read.
	Images map[string]*Image

	// ReloadAllLinked reloads the files the sketch links, from wherever
	// the sketch was loaded (C++ SS.ReloadAllLinked(SS.saveFile)); undo
	// calls it. Nil for a sketch that wasn't loaded from a file.
	ReloadAllLinked func(sk *Sketch)

	// Active is the group the user works in (C++ SS.GW.activeGroup);
	// if unset or gone, ActiveGroup picks one as C++ does on load.
	Active core.HGroup

	// View is the screen projection (C++ SS.GW's offset, projRight,
	// projUp, scale and SS.CameraTangent()); nil is DefaultView, as in
	// C++'s headless build.
	View *View

	// Section is the section view, nil when it is off; see SetSection.
	Section *Section

	// Show holds the view's show flags Entity::IsVisible reads; nil is
	// DefaultVisibility.
	Show *Visibility

	// OnScheduleGenerateAll, if set, is told when a regeneration is
	// scheduled, so that the caller's event loop can call Refresh (C++
	// ScheduleGenerateAll starts SS.refreshTimer).
	OnScheduleGenerateAll func()

	// ChordTolPct is the display chord tolerance as a percentage of the
	// model's size (C++ SS.chordTol); 0 is the default, 0.1.
	ChordTolPct float64

	// CheckClosedContour is whether a sketch's loops are checked for
	// self-intersection, which is slow (C++ SS.checkClosedContour, on
	// by default).
	CheckClosedContour bool

	// Generation counts completed GenerateAll runs, so a GUI can tell
	// when its per-entity drawing caches are stale.
	Generation uint64

	// Unsaved is whether the sketch changed since it was saved (C++
	// SS.unsaved).
	Unsaved bool

	scheduledGenerateAll bool

	// freedom caches GroupFreedom.
	freedom []freedomCache

	// Deleted counts what regeneration pruned because it depended on
	// something deleted, until GenerateAll reports it (C++ SS.deleted).
	Deleted struct {
		Requests, Constraints, NonTrivialConstraints, Groups int
	}

	// Message shows the user a message, as C++ Message does; nil
	// drops it.
	Message func(msg string)

	// DraggedParams, if set, lists the params the user is dragging,
	// which the solver keeps as close to their values as it can (C++
	// SolveSpaceUI::MarkDraggedParams, from the graphics window's
	// pending drag).
	DraggedParams func() []core.HParam

	// OnGenerated, if set, is called near the end of each GenerateAll
	// pass, before the message about what was deleted along with
	// something, with how many requests, constraints and groups were
	// (C++ GenerateAll's user interface part).
	OnGenerated func(requests, constraints, groups int)

	// RegenerateAfterDelete, if set, replaces the regeneration that
	// follows deleting requests, so that a user interface can first
	// clear its selection and editors (C++
	// GraphicsWindow::DeleteTaggedRequests); it must regenerate.
	RegenerateAfterDelete func()

	// Clipboard holds what was copied (C++ SS.clipboard).
	Clipboard Clipboard

	// Undo/redo stacks.
	Undo UndoStack
	Redo UndoStack

	// Handle -> index caches for GetEntity and GetParam. They are
	// validated on every lookup and rebuilt on a miss, so appending to
	// or reordering Entities and Params needs no bookkeeping.
	entityIndex map[core.HEntity]int
	paramIndex  map[core.HParam]int
}

func NewSketch() *Sketch {
	return &Sketch{
		CheckClosedContour: true,
		Groups:             make([]Group, 0),
		GroupOrder:         make([]core.HGroup, 0),
		Constraints:        make([]core.Constraint, 0),
		Requests:           make([]Request, 0),
		Styles:             make([]core.Style, 0),
		Entities:           make([]core.Entity, 0),
		Params:             make([]core.Param, 0),
	}
}

func (s *Sketch) GetConstraint(h core.HConstraint) *core.Constraint {
	for i := range s.Constraints {
		if s.Constraints[i].H == h {
			return &s.Constraints[i]
		}
	}
	return nil
}

func (s *Sketch) GetEntity(h core.HEntity) *core.Entity {
	if i, ok := s.entityIndex[h]; ok && i < len(s.Entities) && s.Entities[i].H == h {
		return &s.Entities[i]
	}
	s.entityIndex = make(map[core.HEntity]int, len(s.Entities))
	for i := range s.Entities {
		s.entityIndex[s.Entities[i].H] = i
	}
	if i, ok := s.entityIndex[h]; ok {
		return &s.Entities[i]
	}
	return nil
}

// MustGetEntity is GetEntity for handles that must exist (C++
// SK.GetEntity, which asserts).
func (s *Sketch) MustGetEntity(h core.HEntity) *core.Entity {
	e := s.GetEntity(h)
	if e == nil {
		panic(fmt.Sprintf("entity %08x not found", h.V))
	}
	return e
}

// MustGetParam is GetParam for handles that must exist (C++
// SK.GetParam, which asserts).
func (s *Sketch) MustGetParam(h core.HParam) *core.Param {
	p := s.GetParam(h)
	if p == nil {
		panic(fmt.Sprintf("param %08x not found", h.V))
	}
	return p
}

func (s *Sketch) GetParam(h core.HParam) *core.Param {
	if i, ok := s.paramIndex[h]; ok && i < len(s.Params) && s.Params[i].H == h {
		return &s.Params[i]
	}
	s.paramIndex = make(map[core.HParam]int, len(s.Params))
	for i := range s.Params {
		s.paramIndex[s.Params[i].H] = i
	}
	if i, ok := s.paramIndex[h]; ok {
		return &s.Params[i]
	}
	return nil
}

func (s *Sketch) GetRequest(h core.HRequest) *Request {
	for i := range s.Requests {
		if s.Requests[i].H == h {
			return &s.Requests[i]
		}
	}
	return nil
}

func (s *Sketch) GetGroup(h core.HGroup) *Group {
	for i := range s.Groups {
		if s.Groups[i].H == h {
			return &s.Groups[i]
		}
	}
	return nil
}

func (s *Sketch) GetStyle(h core.HStyle) *core.Style {
	for i := range s.Styles {
		if s.Styles[i].H == h {
			return &s.Styles[i]
		}
	}
	return nil
}

// AddReferences adds the references group and its three coordinate
// plane workplanes, which every sketch has; the first half of C++
// SolveSpaceUI::NewFile. Call it on an empty sketch.
func (s *Sketch) AddReferences() {
	// Our initial group, that contains the references.
	s.Groups = append(s.Groups, Group{
		H:       core.HGroup{V: core.HGroupReferences},
		Type:    GroupDrawing3D,
		Order:   0,
		Name:    "#references",
		Visible: true,
		Scale:   1,
	})

	// Let's create three two-d coordinate systems, for the coordinate
	// planes; these are our references, present in every sketch.
	for _, hr := range []core.HRequest{HRequestReferenceXY, HRequestReferenceYZ, HRequestReferenceZX} {
		s.Requests = append(s.Requests, Request{
			H:         hr,
			Type:      RequestWorkplane,
			Group:     core.HGroup{V: core.HGroupReferences},
			Workplane: core.FreeIn3D,
		})
	}
}

// The Add* functions add an item with a new handle, one more than
// the largest in use, as C++ IdList::AddAndAssignId does.

func (s *Sketch) AddGroup(g Group) core.HGroup {
	var max uint32
	for i := range s.Groups {
		max = maxU32(max, s.Groups[i].H.V)
	}
	g.H.V = max + 1
	s.Groups = append(s.Groups, g)
	return g.H
}

func (s *Sketch) AddRequest(r Request) core.HRequest {
	var max uint32
	for i := range s.Requests {
		max = maxU32(max, s.Requests[i].H.V)
	}
	r.H.V = max + 1
	s.Requests = append(s.Requests, r)
	return r.H
}

func (s *Sketch) AddEntity(e core.Entity) core.HEntity {
	var max uint32
	for i := range s.Entities {
		max = maxU32(max, s.Entities[i].H.V)
	}
	e.H.V = max + 1
	s.Entities = append(s.Entities, e)
	return e.H
}

func (s *Sketch) AddParam(p core.Param) core.HParam {
	var max uint32
	for i := range s.Params {
		max = maxU32(max, s.Params[i].H.V)
	}
	p.H.V = max + 1
	s.Params = append(s.Params, p)
	return p.H
}

func (s *Sketch) AddConstraint(c core.Constraint) core.HConstraint {
	var max uint32
	for i := range s.Constraints {
		max = maxU32(max, s.Constraints[i].H.V)
	}
	c.H.V = max + 1
	s.Constraints = append(s.Constraints, c)
	return c.H
}

func (s *Sketch) AddStyle(st core.Style) core.HStyle {
	var max uint32
	for i := range s.Styles {
		max = maxU32(max, s.Styles[i].H.V)
	}
	st.H.V = max + 1
	s.Styles = append(s.Styles, st)
	return st.H
}

func maxU32(a, b uint32) uint32 {
	if b > a {
		return b
	}
	return a
}

func (s *Sketch) Clear() {
	s.Groups = s.Groups[:0]
	s.GroupOrder = s.GroupOrder[:0]
	s.Constraints = s.Constraints[:0]
	s.Requests = s.Requests[:0]
	s.Styles = s.Styles[:0]
	s.Entities = s.Entities[:0]
	s.Params = s.Params[:0]
	s.Parameters = nil
	s.ExprErrors = nil
	s.ClearBrokenReferences()
	s.Section = nil
	s.names = knownNames{}
	s.Units = core.UnitMM
}

// CalculateEntityBBox returns the bounding box of the non-construction
// entities (C++ Sketch::CalculateEntityBBox). Circles and arcs count
// their Bezier control points, which is not exact for arcs.
//
// Without includingInvisible, the C++ skips entities for which
// Entity::IsVisible (which depends on UI settings) is false; here that
// is approximated by ActVisible and ForceHidden.
func (s *Sketch) CalculateEntityBBox(includingInvisible bool) core.BBox {
	box := core.BBox{}
	first := true

	includePoint := func(point geom.Vector) {
		if first {
			box.MinP = point
			box.MaxP = point
			first = false
		} else {
			box.Include(point, 0)
		}
	}

	rt := &Runtime{Sketch: s}
	for i := range s.Entities {
		e := &s.Entities[i]
		if e.Construction {
			continue
		}
		if !includingInvisible && (!e.ActVisible || e.ForceHidden) {
			continue
		}

		// arc center point shouldn't be included in bounding box calculation
		if e.IsPoint() && e.H.IsFromRequest() {
			r := s.GetRequest(e.H.Request())
			if r.Type == RequestArcOfCircle && e.H == r.H.Entity(1) {
				continue
			}
		}

		if e.IsPoint() {
			includePoint(e.PointGetNum(s))
			continue
		}

		switch e.Type {
		// Circles and arcs are special cases. We calculate their bounds
		// based on Bezier curve bounds. This is not exact for arcs,
		// but the implementation is rather simple.
		case core.EntityCircle, core.EntityArcOfCircle:
			var sbl srf.SBezierList
			GenerateEntityBeziers(e, &sbl, rt)

			for _, sb := range sbl.Curves {
				for j := 0; j <= sb.Degree; j++ {
					includePoint(sb.Ctrl[j])
				}
			}
		}
	}

	return box
}

func (s *Sketch) GetRunningMeshGroupFor(h core.HGroup) *Group {
	for g := s.GetGroup(h); g != nil; g = g.PreviousGroup(s) {
		if g.IsMeshGroup() {
			return g
		}
	}
	return nil
}

// ParamsToMap returns the current param values as a map from handle
// to float64. Used by Constraint::ModifyToSatisfy and other places
// that need direct param access.
func (s *Sketch) ParamsToMap() map[core.HParam]float64 {
	out := make(map[core.HParam]float64, len(s.Params))
	for _, p := range s.Params {
		out[p.H] = p.Val
	}
	return out
}

// EntitiesToMap returns the entities indexed by their handle.
func (s *Sketch) EntitiesToMap() map[core.HEntity]core.Entity {
	out := make(map[core.HEntity]core.Entity, len(s.Entities))
	for _, e := range s.Entities {
		out[e.H] = e
	}
	return out
}

// Image is a loaded image file (C++ Pixmap): its size in pixels, which
// sets an image request's aspect ratio, and its pixels, for drawing.
type Image struct {
	Width, Height int
	Pixels        image.Image
}

// image is the loaded image for file, or nil (also for a nil sketch).
func (s *Sketch) image(file string) *Image {
	if s == nil {
		return nil
	}
	return s.Images[file]
}
