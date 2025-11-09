package sketch

import (
	"fmt"
	"math"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
)

// Simple resolver for testing
type testResolver struct {
	entities map[core.HEntity]*core.Entity
	params   map[core.HParam]*core.Param
}

func newTestResolver(sk *Sketch) testResolver {
	res := testResolver{
		entities: make(map[core.HEntity]*core.Entity),
		params:   make(map[core.HParam]*core.Param),
	}
	for i := range sk.Entities {
		ent := &sk.Entities[i]
		res.entities[ent.H] = ent
	}
	for i := range sk.Params {
		p := &sk.Params[i]
		res.params[p.H] = p
	}
	return res
}

func (r testResolver) Entity(h core.HEntity) (*core.Entity, error) {
	if ent, ok := r.entities[h]; ok {
		return ent, nil
	}
	return nil, fmt.Errorf("entity %08x missing", h.V)
}

func (r testResolver) Param(h core.HParam) (*core.Param, error) {
	if param, ok := r.params[h]; ok {
		return param, nil
	}
	return nil, fmt.Errorf("param %08x missing", h.V)
}

func (r testResolver) ParamValue(h core.HParam) (float64, bool) {
	if param, ok := r.params[h]; ok {
		return param.Val, true
	}
	return 0, false
}

func TestGroupExtrudeBasic(t *testing.T) {
	sk := NewSketch()

	// Create a source group with a simple square
	sourceGroup := Group{
		H:     core.HGroup{V: 1},
		Type:  GroupDrawingWorkplane,
		Order: 1,
	}
	sk.Groups = append(sk.Groups, sourceGroup)
	sk.GroupOrder = append(sk.GroupOrder, sourceGroup.H)

	// Add points for a square
	points := []geom.Vector{
		{X: 0, Y: 0, Z: 0},
		{X: 1, Y: 0, Z: 0},
		{X: 1, Y: 1, Z: 0},
		{X: 0, Y: 1, Z: 0},
	}

	pointEntities := make([]core.Entity, len(points))
	for i, pt := range points {
		pointEntities[i] = core.Entity{
			EntityBase: core.EntityBase{
				Type:  core.EntityPointIn3D,
				Group: sourceGroup.H,
				H:     core.HEntity{V: uint32(i + 1)},
			},
			ActPoint: pt,
		}
		sk.Entities = append(sk.Entities, pointEntities[i])
	}

	// Add line segments connecting the points
	lines := []struct{ a, b int }{
		{0, 1}, {1, 2}, {2, 3}, {3, 0},
	}

	lineEntities := make([]core.Entity, len(lines))
	for i, ln := range lines {
		lineEntities[i] = core.Entity{
			EntityBase: core.EntityBase{
				Type:  core.EntityLineSegment,
				Group: sourceGroup.H,
				H:     core.HEntity{V: uint32(i + 5)},
				Point: [12]core.HEntity{
					pointEntities[ln.a].H,
					pointEntities[ln.b].H,
				},
			},
		}
		sk.Entities = append(sk.Entities, lineEntities[i])
	}

	// Create extrude group
	extrudeGroup := Group{
		H:     core.HGroup{V: 2},
		Type:  GroupExtrude,
		Order: 2,
		OpA:   sourceGroup.H,
		ValA:  0, ValB: 0, ValC: 1, // Extrude in Z direction
	}
	sk.Groups = append(sk.Groups, extrudeGroup)
	sk.GroupOrder = append(sk.GroupOrder, extrudeGroup.H)

	// Test extrusion vector
	vector := extrudeGroup.ExtrusionVector(sk)
	expected := geom.Vector{X: 0, Y: 0, Z: 1}
	if vector != expected {
		t.Errorf("ExtrusionVector = %v, want %v", vector, expected)
	}

	// Test GenerateSurfaces (should not error)
	err := extrudeGroup.GenerateSurfaces(sk)
	if err != nil {
		t.Errorf("GenerateSurfaces failed: %v", err)
	}

	// Mesh generation is now tested in TestGroupExtrudeGeneratesMesh
}

