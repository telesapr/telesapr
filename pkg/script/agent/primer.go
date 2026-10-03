package agent

import (
	"sort"
	"strings"
	"sync"

	"github.com/telesapr/telesapr/pkg/script"
	"github.com/telesapr/telesapr/pkg/script/symbols"
)

// apiPath is the import path of the package scripts use.
const apiPath = "github.com/telesapr/telesapr/pkg/script/api"

// Instructions is what a program is told when it connects: what the
// tools are for and the order to use them in. Clients cut long
// instructions short, so it stays under MaxInstructions; the rest is
// in the Primer.
const Instructions = `TeleSAPR is a parametric CAD program: sketches of lines, circles and arcs held by constraints, and solids made from them (extrude, lathe, revolve, sweep, loft, fillet, chamfer, shell, mirror, step and repeat). These tools work on the drawing the user has open, by running Go scripts on it.

To draw what the user asks for:
1. Call api_help with no name, once, before the first script: it explains the scripting interface and lists every call. api_help with a name, as "Doc.Extrude" or "Group", says more.
2. Call get_drawing to see what is there: groups, requests, constraints, the selection.
3. Call run_script with Go statements. doc is the drawing; lengths are mm, angles degrees. Variables stay defined between calls. Each call is one step of undo for the user. If a script fails, what it changed is taken back: correct it and send the whole of it again.
4. Read the status run_script returns. A sketch should be "solved", and a finished one has 0 DOF: constrain it with dimensions and with a point on the origin. A "problem" means a group made no solid.
5. Call render and look at the picture before saying the work is done.

Draw what was asked for and no more. Leave what the user drew as it is unless asked to change it. Do not save, export or load files unless asked.`

// MaxInstructions is the most bytes Instructions, and the description
// of a tool, may have: clients are known to cut them at 2 KB.
const MaxInstructions = 2000

// primerHead is the Primer before the list of calls.
const primerHead = `# Scripting TeleSAPR

A script is Go, run by an interpreter inside TeleSAPR. In run_script, write statements as in the body of a function; nothing needs importing or declaring:

    doc                  the drawing (type Doc); every call below is doc.Something(...)
    V(x, y, z)           a point or a direction, in mm
    fmt, math, strings, sort, strconv, os, time and others of Go's library

A variable defined in one run_script call is there in the next. The value of the last statement, if it is an expression, is returned, as are the things printed with fmt.Println.

## How a drawing is built

A drawing is a list of groups, built in order. A sketch group holds what is drawn: requests (a line, a circle, an arc, ...), each with entities (the line and its two end points), and the constraints that hold them. A solid group (extrude, lathe, ...) makes a solid of the sketch group before it and combines it with the solid so far.

- A new drawing has one sketch group, in the XY workplane, and it is active. What is drawn goes into the active group, in its workplane; z is then 0, or use doc.UV(u, v).
- Draw roughly, then constrain: doc.Line(a, b) puts a line somewhere, doc.Length(l, 40) and doc.Horizontal(l) make it stay. The solver moves the geometry to fit, so read positions back (e.Pos(), e.Start().Pos()) rather than assuming them.
- Fix the sketch to the origin with doc.Coincident(point, doc.Origin()). A sketch that is fully held has 0 degrees of freedom (g.Dof()); get_drawing and the status say how many are left.
- doc.Rect returns its four sides, bottom first and going around counter-clockwise, already joined, horizontal and vertical. doc.Polyline(true, points...) is a closed outline, joined but otherwise free.
- A solid needs closed outlines in the sketch. A circle or outline inside another is a hole.
- doc.Extrude(depth, "name") makes a solid of the active sketch along its workplane's normal and becomes the active group. To add to that solid, start another sketch (doc.SketchIn(doc.Workplane("XY"), "name"), doc.SketchAt(point, u, v, "name")), draw, and extrude again; g.SetCombine(Difference) makes the new solid a cut, Intersection keeps what is common.
- doc.Fillet, doc.Chamfer and doc.Shell work on the solid so far; g.SolidEdges() and g.Solid() name its edges and faces.
- To change what exists, look it up by the names get_drawing gives: doc.Group("plate"), doc.Request("r004").Entity(), doc.Constraint("c00a").Set(50). doc.Activate(g) goes back to an earlier group to draw more in it.
- A parameter is a named number: doc.SetParameter("width", "60"), then c.SetExpr("width/2") on a dimension.

## Checking the result

    g.Solved()  g.Dof()  g.Problem()  g.Conflicts()     how a group came out
    g.Volume()  g.Bounds()  g.Check().Sound()           the solid up to and with the group
    doc.Describe()                                      what get_drawing returns

A sketch should have 0 degrees of freedom when it is done. An extrusion keeps 1, its depth, which nothing holds unless a dimension is put on it; that is as the program's own users leave it.

get_drawing lists the requests and constraints of the active group only, which after an Extrude is the extrusion; ask for the sketch by name (get_drawing with group "g002") to see what is in it.

A call that can't do what it is asked stops the script with an error that says why, and run_script then takes back what the script did.

## What the interpreter can't do

- No "for i := range 10" over a number; write "for i := 0; i < 10; i++".
- No generic functions without their type arguments; slices.Sort, slices.Max and maps.Clone are missing (sort.Ints and slices.Contains work).
- A name that isn't defined is reported as "constant definition loop, or a name that isn't defined".

## Example

A plate of 60 by 20 mm, 4 mm thick, with three 5 mm holes in a row, then a round pocket 2 mm deep cut into its underside:

` + primerExample + `
## Every call

api_help with a name says more about one: "Doc.Extrude", "Group", "Group.SetCombine", "Entity".

`

