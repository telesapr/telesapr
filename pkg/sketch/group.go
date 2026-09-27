package sketch

import (
	"fmt"
	"math"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/srf"
)

type GroupType int

const (
	GroupDrawing3D GroupType = 5000 + iota
	GroupDrawingWorkplane
)

const (
	GroupExtrude GroupType = 5100 + iota
	GroupLathe
	GroupRevolve
	GroupHelix
)

const (
	GroupRotate GroupType = 5200 + iota
	GroupTranslate
)

const (
	GroupLinked GroupType = 5300
)

type GroupSubtype int

const (
	GroupWorkplaneByPointOrtho GroupSubtype = 6000 + iota
	GroupWorkplaneByLineSegments
	GroupWorkplaneByPointNormal
)

const (
	GroupOneSided GroupSubtype = 7000 + iota
	GroupTwoSided
	_
	_
	GroupOneSkewed
	GroupTwoSkewed
)

type CombineAs int

const (
	CombineUnion CombineAs = iota
	CombineDifference
	CombineAssemble
	CombineIntersection
)

// Triangle is the legacy mesh-triangle format (vertex indices into a
// vertex array). Kept for backwards compatibility with the old
// CombineMeshes interface; new code should use srf.STriangle.
type Triangle struct {
	A, B, C int
}

// Mesh is the legacy mesh format (separate vertex array and index
// list). Kept for the existing CombineMeshes interface; new code
// should use srf.SMesh, which is what Group.RunningMesh / Group.ThisMesh
// store.
type Mesh struct {
	Vertices  []geom.Vector
	Triangles []Triangle
}

// CombineMeshes performs boolean operations on two meshes
func CombineMeshes(a, b *Mesh, op CombineAs) *Mesh {
	switch op {
	case CombineUnion:
		return combineMeshesUnion(a, b)
	case CombineDifference:
		return combineMeshesDifference(a, b)
	case CombineIntersection:
		return combineMeshesIntersection(a, b)
	case CombineAssemble:
		return combineMeshesAssemble(a, b)
	default:
		return a // fallback
	}
}

// combineMeshesUnion merges two meshes (simple union without cutting)
func combineMeshesUnion(a, b *Mesh) *Mesh {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}

	result := &Mesh{}
	result.Vertices = append(result.Vertices, a.Vertices...)
	result.Triangles = append(result.Triangles, a.Triangles...)
	offsetB := len(result.Vertices)
	result.Vertices = append(result.Vertices, b.Vertices...)
	for _, t := range b.Triangles {
		result.Triangles = append(result.Triangles, Triangle{
			A: t.A + offsetB,
			B: t.B + offsetB,
			C: t.C + offsetB,
		})
	}
	return result
}

// combineMeshesDifference performs boolean difference (A - B).
// Phase 5: legacy implementation; delegates to the centroid-based
// heuristic that uses srf.SMesh internally.
func combineMeshesDifference(a, b *Mesh) *Mesh {
	if a == nil {
		return &Mesh{}
	}
	if b == nil {
		return a
	}

	result := &Mesh{}
	result.Vertices = make([]geom.Vector, len(a.Vertices))
	copy(result.Vertices, a.Vertices)

	for _, tri := range a.Triangles {
		va := a.Vertices[tri.A]
		vb := a.Vertices[tri.B]
		vc := a.Vertices[tri.C]

		centroid := geom.Vector{
			X: (va.X + vb.X + vc.X) / 3,
			Y: (va.Y + vb.Y + vc.Y) / 3,
			Z: (va.Z + vb.Z + vc.Z) / 3,
		}
		if !pointInMesh(centroid, b) {
			result.Triangles = append(result.Triangles, tri)
		}
	}
	return result
}

// combineMeshesIntersection performs boolean intersection (A ∩ B).
func combineMeshesIntersection(a, b *Mesh) *Mesh {
	if a == nil || b == nil {
		return &Mesh{}
	}

	result := &Mesh{}
	result.Vertices = make([]geom.Vector, len(a.Vertices))
	copy(result.Vertices, a.Vertices)

	for _, tri := range a.Triangles {
		va := a.Vertices[tri.A]
		vb := a.Vertices[tri.B]
		vc := a.Vertices[tri.C]

		centroid := geom.Vector{
			X: (va.X + vb.X + vc.X) / 3,
			Y: (va.Y + vb.Y + vc.Y) / 3,
			Z: (va.Z + vb.Z + vc.Z) / 3,
		}
		if pointInMesh(centroid, b) {
			result.Triangles = append(result.Triangles, tri)
		}
	}
	return result
}

