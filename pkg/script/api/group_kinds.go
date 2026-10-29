package api

import (
	"math"
	"slices"
	"strconv"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// The views of a group of one kind hold what only that kind has, as the
// group's page of the text window does: g.AsFillet().SetRadius(3). A
// view of a group of another kind stops the script with an error that
// says what the group is.

// ExtrudeGroup is the view of an extrusion: its draft.
type ExtrudeGroup struct{ g Group }

// SweepGroup is the view of a sweep along a path: its twist, the
// radius of its corners and which way is up.
type SweepGroup struct{ g Group }

// RepeatGroup is the view of a step and repeat, translating or
// rotating, or of a pattern along a path: how many copies it makes.
type RepeatGroup struct{ g Group }

// MirrorGroup is the view of a mirror: its plane and what it copies.
type MirrorGroup struct{ g Group }

// ChamferGroup is the view of a chamfer: its distances.
type ChamferGroup struct{ g Group }

// FilletGroup is the view of a fillet: its radius, and the radius of
// each edge that has its own.
type FilletGroup struct{ g Group }

// ShellGroup is the view of a shell: the thickness of its wall.
type ShellGroup struct{ g Group }

// LoftGroup is the view of a loft: its profiles, guides, turn and
// tangency.
type LoftGroup struct{ g Group }

// LinkGroup is the view of a linked part: the file it links and how.
type LinkGroup struct{ g Group }

// History is what the group depends on, and where it sits in the list
// of groups.
func (g Group) History() GroupHistory { return GroupHistory{g} }

// Analyze measures and checks the solid up to and with the group.
func (g Group) Analyze() GroupAnalysis { return GroupAnalysis{g} }

// AsExtrude is the view of an extrusion.
func (g Group) AsExtrude() ExtrudeGroup {
	g.is("AsExtrude", "an extrude", sketch.GroupExtrude)
	return ExtrudeGroup{g}
}

// AsSweep is the view of a sweep along a path.
func (g Group) AsSweep() SweepGroup {
	g.is("AsSweep", "a sweep", sketch.GroupSweep)
	return SweepGroup{g}
}

// AsRepeat is the view of a step and repeat or a pattern along a path.
func (g Group) AsRepeat() RepeatGroup {
	g.is("AsRepeat", "a step and repeat or a pattern", sketch.GroupTranslate, sketch.GroupRotate, sketch.GroupPathPattern)
	return RepeatGroup{g}
}

// AsMirror is the view of a mirror.
func (g Group) AsMirror() MirrorGroup {
	g.is("AsMirror", "a mirror", sketch.GroupMirror)
	return MirrorGroup{g}
}

// AsChamfer is the view of a chamfer.
func (g Group) AsChamfer() ChamferGroup {
	g.is("AsChamfer", "a chamfer", sketch.GroupChamfer)
	return ChamferGroup{g}
}

// AsFillet is the view of a fillet.
func (g Group) AsFillet() FilletGroup {
	g.is("AsFillet", "a fillet", sketch.GroupFillet)
	return FilletGroup{g}
}

// AsShell is the view of a shell.
func (g Group) AsShell() ShellGroup {
	g.is("AsShell", "a shell", sketch.GroupShell)
	return ShellGroup{g}
}

// AsLoft is the view of a loft.
func (g Group) AsLoft() LoftGroup {
	g.is("AsLoft", "a loft", sketch.GroupLoft)
	return LoftGroup{g}
}

// AsLink is the view of a linked part.
func (g Group) AsLink() LinkGroup {
	g.is("AsLink", "a linked part", sketch.GroupLinked)
	return LinkGroup{g}
}

// is fails unless the group is of one of the types, which what names.
func (g Group) is(view, what string, types ...sketch.GroupType) {
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		if !slices.Contains(types, group.Type) {
			fail("%s is for %s, and %s is a %s", view, what, groupDescription(group), kindName(group.Type))
		}
	})
}

