package api

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/sketch"
)

// This file is Describe: the drawing in words, for a reader that
// can't see it, such as a program that writes scripts.

// unitNames are the units as a .spcs file names them.
var unitNames = map[core.Unit]string{
	core.UnitMM:         "mm",
	core.UnitInches:     "inch",
	core.UnitMeters:     "m",
	core.UnitFeetInches: "ft-in",
}

// shortNum is a number with at most three decimals, and no "-0".
func shortNum(v float64) string {
	v = math.Round(v*1000) / 1000
	if v == 0 {
		v = 0
	}
	return strconv.FormatFloat(v, 'g', -1, 64)
}

// shortVec is a point as "(x, y, z)".
func shortVec(p Vec) string {
	return "(" + shortNum(p.X) + ", " + shortNum(p.Y) + ", " + shortNum(p.Z) + ")"
}

// isDrawing reports whether things are drawn in a group of the type,
// rather than made from another group.
func isDrawing(t sketch.GroupType) bool {
	return t == sketch.GroupDrawing3D || t == sketch.GroupDrawingWorkplane
}

// groupState says how a group came out of the last regeneration.
func groupState(sk *sketch.Sketch, group *sketch.Group) string {
	if why := sk.SuppressedReason(group.H); why != "" {
		return "left out (" + why + ")"
	}
	parts := []string{"solved"}
	if !group.IsSolvedOkay() {
		parts[0] = "NOT SOLVED"
	}
	parts = append(parts, fmt.Sprintf("%d DOF", group.Solved.Dof))
	if group.Problem.Msg != "" {
		parts = append(parts, "problem: "+group.Problem.Msg)
	}
	if !isDrawing(group.Type) {
		if m := displayMesh(sk, group); !m.IsEmpty() {
			hi, lo := m.GetBounding()
			parts = append(parts, "volume "+shortNum(m.CalculateVolume())+" mm³",
				"bounds "+shortVec(lo)+" to "+shortVec(hi))
		}
	}
	return strings.Join(parts, ", ")
}

// requestGeometry says where what a request drew is.
func requestGeometry(sk *sketch.Sketch, req *sketch.Request) string {
	ent := sk.GetEntity(req.H.Entity(0))
	if ent == nil {
		return "not generated"
	}
	at := func(h core.HEntity) string {
		if p := sk.GetEntity(h); p != nil && p.IsPoint() {
			return shortVec(p.PointGetNum(sk))
		}
		return "(?)"
	}
	var s string
	switch {
	case ent.IsPoint():
		s = "point " + shortVec(ent.PointGetNum(sk))
	case ent.Type == core.EntityLineSegment:
		s = "line " + at(ent.Point[0]) + " to " + at(ent.Point[1])
	case ent.Type == core.EntityCircle:
		s = "circle at " + at(ent.Point[0]) + ", radius " + shortNum(ent.CircleGetRadiusNum(sk))
	case ent.Type == core.EntityArcOfCircle:
		s = "arc at " + at(ent.Point[0]) + ", radius " + shortNum(ent.CircleGetRadiusNum(sk)) +
			", from " + at(ent.Point[1]) + " to " + at(ent.Point[2])
	default:
		var points []string
		for _, hp := range ent.Point {
			if hp.V != 0 {
				points = append(points, at(hp))
			}
		}
		s = kindOf(ent)
		if len(points) > 0 {
			s += " " + strings.Join(points, " ")
		}
	}
	if req.Str != "" {
		s += fmt.Sprintf(" %q", req.Str)
	}
	if req.Construction {
		s += ", construction"
	}
	return s
}

// constraintDescription is a constraint as the property browser names
// it, with its value if it has one.
func constraintDescription(c *core.Constraint) string {
	switch {
	case c.Type == core.ConstraintComment:
		return fmt.Sprintf("%s %q", c.DescriptionString(), c.Comment)
	case c.HasLabel():
		return fmt.Sprintf("%s = %g", c.DescriptionString(), shownValue(c))
	}
	return c.DescriptionString()
}