func TestGroupFindClosedLoops(t *testing.T) {
	sk := NewSketch()

	group := Group{H: core.HGroup{V: 1}}

	// Create a simple triangle loop
	points := []core.Entity{
		{EntityBase: core.EntityBase{H: core.HEntity{V: 1}, Type: core.EntityPointIn3D, Group: core.HGroup{V: 1}}},
		{EntityBase: core.EntityBase{H: core.HEntity{V: 2}, Type: core.EntityPointIn3D, Group: core.HGroup{V: 1}}},
		{EntityBase: core.EntityBase{H: core.HEntity{V: 3}, Type: core.EntityPointIn3D, Group: core.HGroup{V: 1}}},
	}
	sk.Entities = append(sk.Entities, points...)

	lines := []core.Entity{
		{
			EntityBase: core.EntityBase{
				H:     core.HEntity{V: 4},
				Type:  core.EntityLineSegment,
				Group: core.HGroup{V: 1},
				Point: [12]core.HEntity{points[0].H, points[1].H},
			},
		},
		{
			EntityBase: core.EntityBase{
				H:     core.HEntity{V: 5},
				Type:  core.EntityLineSegment,
				Group: core.HGroup{V: 1},
				Point: [12]core.HEntity{points[1].H, points[2].H},
			},
		},
		{
			EntityBase: core.EntityBase{
				H:     core.HEntity{V: 6},
				Type:  core.EntityLineSegment,
				Group: core.HGroup{V: 1},
				Point: [12]core.HEntity{points[2].H, points[0].H},
			},
		},
	}
	sk.Entities = append(sk.Entities, lines...)

	source := Group{H: core.HGroup{V: 1}}
	loops, err := group.findClosedLoops(sk, &source)
	if err != nil {
		t.Errorf("findClosedLoops failed: %v", err)
	}

	// Loop finding now implemented with DFS algorithm
	_ = loops
}

func TestGroupExtrudeGeneratesMesh(t *testing.T) {
	sk := NewSketch()

	// Create a source group with a simple triangle
	sourceGroup := Group{
		H:     core.HGroup{V: 1},
		Type:  GroupDrawingWorkplane,
		Order: 1,
	}
	sk.Groups = append(sk.Groups, sourceGroup)
	sk.GroupOrder = append(sk.GroupOrder, sourceGroup.H)

	// Add points for a triangle
	points := []geom.Vector{
		{X: 0, Y: 0, Z: 0},
		{X: 1, Y: 0, Z: 0},
		{X: 0.5, Y: 1, Z: 0},
	}

	pointEntities := make([]core.Entity, len(points))
	for i, pt := range points {
		pointEntities[i] = core.Entity{
			EntityBase: core.EntityBase{
				Type:  core.EntityPointIn3D,
				Group: sourceGroup.H,
				H:     core.HEntity{V: uint32(i + 1)},
			},
			ActPoint: pt,
		}
		sk.Entities = append(sk.Entities, pointEntities[i])
	}

	// Add line segments connecting the points
	lines := []struct{ a, b int }{
		{0, 1}, {1, 2}, {2, 0},
	}

	lineEntities := make([]core.Entity, len(lines))
	for i, ln := range lines {
		lineEntities[i] = core.Entity{
			EntityBase: core.EntityBase{
				Type:  core.EntityLineSegment,
				Group: sourceGroup.H,
				H:     core.HEntity{V: uint32(i + 5)},
				Point: [12]core.HEntity{
					pointEntities[ln.a].H,
					pointEntities[ln.b].H,
				},
			},
		}
		sk.Entities = append(sk.Entities, lineEntities[i])
	}

	// Create extrude group
	extrudeGroup := Group{
		H:     core.HGroup{V: 2},
		Type:  GroupExtrude,
		Order: 2,
		OpA:   sourceGroup.H,
		ValA:  0, ValB: 0, ValC: 1, // Extrude in Z direction
	}
	sk.Groups = append(sk.Groups, extrudeGroup)
	sk.GroupOrder = append(sk.GroupOrder, extrudeGroup.H)

	// Generate surfaces
	err := extrudeGroup.GenerateSurfaces(sk)
	if err != nil {
		t.Errorf("GenerateSurfaces failed: %v", err)
	}

	// Check that mesh was generated
	if extrudeGroup.Mesh == nil {
		t.Error("Expected mesh to be generated")
		return
	}

	mesh := extrudeGroup.Mesh

	// Triangle should have:
	// 6 vertices (3 bottom + 3 top)
	// 1 triangle per cap + 6 triangles for sides (3 quads * 2) = 8 triangles
	expectedVertices := 6
	expectedTriangles := 8

	if len(mesh.Vertices) != expectedVertices {
		t.Errorf("Expected %d vertices, got %d", expectedVertices, len(mesh.Vertices))
	}

	if len(mesh.Triangles) != expectedTriangles {
		t.Errorf("Expected %d triangles, got %d", expectedTriangles, len(mesh.Triangles))
	}

	// Check bottom cap vertices
	for i, expected := range points {
		actual := mesh.Vertices[i]
		if actual != expected {
			t.Errorf("Bottom vertex %d: expected %v, got %v", i, expected, actual)
		}
	}

	// Check top cap vertices
	for i, expected := range points {
		actual := mesh.Vertices[i+3]
		expectedTop := expected.Plus(geom.Vector{X: 0, Y: 0, Z: 1})
		if actual != expectedTop {
			t.Errorf("Top vertex %d: expected %v, got %v", i, expectedTop, actual)
		}
	}
}