// Copies is how many copies a step and repeat or a pattern makes.
func (v RepeatGroup) Copies() (n int) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		if group.Type != sketch.GroupTranslate && group.Type != sketch.GroupRotate && group.Type != sketch.GroupPathPattern {
			fail("%s isn't a step and repeat or a pattern", groupDescription(group))
		}
		n = int(group.ValA)
	})
	return n
}

// SkipFirst reports whether a one-sided step and repeat starts with its
// first copy, leaving the original out of the solid, as the group
// screen's "start with copy #1" says.
func (v RepeatGroup) SkipFirst() (skip bool) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBeStepAndRepeat(group, "SkipFirst")
		skip = group.SkipFirst
	})
	return skip
}

// SetSkipFirst has a one-sided step and repeat start with its first
// copy and leave the original out of the solid, or start with the
// original again. A rotate keeps the angle the script gave it, where
// the window re-spaces the copies when their count is typed. A
// two-sided repeat always has the original, and fails.
func (v RepeatGroup) SetSkipFirst(skip bool) {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBeStepAndRepeat(group, "SetSkipFirst")
		if err := sk.SetGroupSkipFirst(g.H, skip); err != nil {
			fail("%s: %v", groupDescription(group), err)
		}
	})
}

// mustBeStepAndRepeat fails unless the group is a translate or a rotate.
func (g Group) mustBeStepAndRepeat(group *sketch.Group, what string) {
	if group.Type != sketch.GroupTranslate && group.Type != sketch.GroupRotate {
		fail("%s is for a step and repeat, and %s is a %s", what, groupDescription(group), kindName(group.Type))
	}
}

// Skewed reports whether an extrusion leans with its sketch's normal
// rather than standing straight, as the group screen's "skewed" says.
func (v ExtrudeGroup) Skewed() (skewed bool) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupExtrude, "Skewed")
		skewed = group.Subtype == sketch.GroupOneSkewed || group.Subtype == sketch.GroupTwoSkewed
	})
	return skewed
}

// SetSkewed makes an extrusion's sides lean, or stand straight again,
// whether it is one-sided or two-sided, which SetTwoSided keeps.
func (v ExtrudeGroup) SetSkewed(skewed bool) {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupExtrude, "SetSkewed")
		if err := sk.SetGroupSkewed(g.H, skewed); err != nil {
			fail("%v", err)
		}
	})
}

// SetCopies is how many copies a step and repeat or a pattern makes. A
// rotate keeps the angle the script gave it, where the window re-spaces
// the copies over one turn, because a script states its angle itself.
func (v RepeatGroup) SetCopies(n int) {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		if group.Type != sketch.GroupTranslate && group.Type != sketch.GroupRotate && group.Type != sketch.GroupPathPattern {
			fail("%s isn't a step and repeat or a pattern", groupDescription(group))
		}
		if n < 1 || n > 999 || (group.Type == sketch.GroupPathPattern && n < 2) {
			fail("can't repeat %d times", n)
		}
		if err := sk.SetGroupCopies(g.H, strconv.Itoa(n), false); err != nil {
			fail("%v", err)
		}
	})
}

// Draft is an extrusion's draft in degrees: its sides lean into the
// solid by that much from the sketch's plane, out when it is negative.
func (v ExtrudeGroup) Draft() (degrees float64) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupExtrude, "Draft")
		degrees = group.ValC
	})
	return degrees
}

// SetDraft leans an extrusion's sides into the solid by degrees from
// the sketch's plane, so that the far end is smaller (larger when
// negative), as a moulded part's; 0 takes the draft away. The profile
// must be of lines and arcs; an arc may meet a line at a sharp corner,
// but arcs meet each other tangent. A draft at which the sides meet is
// a problem on the group, and the extrusion is made without it. SetExpr
// with sketch.GroupValC gives it an expression. SetDraftFaces leans
// only some sides, and SetDraftNeutral leans them from another plane.
func (v ExtrudeGroup) SetDraft(degrees float64) {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupExtrude, "SetDraft")
		if math.IsNaN(degrees) || math.Abs(degrees) >= 90 {
			fail("a draft is between -90 and 90 degrees, not %g", degrees)
		}
		group.ValC, group.ExprC = degrees, ""
		sk.MarkGroupDirty(group.H, false)
	})
}

