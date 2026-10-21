package tlspr

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/thread"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// fullDocument is a document with every field the format stores set,
// most to values a default would not give.
func fullDocument() *Document {
	sk := sketch.NewSketch()
	g := sketch.Group{
		H: core.HGroup{V: 7}, Type: sketch.GroupHelix, Order: 3, Name: "all \"fields\"",
		OpA: core.HGroup{V: 2}, OpB: core.HGroup{V: 5},
		Visible: true, Suppress: true, RelaxConstraints: true, AllowRedundant: true,
		SuppressDofCalculation: true, AllDimsReference: true, Scale: 2.5,
		ActiveWorkplane: core.HEntity{V: 0x80020000},
		ValA:            3, ValB: -1.25, ValC: 1e-9, Extent: -720.5,
		Subtype: sketch.GroupTwoSkewed, SkipFirst: true,
		MeshCombine: sketch.CombineIntersection, ForceToMesh: true,
		Remap:    core.EntityMap{{Input: core.HEntity{V: 0x40001}, CopyNumber: 2}: {V: 5}},
		LinkFile: "dir/part.tlspr",
		Color:    core.RGBA{Red: 1, Green: 2, Blue: 3, Alpha: 4},
		Density:  7.85,
		Thread: sketch.GroupThread{Circle: core.HEntity{V: 0x30000}, ThreadOptions: sketch.ThreadOptions{
			Spec:     thread.Spec{Form: thread.FormTrapezoidal, Diameter: 20, Pitch: 4, Starts: 2, LeftHand: true},
			Internal: true, LengthExpr: "len * 2", Reverse: true, Through: true, Clearance: 12.7,
			Crest: thread.CrestRounded,
		}},
	}
	g.Predef.Q = geom.Quaternion{W: 0.5, X: -0.5, Y: 0.25, Z: math.Copysign(0, -1)}
	g.Predef.Origin = core.HEntity{V: 0x10001}
	g.Predef.At = geom.Vector{X: 1.5, Y: -2, Z: 1e-7}
	g.Predef.EntityB = core.HEntity{V: 0x20001}
	g.Predef.EntityC = core.HEntity{V: 0x30001}
	g.Predef.SwapUV, g.Predef.NegateU, g.Predef.NegateV = true, true, true
	plain := sketch.Group{H: core.HGroup{V: 1}, Type: sketch.GroupDrawing3D, Name: "", Scale: 1, Remap: core.EntityMap{}}
	sk.Groups = []sketch.Group{plain, g}
	sk.GroupOrder = []core.HGroup{plain.H, g.H}

	sk.Requests = []sketch.Request{
		{H: core.HRequest{V: 1}, Type: sketch.RequestWorkplane, Group: plain.H, Workplane: core.FreeIn3D},
		{
			H: core.HRequest{V: 9}, Type: sketch.RequestImage, Group: g.H, Workplane: core.HEntity{V: 0x80020000},
			Construction: true, Style: core.HStyle{V: 0x100}, ExtraPoints: 2,
			Str: "text\n", Font: "font.ttf", File: "img.png", AspectRatio: 1.5,
		},
	}

	c := core.Constraint{}
	c.H, c.Type, c.Group = core.HConstraint{V: 4}, core.ConstraintPtPtDistance, g.H
	c.Workplane, c.ValA, c.ValP = core.HEntity{V: 0x80020000}, 12.5, core.HParam{V: 0x40010}
	c.PtA, c.PtB = core.HEntity{V: 0x40001}, core.HEntity{V: 0x40002}
	c.EntityA, c.EntityB, c.EntityC, c.EntityD = core.HEntity{V: 1}, core.HEntity{V: 2}, core.HEntity{V: 3}, core.HEntity{V: 4}
	c.Other, c.Other2, c.Reference, c.Comment = true, true, true, "why"
	c.Disp.Offset, c.Disp.Style = geom.Vector{X: 1, Y: -2, Z: 3}, core.HStyle{V: 0x101}
	sk.Constraints = []core.Constraint{c}

	sk.Styles = append(sk.Styles, core.Style{
		H: core.HStyle{V: 0x100}, Name: "custom", Width: 2, WidthAs: core.UnitsMM,
		TextHeight: 11, TextHeightAs: core.UnitsMM, TextAngle: 30,
		TextOrigin: core.TextOriginLeft | core.TextOriginTop,
		Color:      core.RGBA{Red: 255, Alpha: 255}, FillColor: core.RGBA{Blue: 9, Alpha: 128},
		Filled: true, Visible: true, Exportable: true, StippleType: core.StippleDashDot, StippleScale: 15,
		ZIndex: 6,
	})

	sk.Params = []core.Param{
		{H: core.HParam{V: 0x40010}, Val: math.Copysign(0, -1), Known: true},
		{H: core.HParam{V: 0x40012}, Val: math.Inf(-1), Known: true},
		{H: core.HParam{V: 0x40000011}, Val: 1e300, Free: true},
	}

	e := core.Entity{}
	e.H, e.Type, e.Group, e.Workplane = core.HEntity{V: 0x90000}, core.EntityTTFText, g.H, core.FreeIn3D
	e.Construction, e.Style = true, core.HStyle{V: 0x100}
	e.Str, e.Font, e.File = "abc", "f.ttf", "p.png"
	e.Point[0], e.Point[2] = core.HEntity{V: 0x90001}, core.HEntity{V: 0x90003}
	e.ExtraPoints, e.Normal, e.Distance = 1, core.HEntity{V: 0x90020}, core.HEntity{V: 0x90030}
	e.ActPoint, e.ActNormal = geom.Vector{X: 1e-7, Y: 2, Z: 3}, geom.Quaternion{W: 1}
	e.ActDistance, e.ActVisible = 4, true
	// A surface entity of SolveSpace's own: nothing here makes one, but
	// a file that carries one must come back unchanged.
	surface := e
	surface.H, surface.Type = core.HEntity{V: 0x90010}, core.EntitySurfaceRevolved
	sk.Entities = []core.Entity{e, surface}

	sk.Units = core.UnitInches
	doc := &Document{Sketch: sk, Version: Version, Units: core.UnitInches}
	doc.Mesh.AddTriangle3(srf.STriMeta{Face: 0x1234, Red: 10, Alpha: 255},
		geom.Vector{X: 1}, geom.Vector{Y: 1}, geom.Vector{Z: 1})
	sf := srf.SSurface{H: core.HSSurface{V: 1}, Color: core.RGBA{Green: 7}, Face: 0x99, DegM: 1, DegN: 3, TrimBuilt: true}
	for i := 0; i <= sf.DegM; i++ {
		for j := 0; j <= sf.DegN; j++ {
			sf.Ctrl[i][j] = geom.Vector{X: float64(i), Y: float64(j), Z: 0.1}
			sf.Weight[i][j] = 1 + float64(i*j)
		}
	}
	sf.Trim = []srf.STrimBy{
		{Curve: core.HSCurve{V: 2}, StartP: geom.Vector{X: 1}, FinishP: geom.Vector{Y: 1}},
		{Curve: core.HSCurve{V: 3}, Backwards: true, StartP: geom.Vector{Y: 1}, FinishP: geom.Vector{X: 1}},
	}
	doc.Shell.Surface = []srf.SSurface{sf}
	sc := srf.SCurve{H: core.HSCurve{V: 2}, IsExact: true, SurfA: core.HSSurface{V: 1}, SurfB: core.HSSurface{V: 5}}
	sc.Exact.Degree = 2
	for i := range 3 {
		sc.Exact.Ctrl[i] = geom.Vector{X: float64(i)}
		sc.Exact.Weight[i] = 1
	}
	sc.Points = []srf.SCurvePt{{Vertex: true, P: geom.Vector{X: 0}}, {P: geom.Vector{X: 1}}, {Vertex: true, P: geom.Vector{X: 2}}}
	doc.Shell.Curve = []srf.SCurve{sc, {H: core.HSCurve{V: 3}, Exact: srf.SBezier{Degree: 1}}}
	sk.Drawing = &sketch.Drawing{Sheet: "A2", Scale: 0.5, Hidden: true,
		Sections: []sketch.DrawingSection{{View: "top", Offset: 2}},
		Dimensions: []sketch.DrawingDimension{
			{View: "front", Kind: "length", Edges: []topo.Name{{A: 0x10, B: 0x20, N: 1, Of: 2}}, Offset: 10},
			{View: "A", Kind: "distance", Edges: []topo.Name{{A: 0x10, B: 0x30}}, Faces: []srf.FaceID{0x80030011}},
			{View: "iso", Kind: "distance", Faces: []srf.FaceID{0x40, 0x50}, Offset: -4},
		},
		Title: "ALL"}
	return doc
}