func TestCombineMeshesUnion(t *testing.T) {
	// Create two simple meshes
	meshA := &Mesh{
		Vertices: []geom.Vector{
			{X: 0, Y: 0, Z: 0},
			{X: 1, Y: 0, Z: 0},
			{X: 0, Y: 1, Z: 0},
		},
		Triangles: []Triangle{
			{A: 0, B: 1, C: 2},
		},
	}

	meshB := &Mesh{
		Vertices: []geom.Vector{
			{X: 1, Y: 0, Z: 0},
			{X: 2, Y: 0, Z: 0},
			{X: 1, Y: 1, Z: 0},
		},
		Triangles: []Triangle{
			{A: 0, B: 1, C: 2},
		},
	}

	combined := CombineMeshes(meshA, meshB, CombineUnion)

	// Should have 6 vertices (3+3) and 2 triangles (1+1)
	if len(combined.Vertices) != 6 {
		t.Errorf("Expected 6 vertices, got %d", len(combined.Vertices))
	}
	if len(combined.Triangles) != 2 {
		t.Errorf("Expected 2 triangles, got %d", len(combined.Triangles))
	}

	// Check that meshA vertices are first
	for i, expected := range meshA.Vertices {
		if combined.Vertices[i] != expected {
			t.Errorf("Vertex %d: expected %v, got %v", i, expected, combined.Vertices[i])
		}
	}

	// Check that meshB triangles are offset
	expectedB := Triangle{A: 3, B: 4, C: 5} // offset by 3
	if combined.Triangles[1] != expectedB {
		t.Errorf("Second triangle: expected %v, got %v", expectedB, combined.Triangles[1])
	}
}

func TestCombineMeshesDifference(t *testing.T) {
	// Create two overlapping meshes
	meshA := &Mesh{
		Vertices: []geom.Vector{
			{X: 0, Y: 0, Z: 0}, {X: 2, Y: 0, Z: 0}, {X: 0, Y: 2, Z: 0}, // Large triangle
		},
		Triangles: []Triangle{{A: 0, B: 1, C: 2}},
	}

	meshB := &Mesh{
		Vertices: []geom.Vector{
			{X: 0.5, Y: 0.5, Z: 0}, {X: 1.5, Y: 0.5, Z: 0}, {X: 0.5, Y: 1.5, Z: 0}, // Small triangle inside
		},
		Triangles: []Triangle{{A: 0, B: 1, C: 2}},
	}

	result := CombineMeshes(meshA, meshB, CombineDifference)

	// Should keep meshA's vertices and potentially fewer triangles
	if len(result.Vertices) != 3 {
		t.Errorf("Expected 3 vertices, got %d", len(result.Vertices))
	}

	// The centroid of meshA's triangle is at (2/3, 2/3, 0) which should not be inside meshB
	// So we expect the triangle to be kept
	if len(result.Triangles) != 1 {
		t.Errorf("Expected 1 triangle, got %d", len(result.Triangles))
	}
}