// pointInMesh performs a simple point-in-mesh test using ray casting
// This is a basic implementation and may not handle all cases correctly
func pointInMesh(point geom.Vector, mesh *Mesh) bool {
	if mesh == nil || len(mesh.Triangles) == 0 {
		return false
	}

	// Cast ray in +X direction and count intersections
	intersections := 0
	rayDir := geom.Vector{X: 1, Y: 0, Z: 0}

	for _, tri := range mesh.Triangles {
		va := mesh.Vertices[tri.A]
		vb := mesh.Vertices[tri.B]
		vc := mesh.Vertices[tri.C]

		if rayIntersectsTriangle(point, rayDir, va, vb, vc) {
			intersections++
		}
	}

	// Odd number of intersections means point is inside
	return intersections%2 == 1
}

// rayIntersectsTriangle checks if a ray intersects a triangle
func rayIntersectsTriangle(rayOrigin, rayDir, v0, v1, v2 geom.Vector) bool {
	// Möller–Trumbore intersection algorithm
	const EPS = 1e-6

	edge1 := v1.Minus(v0)
	edge2 := v2.Minus(v0)

	h := rayDir.Cross(edge2)
	a := edge1.Dot(h)

	if a > -EPS && a < EPS {
		return false // Ray is parallel to triangle
	}

	f := 1.0 / a
	s := rayOrigin.Minus(v0)
	u := f * s.Dot(h)

	if u < 0.0 || u > 1.0 {
		return false
	}

	q := s.Cross(edge1)
	v := f * rayDir.Dot(q)

	if v < 0.0 || u+v > 1.0 {
		return false
	}

	t := f * edge2.Dot(q)

	return t > EPS // Intersection in positive ray direction
}

// combineMeshesAssemble combines meshes without boolean operation
func combineMeshesAssemble(a, b *Mesh) *Mesh {
	return combineMeshesUnion(a, b)
}

type Group struct {
	Tag int
	H   core.HGroup

	Type                   GroupType
	Order                  int
	OpA                    core.HGroup
	OpB                    core.HGroup
	Visible                bool
	Suppress               bool
	RelaxConstraints       bool
	AllowRedundant         bool
	SuppressDofCalculation bool
	AllDimsReference       bool
	Scale                  float64

	Clean           bool
	DofCheckOk      bool
	ActiveWorkplane core.HEntity
	ValA            float64
	ValB            float64
	ValC            float64

	Solved struct {
		How              core.SolveResult
		Dof              int
		FindToFixTimeout int
		Timeout          bool
		Remove           []core.HConstraint
	}

	Subtype   GroupSubtype
	SkipFirst bool

	BooleanFailed bool

	// SketchDiagnostic captures errors from the last GenerateAll.
	// Cleared at the start of each GenerateAll invocation.
	SketchDiagnostic *SketchDiagnostic
	MeshCombine      CombineAs
	ForceToMesh      bool

	Remap    core.EntityMap
	LinkFile string

	Color core.RgbaColor

	Predef struct {
		Q       geom.Quaternion
		Origin  core.HEntity
		EntityB core.HEntity
		EntityC core.HEntity
		SwapUV  bool
		NegateU bool
		NegateV bool
	}

	// Phase 5 additions: trim loops and the SShell/SMesh pipeline.
	PolyLoops   srf.SPolygon
	BezierLoops srf.SBezierLoopSetSet
	BezierOpens []srf.SBezierLoop
	PolyError   struct {
		How        int
		NotClosed  srf.SEdge
		ErrorPoint geom.Vector
	}

	ThisShell    srf.SShell
	RunningShell srf.SShell
	ThisMesh     srf.SMesh

	RunningMesh     srf.SMesh
	DisplayMesh     srf.SMesh
	DisplayOutlines srf.SOutlineList

	ImpMesh   srf.SMesh
	ImpShell  srf.SShell
	ImpEntity []core.Entity

	Name string

	// Generated mesh for this group
	Mesh *Mesh
}

