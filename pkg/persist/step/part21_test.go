package step

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	text := `ISO-10303-21;
HEADER;
/* a comment */
FILE_DESCRIPTION(('a test'),'2;1');
FILE_NAME('t.stp','2026-10-03',('me'),(''),'','','');
FILE_SCHEMA(('AP242_MANAGED_MODEL_BASED_3D_ENGINEERING_MIM_LF { 1 0 10303 442 1 1 4 }'));
ENDSEC;
DATA;
#1 = NAMED('it''s \X2\00E9\X0\ \X\E9 \S\A \\ \P0\x \X4\0001F600\X0\ \Q\', .T., .UNSPECIFIED., $, *, -12, 1.5E+3,
  .5, 3., "0FF", #2, (1, (2, 3)), LENGTH_MEASURE(2.5));
#2 = ( A() B(#1) c_lower(1) );
#3=SHORT();
ENDSEC;
ANCHOR;
<a> = #1;
ENDSEC;
END-ISO-10303-21;
trailing junk`
	f, err := Parse([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	if got := f.Schema(); got != "AP242_MANAGED_MODEL_BASED_3D_ENGINEERING_MIM_LF" {
		t.Errorf("schema %q", got)
	}
	if Protocol(f.Schema()) != "AP242" {
		t.Errorf("protocol %q", Protocol(f.Schema()))
	}
	if len(f.Order) != 3 || f.Order[0] != 1 || f.Get(4) != nil {
		t.Fatalf("order %v", f.Order)
	}
	ps := f.Get(1).Records[0].Params
	want := []Kind{KindString, KindEnum, KindEnum, KindOmitted, KindDerived, KindInteger, KindReal,
		KindReal, KindReal, KindBinary, KindRef, KindList, KindTyped}
	if len(ps) != len(want) {
		t.Fatalf("%d parameters, want %d", len(ps), len(want))
	}
	for i, k := range want {
		if ps[i].Kind != k {
			t.Errorf("parameter %d is a %v, want a %v", i, ps[i].Kind, k)
		}
	}
	if s := ps[0].Str; s != "it's é é Á \\ x 😀 \\Q\\" {
		t.Errorf("string %q", s)
	}
	if ps[5].Num != -12 || ps[6].Num != 1500 || ps[7].Num != 0.5 || ps[8].Num != 3 {
		t.Errorf("numbers %v %v %v %v", ps[5].Num, ps[6].Num, ps[7].Num, ps[8].Num)
	}
	if ps[1].Str != "T" || ps[9].Str != "0FF" || ps[10].Ref != 2 {
		t.Errorf("enum %q binary %q ref %d", ps[1].Str, ps[9].Str, ps[10].Ref)
	}
	if l := ps[11].List; len(l) != 2 || len(l[1].List) != 2 || l[1].List[1].Num != 3 {
		t.Errorf("list %+v", l)
	}
	if x, ok := num(ps[12]); !ok || x != 2.5 || ps[12].Str != "LENGTH_MEASURE" {
		t.Errorf("typed %+v", ps[12])
	}
	c := f.Get(2)
	if c.Name() != "A+B+C_LOWER" || !c.Is("B") || c.Record("Z") != nil || c.Line != 11 {
		t.Errorf("complex %q at line %d", c.Name(), c.Line)
	}
	for _, k := range []Kind{KindOmitted, KindDerived, KindInteger, KindReal, KindString, KindEnum,
		KindBinary, KindRef, KindList, KindTyped, Kind(99)} {
		if k.String() == "" {
			t.Errorf("kind %d has no name", k)
		}
	}
}

func TestParseErrors(t *testing.T) {
	head := "ISO-10303-21;\nHEADER;\nENDSEC;\nDATA;\n"
	tail := "ENDSEC;\nEND-ISO-10303-21;\n"
	cases := map[string]string{
		"not step":              "STEP-FILE;",
		"no end":                head + "#1=A();\n",
		"duplicate":             head + "#1=A();\n#1=B();\n" + tail,
		"open string":           head + "#1=A('abc);\n" + tail,
		"open comment":          head + "/* abc\n" + tail,
		"open list":             head + "#1=A((1,2);\n" + tail,
		"bad separator":         head + "#1=A(1 2);\n" + tail,
		"bad number":            head + "#1=A(+);\n" + tail,
		"bad enum":              head + "#1=A(.T);\n" + tail,
		"open binary":           head + "#1=A(\"0F);\n" + tail,
		"empty complex":         head + "#1=();\n" + tail,
		"keyword in data":       head + "FOO;\n" + tail,
		"no number":             head + "#=A();\n" + tail,
		"bad section":           "ISO-10303-21;\nFOO;\n",
		"unexpected character":  head + "#1=A(@);\n" + tail,
		"ends in a list":        head + "#1=A(1,",
		"missing semicolon":     head + "#1=A()\n#2=B();\n" + tail,
		"open section":          "ISO-10303-21;\nANCHOR;\n",
		"header without params": "ISO-10303-21;\nHEADER;\nFILE_NAME;\n",
		"huge number":           head + "#99999999999999999999=A();\n" + tail,
	}
	for name, text := range cases {
		var se *SyntaxError
		if _, err := Parse([]byte(text)); err == nil {
			t.Errorf("%s: no error", name)
		} else if errors.As(err, &se) && se.Line < 1 {
			t.Errorf("%s: line %d", name, se.Line)
		}
	}
	deep := head + "#1=A(" + strings.Repeat("(", 100) + strings.Repeat(")", 100) + ");\n" + tail
	if _, err := Parse([]byte(deep)); err == nil || !strings.Contains(err.Error(), "deeply") {
		t.Errorf("deep nesting: %v", err)
	}
	// Numbers too long for an integer read as reals; out of range reals
	// as infinities.
	f, err := Parse([]byte(head + "#1=A(123456789012345678901234567890, 1.E999, DATA_X(1));\n" + tail))
	if err != nil {
		t.Fatal(err)
	}
	if p := f.Get(1).Records[0].Params; p[0].Kind != KindReal || p[1].Kind != KindReal {
		t.Errorf("long numbers %+v", p)
	}
}

func TestProtocols(t *testing.T) {
	for schema, want := range map[string]string{
		"CONFIG_CONTROL_DESIGN": "AP203", "AP203_CONFIGURATION_CONTROLLED_3D_DESIGN_OF_MECHANICAL_PARTS_AND_ASSEMBLIES_MIM_LF": "AP203 edition 2",
		"AUTOMOTIVE_DESIGN_CC2": "AP214", "AP209_MULTIDISCIPLINARY_ANALYSIS_AND_DESIGN_MIM_LF": "AP209",
		"": "an unnamed schema", "OTHER": "OTHER",
	} {
		if got := Protocol(schema); got != want {
			t.Errorf("%q: %q, want %q", schema, got, want)
		}
	}
}

// FuzzStep parses, maps and tessellates whatever it is given, which
// must never panic or hang.
func FuzzStep(f *testing.F) {
	for _, dir := range []string{
		filepath.Join("..", "..", "..", "testdata", "step", "shapes"),
		filepath.Join("..", "..", "..", "testdata", "step", "corpus"),
	} {
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".step") || strings.HasSuffix(e.Name(), ".stp") {
				if b, err := os.ReadFile(filepath.Join(dir, e.Name())); err == nil && len(b) < 20000 {
					f.Add(b)
				}
			}
		}
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		file, err := Parse(data)
		if err != nil {
			return
		}
		m := ReadModel(file)
		// Coarse, so a hostile size can't make the fuzzer slow.
		FromModel(file.Schema(), m, Options{ChordTol: 0.5})
	})
}
