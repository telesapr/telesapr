package agent

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"

	"github.com/telesapr/telesapr/pkg/script/api"
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
1. Call api_help with no name, once, before the first script: it explains the scripting interface and lists every call. api_help with a name, as "Features.Extrude", "Group" or a menu entry, "Features > Fillet Edges", says more.
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

    doc                  the drawing (type Doc); what changes it is grouped as the
                         window's menus are: doc.Sketch.Line(...), doc.Features.Extrude(...)
    V(x, y, z)           a point or a direction, in mm
    fmt, math, strings, sort, strconv, os, time and others of Go's library

A variable defined in one run_script call is there in the next, until the program (or telesapr-cli mcp) is restarted: opening or starting another drawing doesn't clear the names. Defining one again with := gives it a new value and type; inside a loop or an if, := makes a variable of the block's own, as in Go. The value of the last statement, if it is an expression, is returned, as are the things printed with fmt.Println.

## How a drawing is built

A drawing is a list of groups, built in order. A sketch group holds what is drawn: requests (a line, a circle, an arc, ...), each with entities (the line and its two end points), and the constraints that hold them. A solid group (extrude, lathe, ...) makes a solid of the sketch group before it and combines it with the solid so far.

- A new drawing has one sketch group, in the XY workplane, and it is active. What is drawn goes into the active group, in its workplane; z is then 0, or use doc.Sketch.UV(u, v).
- Draw roughly, then constrain: doc.Sketch.Line(a, b) puts a line somewhere, doc.Constrain.Length(l, 40) and doc.Constrain.Horizontal(l) make it stay. The solver moves the geometry to fit, so read positions back (e.Pos(), e.Start().Pos()) rather than assuming them.
- Fix the sketch to the origin with doc.Constrain.Coincident(point, doc.Origin()). A sketch that is fully held has 0 degrees of freedom (g.Dof()); get_drawing and the status say how many are left.
- doc.Sketch.Rect returns its four sides, bottom first and going around counter-clockwise, already joined, horizontal and vertical. doc.Sketch.Polyline(true, points...) is a closed outline, joined but otherwise free.
- A solid needs closed outlines in the sketch. A circle or outline inside another is a hole.
- doc.Features.Extrude(depth, "name") makes a solid of the active sketch along its workplane's normal and becomes the active group. To add to that solid, start another sketch (doc.Features.SketchIn(doc.Workplane("XY"), "name"), doc.Features.SketchAt(point, u, v, "name"), or doc.Features.SketchOnFace(g.Solid().FaceAt(point), "name") in a flat face of the solid at group g, which follows the face), draw, and extrude again; g.SetCombine(Difference) makes the new solid a cut, Intersection keeps what is common.
- doc.Features.Fillet, doc.Features.Chamfer and doc.Features.Shell work on the solid so far; g.SolidEdges() and g.Solid() name its edges and faces.
- What only one kind of group has is on a view of it: g.AsFillet().SetRadius(3), g.AsExtrude().SetDraft(2). g.History() moves a group, g.Analyze() measures the solid up to it.
- To change what exists, look it up by the names get_drawing gives: doc.Group("plate"), doc.Request("r004").Entity(), doc.Constraint("c00a").Set(50). doc.Activate(g) goes back to an earlier group to draw more in it.
- A parameter is a named number: doc.Params.Set("width", "60"), then c.SetExpr("width/2") on a dimension.

## Checking the result

    g.Solved()  g.Dof()  g.Problem()  g.Conflicts()     how a group came out
    a := g.Analyze()
    a.Analyze().Volume()  a.Analyze().Bounds()  a.Analyze().Check().Sound()           the solid up to and with the group
    doc.Describe()                                      what get_drawing returns

A sketch should have 0 degrees of freedom when it is done. An extrusion keeps 1, its depth, which nothing holds unless a dimension is put on it; that is as the program's own users leave it.

get_drawing lists the requests and constraints of the active group only, which after an Extrude is the extrusion; ask for the sketch by name (get_drawing with group "g002") to see what is in it.

A call that can't do what it is asked stops the script with an error that says why, and run_script then takes back what the script did.

## What the interpreter can't do

- No "for i := range 10" over a number; write "for i := 0; i < 10; i++".
- No generic functions without their type arguments; slices.Sort, slices.Max and maps.Clone are missing (sort.Ints and slices.Contains work).
- A name that isn't defined is reported as "constant definition loop, or a name that isn't defined".
- math.Pi and math's other float constants are float64: 2.0 * math.Pi works, a float32 needs float32(math.Pi).

## Example

A plate of 60 by 20 mm, 4 mm thick, with three 5 mm holes in a row, then a round pocket 2 mm deep cut into its underside:

` + primerExample + `
## Every call

api_help with a name says more about one: a menu, "Features"; a call, "Features.Extrude" or "Group.SetCombine"; a type, "Group" or "Entity"; or an entry of the window's menu, "Features > Fillet Edges", for what a script calls in its place.

