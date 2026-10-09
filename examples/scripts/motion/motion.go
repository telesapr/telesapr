// A plate grown through a range of widths, a mesh saved at each step:
// a motion study made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/motion/motion.go [steps] [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/motion/motion.go
//
// A motion study drives one parameter through a range in steps,
// regenerating the model at each, and saves a frame of each step if
// asked: a picture of the window's view for .png, in the application,
// or a mesh. Here a plate's width w goes from 20 to 80; each step's
// volume is checked, and its mesh written to frames/, unless given
// "-". Assembly > Motion Study does the same from the window, where
// .png frames make an animation.
package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func main() {
	steps, write := 6, true
	for _, a := range os.Args[1:] {
		if a == "-" {
			write = false
		} else if n, err := strconv.Atoi(a); err == nil {
			steps = n
		}
	}
	doc := api.Current()
	must(doc.SetParameter("w", "40"))
	sides := doc.Rect(api.V(0, 0, 0), api.V(40, 30, 0))
	must(doc.Length(sides[0], 40).SetExpr("w"))
	doc.Length(sides[1], 30)
	doc.Extrude(5, "plate")

	frames := ""
	if write {
		must(os.MkdirAll("frames", 0o755))
		frames = filepath.Join("frames", "plate-%02d.stl")
	}
	out, err := doc.MotionStudy("w", 20, 80, steps, frames)
	must(err)
	for _, f := range out {
		want := f.Value * 30 * 5
		fmt.Printf("w %5.1f: %8.1f mm³ (%8.1f by the sizes) %s\n", f.Value, f.Volume, want, f.File)
		if !f.Solved || math.Abs(f.Volume-want) > 1e-6*want {
			os.Exit(1)
		}
	}
}
