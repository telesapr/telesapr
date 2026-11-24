package step

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/telesapr/telesapr/pkg/geom"
)

// A file may declare each product's volume, surface area and centre as
// geometric validation properties (the CAx-IF's Recommended Practices
// for Geometric and Assembly Validation Properties, 4.6): a
// PROPERTY_DEFINITION named 'geometric validation property' of the
// product's PRODUCT_DEFINITION_SHAPE, represented by a REPRESENTATION
// of a MEASURE_REPRESENTATION_ITEM (VOLUME_MEASURE or AREA_MEASURE, with
// its unit) or a CARTESIAN_POINT, the centre. Our writer puts all three
// in one representation, other programs one each. What the reader made
// is measured and compared with them, which tells whether the exchange
// held.

// Validation is what a file declares of one product, in millimetres.
type Validation struct {
	// Name is the product's name.
	Name               string
	Volume, Area       float64
	Centre             geom.Vector
	HasVolume, HasArea bool
	HasCentre          bool
	// Shapes are the indices of the shapes the product is made of, in
	// its own space; Whole is a product that holds no shapes itself and
	// is the root of the assembly, measured as every placement.
	Shapes []int
	Whole  bool
}

// readValidations reads the validation properties of the products of
// f, matched to their shapes through the spaces each shape is in
// (spaces[i] for shape i). A product whose shapes can't be told, and
// one used in an assembly that holds none itself, are left out.
func readValidations(f *File, asm *assembly, spaces [][]int) []Validation {
	pdOfPDS := map[int]int{}
	for _, id := range f.Order {
		in := f.Get(id)
		if in.Is("PRODUCT_DEFINITION_SHAPE") {
			if ps := params(in, "PRODUCT_DEFINITION_SHAPE"); len(ps) >= 3 && ps[2].Kind == KindRef {
				pdOfPDS[id] = ps[2].Ref
			}
		}
	}
	// The validation property definitions, by the product definition
	// they are of.
	props := map[int]int{} // PROPERTY_DEFINITION -> PRODUCT_DEFINITION
	for _, id := range f.Order {
		in := f.Get(id)
		if !in.Is("PROPERTY_DEFINITION") || in.Is("PRODUCT_DEFINITION_SHAPE") {
			continue
		}
		ps := params(in, "PROPERTY_DEFINITION")
		if len(ps) < 3 || ps[0].Kind != KindString || ps[2].Kind != KindRef {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(ps[0].Str), "geometric validation property") {
			continue
		}
		def := ps[2].Ref
		if pd, ok := pdOfPDS[def]; ok {
			props[id] = pd
		} else if d := f.Get(def); d != nil && d.Is("PRODUCT_DEFINITION") {
			props[id] = def
		}
	}
	if len(props) == 0 {
		return nil
	}
	byPD := map[int]*Validation{}
	var order []int
	for _, id := range f.Order {
		in := f.Get(id)
		if !in.Is("PROPERTY_DEFINITION_REPRESENTATION") {
			continue
		}
		ps := params(in, "PROPERTY_DEFINITION_REPRESENTATION")
		if len(ps) < 2 || ps[0].Kind != KindRef || ps[1].Kind != KindRef {
			continue
		}
		pd, ok := props[ps[0].Ref]
		if !ok {
			continue
		}
		rep := f.Get(ps[1].Ref)
		if rep == nil || !isRepresentation(rep) {
			continue
		}
		v := byPD[pd]
		if v == nil {
			v = &Validation{Name: productName(f, pd)}
			byPD[pd] = v
			order = append(order, pd)
		}
		readValidationItems(f, rep, v)
	}
	var out []Validation
	for _, pd := range order {
		v := byPD[pd]
		if !v.HasVolume && !v.HasArea && !v.HasCentre {
			continue
		}
		own := map[int]bool{}
		for _, r := range asm.repsOfPD[pd] {
			own[asm.find(r)] = true
		}
		for i, in := range spaces {
			if slices.ContainsFunc(in, func(s int) bool { return own[s] }) {
				v.Shapes = append(v.Shapes, i)
			}
		}
		if len(v.Shapes) == 0 {
			if asm.child[pd] {
				continue
			}
			v.Whole = true
		}
		out = append(out, *v)
	}
	return out
}

