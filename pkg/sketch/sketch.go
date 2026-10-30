package sketch

import (
	"fmt"
	"image"
	"sort"
	"time"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/solver"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/ttf"
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
	// Configurations are named sets of the parameters' expressions,
	// and ActiveConfiguration the one the table holds now, or ""
	// (config.go).
	Configurations      []Configuration
	ActiveConfiguration string
	// Datums are names given to entities the model offers others,
	// as a part offers an assembly (datum.go).
	Datums []Datum
	// Drawing is the model's sheet of views, or nil (drawing.go).
	Drawing *Drawing
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
	// requests give; nil for one that couldn't be read.
	Images map[string]*Image

	// ReloadAllLinked reloads the files the sketch links, from wherever
	// the sketch was loaded; undo calls it. Nil for a sketch that
	// wasn't loaded from a file.
	ReloadAllLinked func(sk *Sketch)

	// Active is the group the user works in; if unset or gone,
	// ActiveGroup picks one on load.
	Active core.HGroup

	// View is the screen projection; nil is DefaultView.
	View *View

	// Section is the section view, nil when it is off; see SetSection.
	Section *Section

	// Show holds the view's show flags Entity::IsVisible reads; nil is
	// DefaultVisibility.
	Show *Visibility

	// ChordTolPct is the display chord tolerance as a percentage of the
	// model's size; 0 is the default, 0.1.
	ChordTolPct float64

	// Fonts is the font list text entities are drawn with; nil for
	// ttf.DefaultFonts.
	Fonts *ttf.FontList

	// Settings is the user's settings store, which the default styles
	// are read from and saved to; nil for none.
	Settings core.Settings

	// MaxSegments is the most segments a curve is cut into for the
	// display; 0 is the default, 20.
	MaxSegments int

	// Mesher is how curved surfaces are triangulated; the zero value is
	// the ear clipper (srf.MesherEarClip).
	Mesher srf.Mesher

	// Exporting, while set, is the tolerance a regeneration follows
	// curves and surfaces at, for a file; unset, the tolerance comes
	// from the model's size and ChordTolPct.
	Exporting srf.Tolerance
	// tol is the tolerance of the last regeneration (Tol).
	tol srf.Tolerance

	// CheckClosedContour is whether a sketch's loops are checked for
	// self-intersection, which is slow.
	CheckClosedContour bool

	// FindToFixTimeout bounds the search for the constraints that make
	// a group inconsistent; 0 is the default, a second. The search is
	// bounded by the clock, so a test that records what it finds sets
	// it long.
	FindToFixTimeout time.Duration

	// Generation counts completed GenerateAll runs, so a GUI can tell
	// when its per-entity drawing caches are stale.
	Generation uint64

	// Unsaved is whether the sketch changed since it was saved.
	Unsaved bool

	scheduledGenerateAll bool
	// scheduledAdded is the group just added whose regeneration is
	// scheduled (ScheduleAdded), which Refresh then makes as
	// GenerateAdded does; zero for none.
	scheduledAdded core.HGroup

	// freedom caches GroupFreedom.
	freedom []freedomCache

	// Deleted counts what regeneration pruned because it depended on
	// something deleted, until GenerateAll reports it.
	Deleted struct {
		Requests, Constraints, NonTrivialConstraints, Groups int
	}

	// Hooks is what the sketch tells whoever owns it and asks of them;
	// nil is NoHooks.
	Hooks Hooks

	// DeferShells, while set, has a regeneration leave each group's
	// solid as it was, marked ShellsStale (shelljob.go); Hooks.Cancel
	// stops one between groups.
	DeferShells bool

	// NoSolidCache has every solid made again at each regeneration, as
	// before solids were kept by what they were made from
	// (solid_cache.go); for a measure of what keeping them saves.
	NoSolidCache bool
	solids       *solidCache
	// NoSolverCache has every group's equations written out again at
	// each solve (solver_cache.go); for a measure of what keeping them
	// saves.
	NoSolverCache bool
	systems       map[core.HGroup]*solver.System

	// Clipboard holds what was copied.
	Clipboard Clipboard

	// Undo/redo stacks.
	Undo UndoStack
	Redo UndoStack

	// Handle -> index caches for GetEntity and GetParam. They are
	// validated on every lookup and rebuilt on a miss, so appending to
	// or reordering Entities and Params needs no bookkeeping.
	entityIndex map[core.HEntity]int
	paramIndex  map[core.HParam]int

	// Timings, when set, collects how long each group's regeneration
	// takes (telesapr-cli --profile).
	Timings *Timings
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

func (s *Sketch) Constraint(h core.HConstraint) *core.Constraint {
	for i := range s.Constraints {
		if s.Constraints[i].H == h {
			return &s.Constraints[i]
		}
	}
	return nil
}

func (s *Sketch) Entity(h core.HEntity) *core.Entity {
	if i, ok := s.entityIndex[h]; ok && i < len(s.Entities) && s.Entities[i].H == h {
		return &s.Entities[i]
	}
	// The list is kept sorted by handle, and a regeneration moves its
	// entities after each group: found by halves, the index is mended
	// for the one entity rather than made again for them all.
	if i := sort.Search(len(s.Entities), func(i int) bool { return s.Entities[i].H.V >= h.V }); i < len(s.Entities) && s.Entities[i].H == h && s.entityIndex != nil {
		s.entityIndex[h] = i
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

// MustEntity is GetEntity for handles that must exist.
func (s *Sketch) MustEntity(h core.HEntity) *core.Entity {
	e := s.Entity(h)
	if e == nil {
		panic(fmt.Sprintf("entity %08x not found", h.V))
	}
	return e
}

// MustParam is GetParam for handles that must exist.
func (s *Sketch) MustParam(h core.HParam) *core.Param {
	p := s.Param(h)
	if p == nil {
		panic(fmt.Sprintf("param %08x not found", h.V))
	}
	return p
}

// SeedParam sets param h to v, adding it if the group it belongs to has
// not been generated yet: the next regeneration takes v as where the
// param's solve starts, as it does a value from the regeneration
// before.
func (s *Sketch) SeedParam(h core.HParam, v float64) {
	if p := s.Param(h); p != nil {
		p.Val = v
		return
	}
	s.Params = append(s.Params, core.Param{H: h, Val: v})
}

func (s *Sketch) Param(h core.HParam) *core.Param {
	if i, ok := s.paramIndex[h]; ok && i < len(s.Params) && s.Params[i].H == h {
		return &s.Params[i]
	}
	// As for Entity.
	if i := sort.Search(len(s.Params), func(i int) bool { return s.Params[i].H.V >= h.V }); i < len(s.Params) && s.Params[i].H == h && s.paramIndex != nil {
		s.paramIndex[h] = i
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

func (s *Sketch) Request(h core.HRequest) *Request {
	for i := range s.Requests {
		if s.Requests[i].H == h {
			return &s.Requests[i]
		}
	}
	return nil
}

func (s *Sketch) Group(h core.HGroup) *Group {
	for i := range s.Groups {
		if s.Groups[i].H == h {
			return &s.Groups[i]
		}
	}
	return nil
}

func (s *Sketch) Style(h core.HStyle) *core.Style {
	for i := range s.Styles {
		if s.Styles[i].H == h {
			return &s.Styles[i]
		}
	}
	return nil
}

// AddReferences adds the references group and its three coordinate
// plane workplanes, which every sketch has. Call it on an empty sketch.
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

// The Add* functions add an item with a new handle, one more than the
// largest in use.

func (s *Sketch) AddGroup(g Group) core.HGroup {
	var top uint32
	for i := range s.Groups {
		top = maxU32(top, s.Groups[i].H.V)
	}
	g.H.V = top + 1
	s.Groups = append(s.Groups, g)
	return g.H
}

func (s *Sketch) AddRequest(r Request) core.HRequest {
	var top uint32
	for i := range s.Requests {
		top = maxU32(top, s.Requests[i].H.V)
	}
	r.H.V = top + 1
	s.Requests = append(s.Requests, r)
	return r.H
}

func (s *Sketch) AddEntity(e core.Entity) core.HEntity {
	var top uint32
	for i := range s.Entities {
		top = maxU32(top, s.Entities[i].H.V)
	}
	e.H.V = top + 1
	s.Entities = append(s.Entities, e)
	return e.H
}

func (s *Sketch) AddParam(p core.Param) core.HParam {
	var top uint32
	for i := range s.Params {
		top = maxU32(top, s.Params[i].H.V)
	}
	p.H.V = top + 1
	s.Params = append(s.Params, p)
	return p.H
}

func (s *Sketch) AddConstraint(c core.Constraint) core.HConstraint {
	var top uint32
	for i := range s.Constraints {
		top = maxU32(top, s.Constraints[i].H.V)
	}
	c.H.V = top + 1
	s.Constraints = append(s.Constraints, c)
	return c.H
}

func (s *Sketch) AddStyle(st core.Style) core.HStyle {
	var top uint32
	for i := range s.Styles {
		top = maxU32(top, s.Styles[i].H.V)
	}
	st.H.V = top + 1
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
// entities. Circles and arcs count their Bezier control points, which
// is not exact for arcs.
//
// Without includingInvisible, hidden entities (ActVisible off, or
// ForceHidden) are skipped.
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
			r := s.Request(e.H.Request())
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

func (s *Sketch) RunningMeshGroupFor(h core.HGroup) *Group {
	for g := s.Group(h); g != nil; g = g.PreviousGroup(s) {
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

// Image is a loaded image file: its size in pixels, which sets an image
// request's aspect ratio, and its pixels, for drawing.
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
