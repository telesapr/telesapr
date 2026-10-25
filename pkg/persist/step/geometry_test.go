package step

import (
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/geom"
)

// wrapData puts instances into a file around them.
func wrapData(data string) string {
	return "ISO-10303-21;\nHEADER;\nFILE_SCHEMA(('CONFIG_CONTROL_DESIGN'));\nENDSEC;\nDATA;\n" +
		data + "ENDSEC;\nEND-ISO-10303-21;\n"
}

func meshArea(res *Result) float64 {
	a := 0.0
	for _, t := range res.Mesh.Triangles {
		n := t.Normal()
		a += n.Magnitude() / 2
	}
	return a
}

// planarFace is a face on the plane z = 0, facing +z, bounded by the
// edges listed, as an open shell on its own.
func planarFace(edges string) string {
	return `#1 = SHELL_BASED_SURFACE_MODEL('',(#2));
#2 = OPEN_SHELL('',(#3));
#3 = ADVANCED_FACE('',(#4),#6,.T.);
#4 = FACE_OUTER_BOUND('',#5,.T.);
#6 = PLANE('',#7);
#7 = AXIS2_PLACEMENT_3D('',#8,#9,#10);
#8 = CARTESIAN_POINT('',(0.,0.,0.));
#9 = DIRECTION('',(0.,0.,1.));
#10 = DIRECTION('',(1.,0.,0.));
` + edges
}

func TestCurveZoo(t *testing.T) {
	t.Parallel()
	// A square whose sides are a trimmed line, a polyline, a Bezier
	// curve and half an ellipse.
	text := wrapData(planarFace(`#5 = EDGE_LOOP('',(#40,#41,#42,#43));
#20 = VERTEX_POINT('',#21);
#21 = CARTESIAN_POINT('',(0.,0.,0.));
#22 = VERTEX_POINT('',#23);
#23 = CARTESIAN_POINT('',(10.,0.,0.));
#24 = VERTEX_POINT('',#25);
#25 = CARTESIAN_POINT('',(10.,10.,0.));
#26 = VERTEX_POINT('',#27);
#27 = CARTESIAN_POINT('',(0.,10.,0.));
#40 = ORIENTED_EDGE('',*,*,#50,.T.);
#41 = ORIENTED_EDGE('',*,*,#51,.T.);
#42 = ORIENTED_EDGE('',*,*,#52,.T.);
#43 = ORIENTED_EDGE('',*,*,#53,.T.);
#50 = EDGE_CURVE('',#20,#22,#60,.T.);
#60 = TRIMMED_CURVE('',#61,(PARAMETER_VALUE(0.)),(PARAMETER_VALUE(10.)),.T.,.PARAMETER.);
#61 = LINE('',#21,#62);
#62 = VECTOR('',#63,1.);
#63 = DIRECTION('',(1.,0.,0.));
#51 = EDGE_CURVE('',#22,#24,#64,.T.);
#64 = POLYLINE('',(#23,#65,#25));
#65 = CARTESIAN_POINT('',(11.,5.,0.));
#52 = EDGE_CURVE('',#24,#26,#66,.T.);
#66 = BEZIER_CURVE('',2,(#25,#67,#27),.UNSPECIFIED.,.F.,.F.);
#67 = CARTESIAN_POINT('',(5.,13.,0.));
#53 = EDGE_CURVE('',#26,#20,#68,.T.);
#68 = ELLIPSE('',#69,5.,2.);
#69 = AXIS2_PLACEMENT_3D('',#70,#9,#71);
#70 = CARTESIAN_POINT('',(0.,5.,0.));
#71 = DIRECTION('',(0.,1.,0.));
`))
	res, err := Read([]byte(text), Options{ChordTol: 1e-4})
	if err != nil {
		t.Fatal(err)
	}
	if p := res.Report.Problem(); p != "" {
		t.Fatal(p)
	}
	want := 100 + 5 + 10 + math.Pi*5*2/2
	if a := meshArea(res); math.Abs(a-want) > 1e-2 {
		t.Errorf("area %.6f, want %.6f", a, want)
	}
}

