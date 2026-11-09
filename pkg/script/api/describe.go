package api

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// This file is Describe: the drawing in words, for a reader that
// can't see it, such as a program that writes scripts. The same walk
// fills the Description that a program reads as data (description.go).

// unitNames are the units as a .tlspr file names them.
var unitNames = map[core.Unit]string{
	core.UnitMM:         "mm",
	core.UnitInches:     "inch",
	core.UnitMeters:     "m",
	core.UnitFeetInches: "ft-in",
}

// shortNum is a number with at most three decimals, and no "-0".
func shortNum(v float64) string {
	return strconv.FormatFloat(round3(v), 'g', -1, 64)
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

// groupState says how a group came out of the last regeneration, in
// words and as data.
func groupState(sk *sketch.Sketch, group *sketch.Group) (string, GroupInfo) {
	info := GroupInfo{Name: groupDescription(group), Kind: kindName(group.Type), DOF: -1}
	if why := sk.SuppressedReason(group.H); why != "" {
		info.LeftOut = why
		return "left out (" + why + ")", info
	}
	parts := []string{"solved"}
	info.Solved = group.IsSolvedOkay()
	if !info.Solved {
		parts[0] = "NOT SOLVED"
		if why := sk.RedundancyText(group.H); why != "" {
			info.Redundant = why
			parts[0] += " (" + why + ")"
		}
	}
	if group.Solved.Dof < 0 {
		parts = append(parts, "DOF not counted")
	} else {
		info.DOF = group.Solved.Dof
		parts = append(parts, fmt.Sprintf("%d DOF", group.Solved.Dof))
	}
	if msg := problemOf(sk, group); msg != "" {
		info.Problem = msg
		parts = append(parts, "problem: "+msg)
	}
	if !isDrawing(group.Type) {
		if m := displayMesh(sk, group); !m.IsEmpty() {
			hi, lo := m.Bounding()
			vol := m.CalculateVolume()
			info.Volume = new(round3(vol))
			info.Bounds = &Bounds{Lo: vec3(lo), Hi: vec3(hi)}
			parts = append(parts, "volume "+shortNum(vol)+" mm³",
				"bounds "+shortVec(lo)+" to "+shortVec(hi))
		}
	}
	return strings.Join(parts, ", "), info
}

// requestGeometry says where what a request drew is, in words and as
// data.
func requestGeometry(sk *sketch.Sketch, req *sketch.Request) (string, RequestInfo) {
	info := RequestInfo{Name: req.DescriptionString(), Construction: req.Construction, Text: req.Str}
	ent := sk.Entity(req.H.Entity(0))
	if ent == nil {
		return "not generated", info
	}
	info.Kind = kindOf(ent)
	// at is a point in words, and adds it to the data; what isn't a
	// point is "(?)" in words and left out of the data.
	at := func(h core.HEntity) string {
		if p := sk.Entity(h); p != nil && p.IsPoint() {
			v := p.PointGetNum(sk)
			info.Points = append(info.Points, vec3(v))
			return shortVec(v)
		}
		return "(?)"
	}
	var s string
	switch {
	case ent.IsPoint():
		s = "point " + at(ent.H)
	case ent.Type == core.EntityLineSegment:
		s = "line " + at(ent.Point[0])
		s += " to " + at(ent.Point[1])
	case ent.Type == core.EntityCircle:
		info.Radius = round3(ent.CircleGetRadiusNum(sk))
		s = "circle at " + at(ent.Point[0]) + ", radius " + shortNum(info.Radius)
	case ent.Type == core.EntityArcOfCircle:
		info.Radius = round3(ent.CircleGetRadiusNum(sk))
		s = "arc at " + at(ent.Point[0]) + ", radius " + shortNum(info.Radius)
		s += ", from " + at(ent.Point[1])
		s += " to " + at(ent.Point[2])
	default:
		var points []string
		for _, hp := range ent.Point {
			if hp.V != 0 {
				points = append(points, at(hp))
			}
		}
		s = info.Kind
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
	return s, info
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

// constraintInfo is a constraint as data.
func constraintInfo(sk *sketch.Sketch, c *core.Constraint) ConstraintInfo {
	name := c.DescriptionString()
	_, typ, _ := strings.Cut(name, "-")
	info := ConstraintInfo{Name: name, Type: typ, Expr: c.Expr, Reference: c.Reference, On: constraintOn(sk, c)}
	switch {
	case c.Type == core.ConstraintComment:
		info.Comment = c.Comment
	case c.HasLabel():
		info.Value = new(shownValue(c))
	}
	return info
}

// constraintOn names the requests and groups whose entities a
// constraint is on.
func constraintOn(sk *sketch.Sketch, c *core.Constraint) []string {
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
	return owners
}

// describeGroup lists a group's requests and constraints, and puts
// them in data; it returns the group's state as data too.
func describeGroup(sk *sketch.Sketch, group *sketch.Group, data *Description) (string, GroupInfo) {
	var b strings.Builder
	state, info := groupState(sk, group)
	data.Group = info.Name
	fmt.Fprintf(&b, "%s, %s: %s\n", groupDescription(group), kindName(group.Type), state)
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
		where, r := requestGeometry(sk, req)
		data.Requests = append(data.Requests, r)
		fmt.Fprintf(&b, "    %s: %s\n", req.DescriptionString(), where)
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
		ci := constraintInfo(sk, c)
		data.Constraints = append(data.Constraints, ci)
		if len(ci.On) > 0 {
			line += ", on " + strings.Join(ci.On, ", ")
		}
		fmt.Fprintf(&b, "    %s\n", line)
	}
	return b.String(), info
}

// Describe lists what the group holds, as text: how it solved, each
// request with where it is, in mm, and each constraint with its value
// and what it is on. The names it gives are the ones Doc.Request and
// Doc.Constraint take. Group.Description is the same as data.
func (g Group) Describe() (s string) {
	g.d.read(func(sk *sketch.Sketch) { s, _ = describeGroup(sk, g.get(sk), &Description{}) })
	return s
}

// Describe says what the drawing is, as text, for a reader that can't
// see it: the document's unit, the parameters, every group in order
// with how it solved, its degrees of freedom, what is wrong with it,
// and the volume and bounds of the model up to it, then the active
// group's requests and constraints as Group.Describe lists them, and
// what is selected. Lengths are in mm. The names it gives are the
// ones Doc.Group, Doc.Request and Doc.Constraint take. Doc.Description
// is the same as data.
func (d *Doc) Describe() (s string) {
	d.read(func(sk *sketch.Sketch) { s, _ = describeDoc(d, sk) })
	return s
}

// describeDoc is Describe, in words and as data.
func describeDoc(d *Doc, sk *sketch.Sketch) (string, Description) {
	var data Description
	var b strings.Builder
	data.Unit, data.Unsaved = unitNames[sk.Units], sk.Unsaved
	fmt.Fprintf(&b, "sketch: unit %s", unitNames[sk.Units])
	if sk.Unsaved {
		b.WriteString(", unsaved changes")
	}
	b.WriteString("\n")

	if len(sk.Parameters) > 0 {
		b.WriteString("parameters:\n")
		for _, p := range sk.Parameters {
			data.Params = append(data.Params, ParamInfo{Name: p.Name, Expr: p.Expr, Value: p.Value, Error: p.Err})
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
		data.ExprErrors = append(data.ExprErrors, e.Error())
		fmt.Fprintf(&b, "expression error: %s\n", e.Error())
	}

	active := sk.ActiveGroup()
	b.WriteString("groups, in the order the model is built:\n")
	for _, hg := range sk.GroupOrder {
		group := sk.Group(hg)
		if hg.V == core.HGroupReferences || group == nil {
			continue
		}
		state, info := groupState(sk, group)
		info.Active = hg == active
		data.Groups = append(data.Groups, info)
		fmt.Fprintf(&b, "    %s, %s: %s", groupDescription(group), kindName(group.Type), state)
		if hg == active {
			b.WriteString("; active")
		}
		b.WriteString("\n")
	}
	if group := sk.Group(active); group != nil {
		text, _ := describeGroup(sk, group, &data)
		b.WriteString("active group " + text)
	}

	first := true
	for _, sel := range d.h.Selection() {
		var what string
		switch {
		case sel.Entity.V != 0:
			what = Entity{d, sel.Entity}.describe(sk)
		case sel.Constraint.V != 0:
			if c := sk.Constraint(sel.Constraint); c != nil {
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
		data.Selected = append(data.Selected, what)
		fmt.Fprintf(&b, "    %s\n", what)
	}
	// What the user points at, which "this" or "here" may mean.
	hv := d.h.Hovered()
	var over string
	switch {
	case hv.Entity.V != 0 && sk.Entity(hv.Entity) != nil:
		over = Entity{d, hv.Entity}.describe(sk)
	case hv.Constraint.V != 0:
		if c := sk.Constraint(hv.Constraint); c != nil {
			over = constraintDescription(c)
		}
	case hv.Edge.A != 0:
		over = "the solid's " + hv.Edge.String()
	case hv.Face != 0:
		over = "the solid's face " + sk.FaceName(hv.Face)
	}
	if over != "" {
		data.Hover = over
		fmt.Fprintf(&b, "under the mouse:\n    %s\n", over)
	}
	return b.String(), data
}
