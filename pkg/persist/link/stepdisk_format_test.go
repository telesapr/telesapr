package link

import (
	"bytes"
	"encoding/gob"
	"errors"
	"math"
	"reflect"
	"slices"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/persist/step"
	"github.com/telesapr/telesapr/pkg/srf"
)

// stepDiskSample has every field of a result set, none at its zero.
func stepDiskSample() *step.Result {
	res := &step.Result{
		Vertices: []geom.Vector{{X: 1, Y: -2, Z: 3.25}, {X: 1e-300, Y: math.Copysign(0, -1), Z: 7}},
		Centers:  []geom.Vector{{X: 0.1, Y: 0.2, Z: 0.3}},
		Report: step.Report{
			Schema: "AP214", Unit: "inch", Shapes: 3, Placements: 5, Faces: 120, Skipped: 2,
			Triangles: 4000, ChordTol: 0.0125, Crossed: 1,
			Reasons: map[string]int{"unsupported OFFSET_CURVE_3D": 2, "ä bound": 1},
			Details: map[string]string{"unsupported OFFSET_CURVE_3D": "#31: an offset curve", "ä bound": "#7"},
		},
	}
	res.Mesh.FlipNormal, res.Mesh.KeepCoplanar, res.Mesh.IsTransparent = true, true, true
	for i := range 3 {
		f := float64(i)
		res.Mesh.Triangles = append(res.Mesh.Triangles, srf.STriangle{
			Tag:  i - 1,
			Meta: srf.NewSTriMeta(srf.FaceID(1<<31+i), core.RGBA{Red: 1, Green: 2, Blue: 3, Alpha: uint8(250 + i)}),
			A:    geom.Vector{X: f, Y: 1, Z: 2}, B: geom.Vector{X: 3, Y: f, Z: 5}, C: geom.Vector{X: 6, Y: 7, Z: -f},
			NA: geom.Vector{Z: 1}, NB: geom.Vector{Y: 1}, NC: geom.Vector{X: -1},
		})
	}
	return res
}

// A kept tessellation reads back as it was written, field for field.
func TestStepDiskFormatRoundTrip(t *testing.T) {
	t.Parallel()
	empty := &step.Result{Report: step.Report{Reasons: map[string]int{}, Details: map[string]string{}}}
	for _, res := range []*step.Result{stepDiskSample(), empty} {
		var buf bytes.Buffer
		if err := encodeStepDisk(&buf, res); err != nil {
			t.Fatal(err)
		}
		back, err := decodeStepDisk(&buf)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(res, back) {
			t.Errorf("read back\n%+v\nwritten\n%+v", back, res)
		}
	}
}

// What is not a whole tessellation of this version is refused, so the
// cache makes it again: one cut short at any byte, one with more after
// it, another version's, and a gob of the version before.
func TestStepDiskFormatRefuses(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := encodeStepDisk(&buf, stepDiskSample()); err != nil {
		t.Fatal(err)
	}
	whole := buf.Bytes()
	for n := range whole {
		if _, err := decodeStepDisk(bytes.NewReader(whole[:n])); err == nil {
			t.Fatalf("cut at %d of %d read", n, len(whole))
		}
	}
	if _, err := decodeStepDisk(bytes.NewReader(append(slices.Clone(whole), 0))); !errors.Is(err, errStepDisk) {
		t.Errorf("a byte more: %v", err)
	}
	other := slices.Clone(whole)
	other[7]++
	if _, err := decodeStepDisk(bytes.NewReader(other)); !errors.Is(err, errStepDisk) {
		t.Errorf("another version: %v", err)
	}
	var old bytes.Buffer
	if err := gob.NewEncoder(&old).Encode(stepDiskSample()); err != nil {
		t.Fatal(err)
	}
	if _, err := decodeStepDisk(&old); !errors.Is(err, errStepDisk) {
		t.Errorf("a gob of version 1: %v", err)
	}
	// A count past the bound is refused before anything is made for it.
	huge := append(slices.Clone(whole[:9]), 0xfe, 0xff, 0xff, 0xff, 0x0f)
	if _, err := decodeStepDisk(bytes.NewReader(huge)); !errors.Is(err, errStepDisk) {
		t.Errorf("a huge count: %v", err)
	}
}
