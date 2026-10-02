package script

import (
	"strings"
	"testing"
)

// helped is what a console shows for the lines, each a call of help.
func helped(t *testing.T, lines ...string) string {
	t.Helper()
	return console(t, newHost(), lines...)
}

func mustContain(t *testing.T, got string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Errorf("help doesn't say %q:\n%s", want, got)
		}
	}
}

// help of a value says what it is and what it can do, with the
// parameters' names and the comments of the source.
func TestHelpOfValues(t *testing.T) {
	got := helped(t, `help(doc.Active())`)
	mustContain(t, got,
		"g002-sketch-in-plane\n\ntype api.Group struct\n    Group is a group of the sketch",
		"Fields:\n    H                    core.HGroup",
		"    Bounds() (min, max Vec)                      Bounds are the corners of the box",
		"    SetColor(red, green, blue int)               SetColor colours the group's solid",
		`help("Group.Method") says more about one.`)

	// A pointer has the methods of what it points at.
	got = helped(t, `help(doc)`)
	mustContain(t, got, "type api.Doc struct", "Made by:\n    Current() *Doc",
		"    Line(a, b Vec) Entity                        Line adds a line segment from a to b.",
		// A declaration too long for its column has its line to itself.
		"    Constrain(t core.ConstraintType, ptA, ptB, entityA, entityB Entity, other, other2 bool) Constraint\n"+
			strings.Repeat(" ", 49)+"Constrain adds a constraint of any type")

	// A list is described by what is in it.
	got = helped(t, `help(doc.Groups())`)
	mustContain(t, got, "[]api.Group: a list of 1; index it, or range over it. Each is:\n\ntype api.Group struct")

	// Types of the other packages, and of Go's.
	got = helped(t, `help(doc.Origin().Pos())`, `help(doc.Active().Type())`, `help(time.Second)`)
	mustContain(t, got, "(0, 0, 0)\n\ntype geom.Vector struct", "    X                    float64",
		"    Plus(b Vector) Vector",
		"type sketch.GroupType int\n\nValues:\n    GroupChamfer GroupDrawing3D GroupDrawingWorkplane GroupExtrude",
		"1s\n\ntype time.Duration int64", "    Seconds() float64")

	got = helped(t, `help(3.5)`, `help("a" == "b")`, `help(nil)`)
	mustContain(t, got, "float64 = 3.5\n", "bool = false\n",
		"nil: a value that isn't there has no type to tell about.")
}

// A function of a package is told from its value; a method isn't,
// and is named instead. A type a script made shows its fields.
func TestHelpOfFunctionsAndScriptTypes(t *testing.T) {
	got := helped(t, `help(V)`, `help(strings.ToUpper)`, `help(doc.Line)`)
	mustContain(t, got, "func V(x, y, z float64) Vec\n    V is the vector (x, y, z).",
		"func ToUpper(s string) string\n    ToUpper returns s with all Unicode letters",
		"func(geom.Vector, geom.Vector) api.Entity\n    A function, or a method of some value")

	got = helped(t, `type hole struct { At Vec; Dia float64 }`, `help(hole{V(1, 2, 0), 5})`)
	mustContain(t, got, "Fields:\n    At                   geom.Vector\n    Dia                  float64",
		"A type made in a script")
}

// help of a name looks it up: a package, a type, a function, a
// method, a constant, with or without its package.
func TestHelpOfNames(t *testing.T) {
	got := helped(t, `help("Group.Bounds")`, `help("api.Doc.Line")`, `help("geom.Vector.Plus")`,
		`help("strings.ToUpper")`, `help("Union")`, `help("Inch")`)
	want := `func (g Group) Bounds() (min, max Vec)
    Bounds are the corners of the box around the model up to and with
    this group.
func (d *Doc) Line(a, b Vec) Entity
    Line adds a line segment from a to b. In a workplane the points are
    put in it.
func (v Vector) Plus(b Vector) Vector
func ToUpper(s string) string
    ToUpper returns s with all Unicode letters mapped to their upper case.
const Union
    How a group's solid combines with the solid so far.
func Inch(v float64) float64
    Inch is v inches in mm.
`
	if got != want {
		t.Errorf("help of names showed:\n%s\nwant:\n%s", got, want)
	}

	got = helped(t, `help("api")`, `help("Entity")`, `help("math")`)
	mustContain(t, got, "package api  // import \"github.com/spacesolve/spacesolve/pkg/script/api\"\n    Package api is what a script sees",
		"Types:\n    BrokenReference      BrokenReference is something a regeneration deleted",
		"\n    Constraint           Constraint is a constraint of the sketch",
		"Functions:\n    Commands() []string", "Constants and variables:\n    Assemble Difference",
		"type api.Entity struct\n    Entity is something on the drawing",
		"    Pos() (p Vec)                                Pos is where a point is.",
		"package math  // import \"math\"", "    Sqrt(x float64) float64")

	got = helped(t, `help("no such thing")`, `help()`)
	mustContain(t, got, `"no such thing" is a string, and nothing help knows is called that.`,
		"doc is the sketch. Lengths are in mm", "help(x) says what a value is")
}

// Every package a script can import has its declarations in the
// table, which is generated: this fails when "make generate" is due.
func TestHelpCoversThePackages(t *testing.T) {
	got := helped(t, `help("api")`)
	for _, name := range []string{"Current", "New", "Open", "V", "Inch"} {
		if !strings.Contains(got, "    "+name+"(") {
			t.Errorf("help(\"api\") doesn't list %s; run make generate", name)
		}
	}
	for _, p := range helpPackages() {
		if !helpName(new(strings.Builder), p) {
			t.Errorf("help knows nothing of package %s", p)
		}
	}
}
