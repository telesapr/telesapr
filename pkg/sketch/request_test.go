package sketch

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
)

func TestRequestGenerateWorkplane(t *testing.T) {
	req := Request{
		H:            core.HRequest{V: 0x10},
		Type:         RequestWorkplane,
		Workplane:    core.FreeIn3D,
		Group:        core.HGroup{V: 2},
		Style:        core.HStyle{V: 3},
		Construction: true,
		Str:          "wp",
	}

	var entities core.EntityList
	var params core.ParamList

	req.Generate(nil, &entities, &params)

	if len(entities) != 3 {
		t.Fatalf("expected 3 entities, got %d", len(entities))
	}

	if len(params) != 7 {
		t.Fatalf("expected 7 params, got %d", len(params))
	}

	pt := entities[0]
	if pt.Type != core.EntityPointIn3D {
		t.Fatalf("expected first entity to be 3D point, got %v", pt.Type)
	}
	if pt.H != req.H.Entity(1) {
		t.Fatalf("unexpected point handle %v", pt.H)
	}

	n := entities[1]
	if n.Type != core.EntityNormalIn3D {
		t.Fatalf("expected normal entity, got %v", n.Type)
	}
	if n.Point[0] != pt.H {
		t.Fatalf("normal not anchored to point: %v", n.Point[0])
	}

	wp := entities[2]
	if wp.Type != core.EntityWorkplane {
		t.Fatalf("expected workplane entity, got %v", wp.Type)
	}
	if wp.Normal != n.H {
		t.Fatalf("workplane normal handle mismatch: %v", wp.Normal)
	}
	if wp.Point[0] != pt.H {
		t.Fatalf("workplane point mismatch: %v", wp.Point[0])
	}
	if wp.Construction != req.Construction {
		t.Fatalf("workplane construction flag mismatch")
	}
}

func TestRequestGenerateDatumPoint(t *testing.T) {
	req := Request{
		H:         core.HRequest{V: 0x21},
		Type:      RequestDatumPoint,
		Workplane: core.FreeIn3D,
		Group:     core.HGroup{V: 4},
		Style:     core.HStyle{V: 5},
	}

	var entities core.EntityList
	var params core.ParamList

	req.Generate(nil, &entities, &params)

	if len(entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(entities))
	}
	if len(params) != 3 {
		t.Fatalf("expected 3 params (xyz), got %d", len(params))
	}

	point := entities[0]
	if point.Type != core.EntityPointIn3D {
		t.Fatalf("expected datum point entity, got %v", point.Type)
	}
	if point.H != req.H.Entity(0) {
		t.Fatalf("unexpected datum point handle %v", point.H)
	}
}

func TestRequestGenerateCircle(t *testing.T) {
	req := Request{
		H:         core.HRequest{V: 0x33},
		Type:      RequestCircle,
		Workplane: core.FreeIn3D,
		Group:     core.HGroup{V: 6},
		Style:     core.HStyle{V: 7},
	}

	var entities core.EntityList
	var params core.ParamList

	req.Generate(nil, &entities, &params)

	if len(entities) != 4 {
		t.Fatalf("expected 4 entities, got %d", len(entities))
	}
	if len(params) != 8 {
		t.Fatalf("expected 8 params (3+4+1), got %d", len(params))
	}

	center := entities[0]
	if center.Type != core.EntityPointIn3D {
		t.Fatalf("expected circle center point, got %v", center.Type)
	}

	normal := entities[1]
	if normal.Type != core.EntityNormalIn3D {
		t.Fatalf("expected circle normal, got %v", normal.Type)
	}
	if normal.H != req.H.Entity(32) {
		t.Fatalf("unexpected normal handle %v", normal.H)
	}

	dist := entities[2]
	if dist.Type != core.EntityDistance {
		t.Fatalf("expected circle distance entity, got %v", dist.Type)
	}
	if dist.H != req.H.Entity(64) {
		t.Fatalf("unexpected distance handle %v", dist.H)
	}

	circle := entities[3]
	if circle.Type != core.EntityCircle {
		t.Fatalf("expected circle entity, got %v", circle.Type)
	}
	if circle.Point[0] != center.H {
		t.Fatalf("circle center mismatch: %v", circle.Point[0])
	}
	if circle.Normal != normal.H {
		t.Fatalf("circle normal mismatch: %v", circle.Normal)
	}
	if circle.Distance != dist.H {
		t.Fatalf("circle distance mismatch: %v", circle.Distance)
	}
}

func TestRequestGenerateArcOfCircle(t *testing.T) {
	req := Request{
		H:         core.HRequest{V: 0x44},
		Type:      RequestArcOfCircle,
		Workplane: core.FreeIn3D,
		Group:     core.HGroup{V: 8},
		Style:     core.HStyle{V: 9},
	}

	var entities core.EntityList
	var params core.ParamList

	req.Generate(nil, &entities, &params)

	if len(entities) != 5 {
		t.Fatalf("expected 5 entities (3 pts + normal + arc), got %d", len(entities))
	}
	if len(params) != 13 {
		t.Fatalf("expected 13 params, got %d", len(params))
	}

	for i := 0; i < 3; i++ {
		if entities[i].Type != core.EntityPointIn3D {
			t.Fatalf("entity %d expected 3D point, got %v", i, entities[i].Type)
		}
		if entities[i].H != req.H.Entity(i+1) {
			t.Fatalf("entity %d unexpected handle %v", i, entities[i].H)
		}
	}

	normal := entities[3]
	if normal.Type != core.EntityNormalIn3D {
		t.Fatalf("expected normal entity, got %v", normal.Type)
	}

	arc := entities[4]
	if arc.Type != core.EntityArcOfCircle {
		t.Fatalf("expected arc entity, got %v", arc.Type)
	}
	if arc.Normal != normal.H {
		t.Fatalf("arc normal mismatch: %v", arc.Normal)
	}
	for i := 0; i < 3; i++ {
		if arc.Point[i] != entities[i].H {
			t.Fatalf("arc point %d mismatch: %v", i, arc.Point[i])
		}
	}
}

