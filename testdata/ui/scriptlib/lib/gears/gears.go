// Package gears is the library package of the ext_script_library
// scenario.
package gears

import "github.com/telesapr/telesapr/pkg/script/api"

// Disc draws a circle of the pitch diameter of a gear of module 2.
func Disc(doc *api.Doc, teeth int) api.Entity {
	return doc.Sketch.Circle(api.V(0, 0, 0), float64(teeth))
}