func (g *Group) Clear() {}

func (g *Group) IsSolvedOkay() bool {
	return g.Solved.How == core.SolveOkay || g.Solved.How == core.SolveRedundantOkay
}

func (g *Group) IsVisible() bool {
	return g.Visible && !g.Suppress
}

func (g *Group) IsMeshGroup() bool {
	return g.Type == GroupExtrude ||
		g.Type == GroupLathe ||
		g.Type == GroupRevolve ||
		g.Type == GroupHelix ||
		g.Type == GroupRotate ||
		g.Type == GroupTranslate
}

// isTransformationGroup returns true if this group transforms an existing mesh
// rather than generating a new one from geometry
func (g *Group) isTransformationGroup() bool {
	return g.Type == GroupRotate || g.Type == GroupTranslate
}

func (g *Group) PreviousGroup(sk *Sketch) *Group {
	if g == nil || sk == nil {
		return nil
	}

	var prev *Group
	for _, gh := range sk.GroupOrder {
		current := sk.GetGroup(gh)
		if current == nil {
			continue
		}
		if current.H == g.H {
			return prev
		}
		prev = current
	}
	return nil
}

func (g *Group) RunningMeshGroup(sk *Sketch) *Group {
	if g == nil || sk == nil {
		return nil
	}

	switch g.Type {
	case GroupRotate, GroupTranslate:
		opA := sk.GetGroup(g.OpA)
		if opA == nil {
			return nil
		}
		return opA.RunningMeshGroup(sk)
	default:
		return g.PreviousGroup(sk)
	}
}

// ExtrusionVector returns the extrusion vector for this group.
func (g *Group) ExtrusionVector(sk *Sketch) geom.Vector {
	if g.Type != GroupExtrude {
		return geom.Vector{}
	}

	vector := geom.Vector{X: g.ValA, Y: g.ValB, Z: g.ValC}
	if vector.Magnitude() < 1e-6 {
		// Try to get from params
		if len(sk.Params) >= 3 {
			vector.X = sk.GetParam(g.H.Param(0)).Val
			vector.Y = sk.GetParam(g.H.Param(1)).Val
			vector.Z = sk.GetParam(g.H.Param(2)).Val
		}
	}
	return vector
}

// LatheCenter returns the center point for lathe operations.
func (g *Group) LatheCenter(sk *Sketch) geom.Vector {
	if g.Type != GroupLathe && g.Type != GroupRevolve {
		return geom.Vector{}
	}
	// For now, use origin - could be extended to use params
	return geom.Vector{X: 0, Y: 0, Z: 0}
}

// HelixParameters returns axis, center, and pitch for helix operations.
// HelixParameters returns axis, center, and pitch for the helix.
// Goal Item 4 implementation: extract from the group's predef params
// (axis from Predef.EntityB's normal, center from Predef.Origin's
// numerical position, pitch from group params).
func (g *Group) HelixParameters(sk *Sketch) (axis, center geom.Vector, pitch float64) {
	if g.Type != GroupHelix {
		return geom.Vector{}, geom.Vector{}, 1.0
	}
	axis = paramValOr(sk, g.H.Param(4), geom.Vector{X: 0, Y: 0, Z: 1})
	center = paramValOrVec(sk, g.H.Param(0), geom.Vector{})
	pitch = paramValOrF(sk, g.H.Param(3), 1.0)
	// Override axis with predef.normal if available.
	if g.Predef.EntityB != (core.HEntity{}) {
		axis = entityValVec(sk, g.Predef.EntityB, geom.Vector{X: 0, Y: 0, Z: 1})
	}
	if g.Predef.Origin != (core.HEntity{}) {
		center = entityValVec(sk, g.Predef.Origin, geom.Vector{})
	}
	return axis, center, pitch
}

