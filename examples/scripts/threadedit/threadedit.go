// A threaded rod whose thread is changed after it is made, as one,
// made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/threadedit/threadedit.go [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/threadedit/threadedit.go
//
// Features.Thread makes four groups; their last, the helix, keeps what
// the thread was made with. AsThread reads that back, and Edit makes
// the four again from new options in one step, as the [edit thread]
// link of the helix group's screen does. Here an M8 thread becomes a
// fine M8x1 with a rounded crest, which prints better, and its length
// follows the parameter "len": changing it in the parameter table
// (Edit > Parameters) regenerates the thread. Its drawing shows the
// thread by the drafting convention, as drawings do unless their
// ModelledThreads is set. The script checks the solid, prints it, and
// saves threadedit.tlspr and the drawing, threadedit.pdf, unless
// given "-".
package main

import (
	"fmt"
	"os"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// check stops the script unless the group's solid is sound.
func check(g api.Group) {
	r := g.Analyze().Check()
	fmt.Printf("%s: %.2f mm³, sound %v, %d dof\n", g, r.Volume, r.Sound(), g.Dof())
	if !r.Sound() || g.Dof() != 0 {
		fmt.Fprintf(os.Stderr, "%s: want a sound solid and 0 dof\n", g)
		os.Exit(1)
	}
}

func main() {
	write := len(os.Args) < 2 || os.Args[1] != "-"
	doc := api.Current()

	rod := doc.Sketch.Circle(api.V(0, 0, 0), 4)
	doc.Constrain.Coincident(rod.Center(), doc.Origin())
	doc.Constrain.Diameter(rod, 8)
	doc.Features.Extrude(30, "rod")

	t := doc.Features.Thread(rod, api.ThreadOptions{Designation: "M8", Length: 12}).AsThread()
	check(t.Groups()[3])

	if err := doc.Params.Set("len", "16"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	o := t.Options()
	o.Designation, o.Crest, o.LengthExpr = "M8x1", "rounded", "len"
	t.Edit(o)
	o = t.Options()
	fmt.Printf("thread %s, %s crest, %.2f mm long (%s)\n", o.Designation, o.Crest, o.Length, o.LengthExpr)
	if o.Designation != "M8x1" || o.Length != 16 {
		fmt.Fprintln(os.Stderr, "the thread was not edited")
		os.Exit(1)
	}
	check(t.Groups()[3])

	d := doc.Drawing.Get()
	d.Title, d.Hidden = "THREADED ROD", true
	doc.Drawing.Set(d)

	if write {
		if err := doc.File.Save("threadedit.tlspr"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err := doc.Drawing.Export("threadedit.pdf"); err != nil {
			fmt.Println("no window to draw in; write the sheet with: telesapr-cli export-drawing -o threadedit.pdf threadedit.tlspr")
		}
	}
}