func TestConics(t *testing.T) {
	t.Parallel()
	// A parabola x = y^2/4 closed by the line x = 4, and a hyperbola
	// x^2 - y^2 = 1 closed by x = cosh 1, in two faces.
	text := wrapData(`#1 = SHELL_BASED_SURFACE_MODEL('',(#2));
#2 = OPEN_SHELL('',(#3,#103));
#3 = ADVANCED_FACE('',(#4),#6,.T.);
#4 = FACE_OUTER_BOUND('',#5,.T.);
#5 = EDGE_LOOP('',(#40,#41));
#6 = PLANE('',#7);
#7 = AXIS2_PLACEMENT_3D('',#8,#9,#10);
#8 = CARTESIAN_POINT('',(0.,0.,0.));
#9 = DIRECTION('',(0.,0.,1.));
#10 = DIRECTION('',(1.,0.,0.));
#20 = VERTEX_POINT('',#21);
#21 = CARTESIAN_POINT('',(4.,-4.,0.));
#22 = VERTEX_POINT('',#23);
#23 = CARTESIAN_POINT('',(4.,4.,0.));
#40 = ORIENTED_EDGE('',*,*,#50,.T.);
#41 = ORIENTED_EDGE('',*,*,#51,.T.);
#50 = EDGE_CURVE('',#20,#22,#60,.T.);
#60 = PARABOLA('',#7,1.);
#51 = EDGE_CURVE('',#22,#20,#61,.T.);
#61 = LINE('',#23,#62);
#62 = VECTOR('',#63,8.);
#63 = DIRECTION('',(0.,-1.,0.));
#103 = ADVANCED_FACE('',(#104),#106,.T.);
#104 = FACE_OUTER_BOUND('',#105,.T.);
#105 = EDGE_LOOP('',(#140,#141));
#106 = PLANE('',#107);
#107 = AXIS2_PLACEMENT_3D('',#108,#9,#10);
#108 = CARTESIAN_POINT('',(0.,0.,5.));
#120 = VERTEX_POINT('',#121);
#121 = CARTESIAN_POINT('',(1.5430806348152437,-1.1752011936438014,5.));
#122 = VERTEX_POINT('',#123);
#123 = CARTESIAN_POINT('',(1.5430806348152437,1.1752011936438014,5.));
#140 = ORIENTED_EDGE('',*,*,#150,.T.);
#141 = ORIENTED_EDGE('',*,*,#151,.T.);
#150 = EDGE_CURVE('',#120,#122,#160,.T.);
#160 = HYPERBOLA('',#107,1.,1.);
#151 = EDGE_CURVE('',#122,#120,#161,.T.);
#161 = LINE('',#123,#162);
#162 = VECTOR('',#63,1.);
`)
	res, err := Read([]byte(text), Options{ChordTol: 1e-5})
	if err != nil {
		t.Fatal(err)
	}
	if p := res.Report.Problem(); p != "" {
		t.Fatal(p)
	}
	// The hyperbola's area: the integral of cosh 1 - sqrt(1 + y^2).
	s1 := math.Sinh(1)
	hyp := 2*s1*math.Cosh(1) - (s1*math.Sqrt(1+s1*s1) + math.Asinh(s1))
	want := 64.0/3 + hyp
	if a := meshArea(res); math.Abs(a-want) > 1e-3 {
		t.Errorf("area %.6f, want %.6f", a, want)
	}
}

