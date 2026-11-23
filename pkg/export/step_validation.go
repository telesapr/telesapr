package export

import (
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// Each part's volume, surface area and centre are written as geometric
// validation properties at the part's level, as the CAx-IF's
// Recommended Practices for Geometric and Assembly Validation Properties
// (4.6, 4.4 and 4.12) have them, the three in one REPRESENTATION: a
// receiver measures the solid it read and compares, which tells it the
// exchange held. They are measured on the shell's mesh at its own
// tolerance made finer, so they lie within the chords' sag of the exact
// ones.

// validation is what a part's validation properties hold.
type validation struct {
	volume, area float64
	centre       geom.Vector
}

// validationFine is how much finer than the shell's own tolerance its
// surfaces are meshed to be measured: at the model's own the knob's
// volume was 0.14% short of the exact one, at a twentieth 0.014%, for
// 0.4 s more. The trims keep the points the shell was built with, so a
// thread's volume stays 0.17% short.
const validationFine = 20

// validationOf measures shell, its lengths times scale, on a mesh
// validationFine times finer than its own.
func validationOf(shell *srf.SShell, scale float64) validation {
	fine := *shell
	fine.Tol = srf.Tol(shell.Tol.ChordTolMm()/validationFine, shell.Tol.MaxSegments()*validationFine)
	fine.Surface = slices.Clone(shell.Surface)
	for i := range fine.Surface {
		fine.Surface[i].Tol = fine.Tol
	}
	var m srf.SMesh
	fine.TriangulateInto(&m)
	var v validation
	var moment geom.Vector
	for _, t := range m.Triangles {
		a, b, c := t.A, t.B, t.C
		// The tetrahedron from the origin: its signed volume, and its
		// centre a quarter of the way from the origin to the corners'
		// sum.
		tv := a.Dot(b.Cross(c)) / 6
		v.volume += tv
		moment = moment.Plus(a.Plus(b).Plus(c).ScaledBy(tv / 4))
		v.area += b.Minus(a).Cross(c.Minus(a)).Magnitude() / 2
	}
	if v.volume != 0 {
		v.centre = moment.ScaledBy(1 / v.volume).ScaledBy(scale)
	}
	v.volume *= scale * scale * scale
	v.area *= scale * scale
	return v
}

// writeValidation writes v as the validation properties of the product
// definition shape def, named for name.
func (w *stepFileWriter) writeValidation(def int, name string, v validation) {
	if v.volume <= 0 {
		return
	}
	if w.areaUnit == 0 {
		// The units of area and volume, mm² and mm³, from the length
		// unit #158.
		id := w.id
		w.printf("#%d=DERIVED_UNIT_ELEMENT(#158,2.);\n", id)
		// Subtypes of DERIVED_UNIT alone, written as themselves: as a
		// complex instance SFA calls them wrongly mapped.
		w.printf("#%d=AREA_UNIT((#%d));\n", id+1, id)
		w.printf("#%d=DERIVED_UNIT_ELEMENT(#158,3.);\n", id+2)
		w.printf("#%d=VOLUME_UNIT((#%d));\n", id+3, id+2)
		w.areaUnit, w.volumeUnit = id+1, id+3
		w.id = id + 4
	}
	id := w.id
	w.printf("#%d=MEASURE_REPRESENTATION_ITEM('volume measure',VOLUME_MEASURE(%s),#%d);\n",
		id, cfmtReal(v.volume), w.volumeUnit)
	w.printf("#%d=MEASURE_REPRESENTATION_ITEM('surface area measure',AREA_MEASURE(%s),#%d);\n",
		id+1, cfmtReal(v.area), w.areaUnit)
	w.printf("#%d=CARTESIAN_POINT('centre point',(%s,%s,%s));\n",
		id+2, cfmtReal(v.centre.X), cfmtReal(v.centre.Y), cfmtReal(v.centre.Z))
	w.printf("#%d=REPRESENTATION('',(#%d,#%d,#%d),#168);\n", id+3, id, id+1, id+2)
	w.printf("#%d=PROPERTY_DEFINITION('geometric validation property',%s,#%d);\n",
		id+4, stepString("volume, area and centroid of "+name), def)
	w.printf("#%d=PROPERTY_DEFINITION_REPRESENTATION(#%d,#%d);\n", id+5, id+4, id+3)
	w.id = id + 6
}