// DraftFaces are the curves of an extrusion's sketch whose sides its
// draft leans, or none when it leans every side.
func (v ExtrudeGroup) DraftFaces() (curves []Entity) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupExtrude, "DraftFaces")
		for _, he := range group.DraftFaces {
			curves = append(curves, Entity{d: g.d, H: he})
		}
	})
	return curves
}

// SetDraftFaces makes an extrusion's draft lean the sides of curves
// only, the rest staying upright; none leans every side again. A curve
// is a line, arc or circle of the extruded sketch, or its copy at an end
// of the extrusion, or the face of its side.
func (v ExtrudeGroup) SetDraftFaces(curves []Entity) {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupExtrude, "SetDraftFaces")
		hs := make([]core.HEntity, len(curves))
		for i, e := range curves {
			hs[i] = e.get(sk).H
		}
		if err := sk.SetDraftFaces(group.H, hs); err != nil {
			fail("%v", err)
		}
	})
}

// DraftNeutral is how far along an extrusion, from the sketch's plane,
// its draft's neutral plane lies, in mm: where the solid has the
// profile's outline.
func (v ExtrudeGroup) DraftNeutral() (mm float64) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupExtrude, "DraftNeutral")
		mm = group.ValB
	})
	return mm
}

// SetDraftNeutral leans an extrusion's drafted sides from the plane mm
// along it from the sketch's plane, negative towards a two-sided
// extrusion's other end: there the solid has the profile's outline, and
// it narrows going away from it both ways, as a moulded part does from
// its parting line. 0 is the sketch's plane. A plane off the extrusion
// is a problem on the group. SetExpr with sketch.GroupPitch gives it an
// expression in the document's unit.
func (v ExtrudeGroup) SetDraftNeutral(mm float64) {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupExtrude, "SetDraftNeutral")
		if math.IsNaN(mm) || math.IsInf(mm, 0) {
			fail("a draft's neutral plane is a distance, not %g", mm)
		}
		group.ValB, group.ExprB = mm, ""
		sk.MarkGroupDirty(group.H, false)
	})
}

// Twist is how far a sweep turns its profile about the path over the
// whole of it, in degrees.
func (v SweepGroup) Twist() (degrees float64) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupSweep, "Twist")
		degrees = group.ValC
	})
	return degrees
}

// SetTwist turns a sweep's profile about the path by degrees over the
// whole of it, evenly along its length: a square swept along a line and
// twisted by 90 is a twisted bar. A twisted sweep's surface is fitted
// to the true one within a tenth of the chord tolerance, as a sweep
// along a cubic is. SetExpr with sketch.GroupValC gives it an
// expression.
func (v SweepGroup) SetTwist(degrees float64) {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupSweep, "SetTwist")
		if math.IsNaN(degrees) || math.IsInf(degrees, 0) {
			fail("a twist is a number of degrees, not %g", degrees)
		}
		group.ValC, group.ExprC = degrees, ""
		sk.MarkGroupDirty(group.H, false)
	})
}

// CornerRadius is a sweep's corner radius in mm, 0 where its corners
// are mitred.
func (v SweepGroup) CornerRadius() (mm float64) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupSweep, "CornerRadius")
		mm = group.ValB
	})
	return mm
}

// SetCornerRadius rounds a sweep's corners on its path by mm: at each
// corner the pieces either side are cut back to where an arc of the
// radius, tangent to both, joins them, and the profile turns about the
// arc's axis, so the solid is exact and needs no surface intersected.
// With none, corners between lines are mitred and corners beside an
// arc joined where the profile's paths along the pieces cross; a corner
// too sharp for that to join is a problem on the group until it has a
// radius. A fitted sweep (along a cubic, twisted, or held to a
// direction) mitres a corner between two lines when it isn't twisted,
// and needs a radius at any other; a corner beside a cubic can't be
// rounded. A radius too large for a piece, too small for the profile to
// clear the arc's axis, or a corner out of an arc's plane, is a problem
// on the group. SetExpr with sketch.GroupPitch gives it an expression.
func (v SweepGroup) SetCornerRadius(mm float64) {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupSweep, "SetCornerRadius")
		if !(mm >= 0) || math.IsInf(mm, 0) {
			fail("a corner radius is a length of 0 or more, not %g", mm)
		}
		group.ValB, group.ExprB = mm, ""
		sk.MarkGroupDirty(group.H, false)
	})
}