// seamlessCylinder is a cylinder whose side is bounded by its two
// circles only, with no seam edge, which some writers do; the circles'
// vertices are a quarter turn apart.
const seamlessCylinder = `#1 = MANIFOLD_SOLID_BREP('',#2);
#2 = CLOSED_SHELL('',(#3,#4,#5));
#3 = ADVANCED_FACE('',(#30,#31),#10,.T.);
#30 = FACE_BOUND('',#32,.T.);
#31 = FACE_BOUND('',#33,.T.);
#32 = EDGE_LOOP('',(#34));
#33 = EDGE_LOOP('',(#35));
#34 = ORIENTED_EDGE('',*,*,#60,.T.);
#35 = ORIENTED_EDGE('',*,*,#61,.F.);
#4 = ADVANCED_FACE('',(#40),#20,.F.);
#40 = FACE_OUTER_BOUND('',#41,.T.);
#41 = EDGE_LOOP('',(#42));
#42 = ORIENTED_EDGE('',*,*,#60,.F.);
#5 = ADVANCED_FACE('',(#50),#21,.T.);
#50 = FACE_OUTER_BOUND('',#51,.T.);
#51 = EDGE_LOOP('',(#52));
#52 = ORIENTED_EDGE('',*,*,#61,.T.);
#10 = CYLINDRICAL_SURFACE('',#11,5.);
#11 = AXIS2_PLACEMENT_3D('',#12,#13,#14);
#12 = CARTESIAN_POINT('',(0.,0.,0.));
#13 = DIRECTION('',(0.,0.,1.));
#14 = DIRECTION('',(1.,0.,0.));
#20 = PLANE('',#11);
#21 = PLANE('',#22);
#22 = AXIS2_PLACEMENT_3D('',#23,#13,#14);
#23 = CARTESIAN_POINT('',(0.,0.,10.));
#60 = EDGE_CURVE('',#70,#70,#80,.T.);
#61 = EDGE_CURVE('',#71,#71,#81,.T.);
#70 = VERTEX_POINT('',#72);
#71 = VERTEX_POINT('',#73);
#72 = CARTESIAN_POINT('',(5.,0.,0.));
#73 = CARTESIAN_POINT('',(0.,5.,10.));
#80 = CIRCLE('',#11,5.);
#81 = CIRCLE('',#22,5.);
`

func TestSeamlessCylinder(t *testing.T) {
	t.Parallel()
	res := mustRead(t, wrapData(seamlessCylinder))
	if p := res.Report.Problem(); p != "" {
		t.Fatal(p)
	}
	r := check.Solid(nil, &res.Mesh)
	want := math.Pi * 25 * 10
	if r.NakedEdges != 0 || r.SelfIntersecting != 0 || math.Abs(r.Volume-want) > 2*math.Pi*5*15*res.Report.ChordTol {
		t.Errorf("volume %g, want %g; %d naked, %d self-intersecting", r.Volume, want, r.NakedEdges, r.SelfIntersecting)
	}
	if len(res.Centers) != 2 {
		t.Errorf("%d centres", len(res.Centers))
	}
}

func TestOffsetAndTrimmedSurfaces(t *testing.T) {
	t.Parallel()
	// The seamless cylinder with its side an offset of a radius 3
	// cylinder by 2, behind a rectangular trim.
	text := strings.Replace(seamlessCylinder, "#10 = CYLINDRICAL_SURFACE('',#11,5.);",
		"#10 = RECTANGULAR_TRIMMED_SURFACE('',#15,0.,6.3,0.,10.,.T.,.T.);\n"+
			"#15 = OFFSET_SURFACE('',#16,2.,.F.);\n#16 = CYLINDRICAL_SURFACE('',#11,3.);", 1)
	res := mustRead(t, wrapData(text))
	if p := res.Report.Problem(); p != "" {
		t.Fatal(p)
	}
	r := check.Solid(nil, &res.Mesh)
	want := math.Pi * 25 * 10
	if r.NakedEdges != 0 || math.Abs(r.Volume-want) > 2*math.Pi*5*15*res.Report.ChordTol {
		t.Errorf("volume %g, want %g; %d naked", r.Volume, want, r.NakedEdges)
	}
}