// constraintOn names the requests and groups whose entities a
// constraint is on.
func constraintOn(sk *sketch.Sketch, c *core.Constraint) string {
	var owners []string
	for _, h := range []core.HEntity{c.PtA, c.PtB, c.EntityA, c.EntityB, c.EntityC, c.EntityD} {
		if h.V == 0 {
			continue
		}
		owner := ownerOf(sk, h)
		known := false
		for _, o := range owners {
			known = known || o == owner
		}
		if !known {
			owners = append(owners, owner)
		}
	}
	return strings.Join(owners, ", ")
}

// describeGroup lists a group's requests and constraints.
func describeGroup(sk *sketch.Sketch, group *sketch.Group) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s, %s: %s\n", groupDescription(group), kindName(group.Type), groupState(sk, group))
	first := true
	for i := range sk.Requests {
		req := &sk.Requests[i]
		if req.Group != group.H {
			continue
		}
		if first {
			b.WriteString("  requests:\n")
			first = false
		}
		fmt.Fprintf(&b, "    %s: %s\n", req.DescriptionString(), requestGeometry(sk, req))
	}
	first = true
	for i := range sk.Constraints {
		c := &sk.Constraints[i]
		if c.Group != group.H {
			continue
		}
		if first {
			b.WriteString("  constraints:\n")
			first = false
		}
		line := constraintDescription(c)
		if c.Expr != "" {
			line += " (" + c.Expr + ")"
		}
		if c.Reference {
			line += ", reference"
		}
		if on := constraintOn(sk, c); on != "" {
			line += ", on " + on
		}
		fmt.Fprintf(&b, "    %s\n", line)
	}
	return b.String()
}

// Describe lists what the group holds, as text: how it solved, each
// request with where it is, in mm, and each constraint with its value
// and what it is on. The names it gives are the ones Doc.Request and
// Doc.Constraint take.
func (g Group) Describe() (s string) {
	g.d.read(func(sk *sketch.Sketch) { s = describeGroup(sk, g.get(sk)) })
	return s
}

// Describe says what the drawing is, as text, for a reader that can't
// see it: the document's unit, the parameters, every group in order
// with how it solved, its degrees of freedom, what is wrong with it,
// and the volume and bounds of the model up to it, then the active
// group's requests and constraints as Group.Describe lists them, and
// what is selected. Lengths are in mm. The names it gives are the
// ones Doc.Group, Doc.Request and Doc.Constraint take.
func (d *Doc) Describe() (s string) {
	d.read(func(sk *sketch.Sketch) {
		var b strings.Builder
		fmt.Fprintf(&b, "sketch: unit %s", unitNames[sk.Units])
		if sk.Unsaved {
			b.WriteString(", unsaved changes")
		}
		b.WriteString("\n")

		if len(sk.Parameters) > 0 {
			b.WriteString("parameters:\n")
			for _, p := range sk.Parameters {
				fmt.Fprintf(&b, "    %s = %s", p.Name, p.Expr)
				if shortNum(p.Value) != strings.TrimSpace(p.Expr) {
					fmt.Fprintf(&b, " = %s", shortNum(p.Value))
				}
				if p.Err != "" {
					fmt.Fprintf(&b, " (error: %s)", p.Err)
				}
				b.WriteString("\n")
			}
		}
		for _, e := range sk.ExprErrors {
			fmt.Fprintf(&b, "expression error: %s\n", e.Error())
		}

		active := sk.ActiveGroup()
		b.WriteString("groups, in the order the model is built:\n")
		for _, hg := range sk.GroupOrder {
			group := sk.GetGroup(hg)
			if hg.V == core.HGroupReferences || group == nil {
				continue
			}
			fmt.Fprintf(&b, "    %s, %s: %s", groupDescription(group), kindName(group.Type), groupState(sk, group))
			if hg == active {
				b.WriteString("; active")
			}
			b.WriteString("\n")
		}
		if group := sk.GetGroup(active); group != nil {
			b.WriteString("active group " + describeGroup(sk, group))
		}

		first := true
		for _, sel := range d.h.Selection() {
			var what string
			switch {
			case sel.Entity.V != 0:
				what = Entity{d, sel.Entity}.describe(sk)
			case sel.Constraint.V != 0:
				if c := sk.GetConstraint(sel.Constraint); c != nil {
					what = constraintDescription(c)
				}
			}
			if what == "" {
				continue
			}
			if first {
				b.WriteString("selected:\n")
				first = false
			}
			fmt.Fprintf(&b, "    %s\n", what)
		}
		s = b.String()
	})
	return s
}
