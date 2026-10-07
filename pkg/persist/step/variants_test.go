package step

import (
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// shapeText is a shape fixture's text.
func shapeText(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(shapesDir, name+".step"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// addEntities puts instances before the end of the data section.
func addEntities(text, entities string) string {
	i := strings.LastIndex(text, "ENDSEC;")
	return text[:i] + entities + text[i:]
}

func mustRead(t *testing.T, text string) *Result {
	t.Helper()
	res, err := Read([]byte(text), Options{})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func volumeOf(res *Result) float64 { return check.Solid(nil, &res.Mesh).Volume }

func TestInchUnit(t *testing.T) {
	text := strings.Replace(shapeText(t, "box"), "#166 = ( LENGTH_UNIT() NAMED_UNIT(*) SI_UNIT(.MILLI.,.METRE.) );",
		"#166 = ( CONVERSION_BASED_UNIT('INCH',#900) LENGTH_UNIT() NAMED_UNIT(#901) );\n"+
			"#900 = LENGTH_MEASURE_WITH_UNIT(LENGTH_MEASURE(25.4),#902);\n"+
			"#901 = DIMENSIONAL_EXPONENTS(1.,0.,0.,0.,0.,0.,0.);\n"+
			"#902 = ( LENGTH_UNIT() NAMED_UNIT(*) SI_UNIT(.MILLI.,.METRE.) );", 1)
	res := mustRead(t, text)
	if res.Report.Unit != "inches" {
		t.Errorf("unit %q", res.Report.Unit)
	}
	want := 6000 * 25.4 * 25.4 * 25.4
	if v := volumeOf(res); math.Abs(v-want) > 1e-6*want {
		t.Errorf("volume %g, want %g", v, want)
	}
	hi, _ := res.Mesh.Bounding()
	if math.Abs(hi.Z-30*25.4) > 1e-9 {
		t.Errorf("top at %g", hi.Z)
	}
}

func TestMetreUnitAndAngleInDegrees(t *testing.T) {
	// A frustum in metres, its half angle in degrees.
	text := shapeText(t, "cone_frustum")
	text = strings.Replace(text, "SI_UNIT(.MILLI.,.METRE.)", "SI_UNIT($,.METRE.)", 1)
	text = strings.Replace(text, "( NAMED_UNIT(*) PLANE_ANGLE_UNIT() SI_UNIT($,.RADIAN.) )",
		"( CONVERSION_BASED_UNIT('DEGREE',#900) NAMED_UNIT(*) PLANE_ANGLE_UNIT() );\n"+
			"#900 = PLANE_ANGLE_MEASURE_WITH_UNIT(PLANE_ANGLE_MEASURE(0.0174532925199433),#901);\n"+
			"#901 = ( NAMED_UNIT(*) PLANE_ANGLE_UNIT() SI_UNIT($,.RADIAN.) )", 1)
	// Write the cone's angle in degrees.
	start := strings.Index(text, "CONICAL_SURFACE(")
	end := start + strings.Index(text[start:], ");")
	args := strings.Split(text[start+len("CONICAL_SURFACE("):end], ",")
	var rad float64
	if _, err := fmtSscan(args[len(args)-1], &rad); err != nil {
		t.Fatal(err)
	}
	args[len(args)-1] = formatFloat(rad * 180 / math.Pi)
	text = text[:start] + "CONICAL_SURFACE(" + strings.Join(args, ",") + text[end:]
	res := mustRead(t, text)
	if res.Report.Unit != "metres" {
		t.Errorf("unit %q", res.Report.Unit)
	}
	if p := res.Report.Problem(); p != "" {
		t.Fatal(p)
	}
	want := math.Pi * 9 / 3 * (36 + 18 + 9) * 1e9
	if v := volumeOf(res); math.Abs(v-want) > 2e-3*want {
		t.Errorf("volume %g, want %g", v, want)
	}
}

// assemblyEntities place the box (#10, part #5) twice in an assembly:
// once as the CAx-IF practice writes it, the part's representation
// first, and once the other way round, as SolidWorks writes it.
const assemblyEntities = `
#1000 = PRODUCT('asm','asm','',(#8));
#1001 = PRODUCT_DEFINITION_FORMATION('','',#1000);
#1002 = PRODUCT_DEFINITION('design','',#1001,#9);
#1003 = PRODUCT_DEFINITION_SHAPE('','',#1002);
#1004 = SHAPE_REPRESENTATION('asm',(#11,#1010,#1020),#165);
#1005 = SHAPE_DEFINITION_REPRESENTATION(#1003,#1004);
#1010 = AXIS2_PLACEMENT_3D('',#1011,#13,#14);
#1011 = CARTESIAN_POINT('',(100.,0.,0.));
#1020 = AXIS2_PLACEMENT_3D('',#1021,#1022,#1023);
#1021 = CARTESIAN_POINT('',(0.,50.,0.));
#1022 = DIRECTION('',(0.,0.,1.));
#1023 = DIRECTION('',(0.,1.,0.));
#1030 = NEXT_ASSEMBLY_USAGE_OCCURRENCE('1','a','',#1002,#5,$);
#1031 = PRODUCT_DEFINITION_SHAPE('','',#1030);
#1032 = ( REPRESENTATION_RELATIONSHIP('','',#10,#1004) REPRESENTATION_RELATIONSHIP_WITH_TRANSFORMATION(#1033) SHAPE_REPRESENTATION_RELATIONSHIP() );
#1033 = ITEM_DEFINED_TRANSFORMATION('','',#11,#1010);
#1034 = CONTEXT_DEPENDENT_SHAPE_REPRESENTATION(#1032,#1031);
#1040 = NEXT_ASSEMBLY_USAGE_OCCURRENCE('2','b','',#1002,#5,$);
#1041 = PRODUCT_DEFINITION_SHAPE('','',#1040);
#1042 = ( REPRESENTATION_RELATIONSHIP('','',#1004,#10) REPRESENTATION_RELATIONSHIP_WITH_TRANSFORMATION(#1043) SHAPE_REPRESENTATION_RELATIONSHIP() );
#1043 = ITEM_DEFINED_TRANSFORMATION('','',#1020,#11);
#1044 = CONTEXT_DEPENDENT_SHAPE_REPRESENTATION(#1042,#1041);
`

func TestAssembly(t *testing.T) {
	res := mustRead(t, addEntities(shapeText(t, "box"), assemblyEntities))
	if res.Report.Shapes != 1 || res.Report.Placements != 2 {
		t.Fatalf("%d shapes placed %d times", res.Report.Shapes, res.Report.Placements)
	}
	r := check.Solid(nil, &res.Mesh)
	if math.Abs(r.Volume-12000) > 1e-6 || r.NakedEdges != 0 {
		t.Errorf("volume %g, %d naked edges", r.Volume, r.NakedEdges)
	}
	// The first copy is moved to x = 100; the second turned a quarter
	// about z and moved to y = 50, so its x runs from -20 to 0.
	want := []struct{ lo, hi geom.Vector }{
		{geom.Vector{X: 100}, geom.Vector{X: 110, Y: 20, Z: 30}},
		{geom.Vector{X: -20, Y: 50}, geom.Vector{Y: 60, Z: 30}},
	}
	n := len(res.Mesh.Triangles) / 2
	for k, w := range want {
		lo := geom.Vector{X: math.Inf(1), Y: math.Inf(1), Z: math.Inf(1)}
		hi := lo.ScaledBy(-1)
		for _, tr := range res.Mesh.Triangles[k*n : (k+1)*n] {
			for _, p := range []geom.Vector{tr.A, tr.B, tr.C} {
				growBox(p, &lo, &hi)
			}
		}
		if !lo.EqualsTol(w.lo, 1e-9) || !hi.EqualsTol(w.hi, 1e-9) {
			t.Errorf("copy %d spans %v to %v, want %v to %v", k, lo, hi, w.lo, w.hi)
		}
	}
	if len(res.Vertices) != 16 {
		t.Errorf("%d vertices, want 16", len(res.Vertices))
	}
}

func TestMirroredPlacement(t *testing.T) {
	// A mapped item whose target turns the axes over keeps the mesh's
	// outside out.
	text := addEntities(shapeText(t, "box"), `
#1000 = SHAPE_REPRESENTATION('outer',(#1001),#165);
#1001 = MAPPED_ITEM('',#1002,#1003);
#1002 = REPRESENTATION_MAP(#11,#10);
#1003 = CARTESIAN_TRANSFORMATION_OPERATOR_3D('',#1004,#1005,#12,-1.,#1006);
#1004 = DIRECTION('',(1.,0.,0.));
#1005 = DIRECTION('',(0.,1.,0.));
#1006 = DIRECTION('',(0.,0.,1.));
`)
	res := mustRead(t, text)
	if res.Report.Placements != 1 {
		t.Fatalf("%d placements", res.Report.Placements)
	}
	if v := volumeOf(res); math.Abs(v-6000) > 1e-6 {
		t.Errorf("volume %g, want 6000", v)
	}
}

func TestColours(t *testing.T) {
	text := addEntities(shapeText(t, "box"), `
#1000 = STYLED_ITEM('',(#1001),#15);
#1001 = PRESENTATION_STYLE_ASSIGNMENT((#1002));
#1002 = SURFACE_STYLE_USAGE(.BOTH.,#1003);
#1003 = SURFACE_SIDE_STYLE('',(#1004));
#1004 = SURFACE_STYLE_FILL_AREA(#1005);
#1005 = FILL_AREA_STYLE('',(#1006));
#1006 = FILL_AREA_STYLE_COLOUR('',#1007);
#1007 = COLOUR_RGB('',0.2,0.4,0.6);
#1010 = OVER_RIDING_STYLED_ITEM('',(#1011),#17,#1000);
#1011 = PRESENTATION_STYLE_ASSIGNMENT((#1012,#1020));
#1020 = CURVE_STYLE('',#1021,POSITIVE_LENGTH_MEASURE(0.1),#1022);
#1021 = DRAUGHTING_PRE_DEFINED_CURVE_FONT('continuous');
#1022 = COLOUR_RGB('',0.,1.,0.);
#1012 = SURFACE_STYLE_USAGE(.BOTH.,#1013);
#1013 = SURFACE_SIDE_STYLE('',(#1014));
#1014 = SURFACE_STYLE_FILL_AREA(#1015);
#1015 = FILL_AREA_STYLE('',(#1016));
#1016 = FILL_AREA_STYLE_COLOUR('',#1017);
#1017 = DRAUGHTING_PRE_DEFINED_COLOUR('red');
`)
	res := mustRead(t, text)
	count := map[core.RGBA]int{}
	for _, tr := range res.Mesh.Triangles {
		count[tr.Meta.RGBA]++
	}
	red := core.NewRGBA(255, 0, 0, 255)
	blue := core.NewRGBA(51, 102, 153, 255)
	if count[red] != 2 || count[blue] != 10 || len(count) != 2 {
		t.Errorf("colours %v", count)
	}
}

func TestUnsupportedSurface(t *testing.T) {
	// One face of the box on a surface this reader doesn't know.
	text := strings.Replace(shapeText(t, "box"), "#52 = PLANE('',#53);", "#52 = FOO_SURFACE('',#53);", 1)
	res := mustRead(t, text)
	r := res.Report
	if r.Skipped != 1 || r.Faces != 5 {
		t.Errorf("%d faces read, %d skipped", r.Faces, r.Skipped)
	}
	if p := r.Problem(); !strings.Contains(p, "1 of 6 faces were left out") || !strings.Contains(p, "FOO_SURFACE (1)") {
		t.Errorf("problem %q", p)
	}
	if d := r.Details["unsupported FOO_SURFACE"]; !strings.Contains(d, "#52") {
		t.Errorf("detail %q", d)
	}
}

func TestBadEntity(t *testing.T) {
	// A circle with no radius: the faces on it are left out, and the
	// report names the entity.
	text := strings.ReplaceAll(shapeText(t, "cylinder"), "CIRCLE('',", "CIRCLE('',$,")
	res := mustRead(t, text)
	if p := res.Report.Problem(); !strings.Contains(p, "bad CIRCLE") {
		t.Errorf("problem %q", p)
	}
}

func TestNoShape(t *testing.T) {
	path := filepath.Join("..", "..", "..", "testdata", "step", "corpus", "ap203e2_csg.stp")
	res, err := ReadFile(path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if p := res.Report.Problem(); !strings.Contains(p, "CSG_SOLID (1)") || !strings.Contains(p, "no solid or shell") {
		t.Errorf("problem %q", p)
	}
	if len(res.Mesh.Triangles) != 0 {
		t.Error("a mesh from nothing")
	}
}

func TestFacetedAndShellModel(t *testing.T) {
	// A faceted tetrahedron as a shell based surface model, with no
	// representation around it, the plane of every face wrong.
	text := `ISO-10303-21;
HEADER;
FILE_SCHEMA(('CONFIG_CONTROL_DESIGN'));
ENDSEC;
DATA;
#1 = SHELL_BASED_SURFACE_MODEL('',(#2));
#2 = CLOSED_SHELL('',(#10,#11,#12,#13));
#10 = FACE_SURFACE('',(#20),#30,.T.);
#11 = FACE_SURFACE('',(#21),#30,.T.);
#12 = FACE_SURFACE('',(#22),#30,.T.);
#13 = FACE_SURFACE('',(#23),#30,.T.);
#20 = FACE_OUTER_BOUND('',#40,.T.);
#21 = FACE_OUTER_BOUND('',#41,.T.);
#22 = FACE_OUTER_BOUND('',#42,.T.);
#23 = FACE_BOUND('',#43,.F.);
#30 = PLANE('',#31);
#31 = AXIS2_PLACEMENT_3D('',#50,$,$);
#40 = POLY_LOOP('',(#50,#52,#51));
#41 = POLY_LOOP('',(#50,#51,#53));
#42 = POLY_LOOP('',(#51,#52,#53));
#43 = POLY_LOOP('',(#50,#52,#53));
#50 = CARTESIAN_POINT('',(0.,0.,0.));
#51 = CARTESIAN_POINT('',(6.,0.,0.));
#52 = CARTESIAN_POINT('',(0.,6.,0.));
#53 = CARTESIAN_POINT('',(0.,0.,6.));
ENDSEC;
END-ISO-10303-21;
`
	res := mustRead(t, text)
	if p := res.Report.Problem(); p != "" {
		t.Fatal(p)
	}
	r := check.Solid(nil, &res.Mesh)
	if math.Abs(r.Volume-36) > 1e-9 || r.NakedEdges != 0 {
		t.Errorf("volume %g, %d naked edges", r.Volume, r.NakedEdges)
	}
	if Protocol(res.Report.Schema) != "AP203" {
		t.Errorf("protocol %q", Protocol(res.Report.Schema))
	}
}

func TestBrepWithVoids(t *testing.T) {
	// A box with a box-shaped void: the inner shell is written turned
	// inside out.
	outer := shapeText(t, "box")
	text := strings.Replace(outer, "#15 = MANIFOLD_SOLID_BREP('',#16);",
		"#15 = BREP_WITH_VOIDS('',#16,(#2000));\n#2000 = ORIENTED_CLOSED_SHELL('',*,#2001,.F.);\n"+
			"#2001 = CLOSED_SHELL('',(#2017,#2057,#2097,#2119,#2141,#2153));", 1)
	// The void is the box again, scaled to a half about its centre:
	// copy its faces under new numbers, scaling every point.
	var b strings.Builder
	for line := range strings.SplitSeq(outer[strings.Index(outer, "#17 = "):strings.Index(outer, "#165 = ")], ";\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		line = renumber(line, 2000)
		if strings.Contains(line, "CARTESIAN_POINT") {
			line = scalePoint(line)
		}
		b.WriteString(line + ";\n")
	}
	res := mustRead(t, addEntities(text, b.String()))
	if p := res.Report.Problem(); p != "" {
		t.Fatal(p)
	}
	r := check.Solid(nil, &res.Mesh)
	if want := 6000 - 750.0; math.Abs(r.Volume-want) > 1e-6 || r.NakedEdges != 0 {
		t.Errorf("volume %g, want %g; %d naked edges", r.Volume, want, r.NakedEdges)
	}
}

func TestReadRecovers(t *testing.T) {
	if _, err := Read([]byte("not step"), Options{}); err == nil {
		t.Error("no error for a file that isn't STEP")
	}
	if _, err := ReadFile(filepath.Join(shapesDir, "missing.step"), Options{}); err == nil {
		t.Error("no error for a missing file")
	}
	// A reference loop: the curve of an edge is a surface curve that
	// names itself. The faces on it are left out; nothing hangs.
	text := shapeText(t, "cylinder")
	i := strings.Index(text, "EDGE_CURVE(")
	j := i + strings.Index(text[i:], ");")
	args := strings.Split(text[i:j], ",")
	args[3] = "#3000"
	text = text[:i] + strings.Join(args, ",") + text[j:]
	text = addEntities(text, "#3000 = SURFACE_CURVE('',#3000,(),.CURVE_3D.);\n")
	res, err := Read([]byte(text), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if p := res.Report.Problem(); !strings.Contains(p, "SURFACE_CURVE") {
		t.Errorf("problem %q", p)
	}
}

func fmtSscan(s string, x *float64) (int, error) {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	*x = v
	return 1, err
}

func formatFloat(x float64) string { return strconv.FormatFloat(x, 'g', 17, 64) }

// renumber adds off to every instance number in a line.
func renumber(line string, off int) string {
	return regexp.MustCompile(`#(\d+)`).ReplaceAllStringFunc(line, func(m string) string {
		n, _ := strconv.Atoi(m[1:])
		return "#" + strconv.Itoa(n+off)
	})
}

// scalePoint halves a CARTESIAN_POINT about the box's centre.
func scalePoint(line string) string {
	i, j := strings.Index(line, "',(")+3, strings.LastIndex(line, "))")
	parts := strings.Split(line[i:j], ",")
	centre := []float64{5, 10, 15}
	for k := range parts {
		v, _ := strconv.ParseFloat(strings.TrimSpace(parts[k]), 64)
		parts[k] = formatFloat(centre[k] + (v-centre[k])/2)
	}
	return line[:i] + strings.Join(parts, ",") + line[j:]
}