func TestUniformAndBezierSurfaces(t *testing.T) {
	t.Parallel()
	// One face on a Bezier patch and one on a uniform B-spline patch,
	// both flat squares of 4 by 4 at z = 0 and 1, read as open shells.
	pts := `#30 = CARTESIAN_POINT('',(0.,0.,0.));
#31 = CARTESIAN_POINT('',(0.,2.,0.));
#32 = CARTESIAN_POINT('',(0.,4.,0.));
#33 = CARTESIAN_POINT('',(2.,0.,0.));
#34 = CARTESIAN_POINT('',(2.,2.,0.));
#35 = CARTESIAN_POINT('',(2.,4.,0.));
#36 = CARTESIAN_POINT('',(4.,0.,0.));
#37 = CARTESIAN_POINT('',(4.,2.,0.));
#38 = CARTESIAN_POINT('',(4.,4.,0.));
`
	face := func(surf string) string {
		return wrapData(`#1 = SHELL_BASED_SURFACE_MODEL('',(#2));
#2 = OPEN_SHELL('',(#3));
#3 = ADVANCED_FACE('',(#4),#20,.T.);
#4 = FACE_OUTER_BOUND('',#5,.T.);
#5 = EDGE_LOOP('',(#40,#41,#42,#43));
#20 = ` + surf + `
` + pts + `#50 = VERTEX_POINT('',#30);
#51 = VERTEX_POINT('',#36);
#52 = VERTEX_POINT('',#38);
#53 = VERTEX_POINT('',#32);
#40 = ORIENTED_EDGE('',*,*,#60,.T.);
#41 = ORIENTED_EDGE('',*,*,#61,.T.);
#42 = ORIENTED_EDGE('',*,*,#62,.T.);
#43 = ORIENTED_EDGE('',*,*,#63,.T.);
#60 = EDGE_CURVE('',#50,#51,#70,.T.);
#61 = EDGE_CURVE('',#51,#52,#71,.T.);
#62 = EDGE_CURVE('',#52,#53,#72,.T.);
#63 = EDGE_CURVE('',#53,#50,#73,.T.);
#70 = QUASI_UNIFORM_CURVE('',1,(#30,#36),.UNSPECIFIED.,.F.,.F.);
#71 = UNIFORM_CURVE('',1,(#36,#37,#38),.UNSPECIFIED.,.F.,.F.);
#72 = POLYLINE('',(#38,#32));
#73 = POLYLINE('',(#32,#30));
`)
	}
	for _, surf := range []string{
		"BEZIER_SURFACE('',2,2,((#30,#31,#32),(#33,#34,#35),(#36,#37,#38)),.UNSPECIFIED.,.F.,.F.,.F.);",
		"QUASI_UNIFORM_SURFACE('',1,1,((#30,#31,#32),(#33,#34,#35),(#36,#37,#38)),.UNSPECIFIED.,.F.,.F.,.F.);",
		"( BOUNDED_SURFACE() B_SPLINE_SURFACE(1,1,((#30,#32),(#36,#38)),.UNSPECIFIED.,.F.,.F.,.F.) " +
			"B_SPLINE_SURFACE_WITH_KNOTS((2,2),(2,2),(0.,1.),(0.,1.),.UNSPECIFIED.) GEOMETRIC_REPRESENTATION_ITEM() " +
			"RATIONAL_B_SPLINE_SURFACE(((1.,2.),(1.,2.))) REPRESENTATION_ITEM('') SURFACE() );",
	} {
		res := mustRead(t, face(surf))
		if p := res.Report.Problem(); p != "" {
			t.Errorf("%s: %s", surf[:20], p)
		}
		if a := meshArea(res); math.Abs(a-16) > 1e-9 {
			t.Errorf("%s: area %g", surf[:20], a)
		}
	}
	// A uniform surface's parameters run beyond its corners, which the
	// edges then don't reach: it reads, whatever it covers.
	res := mustRead(t, face("UNIFORM_SURFACE('',1,1,((#30,#31,#32),(#33,#34,#35),(#36,#37,#38)),.UNSPECIFIED.,.F.,.F.,.F.);"))
	if res.Report.Shapes != 1 {
		t.Error("no shape")
	}
}

func TestCurvesDirectly(t *testing.T) {
	t.Parallel()
	f := buildAxes(geom.Vector{X: 1, Y: 2, Z: 3}, geom.Vector{X: 1, Y: 1, Z: 1}, geom.Vector{X: 1}, true)
	curves := map[string]Curve{
		"line":      &line{p: geom.Vector{X: 1}, d: geom.Vector{Y: 2}},
		"ellipse":   &ellipse{f: f, a: 5, b: 2},
		"parabola":  &conic{f: f, a: 1.5},
		"hyperbola": &conic{f: f, hyperbola: true, a: 2, b: 3},
	}
	pl, err := polylineCurve([]geom.Vector{{}, {X: 1}, {X: 1, Y: 1}})
	if err != nil {
		t.Fatal(err)
	}
	curves["polyline"] = pl
	for name, c := range curves {
		for _, tt := range []float64{0.3, 0.9, 1.7} {
			p := c.Eval(tt)
			if got := c.Param(p); math.Abs(got-tt) > 1e-7 {
				t.Errorf("%s: param of point at %g is %g", name, tt, got)
			}
			h := 1e-6
			fd := c.Eval(tt + h).Minus(c.Eval(tt - h)).ScaledBy(1 / (2 * h))
			if name != "polyline" && !fd.EqualsTol(c.Deriv(tt), 1e-5) {
				t.Errorf("%s: derivative %v, by differences %v", name, c.Deriv(tt), fd)
			}
		}
		lo, hi := c.Range()
		if !(hi > lo) || c.Period() < 0 {
			t.Errorf("%s: range %g..%g period %g", name, lo, hi, c.Period())
		}
	}
	if (&ellipse{f: f, a: 1, b: 1}).Period() != 2*math.Pi {
		t.Error("circle period")
	}
}

