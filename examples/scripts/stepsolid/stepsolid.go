// A STEP file linked as a solid and cut, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/stepsolid/stepsolid.go [file.step]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/stepsolid/stepsolid.go
//
// LinkStep with asSolid set reads a STEP file's faces exactly, so the
// linked part takes part in Booleans as a solid made here does. Given a
// STEP file, the script links it as a solid and prints what was read.
// Given none, it makes one first: a rounded boss on a plate, exported
// as boss.step, then linked back as a solid, a slot cut across it by an
// extrusion, and the volumes compared with the original's; the
// original is suppressed once exported, so the checks are of the link.
package main

import (
	"fmt"
	"math"
	"os"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	doc := api.Current()
	doc.SetChordTolerance(0.05)
	path := "boss.step"
	var plate, boss api.Group
	if len(os.Args) > 1 {
		path = os.Args[1]
	} else {
		doc.Rect(api.V(-30, -20, 0), api.V(30, 20, 0))
		plate = doc.Extrude(6, "plate")
		doc.SketchIn(doc.Workplane("XY"), "boss")
		doc.Circle(api.V(0, 0, 0), 12)
		boss = doc.Extrude(20, "boss")
		if err := doc.Export(path); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
	whole := 0.0
	if boss.Valid() {
		whole = boss.Check().Volume
		plate.SetSuppress(api.SuppressSolid)
		boss.SetSuppress(api.SuppressSolid)
	}
	part, err := doc.LinkStep(path, "part", true)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	for _, l := range part.LinkInfo() {
		fmt.Println(l)
	}
	if p := part.LinkProblem(); p != "" {
		fmt.Println("problem:", p)
	}
	r := part.Check()
	fmt.Printf("linked: %s, %.1f mm³\n", map[bool]string{true: "sound", false: "NOT sound"}[r.Sound()], r.Volume)
	if !boss.Valid() {
		return
	}
	// A slot 6 wide and 6 deep across the boss's top, cut from the
	// linked solid as from any other.
	doc.Sketch3D("")
	o := doc.Point(api.V(0, 0, 14))
	doc.SketchAt(o, api.V(1, 0, 0), api.V(0, 1, 0), "slot")
	doc.Rect(api.V(-40, -3, 14), api.V(40, 3, 14))
	slot := doc.Extrude(10, "slot")
	slot.SetCombine(api.Difference)
	cut := slot.Check()
	fmt.Printf("slotted: %s, %.1f mm³ (the original %.1f)\n",
		map[bool]string{true: "sound", false: "NOT sound"}[cut.Sound()], cut.Volume, whole)
	if !r.Sound() || math.Abs(r.Volume-whole) > 0.01*whole || !cut.Sound() || cut.Volume >= r.Volume {
		os.Exit(1)
	}
}
