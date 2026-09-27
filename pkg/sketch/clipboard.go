package sketch

import (
	"math"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
)

// This file ports solvespace/src/clipboard.cpp: the clipboard that Cut,
// Copy and Paste use. The selection is passed in; the menus and the
// paste-transformed text screen are user interface.

// ClipboardRequest is a copied request: its points in the workplane's
// coordinates, and the entities it came from (C++ ClipboardRequest).
type ClipboardRequest struct {
	Type         RequestType
	ExtraPoints  int
	Style        core.HStyle
	Str          string
	Font         string
	File         string
	Construction bool

	Point    [core.MaxPointsInEntity]geom.Vector
	Distance float64

	OldEnt      core.HEntity
	OldPointEnt [core.MaxPointsInEntity]core.HEntity
	NewReq      core.HRequest
}

// Clipboard holds copied requests, and the constraints among them (C++
// SolveSpaceUI::Clipboard).
type Clipboard struct {
	Requests    []ClipboardRequest
	Constraints []core.Constraint
}

func (c *Clipboard) Clear() {
	c.Requests = nil
	c.Constraints = nil
}

// ContainsEntity reports whether he, or no entity, was copied (C++
// Clipboard::ContainsEntity).
func (c *Clipboard) ContainsEntity(he core.HEntity) bool {
	if he == core.NoEntity {
		return true
	}

	for i := range c.Requests {
		cr := &c.Requests[i]
		if cr.OldEnt == he {
			return true
		}

		for j := 0; j < core.MaxPointsInEntity; j++ {
			if cr.OldPointEnt[j] == he {
				return true
			}
		}
	}
	return false
}

// NewEntityFor is the pasted entity for copied entity he (C++
// Clipboard::NewEntityFor).
func (c *Clipboard) NewEntityFor(he core.HEntity) core.HEntity {
	if he == core.NoEntity {
		return core.NoEntity
	}

	for i := range c.Requests {
		cr := &c.Requests[i]
		if cr.OldEnt == he {
			return cr.NewReq.Entity(0)
		}

		for j := 0; j < core.MaxPointsInEntity; j++ {
			if cr.OldPointEnt[j] == he {
				return cr.NewReq.Entity(1 + j)
			}
		}
	}

	panic("Expected to find entity in some clipboard request")
}

// activeWorkplaneCsys is the active workplane's basis and origin.
func (sk *Sketch) activeWorkplaneCsys() (u, v, n, p geom.Vector) {
	wrkpl := sk.MustGetEntity(sk.ActiveWorkplane())
	wrkpln := sk.MustGetEntity(wrkpl.Normal)
	return wrkpln.NormalU(sk), wrkpln.NormalV(sk), wrkpln.NormalN(sk),
		sk.MustGetEntity(wrkpl.Point[0]).PointGetNum(sk)
}