// SweepUp is the direction a sweep holds its frames to, or zero when
// they turn as little as the path lets them.
func (v SweepGroup) SweepUp() (up Vec) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupSweep, "SweepUp")
		up = group.SweepUp
	})
	return up
}

// SetSweepUp holds a sweep's frames to the direction up, as far as the
// path's tangent allows, rather than turning them as little as the path
// lets them: a profile swept along a helix with up its axis keeps
// facing the axis, as a thread's does. The path must never run along
// up. Zero turns them least again.
func (v SweepGroup) SetSweepUp(up Vec) {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupSweep, "SetSweepUp")
		if err := sk.SetSweepUp(g.H, up); err != nil {
			fail("%v", err)
		}
	})
}

// AsSolid reports whether a link group reads its STEP file as a
// solid rather than as a reference mesh.
func (v LinkGroup) AsSolid() (on bool) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupLinked, "LinkAsSolid")
		on = group.LinkAsSolid
	})
	return on
}

// SetAsSolid reads a link group's STEP file as a solid, or as a
// reference mesh, and reads it again.
func (v LinkGroup) SetAsSolid(on bool) {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupLinked, "SetLinkAsSolid")
		if err := sk.SetGroupLinkAsSolid(g.H, on); err != nil {
			fail("%v", err)
		}
	})
}

// Info describes the file a link group links, a line each, as the
// group's screen shows it: for a STEP file its schema and unit, how
// many shapes, faces and triangles it gave, and the chord tolerance it
// was tessellated to. A sketch or STL file gives none.
func (v LinkGroup) Info() (lines []string) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupLinked, "LinkInfo")
		lines = append(lines, group.LinkInfo...)
	})
	return lines
}

// Problem says what of the file a link group links could not be
// read, such as the faces of a STEP file on a kind of surface the
// reader does not support, or that the file holds no solid it can
// read; "" when all of it was.
func (v LinkGroup) Problem() (msg string) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupLinked, "LinkProblem")
		msg = group.LinkProblem
	})
	return msg
}

// KeepOriginal reports whether a mirror's solid has the original as
// well as the reflection.
func (v MirrorGroup) KeepOriginal() (keep bool) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupMirror, "KeepOriginal")
		keep = !group.SkipFirst
	})
	return keep
}

// SetKeepOriginal has a mirror's solid keep the original beside the
// reflection, or not, which leaves the reflection alone.
func (v MirrorGroup) SetKeepOriginal(keep bool) {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupMirror, "SetKeepOriginal")
		if err := sk.SetGroupSkipFirst(group.H, !keep); err != nil {
			fail("%v", err)
		}
	})
}

// MirrorSolid reports whether a mirror reflects the whole solid so far,
// up to and with its source group, rather than the source group's own
// solid.
func (v MirrorGroup) MirrorSolid() (whole bool) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupMirror, "MirrorSolid")
		whole = group.MirrorSolid
	})
	return whole
}

// SetMirrorSolid has a mirror reflect the whole solid so far, as a body
// made of several groups needs, or only its source group's solid. With
// the whole solid, the mirror's result is the solid so far: with its
// reflection when it keeps the original, or the reflection alone.
func (v MirrorGroup) SetMirrorSolid(whole bool) {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupMirror, "SetMirrorSolid")
		group.MirrorSolid = whole
		sk.MarkGroupDirty(group.H, false)
	})
}

// Plane is the workplane or face a mirror reflects in.
func (v MirrorGroup) Plane() (plane Entity) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupMirror, "Plane")
		plane = Entity{d: g.d, H: group.Predef.EntityB}
	})
	return plane
}

// Distance is a chamfer's distance in mm.
func (v ChamferGroup) Distance() (mm float64) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupChamfer, "Distance")
		mm = group.ValB
	})
	return mm
}

