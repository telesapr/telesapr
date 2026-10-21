// Package gears is a package of a script library: scripts that have
// examples/scripts/library/lib as their library import it as "gears".
// It is plain Go, and imports package api as any script does; it has
// no doc of its own, and draws on the one it is given.
package gears

import "github.com/telesapr/telesapr/pkg/script/api"

// Blank draws the blank of a spur gear of the given teeth and module
// (mm) about the origin, with a bore of the given diameter, and
// extrudes it width thick as a group named name.
func Blank(doc *api.Doc, teeth int, module, bore, width float64, name string) api.Group {
	outside := module * float64(teeth+2)
	rim := doc.Sketch.Circle(api.V(0, 0, 0), outside/2)
	doc.Constrain.Diameter(rim, outside)
	doc.Constrain.Coincident(rim.Center(), doc.Origin())
	hole := doc.Sketch.Circle(api.V(0, 0, 0), bore/2)
	doc.Constrain.Diameter(hole, bore)
	doc.Constrain.Coincident(hole.Center(), doc.Origin())
	return doc.Features.Extrude(width, name)
}
