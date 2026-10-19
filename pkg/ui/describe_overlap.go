package ui

import (
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
)

// describeOverlap lists, under the description of point e, the points
// at its position and the curves through it, as links that select one
// of them: a click on the drawing picks only one of what lies at a
// place, and coincident points are otherwise out of reach. Only what
// the drawing shows is listed, as only that could have been clicked.
func (d *describer) describeOverlap(e *core.Entity) {
	tw, sk := d.tw, d.sk
	o, err := sk.Overlap(e.H)
	if err != nil {
		return
	}
	var rows []*core.Entity
	for _, h := range o.Points {
		if pe := sk.Entity(h); pe != nil && sk.EntityIsVisible(pe) {
			rows = append(rows, pe)
		}
	}
	for _, h := range o.Curves {
		if ce := sk.Entity(h); ce != nil && sk.EntityIsVisible(ce) {
			rows = append(rows, ce)
		}
	}
	if len(rows) == 0 {
		return
	}
	tw.Printf(false, "%FtALSO HERE%E")
	for i, re := range rows {
		desc := d.ss.entityDescription(re)
		if re.IsPoint() {
			desc = d.pointRole(re) + " of " + desc
		} else {
			desc += ", through it"
		}
		tw.Printf(false, "%Bp   %Fl%Ll%D%f%h%s%E",
			parity(i), re.H.V, lf(tw.ScreenSelectEntity), lf(tw.ScreenHoverEntity), desc)
	}
}

// pointRole names what point p is to the entity it is a point of: the
// start or end of a line, the centre of a circle, the origin of a
// workplane. That entity is found by its points, as a group's own
// workplane and a copy in a later group come from no request.
func (d *describer) pointRole(p *core.Entity) string {
	var c *core.Entity
	i := -1
	for j := range d.sk.Entities {
		e := &d.sk.Entities[j]
		if e.Group != p.Group || e.IsPoint() {
			continue
		}
		if i = slices.Index(e.Point[:], p.H); i >= 0 {
			c = e
			break
		}
	}
	switch {
	case i < 0:
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
	case c.Type == core.EntityCubic:
		return "control point"
	}
	return "point"
}