func TestSurfacesDirectly(t *testing.T) {
	t.Parallel()
	f := buildAxes(geom.Vector{X: 1, Y: -2, Z: 3}, geom.Vector{X: 0.3, Y: 1, Z: 0.2}, geom.Vector{}, false)
	circ := &ellipse{f: buildAxes(geom.Vector{X: 4}, geom.Vector{Y: 1}, geom.Vector{X: 1}, true), a: 1, b: 1}
	flat := &ellipse{f: buildAxes(geom.Vector{X: 4}, geom.Vector{Z: 1}, geom.Vector{X: 1}, true), a: 2, b: 1}
	surfs := map[string]Surface{
		"plane":      &plane{f: f},
		"cylinder":   &cone{f: f, r: 3},
		"cone":       &cone{f: f, r: 3, tan: 0.5},
		"sphere":     &sphere{f: f, r: 4},
		"torus":      &torus{f: f, major: 5, r: 2},
		"spindle":    &torus{f: f, major: 1, r: 2},
		"extrusion":  &extrusion{c: flat, d: geom.Vector{Y: 0.5, Z: 2}},
		"revolution": &revolution{c: circ, o: geom.Vector{}, a: geom.Vector{Z: 1}},
		"offset":     &offset{base: &sphere{f: f, r: 4}, d: 1},
	}
	for name, s := range surfs {
		for _, uv := range [][2]float64{{0.4, 0.3}, {2.5, -0.6}, {5.1, 0.9}} {
			p, su, sv := s.Derivs(uv[0], uv[1])
			h := 1e-6
			fu := s.Eval(uv[0]+h, uv[1]).Minus(s.Eval(uv[0]-h, uv[1])).ScaledBy(1 / (2 * h))
			fv := s.Eval(uv[0], uv[1]+h).Minus(s.Eval(uv[0], uv[1]-h)).ScaledBy(1 / (2 * h))
			if !fu.EqualsTol(su, 1e-4) || !fv.EqualsTol(sv, 1e-4) {
				t.Errorf("%s at %v: derivatives %v %v, by differences %v %v", name, uv, su, sv, fu, fv)
			}
			u, v := s.Project(p, nil)
			if q := s.Eval(u, v); !q.EqualsTol(p, 1e-6) {
				t.Errorf("%s: %v projects to %v, %v", name, p, q, [2]float64{u, v})
			}
			u, v = s.Project(p, &[2]float64{uv[0] + 0.01, uv[1] - 0.01})
			if q := s.Eval(u, v); !q.EqualsTol(p, 1e-6) {
				t.Errorf("%s: %v projects from near to %v", name, p, q)
			}
		}
		s.Domain()
		s.Period()
		for side := poleUMin; side <= poleVMax; side++ {
			s.Pole(side)
		}
	}
	// A revolution of a line that meets the axis has a pole there.
	rl := &revolution{c: &line{p: geom.Vector{}, d: geom.Vector{X: 1, Z: 1}}, a: geom.Vector{Z: 1}}
	if _, ok := rl.Pole(poleVMin); ok {
		t.Error("an unbounded line has no pole")
	}
	pc, _ := polylineCurve([]geom.Vector{{}, {X: 1, Z: 1}})
	rp := &revolution{c: pc, a: geom.Vector{Z: 1}}
	if v, ok := rp.Pole(poleVMin); !ok || v != 0 {
		t.Error("a curve from the axis has a pole at its start")
	}
}

