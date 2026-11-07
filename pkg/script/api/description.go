package api

import (
	"math"

	"github.com/telesapr/telesapr/pkg/sketch"
)

// This file is Description: what Describe and DescribeSolid say, as
// data a program reads without parsing the text. The text and the data
// come from one walk of the drawing (describe.go, describe_solid.go),
// so that they say the same; a length is rounded to the thousandth of
// a mm the text shows.

// Description is the drawing, a group of it or the solid at a group,
// as Doc.Describe, Group.Describe and Group.DescribeSolid say it in
// words. What a description doesn't hold is left empty, and out of its
// JSON.
type Description struct {
	// Unit is the document's unit, as a .tlspr file names it: "mm",
	// "inch", "m" or "ft-in".
	Unit string `json:"unit,omitempty"`
	// Unsaved is set when the drawing has changes that aren't saved.
	Unsaved bool `json:"unsaved,omitempty"`
	// Params are the drawing's parameters, in order.
	Params []ParamInfo `json:"params,omitempty"`
	// ExprErrors are the expressions that could not be worked out.
	ExprErrors []string `json:"exprErrors,omitempty"`
	// Groups are the groups in the order the model is built; for
	// Group.Describe, that group alone.
	Groups []GroupInfo `json:"groups,omitempty"`
	// Group is the group whose Requests and Constraints are listed: the
	// active one for Doc.Describe.
	Group string `json:"group,omitempty"`
	// Requests and Constraints are that group's.
	Requests    []RequestInfo    `json:"requests,omitempty"`
	Constraints []ConstraintInfo `json:"constraints,omitempty"`
	// Selected is what the user has selected, each as the text says it.
	Selected []string `json:"selected,omitempty"`
	// Hover is what is under the mouse, which "this" or "here" may mean.
	Hover string `json:"hover,omitempty"`
	// Solid is the solid at the group DescribeSolid was asked of, and
	// Faces and Edges its faces and edges, at most 40 and 60 of them.
	Solid *SolidInfo `json:"solid,omitempty"`
	Faces []FaceInfo `json:"faces,omitempty"`
	Edges []EdgeInfo `json:"edges,omitempty"`
	// Script is the drawing as the Go program that builds it again,
	// when that was asked for instead (Doc.Script).
	Script string `json:"script,omitempty"`
}

// ParamInfo is a parameter of the drawing.
type ParamInfo struct {
	Name  string  `json:"name"`
	Expr  string  `json:"expr"`
	Value float64 `json:"value"`
	// Error is why the expression could not be worked out.
	Error string `json:"error,omitempty"`
}

// GroupInfo is how a group came out of the last regeneration.
type GroupInfo struct {
	// Name is the group as Doc.Group takes it, "g003-plate".
	Name string `json:"name"`
	// Kind is what the group is, as Group.Kind says it.
	Kind   string `json:"kind"`
	Active bool   `json:"active,omitempty"`
	// Solved is whether the group's sketch solved; DOF is the degrees
	// of freedom it has left, -1 when they were not counted.
	Solved bool `json:"solved"`
	DOF    int  `json:"dof"`
	// Problem is why the group made no solid, or that its Boolean
	// failed.
	Problem string `json:"problem,omitempty"`
	// LeftOut is why the group is left out of the model; such a group
	// is neither solved nor counted.
	LeftOut string `json:"leftOut,omitempty"`
	// Volume, in mm³, and Bounds are the model's up to the group, for a
	// group that is not drawn in and has a solid.
	Volume *float64 `json:"volume,omitempty"`
	Bounds *Bounds  `json:"bounds,omitempty"`
}

// Bounds are the corners of a box, in mm.
type Bounds struct {
	Lo [3]float64 `json:"lo"`
	Hi [3]float64 `json:"hi"`
}

// RequestInfo is a request, what was drawn, and where it is.
type RequestInfo struct {
	// Name is the request as Doc.Request takes it, "r004-line-segment".
	Name string `json:"name"`
	// Kind is what its entity is, as Entity.Kind says it: "point",
	// "line", "circle", "arc", ...; "" when it was not generated.
	Kind string `json:"kind,omitempty"`
	// Points are where it is: a point's, a line's two ends, a circle's
	// centre, an arc's centre, start and end; another's points in order.
	Points [][3]float64 `json:"points,omitempty"`
	// Radius is a circle's or an arc's.
	Radius       float64 `json:"radius,omitempty"`
	Construction bool    `json:"construction,omitempty"`
	// Text is a text request's string.
	Text string `json:"text,omitempty"`
}