// SetDistance sets a chamfer's distance in mm; SetExpr with
// sketch.GroupPitch gives it an expression in the document's unit.
func (v ChamferGroup) SetDistance(mm float64) {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupChamfer, "SetDistance")
		if mm <= 0 || math.IsNaN(mm) || math.IsInf(mm, 0) {
			fail("a chamfer's distance must be more than zero, not %g", mm)
		}
		group.ValB, group.ExprB = mm, ""
		sk.MarkGroupDirty(group.H, false)
	})
}

// SecondDistance is a chamfer's distance in mm on each edge's second
// face, or 0 when it is the first distance on both faces. An edge's
// first face is the one of the lower identity (SolidFace.ID), the one
// its name gives first.
func (v ChamferGroup) SecondDistance() (mm float64) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupChamfer, "SecondDistance")
		mm = group.ValC
	})
	return mm
}

// SetSecondDistance makes a chamfer of two distances: Distance on each
// edge's first face and mm on the second; 0 makes it a chamfer of one
// distance again. SetExpr with sketch.GroupValC gives it an expression
// in the document's unit. A chamfer of two distances can't have edges
// that meet at a corner it cuts by its local edit; there it cuts each
// straight edge by a wedge instead, and a circular edge not at all.
func (v ChamferGroup) SetSecondDistance(mm float64) {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupChamfer, "SetSecondDistance")
		if err := sk.SetChamferSecondDistance(group.H, mm); err != nil {
			fail("%v", err)
		}
	})
}

// SwapDistances swaps a chamfer's two distances, which chamfers each
// edge the other way round. It fails for a chamfer of one distance.
func (v ChamferGroup) SwapDistances() {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupChamfer, "SwapDistances")
		if err := sk.SwapChamferDistances(group.H); err != nil {
			fail("%s has one distance; SetSecondDistance gives it a second", groupDescription(group))
		}
	})
}

// Radius is a fillet's radius in mm.
func (v FilletGroup) Radius() (mm float64) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupFillet, "Radius")
		mm = group.ValB
	})
	return mm
}

// SetRadius sets a fillet's radius in mm; SetExpr with
// sketch.GroupPitch gives it an expression in the document's unit.
func (v FilletGroup) SetRadius(mm float64) {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupFillet, "SetRadius")
		if mm <= 0 || math.IsNaN(mm) || math.IsInf(mm, 0) {
			fail("a fillet's radius must be more than zero, not %g", mm)
		}
		group.ValB, group.ExprB = mm, ""
		sk.MarkGroupDirty(group.H, false)
	})
}

// EdgeRadius is the radius in mm a fillet gives its edge i, counting
// the entities Edges gives and then the edges SolidEdges gives, from 0:
// the edge's own, or the fillet's.
func (v FilletGroup) EdgeRadius(i int) (mm float64) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupFillet, "EdgeRadius")
		if i < 0 || i >= len(group.Edges)+len(group.SolidEdges) {
			fail("the fillet has no edge %d", i)
		}
		mm, _, _ = group.EdgeRadius(i)
	})
	return mm
}

// SetEdgeRadius gives a fillet's edge i, counted as EdgeRadius counts,
// a radius of its own in mm, or with 0 the fillet's again. Rounded
// edges that meet at a corner must have one radius; one that ends where
// no other rounded edge does may have any.
func (v FilletGroup) SetEdgeRadius(i int, mm float64) {
	text := v.g.edgeMmText(mm, "a fillet's radius must be more than zero")
	if err := v.SetEdgeRadiusExpr(i, text); err != nil {
		fail("%v", err)
	}
}

// SetEdgeRadiusExpr gives a fillet's edge i, counted as EdgeRadius
// counts, a radius of its own from an expression in the document's
// unit, which may name parameters; "" gives it the fillet's radius
// again.
func (v FilletGroup) SetEdgeRadiusExpr(i int, expr string) (err error) {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupFillet, "SetEdgeRadiusExpr")
		err = sk.SetEdgeRadiusText(g.H, i, expr)
	})
	return err
}

