package sketch

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// TestGroupGenerateExtrude checks the entities Group::Generate makes
// for a one-sided extrusion of a line: the remap numbering (which
// fixes the copies' handles) and the copies' positions.
func TestGroupGenerateExtrude(t *testing.T) {
	sk := NewSketch()
	srcH := sk.AddGroup(Group{Type: GroupDrawing3D, Scale: 1})
	extrH := sk.AddGroup(Group{Type: GroupExtrude, Subtype: GroupOneSided, OpA: srcH, Scale: 1})

	param := func(v uint32, val float64) core.HParam {
		h := core.HParam{V: v}
		sk.Params = append(sk.Params, core.Param{H: h, Val: val})
		return h
	}
	pa := core.HEntity{V: 0x10001}
	pb := core.HEntity{V: 0x10002}
	line := core.HEntity{V: 0x10000}
	sk.Entities = append(sk.Entities,
		core.Entity{H: pa, Type: core.EntityPointIn3D, Group: srcH,
			Param: [8]core.HParam{param(1, 0), param(2, 0), param(3, 0)}},
		core.Entity{H: pb, Type: core.EntityPointIn3D, Group: srcH,
			Param: [8]core.HParam{param(4, 10), param(5, 0), param(6, 0)}},
		core.Entity{H: line, Type: core.EntityLineSegment, Group: srcH,
			Point: [core.MaxPointsInEntity]core.HEntity{pa, pb}},
	)

	g := sk.Group(extrH)
	g.Generate(sk, &sk.Entities, &sk.Params)
	for _, k := range []int{0, 1, 2} {
		p := sk.Param(extrH.Param(k))
		if p == nil {
			t.Fatalf("extrusion param %d not added", k)
		}
		p.Val = []float64{0, 0, 5}[k]
	}

	// Remap ids are handed out in the order C++ asks for them: each
	// point's bottom, top and side line, then the line's bottom, top
	// and side face, then the top and bottom faces.
	want := []struct {
		key  core.EntityKey
		id   uint32
		typ  core.EntityType
		want geom.Vector
	}{
		{core.EntityKey{Input: pa, CopyNumber: RemapBottom}, 1, core.EntityPointNTrans, geom.Vector{}},
		{core.EntityKey{Input: pa, CopyNumber: RemapTop}, 2, core.EntityPointNTrans, geom.Vector{Z: 10}},
		{core.EntityKey{Input: pa, CopyNumber: RemapPtToLine}, 3, core.EntityLineSegment, geom.Vector{}},
		{core.EntityKey{Input: pb, CopyNumber: RemapBottom}, 4, core.EntityPointNTrans, geom.Vector{X: 10}},
		{core.EntityKey{Input: pb, CopyNumber: RemapTop}, 5, core.EntityPointNTrans, geom.Vector{X: 10, Z: 10}},
		{core.EntityKey{Input: pb, CopyNumber: RemapPtToLine}, 6, core.EntityLineSegment, geom.Vector{}},
		{core.EntityKey{Input: line, CopyNumber: RemapBottom}, 7, core.EntityLineSegment, geom.Vector{}},
		{core.EntityKey{Input: line, CopyNumber: RemapTop}, 8, core.EntityLineSegment, geom.Vector{}},
		{core.EntityKey{Input: line, CopyNumber: RemapLineToFace}, 9, core.EntityFaceXProd, geom.Vector{}},
		{core.EntityKey{Input: core.NoEntity, CopyNumber: RemapTop}, 10, core.EntityFaceNormalPt, geom.Vector{}},
		{core.EntityKey{Input: core.NoEntity, CopyNumber: RemapBottom}, 11, core.EntityFaceNormalPt, geom.Vector{}},
	}
	if len(g.Remap) != len(want) {
		t.Errorf("remap has %d entries, want %d", len(g.Remap), len(want))
	}
	for _, w := range want {
		if got := g.Remap[w.key]; got.V != w.id {
			t.Errorf("remap %v = %d, want %d", w.key, got.V, w.id)
			continue
		}
		e := sk.Entity(extrH.Entity(int(w.id)))
		if e == nil {
			t.Errorf("entity for remap %v missing", w.key)
			continue
		}
		if e.Type != w.typ {
			t.Errorf("entity for remap %v has type %d, want %d", w.key, e.Type, w.typ)
		}
		if e.IsPoint() {
			if got := e.PointGetNum(sk); !got.Equals(w.want) {
				t.Errorf("point for remap %v at %v, want %v", w.key, got, w.want)
			}
		}
	}

	// The side face of the line lies in the plane y = 0.
	face := sk.Entity(extrH.Entity(9))
	if n := face.FaceGetNormalNum(sk); !n.Equals(geom.Vector{Y: -1}) && !n.Equals(geom.Vector{Y: 1}) {
		t.Errorf("side face normal %v, want +-y", n)
	}
}

func TestCopyAsEnum(t *testing.T) {
	if CopyAsNumeric != 0 || CopyAsNTrans != 1 {
		t.Fatalf("CopyAs enum values don't match C++ convention")
	}
}
