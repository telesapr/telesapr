package ui

import (
	"strings"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// A STEP file of an assembly (roadmap 16.8, D5): the model is the top
// product, #177 with the shape #169 the header writes, and each part a
// product of its own, written once in its own coordinates, with a solid
// of its faces (each in its colour, as a part's faces are); each place
// a part goes is a NEXT_ASSEMBLY_USAGE_OCCURRENCE of it in the model,
// with the rotation and offset that put it there. A file linked twice
// is one product used twice.

// stepString quotes s for a STEP string.
func stepString(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

// WriteAssembly writes parts as products of the model's, the lengths
// scaled by scale.
func (w *stepFileWriter) WriteAssembly(parts []sketch.AssemblyPart, scale float64) {
	type product struct {
		definition int // PRODUCT_DEFINITION
		shape      int // its SHAPE_REPRESENTATION
		axis       int // its placement, at its origin
	}
	products := map[string]product{}
	for _, p := range parts {
		if _, ok := products[p.Key]; ok {
			continue
		}
		// Each part's solid stands alone: no point or edge is shared
		// with another's.
		w.pointAliases, w.edgeCurveAliases, w.curveAliases, w.orientedEdgeAliases = nil, nil, nil, nil
		w.advancedFaces = w.advancedFaces[:0]

		id := w.id
		name := stepString(p.Name)
		w.printf("#%d=PRODUCT(%s,%s,'',(#180));\n", id, name, name)
		w.printf("#%d=PRODUCT_DEFINITION_FORMATION_WITH_SPECIFIED_SOURCE('',%s,#%d,.MADE.);\n", id+1, name, id)
		w.printf("#%d=PRODUCT_DEFINITION('design',%s,#%d,#178);\n", id+2, name, id+1)
		w.printf("#%d=PRODUCT_DEFINITION_SHAPE('','',#%d);\n", id+3, id+2)
		w.printf("#%d=AXIS2_PLACEMENT_3D('',#173,#171,#172);\n", id+4)
		w.printf("#%d=SHAPE_REPRESENTATION(%s,(#%d),#168);\n", id+5, name, id+4)
		w.printf("#%d=SHAPE_DEFINITION_REPRESENTATION(#%d,#%d);\n", id+6, id+3, id+5)
		products[p.Key] = product{definition: id + 2, shape: id + 5, axis: id + 4}
		w.id = id + 7

		w.exportShellFaces(p.Shell, scale)
		w.printf("#%d=CLOSED_SHELL('',(", w.id)
		for i, af := range w.advancedFaces {
			w.printf("#%d", af)
			if i+1 < len(w.advancedFaces) {
				w.printf(",")
			}
		}
		w.printf("));\n")
		w.printf("#%d=MANIFOLD_SOLID_BREP(%s,#%d);\n", w.id+1, name, w.id)
		w.printf("#%d=ADVANCED_BREP_SHAPE_REPRESENTATION(%s,(#%d,#%d),#168);\n", w.id+2, name, w.id+1, id+4)
		w.printf("#%d=SHAPE_REPRESENTATION_RELATIONSHIP($,$,#%d,#%d);\n", w.id+3, id+5, w.id+2)
		w.id += 4
		w.printf("\n")
	}

	for k, p := range parts {
		pr := products[p.Key]
		id := w.id
		o := p.Offset.ScaledBy(scale)
		z, x := p.Q.RotationN(), p.Q.RotationU()
		w.printf("#%d=NEXT_ASSEMBLY_USAGE_OCCURRENCE('%d',%s,'',#177,#%d,$);\n", id, k+1, stepString(p.Name), pr.definition)
		w.printf("#%d=PRODUCT_DEFINITION_SHAPE('','',#%d);\n", id+1, id)
		w.printf(stepCartesianPointFormat, id+2, o.X, o.Y, o.Z)
		w.printf("#%d=DIRECTION('',(%s));\n", id+3, stepDirection(z))
		w.printf("#%d=DIRECTION('',(%s));\n", id+4, stepDirection(x))
		w.printf("#%d=AXIS2_PLACEMENT_3D('',#%d,#%d,#%d);\n", id+5, id+2, id+3, id+4)
		w.printf("#%d=ITEM_DEFINED_TRANSFORMATION('','',#%d,#%d);\n", id+6, pr.axis, id+5)
		w.printf("#%d=(REPRESENTATION_RELATIONSHIP('','',#%d,#169)"+
			"REPRESENTATION_RELATIONSHIP_WITH_TRANSFORMATION(#%d)"+
			"SHAPE_REPRESENTATION_RELATIONSHIP());\n", id+7, pr.shape, id+6)
		w.printf("#%d=CONTEXT_DEPENDENT_SHAPE_REPRESENTATION(#%d,#%d);\n", id+8, id+7, id+1)
		w.id = id + 9
	}
}

// stepDirection is a unit vector as a STEP direction's three numbers.
func stepDirection(v geom.Vector) string {
	v = v.WithMagnitude(1)
	return strings.Join([]string{cfmtReal(v.X), cfmtReal(v.Y), cfmtReal(v.Z)}, ",")
}

// cfmtReal writes a real for STEP: ten decimals at most, with the point
// a STEP real must have, as "1." or "0.7071067812".
func cfmtReal(v float64) string { return strings.TrimRight(sprintf("%.10f", v), "0") }
