// A plate with a stud under it and a tapped hole through it, both
// threaded cosmetic, then the stud's thread modelled, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/threadcosmetic/threadcosmetic.go [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/threadcosmetic/threadcosmetic.go
//
// A cosmetic thread (ThreadOptions.Cosmetic) is recorded on the part,
// not modelled: the stud, as thick as its M8 thread, and the hole,
// drilled at M10's tap drill, are left as they are, so the solid's
// volume doesn't change; drawings show both threads by the drafting
// convention, and the window draws their turns on the part. Edit makes
// the stud's thread modelled, in one step, and returns the view of the
// thread's helix group, which now keeps it. The script prints the
// volumes and saves threadcosmetic.tlspr unless given "-".
package main

import (
	"fmt"
	"math"
	"os"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// volume is the volume of the solid so far, which must be sound.
func volume(doc *api.Doc) float64 {
	r := doc.Active().Analyze().Check()
	if !r.Sound() {
		fmt.Fprintf(os.Stderr, "%s: want a sound solid: %s\n", doc.Active(), r.String())
		os.Exit(1)
	}
	return r.Volume
}

func main() {
	write := len(os.Args) < 2 || os.Args[1] != "-"
	doc := api.Current()

	doc.Sketch.Rect(api.V(-15, -15, 0), api.V(15, 15, 0))
	doc.Features.Extrude(10, "plate")
	doc.Features.SketchIn(doc.Workplane("XY"), "stud")
	doc.Sketch.Circle(api.V(-8, 0, 0), 4)
	doc.Features.Extrude(-20, "stud")
	doc.Features.SketchIn(doc.Workplane("XY"), "hole")
	doc.Sketch.Circle(api.V(8, 0, 0), 4.25)
	hole := doc.Features.Extrude(12, "drill")
	hole.SetCombine(1) // difference
	plain := volume(doc)

	s := doc.Active().Solid()
	stud := doc.Features.ThreadOnFace(s.FaceAt(api.V(-4, 0, -10)),
		api.ThreadOptions{Length: 12, Cosmetic: true}).AsThread()
	s = doc.Active().Solid()
	tapped := doc.Features.ThreadOnFace(s.FaceAt(api.V(12.25, 0, 5)),
		api.ThreadOptions{FullLength: true, Cosmetic: true}).AsThread()
	cosmetic := volume(doc)
	for _, t := range []api.ThreadGroup{stud, tapped} {
		o := t.Options()
		fmt.Printf("%s cosmetic: internal %v, %.2f mm long, %d group(s)\n",
			o.Designation, o.Internal, o.Length, len(t.Groups()))
	}
	fmt.Printf("volume %.2f mm³ plain, %.2f with the cosmetic threads\n", plain, cosmetic)
	if math.Abs(cosmetic-plain) > 1e-6*plain {
		fmt.Fprintln(os.Stderr, "a cosmetic thread changed the part")
		os.Exit(1)
	}

	o := stud.Options()
	o.Cosmetic = false
	stud = stud.Edit(o)
	fmt.Printf("%s modelled: %d groups, volume %.2f mm³\n", o.Designation, len(stud.Groups()), volume(doc))

	if write {
		if err := doc.File.Save("threadcosmetic.tlspr"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
