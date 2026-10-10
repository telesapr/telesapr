// The model of path_units.script, made by a script
// (TestPathEquivalenceUnits).
package main

import (
	"github.com/telesapr/telesapr/pkg/core"
	. "github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	doc := Current()
	doc.View.SetUnits(core.UnitInches)
	doc.Sketch.Rect(V(-20, 20, 0), V(16, -6, 0))
}
