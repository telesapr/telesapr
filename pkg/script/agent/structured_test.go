package agent

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"
)

// describeText is a Describe's text, for the tests that read only that.
func describeText(d Described, err error) (string, error) { return d.Text, err }

// measureText is a Measure's text, for the tests that read only that.
func measureText(m Measured, err error) (string, error) { return m.Text, err }

// primerPlate is a session with the primer's plate, the example up to
// its extrusion, and its sketch active again.
func primerPlate(t *testing.T) *Session {
	t.Helper()
	s := newSession(t)
	code, _, ok := strings.Cut(primerExample, "\n    edges :=")
	if !ok {
		t.Fatal("the primer's example has no edges to round")
	}
	if res := run(t, s, code+"\ndoc.Activate(outline)"); res.Error != "" {
		t.Fatalf("the primer's plate fails: %s", res.Error)
	}
	return s
}

// TestDescriptionOfPlate reads the primer's plate as data: what the
// text says, as numbers and names.
func TestDescriptionOfPlate(t *testing.T) {
	t.Parallel()
	s := primerPlate(t)
	ctx := context.Background()

	d, err := s.Describe(ctx, Describing{})
	if err != nil {
		t.Fatal(err)
	}
	data := d.Data
	if data.Unit != "mm" || len(data.Groups) != 2 || data.Group != "g002-sketch-in-plane" ||
		len(data.Requests) != 7 || len(data.Constraints) != 20 {
		t.Fatalf("the plate is %d groups, %d requests, %d constraints of %q, in %q:\n%s",
			len(data.Groups), len(data.Requests), len(data.Constraints), data.Group, data.Unit, d.Text)
	}
	sk, plate := data.Groups[0], data.Groups[1]
	if !sk.Active || !sk.Solved || sk.DOF != 0 || sk.Volume != nil || plate.Name != "g003-plate" || plate.Kind != "extrude" || plate.Active {
		t.Errorf("the groups are %+v and %+v", sk, plate)
	}
	if plate.Volume == nil || math.Abs(*plate.Volume-4567.27) > 0.01 || plate.Bounds == nil ||
		plate.Bounds.Lo != [3]float64{0, 0, 0} || plate.Bounds.Hi != [3]float64{60, 20, 4} {
		t.Errorf("the plate's volume is %v and bounds %+v", plate.Volume, plate.Bounds)
	}
	circles := 0
	for _, r := range data.Requests {
		if r.Kind == "circle" {
			circles++
			if r.Radius != 2.5 || len(r.Points) != 1 || r.Points[0][1] != 10 {
				t.Errorf("a hole is %+v", r)
			}
		}
	}
	if circles != 3 || data.Requests[0].Kind != "line" || len(data.Requests[0].Points) != 2 {
		t.Errorf("the requests are %+v", data.Requests)
	}
	diameters := 0
	for _, c := range data.Constraints {
		if c.Type == "diameter" {
			diameters++
			if c.Value == nil || *c.Value != 5 || len(c.On) != 1 || !strings.HasSuffix(c.On[0], "-circle") {
				t.Errorf("a diameter is %+v", c)
			}
		}
	}
	if diameters != 3 {
		t.Errorf("%d diameters, want 3: %+v", diameters, data.Constraints)
	}

	// One group's description holds that group alone.
	if d, err = s.Describe(ctx, Describing{Group: "plate"}); err != nil || len(d.Data.Groups) != 1 ||
		d.Data.Groups[0].Name != "g003-plate" || d.Data.Unit != "" || len(d.Data.Requests) != 0 {
		t.Errorf("the plate group is %+v, %v", d.Data, err)
	}

	if d, err = s.Describe(ctx, Describing{Solid: true, Group: "plate"}); err != nil {
		t.Fatal(err)
	}
	data = d.Data
	if data.Solid == nil || !data.Solid.Sound || data.Solid.Faces != 9 || data.Solid.Edges != 18 ||
		len(data.Faces) != 9 || len(data.Edges) != 18 {
		t.Fatalf("the plate's solid is %+v with %d faces and %d edges listed:\n%s", data.Solid, len(data.Faces), len(data.Edges), d.Text)
	}
	shapes := map[string]int{}
	for _, f := range data.Faces {
		shapes[f.Shape]++
		if f.Point == nil || !strings.Contains(d.Text, f.Name) {
			t.Errorf("a face is %+v", f)
		}
	}
	if shapes["flat"] != 6 || shapes["cylinder"] != 3 {
		t.Errorf("the faces' shapes are %v", shapes)
	}
	for _, e := range data.Edges {
		if !strings.Contains(d.Text, `"`+e.Name+`": `) || len(e.Faces) != 2 || e.Closed == (e.Ends != nil) {
			t.Errorf("an edge is %+v", e)
		}
	}

	if d, err = s.Describe(ctx, Describing{Script: true}); err != nil || d.Data.Script != d.Text || !strings.Contains(d.Text, "package main") {
		t.Errorf("the script's data is %+v, %v", d.Data, err)
	}
}

// TestMeasurementOfPlate reads the primer's plate measured as data.
func TestMeasurementOfPlate(t *testing.T) {
	t.Parallel()
	s := primerPlate(t)
	ctx := context.Background()
	m, err := s.Measure(ctx, Measuring{Group: "plate", Gaps: 1, Walls: 5})
	if err != nil {
		t.Fatal(err)
	}
	data := m.Data
	if data.Group != "g003-plate" || !data.Sound || len(data.Problems) != 0 || data.Faces != 9 || data.Triangles == 0 ||
		math.Abs(data.Volume-4567.27) > 0.01 || data.Bounds.Hi != [3]float64{60, 20, 4} || data.CenterOfMass[2] != 2 {
		t.Errorf("the plate measures as %+v:\n%s", data, m.Text)
	}
	if data.Gaps == nil || data.Gaps.Below != 1 || len(data.Gaps.Gaps) != 0 {
		t.Errorf("the plate's gaps are %+v", data.Gaps)
	}
	w := data.Walls
	if w == nil || !w.Measured || w.Thinnest != 4 || w.Thin != len(w.ThinSpots) && len(w.ThinSpots) != mostListed || w.Samples == 0 {
		t.Errorf("the plate's walls are %+v:\n%s", w, m.Text)
	}
	// What isn't asked for is left out.
	if m, err = s.Measure(ctx, Measuring{}); err != nil || m.Data.Gaps != nil || m.Data.Walls != nil {
		t.Errorf("a plain measure is %+v, %v", m.Data, err)
	}
	raw, err := json.Marshal(m.Data)
	if err != nil || strings.Contains(string(raw), "gaps") || !strings.Contains(string(raw), `"sound":true`) {
		t.Errorf("a plain measure's JSON is %s, %v", raw, err)
	}
}