// ConstraintInfo is a constraint of a group.
type ConstraintInfo struct {
	// Name is the constraint as Doc.Constraint takes it,
	// "c00a-pt-pt-distance", and Type the part after its number.
	Name string `json:"name"`
	Type string `json:"type"`
	// Value is a dimension's, as the drawing labels it, as
	// Constraint.Value has it.
	Value *float64 `json:"value,omitempty"`
	// Expr is the expression that drives the value.
	Expr      string `json:"expr,omitempty"`
	Reference bool   `json:"reference,omitempty"`
	// Comment is a comment's text.
	Comment string `json:"comment,omitempty"`
	// On are the requests and groups whose entities it is on.
	On []string `json:"on,omitempty"`
}

// SolidInfo is the solid DescribeSolid lists the faces and edges of.
type SolidInfo struct {
	// Group is the group asked of, and MadeBy the group that made the
	// solid there, when that is another.
	Group  string `json:"group"`
	MadeBy string `json:"madeBy,omitempty"`
	// Note says why no faces and edges are listed: there is no solid
	// yet, it is a triangle mesh, or its faces could not be found.
	Note string `json:"note,omitempty"`
	// Faces, Edges and Vertices are how many it has, all of them.
	Faces    int `json:"faces,omitempty"`
	Edges    int `json:"edges,omitempty"`
	Vertices int `json:"vertices,omitempty"`
	// Sound is whether its topology is valid; Problems are the first
	// five of ProblemCount problems if not.
	Sound        bool     `json:"sound"`
	ProblemCount int      `json:"problemCount,omitempty"`
	Problems     []string `json:"problems,omitempty"`
}

// FaceInfo is a face of a solid.
type FaceInfo struct {
	// ID is what Solid.Face takes, and Name the face as the window
	// names it; Piece is which of Pieces sheets with this ID it is,
	// when there are more than one.
	ID     uint32 `json:"id"`
	Name   string `json:"name"`
	Piece  int    `json:"piece,omitempty"`
	Pieces int    `json:"pieces,omitempty"`
	// Shape is "flat", "cylinder", "cone", "sphere" or "curved". A flat
	// face has its outward Normal; a cylinder its Axis through Centre
	// and its Radius; a cone its Axis from its apex, Centre, and
	// HalfAngle in degrees; a sphere its Centre and Radius.
	Shape     string      `json:"shape"`
	Normal    *[3]float64 `json:"normal,omitempty"`
	Axis      *[3]float64 `json:"axis,omitempty"`
	Centre    *[3]float64 `json:"centre,omitempty"`
	Radius    float64     `json:"radius,omitempty"`
	HalfAngle float64     `json:"halfAngle,omitempty"`
	// Point is a point of the face that Solid.FaceAt finds it by.
	Point *[3]float64 `json:"point,omitempty"`
	// Edges is how many named edges bound it.
	Edges int `json:"edges"`
}

// EdgeInfo is an edge of a solid.
type EdgeInfo struct {
	// Name is what Solid.Edge takes.
	Name string `json:"name"`
	// Shape is "straight", "arc", "circle" or "curve"; an arc and a
	// circle have their Centre and Radius.
	Shape  string      `json:"shape"`
	Centre *[3]float64 `json:"centre,omitempty"`
	Radius float64     `json:"radius,omitempty"`
	// Length is in mm, an arc's or a circle's as the circle has it.
	Length float64 `json:"length"`
	// Closed is set for an edge that closes on itself, which has no
	// Ends.
	Closed bool           `json:"closed,omitempty"`
	Ends   *[2][3]float64 `json:"ends,omitempty"`
	// Faces are the IDs of the faces it bounds.
	Faces []uint32 `json:"faces,omitempty"`
}

// Description is Describe's data: the drawing's unit, parameters and
// groups, the active group's requests and constraints, what is
// selected and what is under the mouse.
func (d *Doc) Description() (data Description) {
	d.read(func(sk *sketch.Sketch) { _, data = describeDoc(d, sk) })
	return data
}

// Description is Describe's data: how the group came out, and its
// requests and constraints.
func (g Group) Description() (data Description) {
	g.d.read(func(sk *sketch.Sketch) {
		var info GroupInfo
		_, info = describeGroup(sk, g.get(sk), &data)
		data.Groups = []GroupInfo{info}
	})
	return data
}

// SolidDescription is DescribeSolid's data: the solid at the group,
// and its faces and edges.
func (g Group) SolidDescription() (data Description) {
	g.d.read(func(sk *sketch.Sketch) { _, data = describeSolid(sk, g.get(sk)) })
	return data
}

// round3 is a number as shortNum writes it: to a thousandth, with no
// -0.
func round3(v float64) float64 {
	v = math.Round(v*1000) / 1000
	if v == 0 {
		v = 0
	}
	return v
}

// vec3 is a point as the data holds it, rounded as shortVec writes it.
func vec3(p Vec) [3]float64 { return [3]float64{round3(p.X), round3(p.Y), round3(p.Z)} }

// vec3p is vec3 to point to.
func vec3p(p Vec) *[3]float64 {
	v := vec3(p)
	return &v
}