// RotationParameters returns axis, angle, and center for rotation operations.
// Goal Item 4 implementation: extract axis from Predef.EntityB's normal,
// angle from g.ValA (radians), center from Predef.Origin's position.
func (g *Group) RotationParameters(sk *Sketch) (axis geom.Vector, angle float64, center geom.Vector) {
	if g.Type != GroupRotate && g.Type != GroupRevolve {
		return geom.Vector{}, 0, geom.Vector{}
	}
	axis = paramValOr(sk, g.H.Param(4), geom.Vector{X: 0, Y: 0, Z: 1})
	angle = g.ValA
	if angle == 0 {
		angle = math.Pi / 4
	}
	center = paramValOrVec(sk, g.H.Param(0), geom.Vector{})
	if g.Predef.EntityB != (core.HEntity{}) {
		axis = entityValVec(sk, g.Predef.EntityB, geom.Vector{X: 0, Y: 0, Z: 1})
	}
	if g.Predef.Origin != (core.HEntity{}) {
		center = entityValVec(sk, g.Predef.Origin, geom.Vector{})
	}
	return axis, angle, center
}

// TranslationVector returns the translation vector for this group.
// Goal Item 4: extract from g.H.Param(0..2).
func (g *Group) TranslationVector(sk *Sketch) geom.Vector {
	if g.Type != GroupTranslate {
		return geom.Vector{}
	}
	return geom.Vector{
		X: paramValOrF(sk, g.H.Param(0), 1.0),
		Y: paramValOrF(sk, g.H.Param(1), 0.0),
		Z: paramValOrF(sk, g.H.Param(2), 0.0),
	}
}

// paramValOr returns the numeric value of param h, or fallback if not
// present.
func paramValOrF(sk *Sketch, h core.HParam, fallback float64) float64 {
	if p := sk.GetParam(h); p != nil {
		return p.Val
	}
	return fallback
}

// paramValOr returns the numeric value of param h as a Vector, using
// fallback.X/Y/Z if not present.
func paramValOr(sk *Sketch, h core.HParam, fallback geom.Vector) geom.Vector {
	if p := sk.GetParam(h); p != nil {
		return geom.Vector{X: p.Val, Y: 0, Z: 0}
	}
	return fallback
}

// paramValOrVec reads three consecutive params as a Vector.
func paramValOrVec(sk *Sketch, h core.HParam, fallback geom.Vector) geom.Vector {
	if h == (core.HParam{}) {
		return fallback
	}
	x := paramValOrF(sk, h, fallback.X)
	y := paramValOrF(sk, core.HParam{V: h.V + 1}, fallback.Y)
	z := paramValOrF(sk, core.HParam{V: h.V + 2}, fallback.Z)
	return geom.Vector{X: x, Y: y, Z: z}
}

// entityValVec returns the numerical position of entity h, or fallback.
func entityValVec(sk *Sketch, h core.HEntity, fallback geom.Vector) geom.Vector {
	if e := sk.GetEntity(h); e != nil {
		return e.ActPoint
	}
	return fallback
}

// GenerateSurfaces generates surface entities for mesh groups like extrude, lathe, etc.
func (g *Group) GenerateSurfaces(sk *Sketch) error {
	if !g.IsMeshGroup() {
		return nil
	}

	// Generate the group's own mesh
	var ownMesh *Mesh
	switch g.Type {
	case GroupExtrude:
		if err := g.generateExtrudeSurfaces(sk); err != nil {
			return err
		}
		ownMesh = g.Mesh
	case GroupLathe:
		if err := g.generateLatheSurfaces(sk); err != nil {
			return err
		}
		ownMesh = g.Mesh
	case GroupRevolve:
		if err := g.generateRevolveSurfaces(sk); err != nil {
			return err
		}
		ownMesh = g.Mesh
	case GroupHelix:
		if err := g.generateHelixSurfaces(sk); err != nil {
			return err
		}
		ownMesh = g.Mesh
	case GroupRotate:
		if err := g.generateRotateSurfaces(sk); err != nil {
			return err
		}
		ownMesh = g.Mesh
	case GroupTranslate:
		if err := g.generateTranslateSurfaces(sk); err != nil {
			return err
		}
		ownMesh = g.Mesh
	default:
		return nil
	}

	// If this group has OpA and is not a transformation operation, combine meshes
	if g.OpA != (core.HGroup{}) && !g.isTransformationGroup() {
		opAGroup := sk.GetGroup(g.OpA)
		if opAGroup != nil && opAGroup.Mesh != nil {
			combined := CombineMeshes(opAGroup.Mesh, ownMesh, g.MeshCombine)
			g.Mesh = combined
		}
	}

	return nil
}