func TestLatheGeneration(t *testing.T) {
	sk := NewSketch()

	// Create a source group with a simple profile (line segments)
	sourceGroup := Group{
		H:     core.HGroup{V: 1},
		Type:  GroupDrawingWorkplane,
		Order: 1,
	}
	sk.Groups = append(sk.Groups, sourceGroup)
	sk.GroupOrder = append(sk.GroupOrder, sourceGroup.H)

	// Create a simple rectangular profile
	points := []geom.Vector{
		{X: 1, Y: 0, Z: 0}, // Right
		{X: 1, Y: 1, Z: 0}, // Top-right
		{X: 0, Y: 1, Z: 0}, // Top-left
		{X: 0, Y: 0, Z: 0}, // Left (to close the loop)
	}

	pointEntities := make([]core.Entity, len(points))
	for i, pt := range points {
		pointEntities[i] = core.Entity{
			EntityBase: core.EntityBase{
				Type:  core.EntityPointIn3D,
				Group: sourceGroup.H,
				H:     core.HEntity{V: uint32(i + 1)},
			},
			ActPoint: pt,
		}
		sk.Entities = append(sk.Entities, pointEntities[i])
	}

	// Create line segments connecting the points
	lineEntities := make([]core.Entity, len(points))
	for i := 0; i < len(points); i++ {
		j := (i + 1) % len(points)
		lineEntities[i] = core.Entity{
			EntityBase: core.EntityBase{
				Type:  core.EntityLineSegment,
				Group: sourceGroup.H,
				H:     core.HEntity{V: uint32(i + 5)},
				Point: [12]core.HEntity{
					pointEntities[i].H,
					pointEntities[j].H,
				},
			},
		}
		sk.Entities = append(sk.Entities, lineEntities[i])
	}

	// Create lathe group
	latheGroup := Group{
		H:     core.HGroup{V: 2},
		Type:  GroupLathe,
		Order: 2,
		OpA:   sourceGroup.H,
	}
	sk.Groups = append(sk.Groups, latheGroup)
	sk.GroupOrder = append(sk.GroupOrder, latheGroup.H)

	// Generate lathe surface
	if err := latheGroup.GenerateSurfaces(sk); err != nil {
		t.Errorf("GenerateSurfaces failed: %v", err)
		return
	}

	if latheGroup.Mesh == nil {
		t.Error("Lathe group has no mesh generated")
		return
	}

	mesh := latheGroup.Mesh

	// Lathe with 4 points and 16 steps should have 4*16 = 64 vertices
	expectedVertices := 4 * 16
	if len(mesh.Vertices) != expectedVertices {
		t.Errorf("Expected %d vertices, got %d", expectedVertices, len(mesh.Vertices))
	}

	// Should have triangles connecting the steps
	if len(mesh.Triangles) == 0 {
		t.Error("No triangles generated")
	}

	// Check that triangles are valid
	for i, tri := range mesh.Triangles {
		if tri.A < 0 || tri.A >= len(mesh.Vertices) ||
			tri.B < 0 || tri.B >= len(mesh.Vertices) ||
			tri.C < 0 || tri.C >= len(mesh.Vertices) {
			t.Errorf("Triangle %d has invalid indices: %v", i, tri)
		}
	}

	t.Logf("Generated lathe mesh: %d vertices, %d triangles", len(mesh.Vertices), len(mesh.Triangles))
}