// readValidationItems reads the measures and the centre a validation
// representation holds into v.
func readValidationItems(f *File, rep *Instance, v *Validation) {
	ps := repParams(rep)
	length := 1.0
	if ctx := f.Get(ps[2].Ref); ctx != nil {
		length, _ = contextUnits(f, ctx)
	}
	for _, it := range ps[1].List {
		if it.Kind != KindRef {
			continue
		}
		item := f.Get(it.Ref)
		if item == nil {
			continue
		}
		if item.Is("CARTESIAN_POINT") {
			cp := params(item, "CARTESIAN_POINT")
			if len(cp) < 2 || cp[1].Kind != KindList || len(cp[1].List) != 3 {
				continue
			}
			var c [3]float64
			ok := true
			for i, x := range cp[1].List {
				c[i], ok = num(x)
				if !ok {
					break
				}
			}
			if ok {
				v.Centre = geom.Vector{X: c[0], Y: c[1], Z: c[2]}.ScaledBy(length)
				v.HasCentre = true
			}
			continue
		}
		kind, x, unit, ok := measureOf(item)
		if !ok {
			continue
		}
		switch kind {
		case "VOLUME_MEASURE":
			v.Volume = x * unitPower(f, unit, 3, length)
			v.HasVolume = true
		case "AREA_MEASURE":
			v.Area = x * unitPower(f, unit, 2, length)
			v.HasArea = true
		}
	}
}

// measureOf finds the typed measure of a measure item, simple
// (MEASURE_REPRESENTATION_ITEM('volume measure',VOLUME_MEASURE(1.),#u))
// or complex (a MEASURE_WITH_UNIT record among others): its type, value
// and unit.
func measureOf(item *Instance) (kind string, x float64, unit int, ok bool) {
	for _, rec := range item.Records {
		for i, p := range rec.Params {
			if p.Kind != KindTyped || (p.Str != "VOLUME_MEASURE" && p.Str != "AREA_MEASURE") {
				continue
			}
			x, ok = num(p)
			if !ok {
				return "", 0, 0, false
			}
			if i+1 < len(rec.Params) && rec.Params[i+1].Kind == KindRef {
				unit = rec.Params[i+1].Ref
			}
			return p.Str, x, unit, true
		}
	}
	return "", 0, 0, false
}

// unitPower is how many cubic (power 3) or square (2) millimetres one of
// the unit #unit is: a derived unit of a length unit to that power, or,
// when the unit can't be read, the representation's length unit to it.
func unitPower(f *File, unit, power int, length float64) float64 {
	fallback := math.Pow(length, float64(power))
	u := f.Get(unit)
	if u == nil {
		return fallback
	}
	for _, rec := range u.Records {
		for _, p := range rec.Params {
			if p.Kind != KindList || len(p.List) == 0 {
				continue
			}
			factor, read := 1.0, true
			for _, e := range p.List {
				el := f.Get(e.Ref)
				if e.Kind != KindRef || el == nil || !el.Is("DERIVED_UNIT_ELEMENT") {
					read = false
					break
				}
				eps := params(el, "DERIVED_UNIT_ELEMENT")
				if len(eps) < 2 || eps[0].Kind != KindRef || f.Get(eps[0].Ref) == nil {
					read = false
					break
				}
				l, ok := unitFactor(f, f.Get(eps[0].Ref), "LENGTH_UNIT", 0)
				exp, ok2 := num(eps[1])
				if !ok || !ok2 || l <= 0 {
					read = false
					break
				}
				factor *= math.Pow(l, exp)
			}
			if read {
				return factor
			}
		}
	}
	return fallback
}

// productName is the name of the product a product definition is of,
// or "" when it can't be told.
func productName(f *File, pd int) string {
	def := f.Get(pd)
	if def == nil {
		return ""
	}
	ps := params(def, "PRODUCT_DEFINITION")
	if len(ps) < 3 || ps[2].Kind != KindRef {
		return ""
	}
	form := f.Get(ps[2].Ref)
	if form == nil {
		return ""
	}
	fps := form.Records[0].Params
	if len(fps) < 3 || fps[2].Kind != KindRef {
		return ""
	}
	prod := f.Get(fps[2].Ref)
	if prod == nil || !prod.Is("PRODUCT") {
		return ""
	}
	pps := params(prod, "PRODUCT")
	for _, i := range []int{1, 0} {
		if i < len(pps) && pps[i].Kind == KindString && strings.TrimSpace(pps[i].Str) != "" {
			return pps[i].Str
		}
	}
	return ""
}

// The tolerances a declared property is held to. The Recommended
// Practices call a difference under 1% of the volume or the area good;
// the centre is held to 1% of the part's size, the diagonal of its box,
// as the same share of its extent.
const (
	validationRel    = 0.01
	validationCentre = 0.01
)

// validationFiner is how much finer than the link's own the shapes of a
// product whose measures differ are tessellated again before saying so:
// the chords of a thin rod's curved side at the link's tolerance can
// take more than 1% of its volume, at a twentieth a twentieth of that.
const validationFiner = 20

// measured is a mesh's volume, area and first moment, and its box.
type measured struct {
	volume, area float64
	moment       geom.Vector
	lo, hi       geom.Vector
}