// generateExtrudeSurfaces implements extrusion of 2D profiles into 3D surfaces.
func (g *Group) generateExtrudeSurfaces(sk *Sketch) error {
	// Get the source group (OpA)
	source := sk.GetGroup(g.OpA)
	if source == nil {
		return fmt.Errorf("extrude: source group %v not found", g.OpA)
	}

	// Get extrusion vector from group params
	vector := geom.Vector{X: g.ValA, Y: g.ValB, Z: g.ValC}
	if vector.Magnitude() < 1e-6 {
		// If zero vector, try to get from params
		if len(sk.Params) >= 3 {
			vector.X = sk.GetParam(g.H.Param(0)).Val
			vector.Y = sk.GetParam(g.H.Param(1)).Val
			vector.Z = sk.GetParam(g.H.Param(2)).Val
		}
	}

	// Find closed loops in source entities
	loops, err := g.findClosedLoops(sk, source)
	if err != nil {
		return err
	}

	// For each loop, extrude to create surfaces
	for _, loop := range loops {
		if err := g.extrudeLoop(sk, loop, vector); err != nil {
			return err
		}
	}

	return nil
}

// findClosedLoops identifies closed loops of entities in the source group.
func (g *Group) findClosedLoops(sk *Sketch, source *Group) ([][]*core.Entity, error) {
	var loops [][]*core.Entity

	// Collect all curve entities (lines, arcs) in the source group
	var curves []*core.Entity
	for i := range sk.Entities {
		ent := &sk.Entities[i]
		if ent.Group == source.H && g.isCurveEntity(ent) {
			curves = append(curves, ent)
		}
	}

	// Build connectivity graph: point -> list of curves starting/ending there
	connectivity := make(map[core.HEntity][]*core.Entity)
	for _, curve := range curves {
		start := g.getCurveStart(sk, curve)
		end := g.getCurveEnd(sk, curve)
		if start != nil {
			connectivity[start.H] = append(connectivity[start.H], curve)
		}
		if end != nil && end.H != start.H {
			connectivity[end.H] = append(connectivity[end.H], curve)
		}
	}

	// Find all simple closed loops using DFS
	visited := make(map[*core.Entity]bool)
	for _, startCurve := range curves {
		if visited[startCurve] {
			continue
		}
		if loop := g.findLoopDFS(sk, startCurve, startCurve, []*core.Entity{}, visited, connectivity); loop != nil {
			loops = append(loops, loop)
		}
	}

	// Sort loops by area (outermost first) - using signed area for winding
	// Positive area = counterclockwise (outer), negative = clockwise (inner/hole)
	for i := 0; i < len(loops)-1; i++ {
		for j := i + 1; j < len(loops); j++ {
			if g.loopArea(sk, loops[i]) < g.loopArea(sk, loops[j]) {
				loops[i], loops[j] = loops[j], loops[i]
			}
		}
	}

	return loops, nil
}

// isCurveEntity checks if entity represents a curve (line or arc)
func (g *Group) isCurveEntity(ent *core.Entity) bool {
	return ent.Type == core.EntityLineSegment || ent.Type == core.EntityArcOfCircle
}

// getCurveStart returns the starting point entity of a curve
func (g *Group) getCurveStart(sk *Sketch, curve *core.Entity) *core.Entity {
	return g.getEntityEnd(sk, curve, 0)
}

// getCurveEnd returns the ending point entity of a curve
func (g *Group) getCurveEnd(sk *Sketch, curve *core.Entity) *core.Entity {
	return g.getEntityEnd(sk, curve, 1)
}

// findLoopDFS performs DFS to find closed loops
func (g *Group) findLoopDFS(sk *Sketch, current, start *core.Entity, path []*core.Entity, visited map[*core.Entity]bool, connectivity map[core.HEntity][]*core.Entity) []*core.Entity {
	visited[current] = true
	path = append(path, current)

	currentEnd := g.getCurveEnd(sk, current)
	if currentEnd == nil {
		// Invalid curve, backtrack
		visited[current] = false
		return nil
	}

	// Check if we can close the loop
	nextCurves := connectivity[currentEnd.H]
	for _, next := range nextCurves {
		if next == start && len(path) >= 3 {
			// Found closed loop - return the path (without duplicating start)
			return path
		}
		if !visited[next] {
			if loop := g.findLoopDFS(sk, next, start, path, visited, connectivity); loop != nil {
				return loop
			}
		}
	}

	// Backtrack
	visited[current] = false
	return nil
}