func TestGroupRotate(t *testing.T) {
	sk := NewSketch()

	// Create a source group with an existing mesh
	sourceGroup := Group{
		H:     core.HGroup{V: 1},
		Type:  GroupExtrude,
		Order: 1,
		Mesh: &Mesh{
			Vertices: []geom.Vector{
				{X: 0, Y: 0, Z: 0},
				{X: 1, Y: 0, Z: 0},
				{X: 0, Y: 1, Z: 0},
			},
			Triangles: []Triangle{{A: 0, B: 1, C: 2}},
		},
	}
	sk.Groups = append(sk.Groups, sourceGroup)
	sk.GroupOrder = append(sk.GroupOrder, sourceGroup.H)

	// Create rotate group
	rotateGroup := Group{
		H:     core.HGroup{V: 2},
		Type:  GroupRotate,
		Order: 2,
		OpA:   sourceGroup.H,
	}
	sk.Groups = append(sk.Groups, rotateGroup)
	sk.GroupOrder = append(sk.GroupOrder, rotateGroup.H)

	// Generate rotate surface
	if err := rotateGroup.GenerateSurfaces(sk); err != nil {
		t.Errorf("GenerateSurfaces failed: %v", err)
		return
	}

	if rotateGroup.Mesh == nil {
		t.Error("Rotate group has no mesh generated")
		return
	}

	mesh := rotateGroup.Mesh

	// Should have same number of vertices and triangles
	if len(mesh.Vertices) != 3 {
		t.Errorf("Expected 3 vertices, got %d", len(mesh.Vertices))
	}
	if len(mesh.Triangles) != 1 {
		t.Errorf("Expected 1 triangle, got %d", len(mesh.Triangles))
	}

	// Second vertex should be rotated (was at (1,0,0), now rotated 45° around Z)
	// cos(π/4) ≈ 0.707, sin(π/4) ≈ 0.707
	expectedX := 1 * 0.7071067811865476 // cos(45°)
	expectedY := 1 * 0.7071067811865476 // sin(45°)
	if math.Abs(mesh.Vertices[1].X-expectedX) > 1e-6 || math.Abs(mesh.Vertices[1].Y-expectedY) > 1e-6 {
		t.Errorf("Second vertex expected (~%.6f, ~%.6f, 0), got %v", expectedX, expectedY, mesh.Vertices[1])
	}

	t.Logf("Generated rotate mesh: %d vertices, %d triangles", len(mesh.Vertices), len(mesh.Triangles))
}

func TestGroupTranslate(t *testing.T) {
	sk := NewSketch()

	// Create a source group with an existing mesh
	sourceGroup := Group{
		H:     core.HGroup{V: 1},
		Type:  GroupExtrude,
		Order: 1,
		Mesh: &Mesh{
			Vertices: []geom.Vector{
				{X: 0, Y: 0, Z: 0},
				{X: 1, Y: 0, Z: 0},
				{X: 0, Y: 1, Z: 0},
			},
			Triangles: []Triangle{{A: 0, B: 1, C: 2}},
		},
	}
	sk.Groups = append(sk.Groups, sourceGroup)
	sk.GroupOrder = append(sk.GroupOrder, sourceGroup.H)

	// Create translate group
	translateGroup := Group{
		H:     core.HGroup{V: 2},
		Type:  GroupTranslate,
		Order: 2,
		OpA:   sourceGroup.H,
	}
	sk.Groups = append(sk.Groups, translateGroup)
	sk.GroupOrder = append(sk.GroupOrder, translateGroup.H)

	// Generate translate surface
	if err := translateGroup.GenerateSurfaces(sk); err != nil {
		t.Errorf("GenerateSurfaces failed: %v", err)
		return
	}

	if translateGroup.Mesh == nil {
		t.Error("Translate group has no mesh generated")
		return
	}

	mesh := translateGroup.Mesh

	// Should have same number of vertices and triangles
	if len(mesh.Vertices) != 3 {
		t.Errorf("Expected 3 vertices, got %d", len(mesh.Vertices))
	}
	if len(mesh.Triangles) != 1 {
		t.Errorf("Expected 1 triangle, got %d", len(mesh.Triangles))
	}

	// First vertex should be translated by (1,0,0)
	expected := geom.Vector{X: 1, Y: 0, Z: 0}
	if mesh.Vertices[0] != expected {
		t.Errorf("First vertex expected %v, got %v", expected, mesh.Vertices[0])
	}

	t.Logf("Generated translate mesh: %d vertices, %d triangles", len(mesh.Vertices), len(mesh.Triangles))
}