func TestEveryFieldRoundTrips(t *testing.T) {
	doc := fullDocument()
	data := doc.Encode()
	got, err := Decode(data)
	if err != nil {
		t.Fatalf("%v\n%s", err, data)
	}
	if len(got.Warnings) > 0 {
		t.Errorf("warnings %v", got.Warnings)
	}
	if again := got.Encode(); string(again) != string(data) {
		t.Errorf("encoded again:\n%s\nfirst:\n%s", again, data)
	}
	if got.Units != core.UnitInches || got.Version != Version {
		t.Errorf("units %v version %d", got.Units, got.Version)
	}
	want, have := doc.Sketch, got.Sketch
	for _, c := range []struct {
		what      string
		got, want any
	}{
		{"groups", have.Groups, want.Groups},
		{"group order", have.GroupOrder, want.GroupOrder},
		{"requests", have.Requests, want.Requests},
		{"constraints", have.Constraints, want.Constraints},
		{"styles", have.Styles[len(have.Styles)-1:], want.Styles[len(want.Styles)-1:]},
		{"entities", have.Entities, want.Entities},
		{"mesh", got.Mesh, doc.Mesh},
		{"shell", got.Shell, doc.Shell},
		{"drawing", have.Drawing, want.Drawing},
	} {
		if !reflect.DeepEqual(c.got, c.want) {
			t.Errorf("%s:\n got %+v\nwant %+v", c.what, c.got, c.want)
		}
	}
	// NaN and signed zeros don't compare with DeepEqual's ==.
	for i, p := range have.Params {
		w := want.Params[i]
		if p.H != w.H || math.Float64bits(p.Val) != math.Float64bits(w.Val) || p.Known != w.Known || p.Free != w.Free {
			t.Errorf("param %d: got %+v, want %+v", i, p, w)
		}
	}
	if q := have.Groups[1].Predef.Q.Z; !math.Signbit(q) {
		t.Errorf("negative zero lost: %v", q)
	}
}