// loopArea computes approximate area of a loop (for sorting)
func (g *Group) loopArea(sk *Sketch, loop []*core.Entity) float64 {
	if len(loop) < 3 {
		return 0
	}

	// Simple polygon area calculation using shoelace formula
	var area float64
	var points []geom.Vector

	for _, curve := range loop {
		start := g.getCurveStart(sk, curve)
		if start != nil {
			points = append(points, start.ActPoint)
		}
	}

	n := len(points)
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		area += points[i].X*points[j].Y - points[j].X*points[i].Y
	}

	return math.Abs(area) / 2
}

// getEntityEnd returns the point entity at the specified end (0 or 1) of the given entity.
func (g *Group) getEntityEnd(sk *Sketch, ent *core.Entity, end int) *core.Entity {
	var h core.HEntity
	switch ent.Type {
	case core.EntityLineSegment, core.EntityArcOfCircle:
		h = ent.Point[end]
	default:
		return nil
	}
	return sk.GetEntity(h)
}

// extrudeLoop extrudes a single closed loop to create side surfaces and caps.
func (g *Group) extrudeLoop(sk *Sketch, loop []*core.Entity, vector geom.Vector) error {
	if len(loop) < 3 {
		return fmt.Errorf("extrude: loop too small")
	}

	// Extract 2D points from the loop
	var points []geom.Vector
	for _, curve := range loop {
		start := g.getCurveStart(sk, curve)
		if start != nil {
			points = append(points, start.ActPoint)
		}
	}

	if len(points) < 3 {
		return fmt.Errorf("extrude: not enough points in loop")
	}

	// Create mesh for this extrusion
	mesh := &Mesh{}

	// Add bottom cap vertices
	bottomStart := len(mesh.Vertices)
	mesh.Vertices = append(mesh.Vertices, points...)

	// Add top cap vertices (extruded)
	topStart := len(mesh.Vertices)
	for _, p := range points {
		mesh.Vertices = append(mesh.Vertices, p.Plus(vector))
	}

	// Triangulate bottom cap
	bottomTriangles := g.triangulatePolygon(points, bottomStart)
	mesh.Triangles = append(mesh.Triangles, bottomTriangles...)

	// Triangulate top cap (note: winding might be reversed)
	topPoints := make([]geom.Vector, len(points))
	for i, p := range points {
		topPoints[i] = p.Plus(vector)
	}
	topTriangles := g.triangulatePolygon(topPoints, topStart)
	// Reverse winding for top cap
	for i := range topTriangles {
		topTriangles[i].A, topTriangles[i].C = topTriangles[i].C, topTriangles[i].A
	}
	mesh.Triangles = append(mesh.Triangles, topTriangles...)

	// Create side faces
	n := len(points)
	for i := 0; i < n; i++ {
		j := (i + 1) % n

		// Quad: bottom[i], bottom[j], top[j], top[i]
		// Triangulate as two triangles
		a := bottomStart + i
		b := bottomStart + j
		c := topStart + j
		d := topStart + i

		// First triangle: a, b, c
		mesh.Triangles = append(mesh.Triangles, Triangle{A: a, B: b, C: c})
		// Second triangle: a, c, d
		mesh.Triangles = append(mesh.Triangles, Triangle{A: a, B: c, C: d})
	}

	// Store mesh in group
	g.Mesh = mesh

	return nil
}

