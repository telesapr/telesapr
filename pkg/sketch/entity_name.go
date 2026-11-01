package sketch

import (
	"fmt"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/srf"
)

// This file names entities for a person: what an entity is and where,
// as "top face of g003-extrude" or "side edge of g003-extrude from
// r005-line-segment", rather than by a handle no one can act on. Every
// screen and message that names an entity goes through entityName, so
// they word it the same way.

// EntityName names an entity for a person: a request's entity by its
// request, "r005-line-segment of g002-sketch"; an entity a group made
// by what it is, where, and what it was made from, "top edge of
// g003-extrude from r005-line-segment". It names an entity deleted
// since the last regeneration too.
func (sk *Sketch) EntityName(he core.HEntity) string { return sk.entityName(he) }

// entityName is EntityName.
func (sk *Sketch) entityName(he core.HEntity) string {
	if he.IsFromRequest() {
		return sk.requestEntityName(he, true)
	}
	return sk.madeEntityName(he, "", 0)
}

// requestEntityName names an entity of a request: the request itself
// for its entity 0, or for one of its parts, the part's kind and number
// with the request, "point 1 of r005-line-segment". A part is named so
// only as the source of a copy (inGroup false); in a message it is
// named by its request, as it always has been. inGroup adds the group.
func (sk *Sketch) requestEntityName(he core.HEntity, inGroup bool) string {
	var name string
	var group core.HGroup
	if r := sk.Request(he.Request()); r != nil {
		name, group = r.DescriptionString(), r.Group
	} else if r, ok := sk.names.requests[he.Request()]; ok {
		gone := Request{H: he.Request(), Type: r.typ}
		name, group = gone.DescriptionString(), r.group
	} else {
		return fmt.Sprintf("r%03x", he.Request().V)
	}
	if !inGroup {
		if part := he.V & 0xffff; part != 0 {
			kind := "part"
			if e := sk.Entity(he); e != nil {
				kind = entityKindWord(e, false)
			}
			name = fmt.Sprintf("%s %d of %s", kind, part, name)
		}
		return name
	}
	return name + " of " + sk.anyGroupName(group)
}

// madeEntityName names entity he that a group made: kind is the word
// for what it is, "" to take it from the entity or the copy that made
// it. depth bounds the chain of copies of copies that names the
// source.
func (sk *Sketch) madeEntityName(he core.HEntity, kind string, depth int) string {
	hg := he.Group()
	group := sk.anyGroupName(hg)
	g := sk.Group(hg)
	key, made := core.EntityKey{}, false
	if g != nil {
		key, made = g.remapKeyOf(he)
	}
	e := sk.Entity(he)
	solid := g != nil && sweepsSolid(g.Type)
	if made && !solid && key.Input != core.NoEntity && !key.Input.IsFromRequest() {
		// A copy of what a solid's group made, as a step and repeat
		// of an extrusion's edges, is on a solid too.
		src := sk.Group(key.Input.Group())
		solid = src != nil && sweepsSolid(src.Type)
	}
	if kind == "" {
		switch {
		case e != nil:
			kind = entityKindWord(e, solid)
		case made:
			kind = sk.copyKindWord(key, solid)
		}
	}
	if !made {
		if g != nil && g.Type == GroupDrawingWorkplane {
			if w, ok := workplaneParts[he.V&0xffff]; ok {
				return w + " of " + group
			}
		}
		if kind == "" {
			kind = "entity"
		}
		return fmt.Sprintf("%s %d of %s", kind, he.V&0xffff, group)
	}
	s := kind + " of " + group
	if pos := remapPosition(key.CopyNumber); pos != "" {
		s = pos + " " + s
	}
	if src := sk.sourceName(key.Input, depth); src != "" {
		s += " from " + src
	}
	return s + copySuffix(g.Type, key.CopyNumber)
}

// copySuffix says which copy of a step and repeat or a pattern an
// entity is in; a group that makes one of each makes no copies to tell
// apart.
func copySuffix(t GroupType, copyNumber int) string {
	switch {
	case t != GroupTranslate && t != GroupRotate && t != GroupPathPattern:
		return ""
	case copyNumber == RemapLast:
		return " (last copy)"
	case copyNumber >= RemapPattern:
		return fmt.Sprintf(" (copy %d)", copyNumber-RemapPattern+1)
	case copyNumber < RemapLast:
		return fmt.Sprintf(" (copy %d)", copyNumber+1)
	}
	return ""
}

