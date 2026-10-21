// The datum of path_datums.script, made by a script
// (TestPathEquivalenceDatums): the mouse's rectangle, its corner at
// (-20, 20) named "corner".
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	sides := doc.Sketch.Rect(V(-20, 20, 0), V(16, -6, 0))
	doc.Datums.Name("corner", sides[2].End())
}