func TestTransform(t *testing.T) {
	t.Parallel()
	f := buildAxes(geom.Vector{X: 1, Y: 2, Z: 3}, geom.Vector{X: 1, Y: 1}, geom.Vector{Z: 1}, true)
	tr := frameTransform(f)
	p := geom.Vector{X: 0.5, Y: -2, Z: 7}
	if q := tr.Inverse().Apply(tr.Apply(p)); !q.EqualsTol(p, 1e-12) {
		t.Errorf("inverse: %v", q)
	}
	if d := tr.Then(tr.Inverse()); !d.Apply(p).EqualsTol(p, 1e-12) || math.Abs(tr.Det()-1) > 1e-12 {
		t.Error("then")
	}
	if !Identity().Apply(p).EqualsExactly(p) {
		t.Error("identity")
	}
}

func TestBadSplines(t *testing.T) {
	t.Parallel()
	pts := []geom.Vector{{}, {X: 1}, {X: 2}}
	if _, err := newBsplineCurve(0, []float64{0, 1}, pts, nil); err == nil {
		t.Error("degree 0")
	}
	if _, err := newBsplineCurve(3, []float64{0, 0, 0, 1, 1, 1}, pts, nil); err == nil {
		t.Error("too few points")
	}
	if _, err := newBsplineCurve(2, []float64{0, 0, 1, 1, 1}, pts, nil); err == nil {
		t.Error("wrong knot count")
	}
	if _, err := newBsplineCurve(2, []float64{0, 0, 1, 0.5, 1, 1}, pts, nil); err == nil {
		t.Error("decreasing knots")
	}
	if _, err := newBsplineCurve(2, []float64{0, 0, 0, 0, 0, 0}, pts, nil); err == nil {
		t.Error("no range")
	}
	if _, err := newBsplineCurve(2, []float64{0, 0, 0, 1, 1, 1}, pts, []float64{1, 0, 1}); err == nil {
		t.Error("zero weight")
	}
	if _, err := newBsplineCurve(2, []float64{0, 0, 0, 1, 1, 1}, pts, []float64{1, 1}); err == nil {
		t.Error("missing weight")
	}
	if _, err := expandKnots([]int{1, 2}, []float64{0}); err == nil {
		t.Error("multiplicities")
	}
	if _, err := expandKnots([]int{0}, []float64{0}); err == nil {
		t.Error("zero multiplicity")
	}
	grid := [][]geom.Vector{{{}, {X: 1}}, {{Y: 1}, {X: 1, Y: 1}}}
	k := []float64{0, 0, 1, 1}
	for name, w := range map[string][][]float64{
		"short":  {{1, 1}},
		"ragged": {{1, 1}, {1}},
		"zero":   {{1, 1}, {1, 0}},
	} {
		if _, err := newBsplineSurface(1, 1, k, k, grid, w); err == nil {
			t.Errorf("weights %s", name)
		}
	}
	if _, err := newBsplineSurface(1, 1, k, k, [][]geom.Vector{{{}, {X: 1}}, {{Y: 1}}}, nil); err == nil {
		t.Error("ragged rows")
	}
	if _, err := newBsplineSurface(1, 1, k, k, nil, nil); err == nil {
		t.Error("no points")
	}
	if _, err := newBsplineSurface(1, 1, []float64{0, 1}, k, grid, nil); err == nil {
		t.Error("u knots")
	}
	if _, err := newBsplineSurface(1, 1, k, []float64{0, 1}, grid, nil); err == nil {
		t.Error("v knots")
	}
	b, err := newBasis(2, []float64{0, 0, 0, 1, 2, 2, 2}, 4)
	if err != nil || b.spans() != 2 {
		t.Errorf("spans %d, %v", b.spans(), err)
	}
}

func TestUnitNames(t *testing.T) {
	t.Parallel()
	for mm, want := range map[float64]string{
		1: "millimetres", 10: "centimetres", 1000: "metres", 25.4: "inches",
		304.8: "feet", 1e-3: "micrometres", 3: "units of 3 mm",
	} {
		if got := unitName(mm); got != want {
			t.Errorf("%g: %q, want %q", mm, got, want)
		}
	}
}
