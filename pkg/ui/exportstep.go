package ui

// Export a STEP file describing our ratpoly shell (C++ exportstep.cpp).

import (
	"bytes"
	"os"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/cfmt"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// fp is the coordinate precision: LENGTH_EPS is 1e-6, so five decimal
// places round out errors like 0.999999mm (C++ FP).
const stepCartesianPointFormat = "#%d=CARTESIAN_POINT('',(%.5f,%.5f,%.5f));\n"

const stepPrecision = 2 * core.LengthEps

type stepAlias struct {
	reference int
	aliases   []int
}

type stepPointAlias struct {
	alias       stepAlias
	vertexAlias stepAlias
	v           geom.Vector
}

type stepCurveAlias struct {
	alias        stepAlias
	memberPoints []int
	color        core.RgbaColor
}

type stepEdgeCurveAlias struct {
	alias      stepAlias
	prevFinish int
	thisFinish int
	curveID    int
	color      core.RgbaColor
}

type stepOrientedEdgeAlias struct {
	alias       stepAlias
	edgeCurveID int
	flip        bool
}

// stepFileWriter writes STEP (C++ StepFileWriter).
type stepFileWriter struct {
	ss *SolveSpaceUI

	curves        []int
	advancedFaces []int
	f             *bytes.Buffer
	id            int

	pointAliases        []stepPointAlias
	edgeCurveAliases    []stepEdgeCurveAlias
	curveAliases        []stepCurveAlias
	orientedEdgeAliases []stepOrientedEdgeAlias
	exportParts         bool
	currentColor        core.RgbaColor
}

func (w *stepFileWriter) printf(format string, args ...any) {
	w.f.WriteString(cfmt.Sprintf(format, args...))
}

// HasCartesianPointAnAlias reports whether this point was already
// defined with a different ID number, and records it (C++
// StepFileWriter::HasCartesianPointAnAlias). vertex is the id of the
// vertex linked to the point, or less than 1 for none.
func (w *stepFileWriter) HasCartesianPointAnAlias(number int, v geom.Vector, vertex int,
	vertexHasAlias *bool) bool {
	// Look for this point "v" in the alias list.
	for i := range w.pointAliases {
		p := &w.pointAliases[i]
		if p.v.EqualsTol(v, stepPrecision) {
			// This point was already defined. The new number is just an
			// alias.
			p.alias.aliases = append(p.alias.aliases, number)
			if vertex > 0 {
				if p.vertexAlias.reference <= 0 {
					p.vertexAlias.reference = vertex
				}
				p.vertexAlias.aliases = append(p.vertexAlias.aliases, vertex)

				// If the caller is interested let them know if the vertex
				// is new or has an existing alias.
				if vertexHasAlias != nil {
					*vertexHasAlias = len(p.vertexAlias.aliases) != 1
				}
			}
			return true
		}
	}

	// No point was found, it means this is a new point.
	newPoint := stepPointAlias{v: v}
	newPoint.alias.reference = number
	newPoint.alias.aliases = []int{number}
	newPoint.vertexAlias.reference = vertex
	if vertex > 0 {
		newPoint.vertexAlias.aliases = []int{vertex}
	}

	// A new entry in the list.
	w.pointAliases = append(w.pointAliases, newPoint)
	return false
}

// InsertPoint is a cartesian point's index, its first alias if it has
// any (C++ StepFileWriter::InsertPoint).
func (w *stepFileWriter) InsertPoint(number int) int {
	for _, p := range w.pointAliases {
		for _, alias := range p.alias.aliases {
			if alias == number {
				return p.alias.reference
			}
		}
	}

	// ERROR: it should never reach this point...
	return -1
}

// InsertVertex is a vertex's index, its first alias if it has any (C++
// StepFileWriter::InsertVertex).
func (w *stepFileWriter) InsertVertex(number int) int {
	for _, p := range w.pointAliases {
		for _, alias := range p.vertexAlias.aliases {
			if alias == number {
				return p.vertexAlias.reference
			}
		}
	}

	// ERROR: it should never reach this point...
	return -1
}

// HasBSplineCurveAnAlias reports whether this curve was already
// defined with a different ID number, and records it (C++
// StepFileWriter::HasBSplineCurveAnAlias).
func (w *stepFileWriter) HasBSplineCurveAnAlias(number int, points []int) bool {
	for i := range w.curveAliases {
		c := &w.curveAliases[i]
		if len(points) != len(c.memberPoints) {
			continue
		}
		if w.exportParts && !c.color.Equals(w.currentColor) {
			continue
		}
		matches := 0 // is this the same curve?
		// FIXME: this hack should work _most_ of the times
		for _, p := range points {
			for _, m := range c.memberPoints {
				if p == m {
					matches++
				}
			}
		}

		if matches == len(points) {
			// Add this alias.
			c.alias.aliases = append(c.alias.aliases, number)
			return true
		}
	}

	// No curve was found, it means this is a new curve.
	newCurve := stepCurveAlias{memberPoints: points, color: w.currentColor}
	newCurve.alias.reference = number
	newCurve.alias.aliases = []int{number}

	w.curveAliases = append(w.curveAliases, newCurve)
	return false
}

// InsertCurve is a curve's index (C++ StepFileWriter::InsertCurve).
func (w *stepFileWriter) InsertCurve(number int) int {
	for _, c := range w.curveAliases {
		for _, alias := range c.alias.aliases {
			if alias == number && (!w.exportParts || c.color.Equals(w.currentColor)) {
				return c.alias.reference
			}
		}
	}

	// ERROR: it should never reach this point...
	return -1
}

// HasEdgeCurveAnAlias reports whether this edge curve was already
// defined with a different ID number, in either direction, and records
// it (C++ StepFileWriter::HasEdgeCurveAnAlias).
func (w *stepFileWriter) HasEdgeCurveAnAlias(number, prevFinish, thisFinish, curveID int, flip *bool) bool {
	for i := range w.edgeCurveAliases {
		e := &w.edgeCurveAliases[i]
		if w.exportParts && !e.color.Equals(w.currentColor) {
			continue
		}
		// Check both directions.
		if ((prevFinish == e.prevFinish && thisFinish == e.thisFinish) ||
			(prevFinish == e.thisFinish && thisFinish == e.prevFinish)) &&
			curveID == e.curveID {
			e.alias.aliases = append(e.alias.aliases, number)
			if flip != nil {
				*flip = prevFinish == e.thisFinish && thisFinish == e.prevFinish
			}
			return true
		}
	}

	// New edge curve.
	newEdgeCurve := stepEdgeCurveAlias{
		prevFinish: prevFinish,
		thisFinish: thisFinish,
		curveID:    curveID,
		color:      w.currentColor,
	}
	newEdgeCurve.alias.reference = number
	newEdgeCurve.alias.aliases = []int{number}

	w.edgeCurveAliases = append(w.edgeCurveAliases, newEdgeCurve)
	return false
}

// InsertEdgeCurve is an edge curve's index (C++
// StepFileWriter::InsertEdgeCurve).
func (w *stepFileWriter) InsertEdgeCurve(number int) int {
	for _, e := range w.edgeCurveAliases {
		for _, alias := range e.alias.aliases {
			if alias == number && (!w.exportParts || e.color.Equals(w.currentColor)) {
				return e.alias.reference
			}
		}
	}

	// ERROR: it should never reach this point...
	return -1
}

// HasOrientedEdgeAnAlias reports whether this oriented edge was already
// defined with a different ID number, and records it (C++
// StepFileWriter::HasOrientedEdgeAnAlias).
func (w *stepFileWriter) HasOrientedEdgeAnAlias(number, edgeCurveID int, flip bool) bool {
	for i := range w.orientedEdgeAliases {
		e := &w.orientedEdgeAliases[i]
		if edgeCurveID == e.edgeCurveID && flip == e.flip {
			e.alias.aliases = append(e.alias.aliases, number)
			return true
		}
	}

	// New oriented edge.
	newOrientedEdge := stepOrientedEdgeAlias{edgeCurveID: edgeCurveID, flip: flip}
	newOrientedEdge.alias.reference = number
	newOrientedEdge.alias.aliases = []int{number}

	w.orientedEdgeAliases = append(w.orientedEdgeAliases, newOrientedEdge)
	return false
}

// InsertOrientedEdge is an oriented edge's index (C++
// StepFileWriter::InsertOrientedEdge).
func (w *stepFileWriter) InsertOrientedEdge(number int) int {
	for _, e := range w.orientedEdgeAliases {
		for _, alias := range e.alias.aliases {
			if alias == number {
				return e.alias.reference
			}
		}
	}

	// ERROR: it should never reach this point...
	return -1
}

// WriteHeader writes the header, and the units and tolerances, which
// are always the same (C++ StepFileWriter::WriteHeader).
func (w *stepFileWriter) WriteHeader() {
	w.printf(
		"ISO-10303-21;\n"+
			"HEADER;\n"+
			"\n"+
			"FILE_DESCRIPTION((''), '2;1');\n"+
			"\n"+
			"FILE_NAME(\n"+
			"    'output_file',\n"+
			"    '2009-06-07T17:44:47-07:00',\n"+
			"    (''),\n"+
			"    (''),\n"+
			"    'TeleSAPR',\n"+
			"    '',\n"+
			"    ''\n"+
			");\n"+
			"\n"+
			"FILE_SCHEMA (('CONFIG_CONTROL_DESIGN'));\n"+
			"ENDSEC;\n"+
			"\n"+
			"DATA;\n"+
			"\n"+
			"/**********************************************************\n"+
			" * This defines the units and tolerances for the file. It\n"+
			" * is always the same, independent of the actual data.\n"+
			" **********************************************************/\n"+
			"#158=(\n"+
			"LENGTH_UNIT()\n"+
			"NAMED_UNIT(*)\n"+
			"SI_UNIT(.MILLI.,.METRE.)\n"+
			");\n"+
			"#161=(\n"+
			"NAMED_UNIT(*)\n"+
			"PLANE_ANGLE_UNIT()\n"+
			"SI_UNIT($,.RADIAN.)\n"+
			");\n"+
			"#166=(\n"+
			"NAMED_UNIT(*)\n"+
			"SI_UNIT($,.STERADIAN.)\n"+
			"SOLID_ANGLE_UNIT()\n"+
			");\n"+
			"#167=UNCERTAINTY_MEASURE_WITH_UNIT(LENGTH_MEASURE(%f),#158,\n"+
			"'DISTANCE_ACCURACY_VALUE',\n"+
			"'string');\n"+
			"#168=(\n"+
			"GEOMETRIC_REPRESENTATION_CONTEXT(3)\n"+
			"GLOBAL_UNCERTAINTY_ASSIGNED_CONTEXT((#167))\n"+
			"GLOBAL_UNIT_ASSIGNED_CONTEXT((#166,#161,#158))\n"+
			"REPRESENTATION_CONTEXT('ID1','3D')\n"+
			");\n"+
			"#169=SHAPE_REPRESENTATION('',(#170),#168);\n"+
			"#170=AXIS2_PLACEMENT_3D('',#173,#171,#172);\n"+
			"#171=DIRECTION('',(0.,0.,1.));\n"+
			"#172=DIRECTION('',(1.,0.,0.));\n"+
			"#173=CARTESIAN_POINT('',(0.,0.,0.));\n"+
			"\n",
		stepPrecision)

	// Start the ID somewhere beyond the header IDs.
	w.id = 200
}

// WriteProductHeader writes the product definition (C++
// StepFileWriter::WriteProductHeader).
func (w *stepFileWriter) WriteProductHeader() {
	w.printf(
		"#175 = SHAPE_DEFINITION_REPRESENTATION(#176, #169);\n" +
			"#176 = PRODUCT_DEFINITION_SHAPE('Version', 'Test Part', #177);\n" +
			"#177 = PRODUCT_DEFINITION('Version', 'Test Part', #182, #178);\n" +
			"#178 = DESIGN_CONTEXT('3D Mechanical Parts', #181, 'design');\n" +
			"#179 = PRODUCT('1', 'Product', 'Test Part', (#180));\n" +
			"#180 = MECHANICAL_CONTEXT('3D Mechanical Parts', #181, 'mechanical');\n" +
			"#181 = APPLICATION_CONTEXT(\n" +
			"'configuration controlled 3d designs of mechanical parts and assemblies');\n" +
			"#182 = PRODUCT_DEFINITION_FORMATION_WITH_SPECIFIED_SOURCE('Version',\n" +
			"'Test Part', #179, .MADE.);\n" +
			"\n")
}

// ExportCurve writes a curve and returns its id (C++
// StepFileWriter::ExportCurve).
func (w *stepFileWriter) ExportCurve(sb *srf.SBezier) int {
	ret := w.id
	var curvePoints []int

	for i := 0; i <= sb.Degree; i++ {
		if !w.HasCartesianPointAnAlias(w.id+1+i, sb.Ctrl[i], -1, nil) {
			c := sb.Ctrl[i]
			w.printf(stepCartesianPointFormat, w.id+1+i, c.X, c.Y, c.Z)
		}
	}

	for i := 0; i <= sb.Degree; i++ {
		curvePoints = append(curvePoints, w.InsertPoint(w.id+1+i))
	}

	if !w.HasBSplineCurveAnAlias(ret, curvePoints) {
		w.printf("#%d=(\n", ret)
		w.printf("BOUNDED_CURVE()\n")
		w.printf("B_SPLINE_CURVE(%d,(", sb.Degree)
		for i := 0; i <= sb.Degree; i++ {
			w.printf("#%d", w.InsertPoint(ret+i+1))
			if i != sb.Degree {
				w.printf(",")
			}
		}
		w.printf("),.UNSPECIFIED.,.F.,.F.)\n")
		w.printf("B_SPLINE_CURVE_WITH_KNOTS((%d,%d),", sb.Degree+1, sb.Degree+1)
		w.printf("(0.000,1.000),.UNSPECIFIED.)\n")
		w.printf("CURVE()\n")
		w.printf("GEOMETRIC_REPRESENTATION_ITEM()\n")
		w.printf("RATIONAL_B_SPLINE_CURVE((")
		for i := 0; i <= sb.Degree; i++ {
			w.printf("%.10f", sb.Weight[i])
			if i != sb.Degree {
				w.printf(",")
			}
		}
		w.printf("))\n")
		w.printf("REPRESENTATION_ITEM('')\n);\n")
		w.printf("\n")
	}

	w.id = ret + 1 + (sb.Degree + 1)
	return ret
}

// vertexFor writes the point and vertex at a loop's joint, or finds
// the vertex it aliases, and returns the vertex's id.
func (w *stepFileWriter) vertexFor(p geom.Vector) int {
	var vertexHasAlias bool
	switch {
	case !w.HasCartesianPointAnAlias(w.id, p, w.id+1, &vertexHasAlias):
		w.printf(stepCartesianPointFormat, w.id, p.X, p.Y, p.Z)
		w.printf("#%d=VERTEX_POINT('',#%d);\n", w.id+1, w.InsertPoint(w.id))
		return w.id + 1
	case !vertexHasAlias:
		w.printf("#%d=VERTEX_POINT('',#%d);\n", w.id+1, w.InsertPoint(w.id))
		return w.id + 1
	default:
		return w.InsertVertex(w.id + 1)
	}
}

// ExportCurveLoop writes a trim loop as a face bound and returns its id
// (C++ StepFileWriter::ExportCurveLoop).
func (w *stepFileWriter) ExportCurveLoop(loop *srf.SBezierLoop, inner bool) int {
	if len(loop.Curves) < 1 {
		panic("ui: expected at least one loop")
	}

	var listOfTrims []int

	sb := &loop.Curves[len(loop.Curves)-1]

	// Generate "exactly closed" contours, with the same vertex id for
	// the finish of a previous edge and the start of the next one. So we
	// need the finish of the last Bezier in the loop before we start our
	// process.
	lastFinish := w.vertexFor(sb.Finish())
	prevFinish := lastFinish
	w.id += 2

	for i := range loop.Curves {
		sb := &loop.Curves[i]
		curveID := w.ExportCurve(sb)

		var thisFinish int
		if i+1 < len(loop.Curves) {
			thisFinish = w.vertexFor(sb.Finish())
			w.id += 2
		} else {
			thisFinish = lastFinish
		}

		var flipEdge bool
		if !w.HasEdgeCurveAnAlias(w.id, prevFinish, thisFinish, w.InsertCurve(curveID), &flipEdge) {
			w.printf("#%d=EDGE_CURVE('',#%d,#%d,#%d,%s);\n",
				w.id, prevFinish, thisFinish, w.InsertCurve(curveID), ".T.")
			if !w.HasOrientedEdgeAnAlias(w.id+1, w.id, flipEdge) {
				w.printf("#%d=ORIENTED_EDGE('',*,*,#%d,.T.);\n", w.id+1, w.id)
			} else {
				panic("ui: impossible")
			}
		} else if !w.HasOrientedEdgeAnAlias(w.id+1, w.id, flipEdge) {
			dir := byte('T')
			if flipEdge {
				dir = 'F'
			}
			w.printf("#%d=ORIENTED_EDGE('',*,*,#%d,.%c.);\n", w.id+1, w.InsertEdgeCurve(w.id), dir)
		}

		listOfTrims = append(listOfTrims, w.id+1)
		w.id += 2

		prevFinish = thisFinish
	}

	w.printf("#%d=EDGE_LOOP('',(", w.id)
	for i, oe := range listOfTrims {
		w.printf("#%d", w.InsertOrientedEdge(oe))
		if i+1 < len(listOfTrims) {
			w.printf(",")
		}
	}
	w.printf("));\n")

	fb := w.id + 1
	bound := "FACE_OUTER_BOUND"
	if inner {
		bound = "FACE_BOUND"
	}
	w.printf("#%d=%s('',#%d,.T.);\n", fb, bound, w.id)

	w.id += 2

	return fb
}

// ExportSurface writes a trimmed surface as advanced faces (C++
// StepFileWriter::ExportSurface).
func (w *stepFileWriter) ExportSurface(ss *srf.SSurface, sbl *srf.SBezierList) {
	// Read the colour of the surface: use it to tell apart surfaces from
	// different parts.
	w.currentColor = ss.Color

	// First, define the control points for the untrimmed surface, if
	// they were not already defined. We always specify a rational
	// B-spline surface (in fact, just a Bezier surface).
	ctrl := make([][]geom.Vector, ss.DegM+1)
	weight := make([][]float64, ss.DegM+1)
	for i := range ctrl {
		ctrl[i] = ss.Ctrl[i][:ss.DegN+1]
		weight[i] = ss.Weight[i][:ss.DegN+1]
	}
	srfid := w.writeSurface(ss.DegM, ss.DegN, nil, nil, ctrl, weight, false)

	// Now we do the trim curves. We must group each outer loop
	// separately along with its inner faces, so do that now.
	var sblss srf.SBezierLoopSetSet
	var spxyz srf.SPolygon
	var allClosed bool
	var notClosedAt srf.SEdge
	// We specify a surface, so it doesn't check for coplanarity; and we
	// don't want it to give us any open contours. The polygon and chord
	// tolerance are required, because they are used to calculate the
	// contour directions and determine inner vs. outer contours.
	sblss.FindOuterFacesFrom(sbl, &spxyz, ss, w.ss.ExportChordTolMm(),
		&allClosed, &notClosedAt, nil, nil, nil)

	// So in our list of SBezierLoopSet, each set contains at least one
	// loop (the outer boundary), plus any inner loops associated with
	// that outer loop.
	for si := range sblss.Sets {
		w.writeFace(srfid, loopsOf(&sblss.Sets[si]), true, ss.Color)
	}
}

// stepKnot writes a knot: as C++ does for the ends of a Bezier
// patch, and to ten places for others.
func stepKnot(k float64) string {
	if k == float64(int(k)) {
		return cfmt.Sprintf("%.3f", k)
	}
	return cfmt.Sprintf("%.10f", k)
}

// stepKnots are a knot vector's distinct knots and their
// multiplicities, as B_SPLINE_*_WITH_KNOTS takes them; nil knots are
// a Bezier's of degree deg.
func stepKnots(deg int, knots []float64) (mults []int, distinct []float64) {
	if knots == nil {
		return []int{deg + 1, deg + 1}, []float64{0, 1}
	}
	for i, k := range knots {
		if i > 0 && k == knots[i-1] {
			mults[len(mults)-1]++
			continue
		}
		mults = append(mults, 1)
		distinct = append(distinct, k)
	}
	return mults, distinct
}

func stepList[T any](xs []T, f func(T) string) string {
	var b bytes.Buffer
	for i, x := range xs {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(f(x))
	}
	return b.String()
}

// writeSurface writes a rational B-spline surface, its control points
// ctrl[i][j], i along u, and returns its id. Nil knots make it one
// Bezier patch, as C++ writes every surface.
func (w *stepFileWriter) writeSurface(du, dv int, knotsU, knotsV []float64, ctrl [][]geom.Vector, weight [][]float64, closedU bool) int {
	srfid := w.id
	nv := len(ctrl[0])
	for i := range ctrl {
		for j := range ctrl[i] {
			if !w.HasCartesianPointAnAlias(srfid+1+j+i*nv, ctrl[i][j], -1, nil) {
				c := ctrl[i][j]
				w.printf(stepCartesianPointFormat, srfid+1+j+i*nv, c.X, c.Y, c.Z)
			}
		}
	}

	w.printf("#%d=(\n", srfid)
	w.printf("BOUNDED_SURFACE()\n")
	w.printf("B_SPLINE_SURFACE(%d,%d,(", du, dv)
	for i := range ctrl {
		w.printf("(")
		for j := range ctrl[i] {
			w.printf("#%d", w.InsertPoint(srfid+1+j+i*nv))
			if j != nv-1 {
				w.printf(",")
			}
		}
		w.printf(")")
		if i != len(ctrl)-1 {
			w.printf(",")
		}
	}
	closed := ".F."
	if closedU {
		closed = ".T."
	}
	w.printf("),.UNSPECIFIED.,%s,.F.,.F.)\n", closed)
	mu, ku := stepKnots(du, knotsU)
	mv, kv := stepKnots(dv, knotsV)
	itoa := func(n int) string { return cfmt.Sprintf("%d", n) }
	w.printf("B_SPLINE_SURFACE_WITH_KNOTS((%s),(%s),", stepList(mu, itoa), stepList(mv, itoa))
	w.printf("(%s),(%s),.UNSPECIFIED.)\n", stepList(ku, stepKnot), stepList(kv, stepKnot))
	w.printf("GEOMETRIC_REPRESENTATION_ITEM()\n")
	w.printf("RATIONAL_B_SPLINE_SURFACE((")
	for i := range weight {
		w.printf("(")
		for j := range weight[i] {
			w.printf("%.10f", weight[i][j])
			if j != nv-1 {
				w.printf(",")
			}
		}
		w.printf(")")
		if i != len(weight)-1 {
			w.printf(",")
		}
	}
	w.printf("))\n")
	w.printf("REPRESENTATION_ITEM('')\n")
	w.printf("SURFACE()\n")
	w.printf(");\n")

	w.printf("\n")

	w.id = srfid + 1 + len(ctrl)*nv
	return srfid
}

// writeFace writes an advanced face on surface srfid, bounded by
// loops, the first its outer bound when firstOuter, and its colour.
func (w *stepFileWriter) writeFace(srfid int, loops []*srf.SBezierLoop, firstOuter bool, color core.RgbaColor) {
	var listOfLoops []int
	for i, loop := range loops {
		listOfLoops = append(listOfLoops, w.ExportCurveLoop(loop, i > 0 || !firstOuter))
	}

	// And now create the face that corresponds to this outer loop
	// and all of its holes.
	advFaceID := w.id
	w.printf("#%d=ADVANCED_FACE('',(", advFaceID)
	for i, fb := range listOfLoops {
		w.printf("#%d", fb)
		if i+1 < len(listOfLoops) {
			w.printf(",")
		}
	}

	w.printf("),#%d,.T.);\n", srfid)
	w.advancedFaces = append(w.advancedFaces, advFaceID)

	// Export the surface color and transparency
	// https://www.cax-if.org/documents/rec_prac_styling_org_v16.pdf sections 4.4.2 4.2.4 etc.
	// https://tracker.dev.opencascade.org/view.php?id=31550
	w.id++
	w.printf("#%d=COLOUR_RGB('',%.2f,%.2f,%.2f);\n", w.id,
		color.RedF(), color.GreenF(), color.BlueF())

	// This works in Horison EDA but is more verbose.
	w.id++
	w.printf("#%d=FILL_AREA_STYLE_COLOUR('',#%d);\n", w.id, w.id-1)
	w.id++
	w.printf("#%d=FILL_AREA_STYLE('',(#%d));\n", w.id, w.id-1)
	w.id++
	w.printf("#%d=SURFACE_STYLE_FILL_AREA(#%d);\n", w.id, w.id-1)
	w.id++
	w.printf("#%d=SURFACE_STYLE_TRANSPARENT(%.2f);\n", w.id, 1.0-float64(color.AlphaF()))
	w.id++
	w.printf("#%d=SURFACE_STYLE_RENDERING_WITH_PROPERTIES(.NORMAL_SHADING.,#%d,(#%d));\n",
		w.id, w.id-5, w.id-1)
	w.id++
	w.printf("#%d=SURFACE_SIDE_STYLE('',(#%d, #%d));\n", w.id, w.id-3, w.id-1)

	w.id++
	w.printf("#%d=SURFACE_STYLE_USAGE(.BOTH.,#%d);\n", w.id, w.id-1)
	w.id++
	w.printf("#%d=PRESENTATION_STYLE_ASSIGNMENT((#%d));\n", w.id, w.id-1)
	w.id++
	w.printf("#%d=STYLED_ITEM('',(#%d),#%d);\n", w.id, w.id-1, advFaceID)
	w.printf("\n")

	w.id++
}

// stepOneSurfacePerFace makes STEP export write a face of several
// patches as one surface; tests turn it off to compare.
var stepOneSurfacePerFace = true

// stepSheet is a face of several patches that STEP export writes as
// one B-spline surface.
type stepSheet struct {
	grid    *topo.Grid
	patches []core.HSSurface
	// seamTrims are the trims, by surface and index, along the seams
	// between the patches, which bound no face.
	seamTrims map[core.HSSurface]map[int]bool
}

// stepSheets are the faces of several patches of the shell whose
// patches make a grid, by the handles of their patches.
func stepSheets(shell *srf.SShell) map[core.HSSurface]*stepSheet {
	out := map[core.HSSurface]*stepSheet{}
	tp, err := topo.Build(shell)
	if err != nil {
		return out
	}
	for s := range tp.Sheets {
		if len(tp.Sheets[s].Faces) < 2 {
			continue
		}
		grid, err := tp.SheetSpline(s)
		if err != nil {
			continue
		}
		sh := &stepSheet{grid: grid, seamTrims: map[core.HSSurface]map[int]bool{}}
		inSheet := map[core.HSSurface]bool{}
		for _, f := range tp.Sheets[s].Faces {
			h := tp.Faces[f].Surface
			if !inSheet[h] {
				inSheet[h] = true
				sh.patches = append(sh.patches, h)
			}
		}
		// A trim is a seam's when every piece of it is.
		mixed := map[core.HSSurface]map[int]bool{}
		for ce := range tp.CoEdges {
			c := &tp.CoEdges[ce]
			if !inSheet[c.Surface] {
				continue
			}
			m := sh.seamTrims
			if !tp.Edges[c.Edge].Seam {
				m = mixed
			}
			if m[c.Surface] == nil {
				m[c.Surface] = map[int]bool{}
			}
			m[c.Surface][c.Trim] = true
		}
		for h, ts := range mixed {
			for t := range ts {
				delete(sh.seamTrims[h], t)
			}
		}
		for _, h := range sh.patches {
			out[h] = sh
		}
	}
	return out
}

// ExportSheet writes a face of several patches as one advanced face on
// one B-spline surface: the trims of all its patches but those along
// its seams, joined into loops, each trim running as the face's
// patches run it. When they don't join, it writes the patches one by
// one after all.
func (w *stepFileWriter) ExportSheet(shell *srf.SShell, sh *stepSheet, scale float64) {
	var pieces []srf.SBezier
	var cols []int
	column := map[core.HSSurface]int{}
	for _, c := range sh.grid.Cells {
		column[c.H] = c.I
	}
	var color core.RgbaColor
	for k, h := range sh.patches {
		p := shell.FindSurface(h)
		if k == 0 {
			color = p.Color
		}
		for ti := range p.Trim {
			if sh.seamTrims[h][ti] {
				continue
			}
			one := *p
			one.Trim = p.Trim[ti : ti+1]
			var sbl srf.SBezierList
			one.MakeSectionEdgesInto(shell, nil, &sbl)
			stb := &p.Trim[ti]
			// The trim's pieces as it runs, from StartP to FinishP.
			if len(sbl.Curves) > 0 && !sbl.Curves[0].Start().EqualsTol(stb.StartP, 10*geom.LengthEps) {
				for i, j := 0, len(sbl.Curves)-1; i < j; i, j = i+1, j-1 {
					sbl.Curves[i], sbl.Curves[j] = sbl.Curves[j], sbl.Curves[i]
				}
				for i := range sbl.Curves {
					sbl.Curves[i].Reverse()
				}
			}
			pieces = append(pieces, sbl.Curves...)
			for range sbl.Curves {
				cols = append(cols, column[h])
			}
		}
	}
	loops, ok := chainLoops(pieces, cols)
	if ok && sh.grid.ClosedU {
		loops, ok = sh.seamLoops(shell, loops)
	}
	if !ok {
		for _, h := range sh.patches {
			p := shell.FindSurface(h)
			var sbl srf.SBezierList
			p.MakeSectionEdgesInto(shell, nil, &sbl)
			p.ScaleSelfBy(scale)
			sbl.ScaleSelfBy(scale)
			w.ExportSurface(p, &sbl)
		}
		return
	}
	w.currentColor = color
	bs := sh.grid.Surface
	ctrl := make([][]geom.Vector, len(bs.Ctrl))
	for i := range ctrl {
		ctrl[i] = make([]geom.Vector, len(bs.Ctrl[i]))
		for j := range ctrl[i] {
			ctrl[i][j] = bs.Ctrl[i][j].ScaledBy(scale)
		}
	}
	srfid := w.writeSurface(bs.DegU, bs.DegV, bs.KnotsU, bs.KnotsV, ctrl, bs.Weight, sh.grid.ClosedU)
	// STEP runs a bound with the face on its left, seen from outside;
	// SolveSpace's trims run the other way.
	var ls []*srf.SBezierLoop
	for _, l := range loops {
		var r srf.SBezierLoop
		for i := len(l.curves) - 1; i >= 0; i-- {
			c := l.curves[i]
			c.Reverse()
			c.ScaleSelfBy(scale)
			r.Curves = append(r.Curves, c)
		}
		ls = append(ls, &r)
	}
	w.writeFace(srfid, ls, len(ls) == 1, color)
}

// stepLoop is a loop of a face's trims, as SolveSpace runs them, and
// the column of the grid each comes from.
type stepLoop struct {
	curves []srf.SBezier
	cols   []int
}

// chainLoops joins curves end to end into closed loops, or fails.
func chainLoops(cs []srf.SBezier, cols []int) ([]stepLoop, bool) {
	const tol = 10 * geom.LengthEps
	used := make([]bool, len(cs))
	var loops []stepLoop
	for first := range cs {
		if used[first] {
			continue
		}
		used[first] = true
		loop := stepLoop{curves: []srf.SBezier{cs[first]}, cols: []int{cols[first]}}
		for !loop.curves[len(loop.curves)-1].Finish().EqualsTol(loop.curves[0].Start(), tol) {
			end := loop.curves[len(loop.curves)-1].Finish()
			next := -1
			for k := range cs {
				if !used[k] && cs[k].Start().EqualsTol(end, tol) {
					next = k
					break
				}
			}
			if next < 0 {
				return nil, false
			}
			used[next] = true
			loop.curves = append(loop.curves, cs[next])
			loop.cols = append(loop.cols, cols[next])
		}
		loops = append(loops, loop)
	}
	return loops, len(loops) > 0
}

// seamLoops joins the two loops of a ring's face that go round it,
// as a cylinder's two circles do, into one, along the seam where the
// surface's parameter u starts and ends, which the loop runs up and
// back down: STEP readers cut a closed surface there, and mesh a face
// with no seam badly. A loop goes round when it passes from the last
// column of patches to the first, and meets the seam where it does.
// Loops that don't go round, as a hole's, stay as they are.
func (sh *stepSheet) seamLoops(shell *srf.SShell, loops []stepLoop) ([]stepLoop, bool) {
	ni := 0
	for _, c := range sh.grid.Cells {
		ni = max(ni, c.I+1)
	}
	if ni < 3 {
		// Which way a loop passes between two columns is not clear.
		return nil, false
	}
	type crossing struct {
		l, k int // the loop, and its curve that starts on the seam
	}
	var round []crossing
	var others []stepLoop
	for li, l := range loops {
		var at []int
		for k := range l.cols {
			prev := l.cols[(k+len(l.cols)-1)%len(l.cols)]
			if d := l.cols[k] - prev; d == ni-1 || d == 1-ni {
				at = append(at, k)
			}
		}
		switch len(at) {
		case 0:
			others = append(others, l)
		case 1:
			round = append(round, crossing{li, at[0]})
		default:
			return nil, false
		}
	}
	if len(round) == 0 {
		return loops, true
	}
	if len(round) != 2 {
		return nil, false
	}
	// The seam: the sides at u = 0 of the first column's patches, from
	// v = 0 up.
	ps, err := sh.grid.Surface.Patches()
	if err != nil {
		return nil, false
	}
	var seam []srf.SBezier
	for j := range ps[0] {
		seam = append(seam, ps[0][j].EdgeCurve(3))
	}
	// Where along the seam a point is: its piece and parameter, as one
	// number, piece plus parameter.
	along := func(p geom.Vector) (float64, bool) {
		for j := range seam {
			t, _ := seam[j].ClosestPointTo(p, true)
			if t >= -1e-9 && t <= 1+1e-9 && seam[j].PointAt(t).EqualsTol(p, 100*geom.LengthEps) {
				return float64(j) + max(0, min(1, t)), true
			}
		}
		return 0, false
	}
	pa := loops[round[0].l].curves[round[0].k].Start()
	pb := loops[round[1].l].curves[round[1].k].Start()
	ta, okA := along(pa)
	tb, okB := along(pb)
	if !okA || !okB || ta == tb {
		return nil, false
	}
	if ta > tb {
		round[0], round[1], pa, pb, ta, tb = round[1], round[0], pb, pa, tb, ta
	}
	// The seam from ta to tb, piece by piece.
	var up []srf.SBezier
	for j := int(ta); j < len(seam) && float64(j) < tb; j++ {
		c := seam[j]
		lo, hi := max(0, ta-float64(j)), min(1, tb-float64(j))
		if hi-lo < 1e-12 {
			continue
		}
		if hi < 1 {
			var bef, aft srf.SBezier
			c.SplitAt(hi, &bef, &aft)
			c = bef
			lo /= hi
		}
		if lo > 0 {
			var bef, aft srf.SBezier
			c.SplitAt(lo, &bef, &aft)
			c = aft
		}
		up = append(up, c)
	}
	if len(up) == 0 {
		return nil, false
	}
	up[0].Ctrl[0], up[len(up)-1].Ctrl[up[len(up)-1].Degree] = pa, pb
	var down []srf.SBezier
	for i := len(up) - 1; i >= 0; i-- {
		c := up[i]
		c.Reverse()
		down = append(down, c)
	}
	rot := func(c crossing) []srf.SBezier {
		l := loops[c.l].curves
		return append(append([]srf.SBezier(nil), l[c.k:]...), l[:c.k]...)
	}
	// Round the lower loop from where it meets the seam, up the seam,
	// round the upper loop, and down again: each round loop keeps its
	// own direction, and the seam is run once each way.
	var joined stepLoop
	joined.curves = append(joined.curves, rot(round[0])...)
	joined.curves = append(joined.curves, up...)
	joined.curves = append(joined.curves, rot(round[1])...)
	joined.curves = append(joined.curves, down...)
	return append([]stepLoop{joined}, others...), true
}

// WriteFooter ends the file (C++ StepFileWriter::WriteFooter).
func (w *stepFileWriter) WriteFooter() {
	w.printf("\n" +
		"ENDSEC;\n" +
		"\n" +
		"END-ISO-10303-21;\n")
}

// ExportSurfacesTo writes the active group's shell as STEP (C++
// StepFileWriter::ExportSurfacesTo).
func (ss *SolveSpaceUI) ExportSurfacesTo(filename string) {
	g := ss.SK.GetGroup(ss.GW.activeGroupH())
	shell := &g.RunningShell

	if len(shell.Surface) == 0 {
		extra := ""
		if len(g.RunningMesh.Triangles) > 0 {
			extra = "\n\nThe model does contain triangles from a mesh, but " +
				"a triangle mesh cannot be exported as a STEP file. Try " +
				"File -> Export Mesh... instead."
		}
		ss.Error("The model does not contain any surfaces to export.%s", extra)
		return
	}

	f, err := os.Create(filename)
	if err != nil {
		ss.Error("Couldn't write to '%s'", filename)
		return
	}

	var out bytes.Buffer
	w := &stepFileWriter{ss: ss, f: &out, exportParts: true}

	w.WriteHeader()
	w.WriteProductHeader()

	// An assembly: a product a part, placed (roadmap 16.8).
	if parts := ss.SK.AssemblyParts(g.H); len(parts) >= 2 {
		w.WriteAssembly(parts, 1.0/ss.ExportScale)
		w.WriteFooter()
		_, err = f.Write(out.Bytes())
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			ss.Error("Couldn't write to '%s'", filename)
		}
		return
	}

	w.exportShellFaces(shell, 1.0/ss.ExportScale)

	w.printf("#%d=CLOSED_SHELL('',(", w.id)
	for i, af := range w.advancedFaces {
		w.printf("#%d", af)
		if i+1 < len(w.advancedFaces) {
			w.printf(",")
		}
	}
	w.printf("));\n")
	w.printf("#%d=MANIFOLD_SOLID_BREP('brep',#%d);\n", w.id+1, w.id)
	w.printf("#%d=ADVANCED_BREP_SHAPE_REPRESENTATION('',(#%d,#170),#168);\n", w.id+2, w.id+1)
	w.printf("#%d=SHAPE_REPRESENTATION_RELATIONSHIP($,$,#169,#%d);\n", w.id+3, w.id+2)

	w.WriteFooter()

	_, err = f.Write(out.Bytes())
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		ss.Error("Couldn't write to '%s'", filename)
	}
}