// generateLatheSurfaces implements lathe (rotation around axis) of 2D profiles into 3D surfaces
func (g *Group) generateLatheSurfaces(sk *Sketch) error {
	source := sk.GetGroup(g.OpA)
	if source == nil {
		return fmt.Errorf("lathe: source group %v not found", g.OpA)
	}

	loops, err := g.findClosedLoops(sk, source)
	if err != nil {
		return err
	}

	if len(loops) == 0 {
		return fmt.Errorf("lathe: no closed loops found in source")
	}

	// For simplicity, use the first loop
	loop := loops[0]

	// Extract 2D points
	var points []geom.Vector
	for _, curve := range loop {
		start := g.getCurveStart(sk, curve)
		if start != nil {
			points = append(points, start.ActPoint)
		}
	}

	if len(points) < 3 {
		return fmt.Errorf("lathe: not enough points in loop")
	}

	// Lathe parameters: axis and angle range
	axis := geom.Vector{X: 0, Y: 0, Z: 1}
	center := g.LatheCenter(sk)
	steps := 16 // Number of rotation steps

	return g.revolveProfile(sk, points, axis, center, 0, 2*math.Pi, steps)
}

// generateRevolveSurfaces implements revolve (similar to lathe)
func (g *Group) generateRevolveSurfaces(sk *Sketch) error {
	// Revolve is essentially the same as lathe for full 360 degrees
	return g.generateLatheSurfaces(sk)
}

// revolveProfile rotates a 2D profile around an axis to create a 3D surface
func (g *Group) revolveProfile(sk *Sketch, profile []geom.Vector, axis geom.Vector, center geom.Vector, startAngle, endAngle float64, steps int) error {
	mesh := &Mesh{}
	angleStep := (endAngle - startAngle) / float64(steps-1)

	// Generate vertices: for each step, rotate all profile points
	for step := 0; step < steps; step++ {
		angle := startAngle + float64(step)*angleStep
		rotation := geom.QuaternionFromAxisAngle(axis, angle)

		for _, point := range profile {
			// Translate to center, rotate, translate back
			translated := point.Minus(center)
			rotated := rotation.Rotate(translated)
			final := rotated.Plus(center)

			mesh.Vertices = append(mesh.Vertices, final)
		}
	}

	// Generate triangles: connect adjacent rotation steps
	profileLen := len(profile)
	for step := 0; step < steps-1; step++ {
		offset1 := step * profileLen
		offset2 := (step + 1) * profileLen

		for i := 0; i < profileLen; i++ {
			j := (i + 1) % profileLen

			// Quad: p1[i], p1[j], p2[j], p2[i]
			a := offset1 + i
			b := offset1 + j
			c := offset2 + j
			d := offset2 + i

			// First triangle: a, b, c
			mesh.Triangles = append(mesh.Triangles, Triangle{A: a, B: b, C: c})
			// Second triangle: a, c, d
			mesh.Triangles = append(mesh.Triangles, Triangle{A: a, B: c, C: d})
		}
	}

	g.Mesh = mesh
	return nil
}

// triangulatePolygon creates triangles for a simple polygon using fan triangulation
func (g *Group) triangulatePolygon(points []geom.Vector, offset int) []Triangle {
	var triangles []Triangle
	n := len(points)
	if n < 3 {
		return triangles
	}

	// Simple fan triangulation from first vertex
	for i := 1; i < n-1; i++ {
		triangles = append(triangles, Triangle{
			A: offset,
			B: offset + i,
			C: offset + i + 1,
		})
	}

	return triangles
}

// generateHelixSurfaces implements helix (combined rotation and translation)
func (g *Group) generateHelixSurfaces(sk *Sketch) error {
	source := sk.GetGroup(g.OpA)
	if source == nil {
		return fmt.Errorf("helix: source group %v not found", g.OpA)
	}

	loops, err := g.findClosedLoops(sk, source)
	if err != nil {
		return err
	}

	if len(loops) == 0 {
		return fmt.Errorf("helix: no closed loops found in source")
	}

	// For simplicity, use the first loop
	loop := loops[0]

	// Extract 2D points
	var points []geom.Vector
	for _, curve := range loop {
		start := g.getCurveStart(sk, curve)
		if start != nil {
			points = append(points, start.ActPoint)
		}
	}

	if len(points) < 3 {
		return fmt.Errorf("helix: not enough points in loop")
	}

	// Helix parameters
	axis, center, pitch := g.HelixParameters(sk)
	steps := 32 // More steps for smoother helix

	return g.revolveProfileWithPitch(sk, points, axis, center, 0, 4*math.Pi, pitch, steps)
}