func TestRequestGenerateCubicWithExtraPoints(t *testing.T) {
	req := Request{
		H:           core.HRequest{V: 0x55},
		Type:        RequestCubic,
		ExtraPoints: 2,
		Workplane:   core.HEntity{V: 0x200},
		Group:       core.HGroup{V: 10},
		Style:       core.HStyle{V: 11},
	}

	var entities core.EntityList
	var params core.ParamList

	req.Generate(nil, &entities, &params)

	if len(entities) != 7 {
		t.Fatalf("expected 7 entities (6 pts + cubic), got %d", len(entities))
	}
	if len(params) != 12 {
		t.Fatalf("expected 12 params (6 pts * 2), got %d", len(params))
	}

	for i := 0; i < 6; i++ {
		if entities[i].Type != core.EntityPointIn2D {
			t.Fatalf("point %d expected 2D type, got %v", i, entities[i].Type)
		}
		if entities[i].H != req.H.Entity(i+1) {
			t.Fatalf("point %d unexpected handle %v", i, entities[i].H)
		}
	}

	cubic := entities[6]
	if cubic.Type != core.EntityCubic {
		t.Fatalf("expected cubic entity, got %v", cubic.Type)
	}
	if cubic.ExtraPoints != req.ExtraPoints {
		t.Fatalf("expected extra points %d, got %d", req.ExtraPoints, cubic.ExtraPoints)
	}
}

func TestRequestGenerateImageDefaultsAspectRatio(t *testing.T) {
	req := Request{
		H:         core.HRequest{V: 0x66},
		Type:      RequestImage,
		Workplane: core.FreeIn3D,
		Group:     core.HGroup{V: 12},
		Style:     core.HStyle{V: 13},
	}

	var entities core.EntityList
	var params core.ParamList

	req.Generate(nil, &entities, &params)

	if len(entities) != 6 {
		t.Fatalf("expected 6 entities, got %d", len(entities))
	}

	image := entities[len(entities)-1]
	if image.Type != core.EntityImage {
		t.Fatalf("expected image entity, got %v", image.Type)
	}
	if image.AspectRatio != 1.0 {
		t.Fatalf("expected default aspect ratio of 1.0, got %f", image.AspectRatio)
	}
}

func TestRequestDescriptionString(t *testing.T) {
	ref := Request{H: HRequestReferenceXY, Type: RequestWorkplane}
	if desc := ref.DescriptionString(); desc != "r001-#XY" {
		t.Fatalf("unexpected description for reference: %s", desc)
	}

	req := Request{H: core.HRequest{V: 0xabc}, Type: RequestLineSegment}
	if desc := req.DescriptionString(); desc != "rabc-line-segment" {
		t.Fatalf("unexpected description: %s", desc)
	}

	unknown := Request{H: core.HRequest{V: 0x123}, Type: RequestType(9999)}
	if desc := unknown.DescriptionString(); desc != "r123-" {
		t.Fatalf("unexpected description for unknown type: %s", desc)
	}
}

func TestRequestIndexOfPoint(t *testing.T) {
	req := Request{H: core.HRequest{V: 0x77}, Type: RequestLineSegment}

	if got := req.IndexOfPoint(req.H.Entity(1)); got != 0 {
		t.Fatalf("expected first point index 0, got %d", got)
	}
	if got := req.IndexOfPoint(req.H.Entity(2)); got != 1 {
		t.Fatalf("expected second point index 1, got %d", got)
	}
	if got := req.IndexOfPoint(req.H.Entity(0)); got != -1 {
		t.Fatalf("expected no match for entity(0), got %d", got)
	}

	datum := Request{H: core.HRequest{V: 0x88}, Type: RequestDatumPoint}
	if got := datum.IndexOfPoint(datum.H.Entity(0)); got != 0 {
		t.Fatalf("expected datum point index 0, got %d", got)
	}
	if got := datum.IndexOfPoint(datum.H.Entity(1)); got != -1 {
		t.Fatalf("expected no extra point for datum, got %d", got)
	}
}

func TestRequestMappingHelpers(t *testing.T) {
	for _, mapping := range entReqMappings {
		extra := 0
		if mapping.useExtraPoints {
			extra = 2
		}

		entityType, points, hasNormal, hasDistance, found := getRequestInfo(mapping.requestType, extra)
		if !found {
			t.Fatalf("missing request info for %v", mapping.requestType)
		}
		if entityType != mapping.entityType {
			t.Fatalf("request %v expected entity %v, got %v", mapping.requestType, mapping.entityType, entityType)
		}
		expectPoints := mapping.points
		if mapping.useExtraPoints {
			expectPoints += extra
		}
		if points != expectPoints {
			t.Fatalf("request %v expected %d points, got %d", mapping.requestType, expectPoints, points)
		}
		if hasNormal != mapping.hasNormal {
			t.Fatalf("request %v normal mismatch", mapping.requestType)
		}
		if hasDistance != mapping.hasDistance {
			t.Fatalf("request %v distance mismatch", mapping.requestType)
		}

		reqType, _, _, _, ok := getEntityInfo(mapping.entityType, extra)
		if !ok {
			t.Fatalf("missing entity info for %v", mapping.entityType)
		}
		if reqType != mapping.requestType {
			t.Fatalf("entity %v expected request %v, got %v", mapping.entityType, mapping.requestType, reqType)
		}
	}
}