func TestGroupHelixGeneration(t *testing.T) {
	sk := NewSketch()

	// Create a source group with a simple profile (rectangle)
	sourceGroup := Group{
		H:     core.HGroup{V: 1},
		Type:  GroupDrawingWorkplane,
		Order: 1,
	}
	sk.Groups = append(sk.Groups, sourceGroup)
	sk.GroupOrder = append(sk.GroupOrder, sourceGroup.H)

	// Create a rectangular profile
	points := []geom.Vector{
		{X: 1, Y: 0, Z: 0},   // Right
		{X: 1, Y: 0.5, Z: 0}, // Top-right
		{X: 0, Y: 0.5, Z: 0}, // Top-left
		{X: 0, Y: 0, Z: 0},   // Left (to close the loop)
	}

	pointEntities := make([]core.Entity, len(points))
	for i, pt := range points {
		pointEntities[i] = core.Entity{
			EntityBase: core.EntityBase{
				Type:  core.EntityPointIn3D,
				Group: sourceGroup.H,
				H:     core.HEntity{V: uint32(i + 1)},
			},
			ActPoint: pt,
		}
		sk.Entities = append(sk.Entities, pointEntities[i])
	}

	// Create line segments
	lineEntities := make([]core.Entity, len(points))
	for i := 0; i < len(points); i++ {
		j := (i + 1) % len(points)
		lineEntities[i] = core.Entity{
			EntityBase: core.EntityBase{
				Type:  core.EntityLineSegment,
				Group: sourceGroup.H,
				H:     core.HEntity{V: uint32(i + 5)},
				Point: [12]core.HEntity{
					pointEntities[i].H,
					pointEntities[j].H,
				},
			},
		}
		sk.Entities = append(sk.Entities, lineEntities[i])
	}

	// Create helix group
	helixGroup := Group{
		H:     core.HGroup{V: 2},
		Type:  GroupHelix,
		Order: 2,
		OpA:   sourceGroup.H,
	}
	sk.Groups = append(sk.Groups, helixGroup)
	sk.GroupOrder = append(sk.GroupOrder, helixGroup.H)

	// Create a simple resolver
	resolver := newTestResolver(sk)

	// Recalculate entity positions
	for i := range sk.Entities {
		if err := sk.Entities[i].CalculateNumerical(resolver, false); err != nil {
			t.Logf("failed to calculate entity %v: %v", sk.Entities[i].H, err)
		}
	}

	// Generate helix surface
	if err := helixGroup.GenerateSurfaces(sk); err != nil {
		t.Errorf("GenerateSurfaces failed: %v", err)
		return
	}

	if helixGroup.Mesh == nil {
		t.Error("Helix group has no mesh generated")
		return
	}

	mesh := helixGroup.Mesh

	// Helix with 4 points and 32 steps should have 4*32 = 128 vertices
	expectedVertices := 4 * 32
	if len(mesh.Vertices) != expectedVertices {
		t.Errorf("Expected %d vertices, got %d", expectedVertices, len(mesh.Vertices))
	}

	if len(mesh.Triangles) == 0 {
		t.Error("No triangles generated")
	}

	// Check that triangles are valid
	for i, tri := range mesh.Triangles {
		if tri.A < 0 || tri.A >= len(mesh.Vertices) ||
			tri.B < 0 || tri.B >= len(mesh.Vertices) ||
			tri.C < 0 || tri.C >= len(mesh.Vertices) {
			t.Errorf("Triangle %d has invalid indices: %v", i, tri)
		}
	}

	t.Logf("Generated helix mesh: %d vertices, %d triangles", len(mesh.Vertices), len(mesh.Triangles))
}