// exportShellFaces writes the faces of shell, scaled by scale, adding
// them to w.advancedFaces.
func (w *stepFileWriter) exportShellFaces(shell *srf.SShell, scale float64) {
	sheets := map[core.HSSurface]*stepSheet{}
	if stepOneSurfacePerFace {
		sheets = stepSheets(shell)
	}
	for i := range shell.Surface {
		// C++ ranges over the surfaces by reference, so the export scale
		// below scales the shell itself.
		srfc := &shell.Surface[i]
		if len(srfc.Trim) == 0 {
			continue
		}
		if sh, ok := sheets[srfc.H]; ok {
			// A face of several patches, written as one surface where
			// its first patch comes.
			if sh != nil {
				w.ExportSheet(shell, sh, scale)
				for _, h := range sh.patches {
					sheets[h] = nil
				}
			}
			continue
		}

		// Get all of the loops of Beziers that trim our surface (with
		// each Bezier split so that we use the section as t goes from 0
		// to 1), and the piecewise linearization of those loops in xyz
		// space.
		var sbl srf.SBezierList
		srfc.MakeSectionEdgesInto(shell, nil, &sbl)

		// Apply the export scale factor.
		srfc.ScaleSelfBy(scale)
		sbl.ScaleSelfBy(scale)

		w.ExportSurface(srfc, &sbl)
	}
}

// WriteWireframe writes the curves as a wireframe (C++
// StepFileWriter::WriteWireframe).
func (w *stepFileWriter) WriteWireframe() {
	w.printf("#%d=GEOMETRIC_CURVE_SET('curves',(", w.id)
	for i, c := range w.curves {
		w.printf("#%d", c)
		if i+1 < len(w.curves) {
			w.printf(",")
		}
	}
	w.printf("));\n")
	w.printf("#%d=GEOMETRICALLY_BOUNDED_WIREFRAME_SHAPE_REPRESENTATION"+
		"('',(#%d,#170),#168);\n", w.id+1, w.id)
	w.printf("#%d=SHAPE_REPRESENTATION_RELATIONSHIP($,$,#169,#%d);\n", w.id+2, w.id+1)

	w.id += 3
	w.curves = nil
}