// primerExample is the script the Primer shows; a test runs it.
const primerExample = `    width, height, thick, bore := 60.0, 20.0, 4.0, 5.0
    sides := doc.Rect(V(0, 0, 0), V(width, height, 0))
    doc.Coincident(sides[0].Start(), doc.Origin())
    doc.Length(sides[0], width)
    doc.Length(sides[1], height)
    for i := 1; i <= 3; i++ {
        x := width * float64(i) / 4
        hole := doc.Circle(V(x, height/2, 0), bore/2)
        doc.Diameter(hole, bore)
        doc.Distance(hole.Center(), sides[0], height/2)
        doc.Distance(hole.Center(), sides[3], x)
    }
    outline := doc.Active()
    fmt.Println("solved:", outline.Solved(), "DOF:", outline.Dof())
    plate := doc.Extrude(thick, "plate")
    fmt.Println("sound:", plate.Check().Sound(), "volume:", plate.Volume())

    doc.SketchIn(doc.Workplane("XY"), "pocket-outline")
    round := doc.Circle(V(7, 10, 0), 3)
    doc.Diameter(round, 6)
    doc.Distance(round.Center(), sides[0], 10)
    doc.Distance(round.Center(), sides[3], 7)
    pocket := doc.Extrude(2, "pocket")
    pocket.SetCombine(Difference)
    fmt.Println("sound:", pocket.Check().Sound(), "volume:", pocket.Volume())
`

// primerTypes are the types of package api whose methods the Primer
// lists by name, after Doc's, which it lists in full.
var primerTypes = []string{"Group", "Entity", "Request", "Constraint", "Param", "Solid", "SolidEdge", "SolidFace", "SolidVertex", "Section"}

// Primer is what a program that writes scripts should read first: how
// scripts and drawings work, what the interpreter lacks, an example,
// and every call of the scripting interface, taken from its
// documentation.
var Primer = sync.OnceValue(func() string {
	var b strings.Builder
	b.WriteString(primerHead)
	script.Help(&b, "Doc")

	p := symbols.Docs()[apiPath]
	if p == nil {
		return b.String()
	}
	b.WriteString("\nThe methods of the other types:\n")
	for _, name := range primerTypes {
		td := p.Types[name]
		if td == nil {
			continue
		}
		methods := make([]string, 0, len(td.Methods))
		for m := range td.Methods {
			methods = append(methods, m)
		}
		sort.Strings(methods)
		b.WriteString("    " + name + ": " + strings.Join(methods, " ") + "\n")
	}
	funcs := make([]string, 0, len(p.Funcs))
	for f := range p.Funcs {
		funcs = append(funcs, f)
	}
	sort.Strings(funcs)
	b.WriteString("\nFunctions: " + strings.Join(funcs, " ") + "\n")
	return b.String()
})