// Thickness is a shell's wall thickness in mm.
func (v ShellGroup) Thickness() (mm float64) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupShell, "Thickness")
		mm = group.ValB
	})
	return mm
}

// SetThickness sets a shell's wall thickness in mm; SetExpr with
// sketch.GroupPitch gives it an expression in the document's unit.
func (v ShellGroup) SetThickness(mm float64) {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupShell, "SetThickness")
		if mm <= 0 || math.IsNaN(mm) || math.IsInf(mm, 0) {
			fail("a shell's thickness must be more than zero, not %g", mm)
		}
		group.ValB, group.ExprB = mm, ""
		sk.MarkGroupDirty(group.H, false)
	})
}

// Profiles are the sketches a loft joins, in order.
func (v LoftGroup) Profiles() (out []Group) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupLoft, "Profiles")
		for _, h := range group.LoftProfiles() {
			out = append(out, Group{g.d, h})
		}
	})
	return out
}

// Turn is how many curves a loft turns its second profile by before
// joining curve by curve; a smooth loft turns each after the first.
func (v LoftGroup) Turn() (n int) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupLoft, "Turn")
		n = int(group.ValA)
	})
	return n
}

// Tangency is how square a loft leaves its first profile and reaches
// its last, 0 for an end left free (see SetTangency).
func (v LoftGroup) Tangency() (start, end float64) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupLoft, "Tangency")
		start, end = group.ValB, group.ValC
	})
	return start, end
}

// SetTangency makes a loft leave its first profile square to its plane
// and reach its last square to its, the surface's slope across there
// start and end times the loft's length; 0 leaves an end free, shaped
// by the profiles alone. A loft of two profiles with an end set is one
// smooth surface, not ruled patches. On a closed loft, which has no
// ends, start and end hold the ring square to its first and last
// profiles, as SetProfileTangency does those between. SetExpr with
// sketch.GroupPitch and sketch.GroupValC gives them expressions.
func (v LoftGroup) SetTangency(start, end float64) {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupLoft, "SetTangency")
		for _, x := range []float64{start, end} {
			if math.IsNaN(x) || math.IsInf(x, 0) {
				fail("a loft's tangency is a number, not %g", x)
			}
		}
		group.ValB, group.ExprB = start, ""
		group.ValC, group.ExprC = end, ""
		sk.MarkGroupDirty(group.H, false)
	})
}

// AlongSides reports whether a loft leaves its first profile (start)
// and reaches its last (end) along the sides of the solid each is drawn
// on (see SetAlongSides).
func (v LoftGroup) AlongSides() (start, end bool) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupLoft, "AlongSides")
		start, end = group.LoftAlongSides[0], group.LoftAlongSides[1]
	})
	return start, end
}

// SetAlongSides has a loft drawn from a face of a solid carry on its
// sides smoothly: at each point of the first profile (start) it leaves
// along the side face meeting the profile's edge there, in that face's
// tangent plane, rather than square to the profile's plane, and
// likewise reaches the last (end). Where the sides are square to the
// face that is the same; where they lean or curve, as a drafted or
// turned body's do, the loft follows them. The profile must lie along
// the edges of the solid before the loft. Tangency sets how far the
// slope reaches, 1 where it is 0. A loft held by guides holds them
// too.
func (v LoftGroup) SetAlongSides(start, end bool) {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupLoft, "SetAlongSides")
		if err := sk.SetLoftAlongSides(group.H, start, end); err != nil {
			fail("%v", err)
		}
	})
}

// Closed reports whether a smooth loft goes from its last profile back
// to its first (see SetClosed).
func (v LoftGroup) Closed() (closed bool) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupLoft, "Closed")
		closed = group.LoftClosed
	})
	return closed
}

// SetClosed has a smooth loft go from its last profile back to its
// first, a ring with no caps, as a sweep round a closed path is, the
// surface as smooth where it closes as elsewhere; false opens it again.
// A closed loft goes through three profiles at least, and has no turn
// and no ends to carry a solid's sides on (SetAlongSides); any of its
// profiles can be held square (SetProfileTangency, or SetTangency for
// the first and last), and the ring held by guides that go round it,
// closing on themselves (SetGuides), or both.
func (v LoftGroup) SetClosed(closed bool) {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupLoft, "SetClosed")
		if err := sk.SetLoftClosed(group.H, closed); err != nil {
			fail("%v", err)
		}
	})
}

