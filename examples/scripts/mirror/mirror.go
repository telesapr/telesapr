// A symmetric bracket: one half drawn and extruded, then mirrored, made
// by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/mirror/mirror.go [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/mirror/mirror.go
//
// The half is an L from the middle plane x = 0 out to x = 30, with a
// hole in its foot; New Group > Mirror in the YZ workplane makes the
// other half, hole and all, and joins it on, a T. The script prints what
// Check Solid finds and saves bracket.tlspr, unless given "-".
package main

import (
	"fmt"
	"os"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	doc := api.Current()
	doc.Polyline(true, api.V(0, 0, 0), api.V(30, 0, 0), api.V(30, 6, 0),
		api.V(6, 6, 0), api.V(6, 24, 0), api.V(0, 24, 0))
	// A hole through the foot.
	doc.Circle(api.V(18, 3, 0), 1.5)
	half := doc.Extrude(20, "half")

	// Mirror reflects the active group, the half, in the YZ workplane,
	// and joins the reflection on.
	both := doc.Mirror(doc.Workplane("YZ"), "bracket")
	r := both.Check()
	fmt.Printf("bracket: sound %v, %d faces, volume %.1f mm³ (half %.1f)\n",
		r.Sound(), r.Faces, r.Volume, half.Check().Volume)
	if !r.Sound() {
		os.Exit(1)
	}
	if len(os.Args) > 1 && os.Args[1] == "-" {
		return
	}
	if err := doc.Save("bracket.tlspr"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