// CopySelection copies the selected entities that requests generate
// (not workplanes), the selected comments, and the constraints among
// the copied entities (C++ GraphicsWindow::CopySelection).
func (sk *Sketch) CopySelection(selection []Selection) {
	cb := &sk.Clipboard
	cb.Clear()

	u, v, n, p := sk.activeWorkplaneCsys()

	for _, s := range selection {
		if s.Entity.V == 0 {
			continue
		}
		// Work only on entities that have requests that will generate them.
		e := sk.MustGetEntity(s.Entity)
		req, pts, _, hasDistance, ok := getEntityInfo(e.Type, e.ExtraPoints)
		if !ok {
			if !e.H.IsFromRequest() {
				continue
			}
			r := sk.GetRequest(e.H.Request())
			if r.Type != RequestDatumPoint {
				continue
			}
			req, pts, _, hasDistance, ok = getEntityInfo(core.EntityType(0), e.ExtraPoints)
			if !ok {
				panic("No entity info")
			}
		}
		if req == RequestWorkplane {
			continue
		}

		cr := ClipboardRequest{}
		cr.Type = req
		cr.ExtraPoints = e.ExtraPoints
		cr.Style = e.Style
		cr.Str = e.Str
		cr.Font = e.Font
		cr.File = e.File
		cr.Construction = e.Construction
		for i := 0; i < pts; i++ {
			var pt geom.Vector
			if req == RequestDatumPoint {
				pt = e.PointGetNum(sk)
			} else {
				pt = sk.MustGetEntity(e.Point[i]).PointGetNum(sk)
			}
			pt = pt.Minus(p)
			pt = pt.DotInToCsys(u, v, n)
			cr.Point[i] = pt
		}
		if hasDistance {
			cr.Distance = sk.MustGetEntity(e.Distance).DistanceGetNum(sk)
		}

		cr.OldEnt = e.H
		for i := 0; i < pts; i++ {
			cr.OldPointEnt[i] = e.Point[i]
		}

		cb.Requests = append(cb.Requests, cr)
	}

	for _, s := range selection {
		if s.Constraint.V == 0 {
			continue
		}

		c := sk.GetConstraint(s.Constraint)
		if c.Type == core.ConstraintComment {
			cb.Constraints = append(cb.Constraints, *c)
		}
	}

	for _, c := range sk.Constraints {
		if !cb.ContainsEntity(c.PtA) ||
			!cb.ContainsEntity(c.PtB) ||
			!cb.ContainsEntity(c.EntityA) ||
			!cb.ContainsEntity(c.EntityB) ||
			!cb.ContainsEntity(c.EntityC) ||
			!cb.ContainsEntity(c.EntityD) ||
			c.Type == core.ConstraintComment {
			continue
		}
		cb.Constraints = append(cb.Constraints, c)
	}
}