// ProfileTangency is how square a smooth loft passes profile, one of
// the sketches between its ends or any of a closed loft's, 0 for free
// (see SetProfileTangency).
func (v LoftGroup) ProfileTangency(profile Group) (t float64) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupLoft, "ProfileTangency")
		t = group.LoftProfileTangency(profile.H)
	})
	return t
}

// SetProfileTangency holds a smooth loft square to the plane of
// profile, one of the sketches it goes through between its ends, the
// surface's slope across there t times the loft's length, as SetTangency
// does at the ends; 0 leaves it free, shaped by the profiles about it.
// The loft is then smooth through that profile but no longer bends as
// one curve across it: a waist that runs straight through the middle.
// A closed loft is held so too, round the ring, at its first and last
// profiles as well (what SetTangency sets), and a loft held by guides
// still holds them, leaving the profile square and turning onto a guide
// that crosses it slanting.
func (v LoftGroup) SetProfileTangency(profile Group, t float64) {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupLoft, "SetProfileTangency")
		if err := sk.SetLoftProfileTangency(group.H, profile.H, t); err != nil {
			fail("%v", err)
		}
	})
}

// Guides are a loft's guide curves, the line segments, arcs and cubics
// that hold it (see SetGuides), or none.
func (v LoftGroup) Guides() (curves []Entity) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupLoft, "Guides")
		for _, he := range group.Edges {
			curves = append(curves, Entity{d: g.d, H: he})
		}
	})
	return curves
}

// SetGuides holds a loft to guide curves: line segments, arcs and
// cubics drawn before it, each chain of them that joins end to end one
// guide, from a point of the first profile through every profile to
// the last; on a closed loft, from a point of the first profile round
// the ring through every profile back to it, closing on itself. One
// guide moves the loft's sections across so that it runs along the
// guide; two move, turn and scale them, so that the loft
// widens and narrows between them. None takes the guides away. A guide
// that misses a profile is the group's problem, with how far.
func (v LoftGroup) SetGuides(curves []Entity) {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupLoft, "SetGuides")
		hs := make([]core.HEntity, len(curves))
		for i, e := range curves {
			hs[i] = e.get(sk).H
		}
		if err := sk.SetLoftGuides(group.H, hs); err != nil {
			fail("%v", err)
		}
	})
}

// SetTurn turns a loft's second profile by n curves, which joins its
// curves to others of the first: a square lofted to a square turned by
// one twists by a quarter.
func (v LoftGroup) SetTurn(n int) {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupLoft, "SetTurn")
		group.ValA, group.ExprA = float64(n), ""
		sk.MarkGroupDirty(group.H, false)
	})
}

// A feature's inputs, what it is built from, can be changed after it
// is made (ROADMAP F7): the setters below reach the same checks as
// the feature's constructor, including a trial on the solid before the
// feature, so a change the kernel can't make fails and nothing moves.
// Picks on the solid come from the group before the feature, as
// doc.Group("box").Solid().EdgeAt(p) gives them. The feature's source
// group is not changed this way.

// entityHandles are the entities' handles in the sketch.
func entityHandles(sk *sketch.Sketch, es []Entity) []core.HEntity {
	hs := make([]core.HEntity, len(es))
	for i, e := range es {
		hs[i] = e.get(sk).H
	}
	return hs
}

// edgeNames are the solid edges' names.
func edgeNames(es []SolidEdge) []topo.Name {
	names := make([]topo.Name, len(es))
	for i, e := range es {
		names[i] = e.name
	}
	return names
}