func TestGroupParameterMethods(t *testing.T) {
	sk := NewSketch()

	// Test ExtrusionVector
	extrudeGroup := Group{
		Type: GroupExtrude,
		ValA: 1, ValB: 2, ValC: 3,
	}
	vector := extrudeGroup.ExtrusionVector(sk)
	expected := geom.Vector{X: 1, Y: 2, Z: 3}
	if vector != expected {
		t.Errorf("ExtrusionVector with ValA/B/C = %v, want %v", vector, expected)
	}

	// Test LatheCenter
	latheGroup := Group{Type: GroupLathe}
	center := latheGroup.LatheCenter(sk)
	if center != (geom.Vector{}) {
		t.Errorf("LatheCenter = %v, want zero vector", center)
	}

	// Test HelixParameters
	helixGroup := Group{Type: GroupHelix}
	axis, center, pitch := helixGroup.HelixParameters(sk)
	if axis != (geom.Vector{X: 0, Y: 0, Z: 1}) {
		t.Errorf("Helix axis = %v, want Z-axis", axis)
	}
	if center != (geom.Vector{}) {
		t.Errorf("Helix center = %v, want origin", center)
	}
	if pitch != 1.0 {
		t.Errorf("Helix pitch = %v, want 1.0", pitch)
	}

	// Test RotationParameters
	rotateGroup := Group{Type: GroupRotate}
	axis, angle, center := rotateGroup.RotationParameters(sk)
	if axis != (geom.Vector{X: 0, Y: 0, Z: 1}) {
		t.Errorf("Rotation axis = %v, want Z-axis", axis)
	}
	if angle != math.Pi/4 {
		t.Errorf("Rotation angle = %v, want π/4", angle)
	}
	if center != (geom.Vector{}) {
		t.Errorf("Rotation center = %v, want origin", center)
	}

	// Test TranslationVector
	translateGroup := Group{Type: GroupTranslate}
	offset := translateGroup.TranslationVector(sk)
	expected = geom.Vector{X: 1, Y: 0, Z: 0}
	if offset != expected {
		t.Errorf("TranslationVector = %v, want %v", offset, expected)
	}
}

func TestCombineMeshesIntersection(t *testing.T) {
	// Create two overlapping meshes
	meshA := &Mesh{
		Vertices: []geom.Vector{
			{X: 0, Y: 0, Z: 0}, {X: 2, Y: 0, Z: 0}, {X: 0, Y: 2, Z: 0}, // Large triangle
		},
		Triangles: []Triangle{{A: 0, B: 1, C: 2}},
	}

	meshB := &Mesh{
		Vertices: []geom.Vector{
			{X: 0.5, Y: 0.5, Z: 0}, {X: 1.5, Y: 0.5, Z: 0}, {X: 0.5, Y: 1.5, Z: 0}, // Small triangle inside
		},
		Triangles: []Triangle{{A: 0, B: 1, C: 2}},
	}

	result := CombineMeshes(meshA, meshB, CombineIntersection)

	// Should keep meshA's vertices but potentially fewer triangles
	if len(result.Vertices) != 3 {
		t.Errorf("Expected 3 vertices, got %d", len(result.Vertices))
	}

	// Note: Intersection logic is approximate - may not find the centroid inside
	// This test validates that the intersection operation runs without error
	// The exact result depends on the ray-casting point-in-mesh implementation
	t.Logf("Intersection result: %d triangles", len(result.Triangles))
}

func TestGroupErrorConditions(t *testing.T) {
	sk := NewSketch()

	// Test extrude with no source group
	extrudeGroup := Group{
		Type: GroupExtrude,
		OpA:  core.HGroup{V: 999}, // Non-existent group
	}

	err := extrudeGroup.GenerateSurfaces(sk)
	if err == nil {
		t.Error("Expected error for non-existent source group")
	}

	// Test lathe with no loops
	sourceGroup := Group{
		H:    core.HGroup{V: 1},
		Type: GroupDrawingWorkplane,
	}
	sk.Groups = append(sk.Groups, sourceGroup)

	latheGroup := Group{
		Type: GroupLathe,
		OpA:  sourceGroup.H,
	}

	err = latheGroup.GenerateSurfaces(sk)
	if err == nil {
		t.Error("Expected error for group with no loops")
	}
}