func TestFormatReal(t *testing.T) {
	tenth, fifth := 0.1, 0.2
	for _, c := range []struct {
		v    float64
		want string
	}{
		{0, "0"}, {math.Copysign(0, -1), "-0"}, {1.5, "1.5"}, {-30, "-30"},
		{1e-6, "0.000001"}, {9.99e-7, "9.99e-07"}, {1e20, "100000000000000000000"}, {1e21, "1e+21"},
		{tenth + fifth, "0.30000000000000004"},
		{math.NaN(), "nan"}, {math.Inf(1), "inf"}, {math.Inf(-1), "-inf"},
	} {
		if got := FormatReal(c.v); got != c.want {
			t.Errorf("FormatReal(%v) = %q, want %q", c.v, got, c.want)
		}
	}
	d := &decoder{doc: &Document{}}
	for _, v := range []float64{math.NaN(), 5e-324, math.MaxFloat64, -1e-300, 123456.789} {
		back := d.parseReal(Pos{}, FormatReal(v))
		if math.Float64bits(back) != math.Float64bits(v) && !(math.IsNaN(v) && math.IsNaN(back)) {
			t.Errorf("%v read back as %v", v, back)
		}
	}
}

func TestVersions(t *testing.T) {
	_, err := Decode([]byte("telesapr 3\nunits mm\n"))
	var ve *VersionError
	if !errors.As(err, &ve) || ve.File != 3 || ve.Reader != Version {
		t.Fatalf("got %v", err)
	}
	if !strings.Contains(err.Error(), "version 3") || !strings.Contains(err.Error(), "up to version 2") {
		t.Errorf("message %q", err.Error())
	}
	for _, src := range []string{"telesapr 0\n", "telesapr one\n", "telesapr \"1\"\n", "telesapr\n", "telesapr 1 2\n"} {
		if _, err := Decode([]byte(src)); err == nil {
			t.Errorf("%q decoded", src)
		}
	}
	doc, err := Decode([]byte("telesapr 1\n"))
	if err != nil || doc.Version != 1 || doc.Units != core.UnitMM {
		t.Errorf("minimal file: %v %+v", err, doc)
	}
}