// sweepsSolid reports whether a group of type t makes a solid by
// sweeping a sketch, so that the curves it makes are edges.
func sweepsSolid(t GroupType) bool {
	return t == GroupExtrude || t == GroupLathe || t == GroupRevolve || t == GroupHelix
}

// workplaneParts name the entities a sketch's group makes for its
// workplane, by their numbers in the group.
var workplaneParts = map[uint32]string{0: "workplane", 1: "normal", 2: "origin"}

// sourceName names the entity a copy was made from, short: a request's
// without its group, which its handle already says. It is "" for no
// entity, as an extrusion's top face is made from, and for one this
// sketch can't name, as a linked part's own.
func (sk *Sketch) sourceName(in core.HEntity, depth int) string {
	switch {
	case in == core.NoEntity:
		return ""
	case in.V&(1<<30) != 0 && in.V&(1<<31) == 0:
		// A face no entity names, copied as a step and repeat copies
		// the solid's faces.
		return sk.FaceName(srf.FaceID(in.V))
	case in.IsFromRequest():
		if sk.Request(in.Request()) == nil {
			if _, ok := sk.names.requests[in.Request()]; !ok {
				return ""
			}
		}
		return sk.requestEntityName(in, false)
	case depth < 4 && sk.Group(in.Group()) != nil:
		return sk.madeEntityName(in, "", depth+1)
	}
	return ""
}

// remapKeyOf is the key in the group's remap table that made he.
func (g *Group) remapKeyOf(he core.HEntity) (core.EntityKey, bool) {
	for key, id := range g.Remap {
		if g.H.Entity(int(id.V)) == he {
			return key, true
		}
	}
	return core.EntityKey{}, false
}

// remapPosition is where on the solid a copy lies, by its copy number:
// "top" and "bottom" of an extrusion, "start" and "end" of a lathe, and
// "side" for what a sweep makes of an entity, the face of a line or
// the edge of a point. It is "" for the copies of a step and repeat.
func remapPosition(copyNumber int) string {
	switch copyNumber {
	case RemapTop:
		return "top"
	case RemapBottom:
		return "bottom"
	case RemapLatheStart:
		return "start"
	case RemapLatheEnd:
		return "end"
	case RemapPtToLine, RemapLineToFace, RemapPtToArc:
		return "side"
	case RemapLatheArcCenter:
		return "centre"
	}
	return ""
}

// copyKindWord is what a copy is, when the entity itself is gone: what
// its copy number makes, or else what it was copied from.
func (sk *Sketch) copyKindWord(key core.EntityKey, solid bool) string {
	switch key.CopyNumber {
	case RemapLineToFace:
		return "face"
	case RemapPtToLine, RemapPtToArc:
		return "edge"
	case RemapPtToNormal:
		return "normal"
	case RemapLatheArcCenter:
		return "point"
	}
	if key.Input == core.NoEntity {
		// What an extrusion or a lathe makes of no entity is its end
		// faces.
		return "face"
	}
	if e := sk.Entity(key.Input); e != nil {
		return entityKindWord(e, solid)
	}
	return ""
}

// entityKindWord is what an entity is, in words: on a solid (solid
// true), a curve is an edge.
func entityKindWord(e *core.Entity, solid bool) string {
	switch {
	case e.IsPoint():
		return "point"
	case e.IsNormal():
		return "normal"
	case e.IsDistance():
		return "distance"
	case e.IsFace():
		return "face"
	}
	switch e.Type {
	case core.EntityWorkplane:
		return "workplane"
	case core.EntityTTFText:
		return "text"
	case core.EntityImage:
		return "image"
	}
	if solid {
		return "edge"
	}
	switch e.Type {
	case core.EntityLineSegment:
		return "line segment"
	case core.EntityCubic, core.EntityCubicPeriodic:
		return "curve"
	case core.EntityCircle:
		return "circle"
	case core.EntityArcOfCircle:
		return "arc"
	}
	return "entity"
}
