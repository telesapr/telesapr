package ui

import (
	"github.com/telesapr/telesapr/pkg/core"
)

// describeOverlap lists, under the description of point e, the points
// at its position and the curves through it, as links that select one
// of them: a click on the drawing picks only one of what lies at a
// place, and coincident points are otherwise out of reach. The sketch
// lists only what the drawing shows, as only that could have been
// clicked.
func (d *describer) describeOverlap(e *core.Entity) {
	tw, sk := d.tw, d.sk
	o, err := sk.Overlap(e.H)
	if err != nil {
		return
	}
	var rows []*core.Entity
	for _, h := range o.Points {
		if pe := sk.Entity(h); pe != nil {
			rows = append(rows, pe)
		}
	}
	for _, h := range o.Curves {
		if ce := sk.Entity(h); ce != nil {
			rows = append(rows, ce)
		}
	}
	if len(rows) == 0 {
		return
	}
	var owners map[core.HEntity]pointOwner
	if len(o.Points) > 0 {
		owners = d.pointOwners()
	}
	tw.Printf(false, "%FtALSO HERE%E")
	for i, re := range rows {
		desc := d.ss.entityDescription(re)
		if re.IsPoint() {
			desc = owners[re.H].role() + " of " + desc
		} else {
			desc += ", through it"
		}
		tw.Printf(false, "%Bp   %Fl%Ll%D%f%h%s%E",
			parity(i), re.H.V, lf(tw.ScreenSelectEntity), lf(tw.ScreenHoverEntity), desc)
	}
}

// pointOwner is the entity a point is a point of, and which of its
// points: the start or end of a line, the centre of a circle, the
// origin of a workplane. A point with no owner is a datum point.
type pointOwner struct {
	e *core.Entity
	i int
}

// pointOwners maps each point of a line, circle, arc, cubic or
// workplane to its owner. The owners are found by their points, as a
// group's own workplane and a copy in a later group come from no
// request.
func (d *describer) pointOwners() map[core.HEntity]pointOwner {
	owners := make(map[core.HEntity]pointOwner)
	for j := range d.sk.Entities {
		e := &d.sk.Entities[j]
		switch e.Type {
		case core.EntityLineSegment, core.EntityCircle, core.EntityArcOfCircle,
			core.EntityCubic, core.EntityCubicPeriodic, core.EntityWorkplane:
		default:
			continue
		}
		for i, h := range e.Point {
			// The first owner in the order of the entities names
			// the point: an extrusion's copy of a circle's centre
			// is also an end of the edge drawn through it.
			if _, named := owners[h]; h.V != 0 && !named {
				owners[h] = pointOwner{e, i}
			}
		}
	}
	return owners
}

// role names what the point is to its owner.
func (o pointOwner) role() string {
	c, i := o.e, o.i
	switch {
	case c == nil:
		return "point"
	case c.Type == core.EntityLineSegment && i < 2:
		return [...]string{"start", "end"}[i]
	case c.Type == core.EntityWorkplane && i == 0:
		return "origin"
	case c.Type == core.EntityCircle && i == 0:
		return "centre"
	case c.Type == core.EntityArcOfCircle && i < 3:
		return [...]string{"centre", "start", "end"}[i]
	case c.Type == core.EntityCubic && i == 0:
		return "start"
	case c.Type == core.EntityCubic && i == 3+c.ExtraPoints:
		return "end"
	case c.Type == core.EntityCubic && (i == 1 || i == 2+c.ExtraPoints):
		// The two points that set the end tangents; the others
		// lie on the curve.
		return "control point"
	}
	return "point"
}