`

// primerExample is the script the Primer shows; a test runs it.
const primerExample = `    width, height, thick, bore := 60.0, 20.0, 4.0, 5.0
    sides := doc.Sketch.Rect(V(0, 0, 0), V(width, height, 0))
    doc.Constrain.Coincident(sides[0].Start(), doc.Origin())
    doc.Constrain.Length(sides[0], width)
    doc.Constrain.Length(sides[1], height)
    for i := 1; i <= 3; i++ {
        x := width * float64(i) / 4
        hole := doc.Sketch.Circle(V(x, height/2, 0), bore/2)
        doc.Constrain.Diameter(hole, bore)
        doc.Constrain.Distance(hole.Center(), sides[0], height/2)
        doc.Constrain.Distance(hole.Center(), sides[3], x)
    }
    outline := doc.Active()
    fmt.Println("solved:", outline.Solved(), "DOF:", outline.Dof())
    plate := doc.Features.Extrude(thick, "plate")
    fmt.Println("sound:", plate.Analyze().Check().Sound(), "volume:", plate.Analyze().Volume())

    doc.Features.SketchIn(doc.Workplane("XY"), "pocket-outline")
    round := doc.Sketch.Circle(V(7, 10, 0), 3)
    doc.Constrain.Diameter(round, 6)
    doc.Constrain.Distance(round.Center(), sides[0], 10)
    doc.Constrain.Distance(round.Center(), sides[3], 7)
    pocket := doc.Features.Extrude(2, "pocket")
    pocket.SetCombine(Difference)
    fmt.Println("sound:", pocket.Analyze().Check().Sound(), "volume:", pocket.Analyze().Volume())
`

// primerTypes are the types of package api whose methods the Primer
// lists after Doc's and Group's.
var primerTypes = []string{"Entity", "Request", "Constraint", "Param", "Style", "Solid", "SolidEdge", "SolidFace", "SolidVertex", "Section"}

// Primer is what a program that writes scripts should read first: how
// scripts and drawings work, what the interpreter lacks, an example,
// and every call of the scripting interface by name, as the menus of
// the window group them, taken from its documentation.
var Primer = sync.OnceValue(func() string {
	var b strings.Builder
	b.WriteString(primerHead)
	p := symbols.Docs()[apiPath]
	if p == nil {
		return b.String()
	}
	methods := func(typ string, leave ...string) string {
		td := p.Types[typ]
		if td == nil {
			return ""
		}
		names := make([]string, 0, len(td.Methods))
		for m := range td.Methods {
			if !slices.Contains(leave, m) {
				names = append(names, m)
			}
		}
		slices.Sort(names)
		return strings.Join(names, " ")
	}

	fmt.Fprintf(&b, "doc finds what the drawing holds:\n    %s\n", methods("Doc"))
	b.WriteString("\nWhat changes it, menu by menu:\n")
	for f := range reflect.TypeFor[api.Doc]().Fields() {
		if f.IsExported() {
			fmt.Fprintf(&b, "    doc.%-10s %s\n", f.Name, methods(f.Type.Name()))
		}
	}

	views := groupViews()
	accessors := make([]string, len(views))
	for i, v := range views {
		accessors[i] = v[0]
	}
	fmt.Fprintf(&b, "\nA group g, as doc.Group(\"plate\") or what a call of doc.Features returns:\n    %s\n", methods("Group", accessors...))
	b.WriteString("and its views, of what only some groups have:\n")
	for _, v := range views {
		fmt.Fprintf(&b, "    g.%-12s %s\n", v[0]+"()", methods(v[1]))
	}

	b.WriteString("\nThe methods of the other types:\n")
	for _, name := range primerTypes {
		if m := methods(name); m != "" {
			b.WriteString("    " + name + ": " + m + "\n")
		}
	}
	funcs := make([]string, 0, len(p.Funcs))
	for f := range p.Funcs {
		funcs = append(funcs, f)
	}
	slices.Sort(funcs)
	b.WriteString("\nFunctions: " + strings.Join(funcs, " ") + "\n")
	return b.String()
})

// groupViews are the methods of api.Group that give a view of the
// group, with the view's type: History, Analyze and the As of each
// kind, in the order Group declares them.
func groupViews() [][2]string {
	var out [][2]string
	order := []string{"History", "Analyze"}
	g := reflect.TypeFor[api.Group]()
	for m := range g.Methods() {
		t := m.Type
		if t.NumIn() != 1 || t.NumOut() != 1 || t.Out(0).PkgPath() != g.PkgPath() {
			continue
		}
		if slices.Contains(order, m.Name) || strings.HasPrefix(m.Name, "As") && strings.HasSuffix(t.Out(0).Name(), "Group") {
			out = append(out, [2]string{m.Name, t.Out(0).Name()})
		}
	}
	slices.SortStableFunc(out, func(a, b [2]string) int {
		ia, ib := slices.Index(order, a[0]), slices.Index(order, b[0])
		if ia < 0 {
			ia = len(order)
		}
		if ib < 0 {
			ib = len(order)
		}
		return ia - ib
	})
	return out
}
