package script

import "testing"

// A call of two results beside other arguments is said in the
// script's words, not the interpreter's; alone it prints both.
func TestSeveralValuesBesideArguments(t *testing.T) {
	got := console(t, newHost(),
		`fmt.Println(doc.Active().Bounds())`,
		`fmt.Println("box", doc.Active().Bounds())`,
		`fmt.Printf("%v\n", doc.Active().Bounds())`,
	)
	const msg = "error: a call that gives several values (geom.Vector, geom.Vector) can't stand beside other arguments; name them first, as lo, hi := g.Bounds()\n"
	want := "{0 0 0} {0 0 0}\n" + msg + msg
	if got != want {
		t.Errorf("the console showed:\n%s\nwant:\n%s", got, want)
	}
}
