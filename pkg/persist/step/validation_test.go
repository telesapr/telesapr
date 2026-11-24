package step

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// withValidation is the STEP file data with its first product's
// validation properties added as the Recommended Practices write them
// and other programs do, a property definition each: volume and area
// in a derived unit of a length unit of metres when inMetres, else of
// the representation's own, and the centre in the shape's context.
func withValidation(t *testing.T, data []byte, volume, area float64, centre [3]float64, inMetres bool) []byte {
	t.Helper()
	f, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	pds, ctx, length := 0, 0, 0
	for _, id := range f.Order {
		in := f.Get(id)
		if in.Is("SHAPE_DEFINITION_REPRESENTATION") && pds == 0 {
			ps := params(in, "SHAPE_DEFINITION_REPRESENTATION")
			pds = ps[0].Ref
			ctx = repParams(f.Get(ps[1].Ref))[2].Ref
		}
		if in.Is("LENGTH_UNIT") && length == 0 {
			length = id
		}
	}
	if pds == 0 || ctx == 0 || length == 0 {
		t.Fatal("no product shape, context or length unit")
	}
	id := slices.Max(f.Order) + 1
	var b strings.Builder
	line := func(format string, a ...any) {
		fmt.Fprintf(&b, "#%d=", id)
		fmt.Fprintf(&b, format, a...)
		b.WriteString(";\n")
		id++
	}
	unit := length
	scale := 1.0
	if inMetres {
		unit = id
		line("(LENGTH_UNIT()NAMED_UNIT(*)SI_UNIT($,.METRE.))")
		scale = 1e-3
	}
	vu := id + 1
	line("DERIVED_UNIT_ELEMENT(#%d,3.)", unit)
	line("(DERIVED_UNIT((#%d))VOLUME_UNIT())", id-1)
	au := id + 1
	line("DERIVED_UNIT_ELEMENT(#%d,2.)", unit)
	line("(AREA_UNIT()DERIVED_UNIT((#%d)))", id-1)
	prop := func(what, item string) {
		line("PROPERTY_DEFINITION('geometric validation property','%s',#%d)", what, pds)
		line("%s", item)
		line("REPRESENTATION('%s',(#%d),#%d)", what, id-1, ctx)
		line("PROPERTY_DEFINITION_REPRESENTATION(#%d,#%d)", id-3, id-1)
	}
	prop("volume", fmt.Sprintf("MEASURE_REPRESENTATION_ITEM('volume measure',VOLUME_MEASURE(%.17g),#%d)",
		volume*scale*scale*scale, vu))
	prop("surface area", fmt.Sprintf("MEASURE_REPRESENTATION_ITEM('surface area measure',AREA_MEASURE(%.17g),#%d)",
		area*scale*scale, au))
	prop("centroid", fmt.Sprintf("CARTESIAN_POINT('centre point',(%.17g,%.17g,%.17g))", centre[0], centre[1], centre[2]))
	s := string(data)
	at := strings.LastIndex(s, "ENDSEC;")
	return []byte(s[:at] + b.String() + s[at:])
}

// The validation properties of OpenCASCADE's shapes, as it measures
// them, agree with what is read, as a mesh at the link's tolerance or,
// for the curved ones, after the finer look; declared in metres they
// still do; a volume 5% off and a centre moved are said, by part.
func TestValidationProperties(t *testing.T) {
	t.Parallel()
	for _, e := range readExchange(t) {
		path := filepath.Join(exchangeDir, e.name+"_AP242.step")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		c := [3]float64{e.centre.X, e.centre.Y, e.centre.Z}
		t.Run(e.name, func(t *testing.T) {
			t.Parallel()
			for _, metres := range []bool{false, true} {
				res, err := Read(withValidation(t, data, e.volume, e.area, c, metres), Options{})
				if err != nil {
					t.Fatal(err)
				}
				r := res.Report
				if r.Validated != 1 || len(r.Mismatches) > 0 || r.Problem() != "" {
					t.Errorf("metres %v: validated %d, problem %q", metres, r.Validated, r.Problem())
				}
			}
			off := [3]float64{c[0] + 5, c[1], c[2]}
			res, err := Read(withValidation(t, data, e.volume*1.05, e.area, off, false), Options{})
			if err != nil {
				t.Fatal(err)
			}
			r := res.Report
			if r.Validated != 0 || len(r.Mismatches) != 1 {
				t.Fatalf("validated %d, mismatches %q", r.Validated, r.Mismatches)
			}
			p := r.Problem()
			for _, want := range []string{"volume", "declared", "centre"} {
				if !strings.Contains(p, want) {
					t.Errorf("problem %q says nothing of %s", p, want)
				}
			}
			if strings.Contains(p, "area") {
				t.Errorf("problem %q blames the area, which is right", p)
			}
		})
	}
}

// A file with no validation properties says nothing of them.
func TestValidationPropertiesAbsent(t *testing.T) {
	t.Parallel()
	res, err := ReadFile(filepath.Join(exchangeDir, "hole_AP242.step"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if r := res.Report; r.Validated != 0 || len(r.Mismatches) > 0 || r.Problem() != "" {
		t.Errorf("validated %d, problem %q", r.Validated, r.Problem())
	}
}