// SetEdges makes the fillet round edges, line segments and circles as
// Features.Fillet takes, and solid, edges of the solid before it as
// Features.FilletSolid takes, instead of the edges it rounds now. An
// edge's own radius stays with it, an edge taken away takes its
// radius with it, and an edge added takes the fillet's. The fillet is
// tried first, so an edge it can't round fails and nothing changes.
func (v FilletGroup) SetEdges(edges []Entity, solid []SolidEdge) {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupFillet, "SetEdges")
		if err := sk.SetFilletEdges(group.H, entityHandles(sk, edges), edgeNames(solid)); err != nil {
			fail("%v", err)
		}
	})
}

// SetEdges makes the chamfer cut edges, line segments and circles as
// Features.Chamfer takes, and solid, edges of the solid before it as
// Features.ChamferSolid takes, instead of the edges it cuts now. The
// chamfer is tried first with its distances, so an edge it can't cut
// fails and nothing changes.
func (v ChamferGroup) SetEdges(edges []Entity, solid []SolidEdge) {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupChamfer, "SetEdges")
		if err := sk.SetChamferEdges(group.H, entityHandles(sk, edges), edgeNames(solid)); err != nil {
			fail("%v", err)
		}
	})
}

// Faces are the faces of the solid before the shell that it takes
// away.
func (v ShellGroup) Faces() (faces []SolidFace) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupShell, "Faces")
		s := Solid{d: g.d}
		if prev := group.PreviousGroup(sk); prev != nil {
			s.g = prev.H
		}
		for _, id := range group.SolidFaces {
			faces = append(faces, SolidFace{s: s, id: id})
		}
	})
	return faces
}

// SetFaces makes the shell open where faces, of the solid before it as
// Features.ShellSolid takes them, are taken away, instead of the faces
// it takes away now; a face entity's face is Solid.FaceAt a point on
// it. The shell is tried first, so a face it can't open fails and
// nothing changes.
func (v ShellGroup) SetFaces(faces []SolidFace) {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupShell, "SetFaces")
		ids := make([]srf.FaceID, len(faces))
		for i, f := range faces {
			ids[i] = f.id
		}
		if err := sk.SetShellFaces(group.H, ids); err != nil {
			fail("%v", err)
		}
	})
}

// SetPath makes the sweep follow path, line segments, arcs and cubics
// as Features.Sweep takes, and solid, edges of the solid before it as
// Features.SweepEdges takes, together one chain, instead of the path it
// follows now. The path is tried first, so one the profile can't be
// swept along fails and nothing changes.
func (v SweepGroup) SetPath(path []Entity, solid []SolidEdge) {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupSweep, "SetPath")
		if err := sk.SetSweepPath(group.H, entityHandles(sk, path), edgeNames(solid)); err != nil {
			fail("%v", err)
		}
	})
}

// SetPath makes a pattern along a path repeat its solid along path,
// line segments and arcs as Features.PatternAlongPath takes, instead of
// the path it follows now; a step and repeat has no path. The path is
// tried first, so one the copies can't be placed along fails and
// nothing changes.
func (v RepeatGroup) SetPath(path []Entity) {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupPathPattern, "SetPath")
		if err := sk.SetPatternPath(group.H, entityHandles(sk, path)); err != nil {
			fail("%v", err)
		}
	})
}

// SetProfiles makes the loft go through profiles, two or more sketches
// in order as Features.LoftThrough takes them, instead of the profiles
// it goes through now: the first and the last are its ends. A profile
// kept between the ends keeps how square the loft is held to it; the
// guides stay. The loft is tried first, so profiles it can't join fail
// and nothing changes.
func (v LoftGroup) SetProfiles(profiles []Group) {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupLoft, "SetProfiles")
		hs := make([]core.HGroup, len(profiles))
		for i, p := range profiles {
			hs[i] = p.get(sk).H
		}
		if err := sk.SetLoftProfiles(group.H, hs); err != nil {
			fail("%v", err)
		}
	})
}

// SetPlane makes the mirror reflect in plane, a workplane or a planar
// face made before it, instead of the plane it reflects in now.
func (v MirrorGroup) SetPlane(plane Entity) {
	g := v.g
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		g.mustBe(group, sketch.GroupMirror, "SetPlane")
		if err := sk.SetMirrorPlane(group.H, plane.get(sk).H); err != nil {
			fail("%v", err)
		}
	})
}