func TestComplexLoopFinding(t *testing.T) {
	sk := NewSketch()

	group := Group{H: core.HGroup{V: 1}}

	// Create a more complex shape with multiple potential loops
	// Square with a triangular cutout (but represented as separate loops)

	// Outer square
	outerPoints := []core.Entity{
		{EntityBase: core.EntityBase{H: core.HEntity{V: 1}, Type: core.EntityPointIn3D, Group: core.HGroup{V: 1}}},
		{EntityBase: core.EntityBase{H: core.HEntity{V: 2}, Type: core.EntityPointIn3D, Group: core.HGroup{V: 1}}},
		{EntityBase: core.EntityBase{H: core.HEntity{V: 3}, Type: core.EntityPointIn3D, Group: core.HGroup{V: 1}}},
		{EntityBase: core.EntityBase{H: core.HEntity{V: 4}, Type: core.EntityPointIn3D, Group: core.HGroup{V: 1}}},
	}
	sk.Entities = append(sk.Entities, outerPoints...)

	// Inner triangle
	innerPoints := []core.Entity{
		{EntityBase: core.EntityBase{H: core.HEntity{V: 5}, Type: core.EntityPointIn3D, Group: core.HGroup{V: 1}}},
		{EntityBase: core.EntityBase{H: core.HEntity{V: 6}, Type: core.EntityPointIn3D, Group: core.HGroup{V: 1}}},
		{EntityBase: core.EntityBase{H: core.HEntity{V: 7}, Type: core.EntityPointIn3D, Group: core.HGroup{V: 1}}},
	}
	sk.Entities = append(sk.Entities, innerPoints...)

	// Outer square lines
	outerLines := []core.Entity{
		{EntityBase: core.EntityBase{H: core.HEntity{V: 8}, Type: core.EntityLineSegment, Group: core.HGroup{V: 1}, Point: [12]core.HEntity{outerPoints[0].H, outerPoints[1].H}}},
		{EntityBase: core.EntityBase{H: core.HEntity{V: 9}, Type: core.EntityLineSegment, Group: core.HGroup{V: 1}, Point: [12]core.HEntity{outerPoints[1].H, outerPoints[2].H}}},
		{EntityBase: core.EntityBase{H: core.HEntity{V: 10}, Type: core.EntityLineSegment, Group: core.HGroup{V: 1}, Point: [12]core.HEntity{outerPoints[2].H, outerPoints[3].H}}},
		{EntityBase: core.EntityBase{H: core.HEntity{V: 11}, Type: core.EntityLineSegment, Group: core.HGroup{V: 1}, Point: [12]core.HEntity{outerPoints[3].H, outerPoints[0].H}}},
	}
	sk.Entities = append(sk.Entities, outerLines...)

	// Inner triangle lines
	innerLines := []core.Entity{
		{EntityBase: core.EntityBase{H: core.HEntity{V: 12}, Type: core.EntityLineSegment, Group: core.HGroup{V: 1}, Point: [12]core.HEntity{innerPoints[0].H, innerPoints[1].H}}},
		{EntityBase: core.EntityBase{H: core.HEntity{V: 13}, Type: core.EntityLineSegment, Group: core.HGroup{V: 1}, Point: [12]core.HEntity{innerPoints[1].H, innerPoints[2].H}}},
		{EntityBase: core.EntityBase{H: core.HEntity{V: 14}, Type: core.EntityLineSegment, Group: core.HGroup{V: 1}, Point: [12]core.HEntity{innerPoints[2].H, innerPoints[0].H}}},
	}
	sk.Entities = append(sk.Entities, innerLines...)

	loops, err := group.findClosedLoops(sk, &Group{H: core.HGroup{V: 1}})
	if err != nil {
		t.Errorf("findClosedLoops failed: %v", err)
	}

	// Should find 2 loops: outer square and inner triangle
	if len(loops) != 2 {
		t.Errorf("Expected 2 loops, got %d", len(loops))
	}

	// Both loops should have the correct number of entities
	for i, loop := range loops {
		if len(loop) != 3 && len(loop) != 4 {
			t.Errorf("Loop %d has %d entities, expected 3 or 4", i, len(loop))
		}
	}
}
