package agent

import (
	"strconv"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/script/api"
)

// This file is what Describe and Measure return as data beside their
// text, so that a program on the other end (an MCP client, the
// browser's page, a test) reads numbers and names instead of parsing
// prose. The data says what the text says, nothing more.

// Description is the drawing, a group of it, the solid at a group or
// the drawing's script, as data: api.Doc.Description,
// api.Group.Description, api.Group.SolidDescription, or the Script
// alone.
type Description = api.Description

// Described is what Describe returns: the text, and the same as data.
type Described struct {
	Text string      `json:"text"`
	Data Description `json:"data"`
}

// Measured is what Measure returns: the text, and the same as data.
type Measured struct {
	Text string      `json:"text"`
	Data Measurement `json:"data"`
}

// Measurement is what Measure says, as data. Its numbers are the ones
// the text shows, to six significant digits; what wasn't asked for is
// left out.
type Measurement struct {
	// Group is the group the model is measured up to; "" when the
	// selection is measured.
	Group string `json:"group,omitempty"`
	// Selection is what the user selected, measured, for Selected; the
	// rest is then zero.
	Selection *Selection `json:"selection,omitempty"`
	// Sound is whether the solid is closed, does not cross itself, its
	// Boolean worked and the group made its part (check.Report.Sound);
	// Problems are what is wrong if not, and Problem why the group made
	// no solid.
	Sound    bool     `json:"sound"`
	Problem  string   `json:"problem,omitempty"`
	Problems []string `json:"problems,omitempty"`
	// Faces are the faces of the shell, of Patches surfaces, and
	// Triangles the triangles of its mesh; 0 triangles means there is
	// no solid.
	Faces     int `json:"faces,omitempty"`
	Patches   int `json:"patches,omitempty"`
	Triangles int `json:"triangles"`
	// Volume is in mm³; CenterOfMass and Bounds in mm.
	Volume       float64    `json:"volume,omitempty"`
	CenterOfMass [3]float64 `json:"centerOfMass"`
	Bounds       api.Bounds `json:"bounds"`
	// Gaps are the gaps asked for, Walls the walls.
	Gaps  *GapReport `json:"gaps,omitempty"`
	Walls *WallInfo  `json:"walls,omitempty"`
}

// Selection is the selection measured, as Analyze > Measure Selection
// measures it.
type Selection struct {
	// Kind is what was measured: "length", "radius", "distance",
	// "point to line", "angle", ...
	Kind string `json:"kind"`
	// Value is in Unit, "mm" or "degrees".
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

// GapReport is the gaps less than Below mm between the parts, the
// nearest first, at most 20 of them and More more.
type GapReport struct {
	Below float64   `json:"below"`
	Gaps  []GapInfo `json:"gaps,omitempty"`
	More  int       `json:"more,omitempty"`
	// Tolerance is how far each gap may be from the true one, in mm.
	Tolerance float64 `json:"tolerance,omitempty"`
}

// GapInfo is a gap between two parts: 0 where they touch or overlap.
// A and B name the parts, and the solid of each where a part was taken
// apart; PointA and PointB are the nearest points of each.
type GapInfo struct {
	Distance float64    `json:"distance"`
	A        string     `json:"a"`
	B        string     `json:"b"`
	PointA   [3]float64 `json:"pointA"`
	PointB   [3]float64 `json:"pointB"`
}

// WallInfo is how thick the walls of the parts are.
type WallInfo struct {
	// Below is the thickness asked about, in mm. Measured is false when
	// no wall could be measured, and the rest is then left out.
	Below    float64 `json:"below"`
	Measured bool    `json:"measured"`
	// Thinnest is the thinnest wall, from At to To.
	Thinnest float64     `json:"thinnest,omitempty"`
	At       *[3]float64 `json:"at,omitempty"`
	To       *[3]float64 `json:"to,omitempty"`
	// Thin is how many of Samples points measured are thinner than
	// Below, and ThinSpots the thinnest of them, at most 20.
	Samples   int        `json:"samples,omitempty"`
	Thin      int        `json:"thin,omitempty"`
	ThinSpots []ThinSpot `json:"thinSpots,omitempty"`
	// Tolerance is how far each thickness may be from the true one, and
	// Step how far apart the points measured are at most, in mm.
	Tolerance float64 `json:"tolerance,omitempty"`
	Step      float64 `json:"step,omitempty"`
	// Unmeasured points found the mesh open or parts in each other;
	// NearCut says a thinner wall may be missed.
	Unmeasured int  `json:"unmeasured,omitempty"`
	NearCut    bool `json:"nearCut,omitempty"`
}

// ThinSpot is a point where a wall is thinner than asked about.
type ThinSpot struct {
	Thickness float64    `json:"thickness"`
	At        [3]float64 `json:"at"`
}

// g6 is a number as the text of a measure writes it, %.6g.
func g6(v float64) float64 {
	r, _ := strconv.ParseFloat(strconv.FormatFloat(v, 'g', 6, 64), 64)
	return r
}

// v6 is a point as the text of a measure writes it.
func v6(v geom.Vector) [3]float64 { return [3]float64{g6(v.X), g6(v.Y), g6(v.Z)} }

// checkMeasurement is a Check Solid report as data.
func checkMeasurement(group string, r *check.Report) Measurement {
	return Measurement{Group: group, Sound: r.Sound(), Problem: r.Problem, Problems: r.Faults(),
		Faces: r.Faces, Patches: r.Patches, Triangles: r.Triangles, Volume: g6(r.Volume),
		CenterOfMass: v6(r.CenterOfMass), Bounds: api.Bounds{Lo: v6(r.Min), Hi: v6(r.Max)}}
}

// g3 is a tolerance as the text of a measure writes it, %.3g.
func g3(v float64) float64 {
	r, _ := strconv.ParseFloat(strconv.FormatFloat(v, 'g', 3, 64), 64)
	return r
}