// PasteClipboard pastes the clipboard into the active group and
// workplane: scaled (mirrored if negative) in the workplane, rotated by
// theta about its normal, and moved by trans. It returns what C++
// selects afterwards: the pasted entities and comments (C++
// GraphicsWindow::PasteClipboard).
func (sk *Sketch) PasteClipboard(trans geom.Vector, theta, scale float64) []Selection {
	cb := &sk.Clipboard
	u, v, n, p := sk.activeWorkplaneCsys()
	var selected []Selection

	// For arcs, reflection involves swapping the endpoints, or otherwise
	// the arc gets inverted.
	mapPoint := func(he core.HEntity) core.HEntity {
		if he.V == 0 {
			return he
		}

		if scale < 0 {
			hr := he.Request()
			r := sk.GetRequest(hr)
			if r.Type == RequestArcOfCircle {
				if he == hr.Entity(2) {
					return hr.Entity(3)
				} else if he == hr.Entity(3) {
					return hr.Entity(2)
				}
			}
		}
		return he
	}

	for ci := range cb.Requests {
		cr := &cb.Requests[ci]
		hr := sk.AddRequestOfType(cr.Type, false)
		r := sk.GetRequest(hr)
		r.ExtraPoints = cr.ExtraPoints
		r.Style = cr.Style
		r.Str = cr.Str
		r.Font = cr.Font
		r.File = cr.File
		r.Construction = cr.Construction
		// Need to regen to get the right number of points, if extraPoints
		// changed.
		sk.GenerateAllTyped(GenerateRegen)
		r = sk.GetRequest(hr)
		sk.MarkGroupDirty(r.Group, false)
		_, pts, _, hasDistance, _ := getRequestInfo(r.Type, r.ExtraPoints)
		for i := 0; i < pts; i++ {
			pt := cr.Point[i]
			// We need the reflection to occur within the workplane; it may
			// otherwise correspond to just a rotation as projected.
			if scale < 0 {
				pt.X *= -1
			}
			// Likewise the scale, which could otherwise take us out of the
			// workplane.
			pt = pt.ScaledBy(math.Abs(scale))
			pt = pt.ScaleOutOfCsys(u, v, geom.Vector{})
			pt = pt.Plus(p)
			pt = pt.RotatedAroundAxis(n, theta)
			pt = pt.Plus(trans)
			j := i + 1
			if r.Type == RequestDatumPoint {
				j = i
			}
			sk.MustGetEntity(mapPoint(hr.Entity(j))).PointForceTo(sk, pt)
		}
		if hasDistance {
			sk.MustGetEntity(hr.Entity(64)).DistanceForceTo(sk, cr.Distance*math.Abs(scale))
		}

		cr.NewReq = hr
		selected = append(selected, Selection{Entity: hr.Entity(0)})
		for i := 0; i < pts; i++ {
			j := i + 1
			if r.Type == RequestDatumPoint {
				j = i
			}
			selected = append(selected, Selection{Entity: hr.Entity(j)})
		}
	}

	for ci := range cb.Constraints {
		cc := &cb.Constraints[ci]
		c := core.Constraint{}
		c.Group = sk.ActiveGroup()
		c.Workplane = sk.ActiveWorkplane()
		c.Type = cc.Type
		c.ValA = cc.ValA
		c.PtA = cb.NewEntityFor(mapPoint(cc.PtA))
		c.PtB = cb.NewEntityFor(mapPoint(cc.PtB))
		c.EntityA = cb.NewEntityFor(cc.EntityA)
		c.EntityB = cb.NewEntityFor(cc.EntityB)
		c.EntityC = cb.NewEntityFor(cc.EntityC)
		c.EntityD = cb.NewEntityFor(cc.EntityD)
		c.Other = cc.Other
		c.Other2 = cc.Other2
		c.Reference = cc.Reference
		c.Disp = cc.Disp
		c.Comment = cc.Comment
		dontAddConstraint := false
		switch c.Type {
		case core.ConstraintComment:
			c.Disp.Offset = c.Disp.Offset.Plus(trans)
		case core.ConstraintPtLineDistance:
			c.ValA *= scale
		case core.ConstraintPtPtDistance, core.ConstraintProjPtDistance, core.ConstraintDiameter:
			c.ValA *= math.Abs(scale)
		case core.ConstraintArcLineTangent:
			if scale < 0 {
				// When mirroring swap the end point of the arc. The reason is that arcs are
				// defined as counter-clockwise from the start point to the end point. And we
				// swapped its points in mapPoint above to avoid inverting it.
				c.Other = !c.Other
			}
		case core.ConstraintCubicLineTangent:
			// Nothing to do for CUBIC unlike the ARC_OF_CIRCLE above and below.
		case core.ConstraintCurveCurveTangent:
			if scale < 0 {
				// When mirroring swap the end points of arcs. The reason is that arcs are
				// defined as counter-clockwise from the start point to the end point. And we
				// swapped their points in mapPoint above to avoid inverting them.
				// CUBIC splines do not need this.
				if sk.MustGetEntity(c.EntityA).Type == core.EntityArcOfCircle {
					c.Other = !c.Other
				}
				if sk.MustGetEntity(c.EntityB).Type == core.EntityArcOfCircle {
					c.Other2 = !c.Other2
				}
			}
		case core.ConstraintHorizontal, core.ConstraintVertical:
			// When rotating 90 or 270 degrees, swap the vertical / horizontal constraints
			if math.Mod(theta+(math.Pi/2), math.Pi) == 0 {
				if c.Type == core.ConstraintHorizontal {
					c.Type = core.ConstraintVertical
				} else {
					c.Type = core.ConstraintHorizontal
				}
			} else if math.Mod(theta, math.Pi/2) != 0 {
				dontAddConstraint = true
			}
		}
		if !dontAddConstraint {
			hc := sk.AddConstraintRemember(&c, false)
			if c.Type == core.ConstraintComment {
				selected = append(selected, Selection{Constraint: hc})
			}
		}
	}
	return selected
}