// generateRotateSurfaces implements rotate (copy and rotate around axis)
func (g *Group) generateRotateSurfaces(sk *Sketch) error {
	// Get the mesh from OpA
	opAGroup := sk.GetGroup(g.OpA)
	if opAGroup == nil || opAGroup.Mesh == nil {
		return fmt.Errorf("rotate: source group %v not found or has no mesh", g.OpA)
	}

	// Rotation parameters
	axis, angle, center := g.RotationParameters(sk)

	// Rotate all vertices of the source mesh
	rotation := geom.QuaternionFromAxisAngle(axis, angle)

	result := &Mesh{}
	result.Vertices = make([]geom.Vector, len(opAGroup.Mesh.Vertices))
	for i, v := range opAGroup.Mesh.Vertices {
		// Translate to center, rotate, translate back
		translated := v.Minus(center)
		rotated := rotation.Rotate(translated)
		result.Vertices[i] = rotated.Plus(center)
	}

	// Copy triangles unchanged
	result.Triangles = make([]Triangle, len(opAGroup.Mesh.Triangles))
	copy(result.Triangles, opAGroup.Mesh.Triangles)

	g.Mesh = result
	return nil
}

// generateTranslateSurfaces implements translate (copy and move)
func (g *Group) generateTranslateSurfaces(sk *Sketch) error {
	// Get the mesh from OpA
	opAGroup := sk.GetGroup(g.OpA)
	if opAGroup == nil || opAGroup.Mesh == nil {
		return fmt.Errorf("translate: source group %v not found or has no mesh", g.OpA)
	}

	// Translation vector
	offset := g.TranslationVector(sk)

	// Translate all vertices of the source mesh
	result := &Mesh{}
	result.Vertices = make([]geom.Vector, len(opAGroup.Mesh.Vertices))
	for i, v := range opAGroup.Mesh.Vertices {
		result.Vertices[i] = v.Plus(offset)
	}

	// Copy triangles unchanged
	result.Triangles = make([]Triangle, len(opAGroup.Mesh.Triangles))
	copy(result.Triangles, opAGroup.Mesh.Triangles)

	g.Mesh = result
	return nil
}

// revolveProfileWithPitch creates a helix by combining rotation and translation
func (g *Group) revolveProfileWithPitch(sk *Sketch, profile []geom.Vector, axis geom.Vector, center geom.Vector, startAngle, endAngle, pitch float64, steps int) error {
	mesh := &Mesh{}
	angleStep := (endAngle - startAngle) / float64(steps-1)
	heightStep := pitch * angleStep / (2 * math.Pi) // Convert pitch to height per step

	// Generate vertices: for each step, rotate and translate profile points
	for step := 0; step < steps; step++ {
		angle := startAngle + float64(step)*angleStep
		height := float64(step) * heightStep
		rotation := geom.QuaternionFromAxisAngle(axis, angle)

		for _, point := range profile {
			// Translate to center, rotate, translate back, then add height
			translated := point.Minus(center)
			rotated := rotation.Rotate(translated)
			final := rotated.Plus(center).Plus(axis.ScaledBy(height))
			mesh.Vertices = append(mesh.Vertices, final)
		}
	}

	// Generate triangles: connect adjacent steps
	profileLen := len(profile)
	for step := 0; step < steps-1; step++ {
		offset1 := step * profileLen
		offset2 := (step + 1) * profileLen

		for i := 0; i < profileLen; i++ {
			j := (i + 1) % profileLen

			// Quad: p1[i], p1[j], p2[j], p2[i]
			a := offset1 + i
			b := offset1 + j
			c := offset2 + j
			d := offset2 + i

			// First triangle: a, b, c
			mesh.Triangles = append(mesh.Triangles, Triangle{A: a, B: b, C: c})
			// Second triangle: a, c, d
			mesh.Triangles = append(mesh.Triangles, Triangle{A: a, B: c, C: d})
		}
	}

	g.Mesh = mesh
	return nil
}

type GroupList []Group

// SketchDiagnostic captures transient per-group errors during
// Sketch::GenerateAll. Cleared on each invocation.
type SketchDiagnostic struct {
	AddConstraintErrors []string
}