func TestWarnings(t *testing.T) {
	src := `telesapr 1
units furlong
future-thing 1 2
param wall = 2.5
group 0x1 sketch-3d "a" {
  new-field 1
  combine xor
  predef {
    new-predef
  }
  remap {
    new-map 1
  }
}
group 0x2 morph "b" {
  order 1
}
request 0x1 nurbs {
  new-field
}
constraint 0x1 tangent-to-everything {
  new-field
}
style 0x100 "s" {
  text-origin left middle
  new-field
}
entity 0x1 blob {
  new-field
}
mesh {
  quad 1
}
shell {
  patch {
  }
  surface 0x1 {
    new-field
  }
  curve 0x1 {
    new-field
  }
}
`
	doc, err := Decode([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, w := range doc.Warnings {
		got = append(got, w.Error())
	}
	want := []string{
		`2:7: unknown unit "furlong"`,
		`3:1: unknown statement "future-thing"; skipped`,
		`6:3: unknown field "new-field" in group; skipped`,
		`7:11: unknown combine "xor"`,
		`9:5: unknown field "new-predef" in predef; skipped`,
		`12:5: unknown field "new-map" in remap; skipped`,
		`15:11: unknown group type "morph"`,
		`15:1: group 0x2 skipped`,
		`18:13: unknown request type "nurbs"`,
		`19:3: unknown field "new-field" in request; skipped`,
		`18:1: request 0x1 skipped`,
		`21:16: unknown constraint type "tangent-to-everything"`,
		`22:3: unknown field "new-field" in constraint; skipped`,
		`21:1: constraint 0x1 skipped`,
		`25:20: unknown text origin "middle"`,
		`26:3: unknown field "new-field" in style; skipped`,
		`28:12: unknown entity type "blob"`,
		`29:3: unknown field "new-field" in entity; skipped`,
		`28:1: entity 0x1 skipped`,
		`32:3: unknown field "quad" in mesh; skipped`,
		`35:3: unknown field "patch" in shell; skipped`,
		`38:5: unknown field "new-field" in surface; skipped`,
		`41:5: unknown field "new-field" in curve; skipped`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("warnings:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	sk := doc.Sketch
	if p := sk.Parameter("wall"); p == nil || p.Value != 2.5 {
		t.Errorf("parameter wall: %+v", p)
	}
	if len(sk.Groups) != 1 || len(sk.Requests) != 0 || len(sk.Constraints) != 0 || len(sk.Entities) != 0 {
		t.Errorf("kept %d groups, %d requests, %d constraints, %d entities",
			len(sk.Groups), len(sk.Requests), len(sk.Constraints), len(sk.Entities))
	}
	if st := sk.Style(core.HStyle{V: 0x100}); st == nil || st.TextOrigin != core.TextOriginLeft {
		t.Errorf("style %+v", st)
	}
}

func TestMalformed(t *testing.T) {
	for _, c := range []struct {
		src, want string
	}{
		{"telesapr 1\ntelesapr 1\n", "2:1: a second header"},
		{"telesapr 1\nunits\n", `2:1: "units" takes 1 values, not 0`},
		{"telesapr 1\nunits = mm\n", `2:1: "units" takes no expression`},
		{"telesapr 1\nunits mm {\n}\n", `2:1: "units" is not a block`},
		{"telesapr 1\ngroup 0x1 sketch-3d \"a\"\n", `2:1: "group" must be a block`},
		{"telesapr 1\ngroup 0x1 sketch-3d {\n}\n", `2:1: "group" takes 3 values, not 2`},
		{"telesapr 1\ngroup 1 sketch-3d \"a\" {\n}\n", `2:7: expected a handle such as 0x1f, not "1"`},
		{"telesapr 1\ngroup 0xzz sketch-3d \"a\" {\n}\n", `2:7: bad handle "0xzz"`},
		{"telesapr 1\ngroup 0x1 \"sketch-3d\" \"a\" {\n}\n", "2:11: expected a name or number, not a string"},
		{"telesapr 1\ngroup 0x1 sketch-3d a {\n}\n", `2:21: expected a string in quotes, not "a"`},
		{"telesapr 1\ngroup 0x1 sketch-3d \"a\" {\n  order x\n}\n", `3:9: expected an integer, not "x"`},
		{"telesapr 1\ngroup 0x1 sketch-3d \"a\" {\n  scale big\n}\n", `3:9: expected a number, not "big"`},
		{"telesapr 1\ngroup 0x1 sketch-3d \"a\" {\n  copies 3\n}\n", `3:3: "copies" is written as 'copies = <expression>'`},
		{"telesapr 1\ngroup 0x1 sketch-3d \"a\" {\n  copies = n+\n}\n", `3:10: "n+" is not a valid expression`},
		{"telesapr 1\nparam 2a = 1\n", `2:7: "2a" can't name a parameter`},
		{"telesapr 1\nparam a = 1\nparam a = 2\n", `3:7: a second parameter "a"`},
		{"telesapr 1\nparam a 1\n", `2:1: a parameter is written as 'param <name> = <expression>'`},
		{"telesapr 1\nparam \"a\" = 1\n", `2:1: a parameter is written as`},
		{"telesapr 1\nparam a = (\n", `2:9: "(" is not a valid expression`},
		{"telesapr 1\ngroup 0x1 sketch-3d \"a\" {\n  visible 1\n}\n", `3:3: "visible" takes 0 values, not 1`},
		{"telesapr 1\ngroup 0x1 sketch-3d \"a\" {\n  color 1 2 3 256\n}\n", "3:15: a colour component is 0 to 255, not 256"},
		{"telesapr 1\ngroup 0x1 sketch-3d \"a\" {\n  remap {\n    map -1 0x1 0\n  }\n}\n", "4:9: a remap index is not negative"},
		{"telesapr 1\nstyle 0x5 \"builtin\" {\n}\n", "2:7: style 0x5 is a built-in style, which is not saved"},
		{"telesapr 1\nentity 0x1 line-segment {\n  points 0x1 0x2 0x3 0x4 0x5 0x6 0x7 0x8 0x9 0xa 0xb 0xc 0xd 0xe 0xf\n}\n", "3:3: an entity has at most"},
		{"telesapr 1\nmesh {\n  triangle 0x1 1 2 3\n}\n", `3:3: "triangle" takes 14 values, not 4`},
		{"telesapr 1\nshell {\n  surface 0x1 {\n    degree 6 1\n  }\n}\n", "4:12: 6 is out of range 0 to 5"},
		{"telesapr 1\nshell {\n  surface 0x1 {\n    trim 0x1 sideways 0 0 0 0 0 0\n  }\n}\n", `4:14: a trim runs forward or backward, not "sideways"`},
		{"telesapr 1\nshell {\n  curve 0x1 {\n    ctrl 9 0 0 0 1\n  }\n}\n", "4:10: 9 is out of range 0 to 5"},
		{"telesapr 1\nvalue 0x1\n", `2:1: "value" takes 2 values, not 1`},
	} {
		_, err := Decode([]byte(c.src))
		if err == nil || !strings.HasPrefix(err.Error(), c.want) {
			t.Errorf("%q: error %v, want %q", c.src, err, c.want)
		}
	}
}

func TestOverflowingNumbersRead(t *testing.T) {
	doc, err := Decode([]byte("telesapr 1\nvalue 0x1 1e999\nconstraint 0x1 diameter {\n  value = -1e999\n}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if v := doc.Sketch.Params[0].Val; !math.IsInf(v, 1) {
		t.Errorf("value %v", v)
	}
	if v := doc.Sketch.Constraints[0].ValA; !math.IsInf(v, -1) {
		t.Errorf("constraint value %v", v)
	}
}

func TestDecodeSortsByHandle(t *testing.T) {
	src := `telesapr 1
group 0x3 sketch-3d "c" {
  order 1
}
group 0x1 sketch-3d "a" {
  order 2
}
request 0x5 workplane {
}
request 0x2 workplane {
}
value 0x9 1
value 0x4 2
`
	doc, err := Decode([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	sk := doc.Sketch
	if sk.Groups[0].H.V != 1 || sk.GroupOrder[0].V != 3 || sk.Requests[0].H.V != 2 || sk.Params[0].H.V != 4 {
		t.Errorf("groups %v, order %v, requests %v, params %v", sk.Groups, sk.GroupOrder, sk.Requests, sk.Params)
	}
	if !core.IsFreeIn3D(sk.Requests[0].Workplane) {
		t.Errorf("a request without a workplane is free in 3D, got %v", sk.Requests[0].Workplane)
	}
}

func TestEnums(t *testing.T) {
	if enumName(groupTypes, sketch.GroupType(1)) != "unknown" {
		t.Error("an unknown value is not named unknown")
	}
	defer func() {
		if recover() == nil {
			t.Error("a duplicate name did not panic")
		}
	}()
	newEnum("dup", map[int]string{1: "a", 2: "a"})
}

// Parameters and stored expressions are read, evaluated and written
// back as they were.
func TestExpressions(t *testing.T) {
	src := `telesapr 2

units mm

param wall = 2.5
param n = 3
param bore = wall*4

group 0x1 sketch-3d "a" {
  copies = n+1
  pitch = wall/2
  val-c = 7
  color 0 0 0 0
  scale = bore/5
}

group 0x2 sketch-3d "b" {
  order 1
  color 0 0 0 0
  scale 3
}

constraint 0x1 pt-pt-distance {
  group 0x1
  value = bore
}

constraint 0x2 angle {
  group 0x1
  value = 30
}
`
	doc, err := Decode([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Warnings) > 0 || len(doc.ExprErrors) > 0 {
		t.Fatalf("warnings %v, expression errors %v", doc.Warnings, doc.ExprErrors)
	}
	sk := doc.Sketch
	if len(sk.Parameters) != 3 || sk.Parameter("bore").Value != 10 {
		t.Fatalf("parameters %+v", sk.Parameters)
	}
	g := sk.Group(core.HGroup{V: 1})
	if g.ValA != 4 || g.ExprA != "n+1" || g.ValB != 1.25 || g.ExprB != "wall/2" ||
		g.ValC != 7 || g.ExprC != "" || g.Scale != 2 || g.ExprScale != "bore/5" {
		t.Fatalf("group values %+v", g)
	}
	if g := sk.Group(core.HGroup{V: 2}); g.Scale != 3 || g.ExprScale != "" {
		t.Fatalf("plain scale %v %q", g.Scale, g.ExprScale)
	}
	c := sk.Constraint(core.HConstraint{V: 1})
	if c.ValA != 10 || c.Expr != "bore" {
		t.Fatalf("constraint %q = %v", c.Expr, c.ValA)
	}
	if c := sk.Constraint(core.HConstraint{V: 2}); c.ValA != 30 || c.Expr != "" {
		t.Fatalf("plain value %q = %v", c.Expr, c.ValA)
	}
	if got := string(doc.Encode()); got != src {
		t.Fatalf("encoded:\n%s\nwant:\n%s", got, src)
	}
}

// Expressions a file holds that cannot be evaluated are kept, and
// reported where they are.
func TestExpressionErrors(t *testing.T) {
	src := `telesapr 2

units mm

param a = b
param b = a
param c = zz

group 0x1 sketch-3d "a" {
  copies = c*0
  color 0 0 0 0
}

constraint 0x1 pt-pt-distance {
  group 0x1
  value = nope
}
`
	doc, err := Decode([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range doc.ExprErrors {
		got = append(got, e.Error())
	}
	want := []string{
		"5:9: parameter a: cycle: a -> b -> a",
		"6:9: parameter b: cycle: a -> b -> a",
		`7:9: parameter c: unknown parameter "zz"`,
		`16:9: constraint c001 = nope: unknown parameter "nope"`,
		"10:10: group g001 copies = c*0: can't repeat fewer than 1 time",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("errors:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if len(doc.Warnings) > 0 {
		t.Errorf("warnings %v", doc.Warnings)
	}
	if g := doc.Sketch.Group(core.HGroup{V: 1}); g.ValA != 1 {
		t.Errorf("copies %v, want the default 1", g.ValA)
	}
	if got := string(doc.Encode()); got != src {
		t.Fatalf("encoded:\n%s\nwant:\n%s", got, src)
	}
}

// A scale of 0, which the groups of a new file have, is not written,
// and reads back as 1.
func TestZeroScaleNotWritten(t *testing.T) {
	sk := sketch.NewSketch()
	sk.NewFile()
	sk.GenerateAll()
	data := Encode(sk, Options{})
	if strings.Contains(string(data), "scale") {
		t.Fatalf("a new file writes a scale:\n%s", data)
	}
	doc, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range doc.Sketch.Groups {
		if g.Scale != 1 {
			t.Errorf("group %v reads back scale %v", g.H, g.Scale)
		}
	}
}

// From version 2, what a file stores with "=" for a length is in its
// unit; version 1 stored millimetres, and an expression read from it is
// rewritten to give the same length in the file's unit.
func TestUnitsInFiles(t *testing.T) {
	body := `
units inch

param wall = 2

group 0x1 sketch-3d "a" {
  copies = 3
  pitch = 1/4
  color 0 0 0 0
  scale 2
}

constraint 0x1 pt-pt-distance {
  group 0x1
  value = wall*4
}

constraint 0x2 pt-pt-distance {
  group 0x1
  value = 2
}

constraint 0x3 angle {
  group 0x1
  value = 30
}
`
	v2, err := Decode([]byte("telesapr 2\n" + body))
	if err != nil {
		t.Fatal(err)
	}
	sk := v2.Sketch
	g := sk.Group(core.HGroup{V: 1})
	c := func(h uint32) *core.Constraint { return sk.Constraint(core.HConstraint{V: h}) }
	if sk.Units != core.UnitInches || g.ValB != 6.35 || g.ValA != 3 || g.Scale != 2 ||
		c(1).ValA != 203.2 || c(2).ValA != 50.8 || c(3).ValA != 30 {
		t.Fatalf("version 2: units %v pitch %v copies %v scale %v values %v %v %v",
			sk.Units, g.ValB, g.ValA, g.Scale, c(1).ValA, c(2).ValA, c(3).ValA)
	}
	if got := string(v2.Encode()); got != "telesapr 2\n"+body {
		t.Errorf("version 2 encodes as:\n%s", got)
	}

	v1, err := Decode([]byte("telesapr 1\n" + body))
	if err != nil {
		t.Fatal(err)
	}
	sk = v1.Sketch
	g = sk.Group(core.HGroup{V: 1})
	// The same lengths but for the rounding of /25.4*25.4.
	near := func(a, b float64) bool { return math.Abs(a-b) < 1e-12 }
	if !near(g.ValB, 0.25) || g.ExprB != "(1/4)/25.4" || !near(c(1).ValA, 8) || c(1).Expr != "(wall*4)/25.4" ||
		c(2).ValA != 2 || c(3).ValA != 30 {
		t.Fatalf("version 1: pitch %v %q, values %v %q, %v, %v",
			g.ValB, g.ExprB, c(1).ValA, c(1).Expr, c(2).ValA, c(3).ValA)
	}
	up := string(v1.Encode())
	for _, want := range []string{"telesapr 2\n", "  pitch = (1/4)/25.4\n", "  value = (wall*4)/25.4\n",
		"  value = 0.07874015748031496\n", "  value = 30\n"} {
		if !strings.Contains(up, want) {
			t.Errorf("the upgrade has no %q:\n%s", want, up)
		}
	}
}
