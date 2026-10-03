// Editing the history of a model from a script: what each group uses,
// moving a group, suppressing one with what uses it, and what a
// deletion takes with it.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/history/history.go
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/history/history.go
//
// It makes a block with a boss on top and copies of the boss, then
// edits that history and prints what each step did. It writes
// history.tlspr where it is run, before the last step deletes most of
// it, unless it is given "-".
package main

import (
	"fmt"
	"os"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func main() {
	doc := api.Current()

	// A block, a boss drawn on the block's top, and copies of the boss.
	base := doc.Active()
	doc.Rect(api.V(0, 0, 0), api.V(40, 20, 0))
	block := doc.Extrude(10, "block")
	var top api.Entity
	for _, e := range block.Entities() {
		if e.Kind() == "point" && e.Pos().Z > 5 {
			top = e
			break
		}
	}
	boss := doc.SketchAt(top, api.V(1, 0, 0), api.V(0, 1, 0), "boss-sketch")
	doc.Circle(api.V(10, 10, 10), 3)
	post := doc.Extrude(5, "boss")
	doc.Translate(2, api.V(10, 0, 0), "bosses")
	doc.Regen()
	solid := doc.Active().Check().Volume

	// What each group uses, and what uses it.
	for _, g := range doc.Groups() {
		fmt.Printf("%s uses %v, used by %v\n", g, g.DependsOn(), g.UsedBy())
	}

	// The boss can't come before the sketch it extrudes, and the
	// error says why.
	if err := post.MoveBefore(boss); err == nil {
		fail("the boss moved before its sketch")
	} else {
		fmt.Println("refused:", err)
	}

	// Suppressed whole, the boss sketch takes the boss and its copies
	// out of the model; nothing is deleted, and they come back.
	boss.SetSuppress(api.SuppressWhole)
	fmt.Printf("%s: %q; the solid is %.0f mm³\n", post, post.LeftOut(), doc.Groups()[1].Check().Volume)
	// The active group had to move back to the block; the last group
	// is made active again, and the whole model regenerated, since the
	// chord tolerance follows the size of what is shown.
	boss.SetSuppress(api.Unsuppressed)
	doc.Activate(doc.Groups()[len(doc.Groups())-1])
	doc.Regen()
	if v := doc.Active().Check().Volume; v != solid {
		fail("back, the solid is %v, was %v", v, solid)
	}

	if len(os.Args) < 2 || os.Args[1] != "-" {
		must(doc.Save("history.tlspr"))
	}

	// Deleting the block's sketch deletes what was built on it, and the
	// list says what and why. (In the application, Edit > Undo brings
	// it all back; in a script, Undo would take back the whole run.)
	base.Delete()
	for _, b := range doc.BrokenReferences() {
		fmt.Println(b.Text)
	}
	if len(doc.BrokenReferences()) != 4 || len(doc.Groups()) != 1 {
		fail("deleting the sketch broke %d references and left %d groups, want 4 and 1",
			len(doc.BrokenReferences()), len(doc.Groups()))
	}
}