func (m *measured) add(o measured) {
	m.volume += o.volume
	m.area += o.area
	m.moment = m.moment.Plus(o.moment)
	growBox(o.lo, &m.lo, &m.hi)
	growBox(o.hi, &m.lo, &m.hi)
}

func emptyMeasured() measured {
	inf := math.Inf(1)
	return measured{lo: geom.Vector{X: inf, Y: inf, Z: inf}, hi: geom.Vector{X: -inf, Y: -inf, Z: -inf}}
}

// measure measures a shape's faces, in its own space.
func measure(faces [][]meshTri) measured {
	m := emptyMeasured()
	for _, tris := range faces {
		for _, t := range tris {
			a, b, c := t.p[0], t.p[1], t.p[2]
			tv := a.Dot(b.Cross(c)) / 6
			m.volume += tv
			m.moment = m.moment.Plus(a.Plus(b).Plus(c).ScaledBy(tv / 4))
			m.area += b.Minus(a).Cross(c.Minus(a)).Magnitude() / 2
			for _, p := range t.p {
				growBox(p, &m.lo, &m.hi)
			}
		}
	}
	return m
}

// placedMeasure is m as placed by t: a rigid placement keeps the volume
// and area and moves the centre; a mirror turns the volume's sign,
// which the placed triangles' turned order (FromModel) turns back.
func placedMeasure(m measured, t Transform) measured {
	out := emptyMeasured()
	out.volume, out.area = m.volume, m.area
	if m.volume != 0 {
		out.moment = t.Apply(m.moment.ScaledBy(1 / m.volume)).ScaledBy(m.volume)
	}
	for i := range 8 {
		p := m.lo
		if i&1 != 0 {
			p.X = m.hi.X
		}
		if i&2 != 0 {
			p.Y = m.hi.Y
		}
		if i&4 != 0 {
			p.Z = m.hi.Z
		}
		growBox(t.Apply(p), &out.lo, &out.hi)
	}
	return out
}

// checkValidations compares each product's declared properties with its
// shapes' meshes, at the link's tolerance and, where they differ, again
// at a finer one; each difference left is a sentence in rep.Mismatches,
// and each product that agrees counts in rep.Validated.
func checkValidations(m *Model, meshes []shapeMesh, tol float64, rep *Report) {
	if len(m.Validations) == 0 {
		return
	}
	coarse := make([]measured, len(meshes))
	for i := range meshes {
		coarse[i] = measure(meshes[i].faces)
	}
	fine := map[int]measured{}
	fineOf := func(i int) measured {
		if f, ok := fine[i]; ok {
			return f
		}
		var scratch Report
		scratch.Reasons, scratch.Details = map[string]int{}, map[string]string{}
		f := measure(tessellateShape(m.Shapes[i], tol/validationFiner, &scratch).faces)
		fine[i] = f
		return f
	}
	total := func(v *Validation, of func(int) measured) measured {
		sum := emptyMeasured()
		if v.Whole {
			for _, pl := range m.Placements {
				sum.add(placedMeasure(of(pl.Shape), pl.T))
			}
			return sum
		}
		for _, i := range v.Shapes {
			sum.add(of(i))
		}
		return sum
	}
	for i := range m.Validations {
		v := &m.Validations[i]
		if validationDiffers(v, total(v, func(i int) measured { return coarse[i] })) == "" {
			rep.Validated++
			continue
		}
		if why := validationDiffers(v, total(v, fineOf)); why != "" {
			name := "the part"
			if v.Name != "" {
				name = fmt.Sprintf("%q", v.Name)
			}
			rep.Mismatches = append(rep.Mismatches,
				fmt.Sprintf("%s is not what the file declares it to be: %s", name, why))
			continue
		}
		rep.Validated++
	}
}

// validationDiffers says how what was measured differs from what v
// declares, beyond the tolerances, or is "".
func validationDiffers(v *Validation, got measured) string {
	var out []string
	rel := func(what string, declared, read float64, unit string) {
		if declared == 0 || math.Abs(read-declared) <= validationRel*math.Abs(declared) {
			return
		}
		out = append(out, fmt.Sprintf("%s %.6g %s declared, %.6g read (%+.2g%%)",
			what, declared, unit, read, 100*(read-declared)/declared))
	}
	if v.HasVolume {
		rel("volume", v.Volume, got.volume, "mm³")
	}
	if v.HasArea {
		rel("area", v.Area, got.area, "mm²")
	}
	if v.HasCentre && got.volume != 0 {
		c := got.moment.ScaledBy(1 / got.volume)
		size := got.hi.Minus(got.lo).Magnitude()
		if d := c.Minus(v.Centre).Magnitude(); d > validationCentre*size {
			out = append(out, fmt.Sprintf("centre %.3f mm from the one declared", d))
		}
	}
	return strings.Join(out, ", ")
}
