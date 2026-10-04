// The cylinder of path_drawing_faces.script and its end face's
// diameter on the front view of the drawing, made by a script
// (TestPathEquivalenceDrawingFaces, roadmap 29.2).
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	c := doc.Circle(V(0, 0, 0), 10)
	// Clicked on the origin, the centre is held on it.
	doc.Coincident(doc.ActiveWorkplane().Points()[0], c.Center())
	g := doc.Extrude(80, "")
	doc.Regen()
	// The end face nearer the viewer, as the front view looks.
	var end Entity
	for _, f := range g.Faces() {
		if end.H.V == 0 || f.FacePoint().Z > end.FacePoint().Z {
			end = f
		}
	}
	doc.AddDrawingFaceDimension("front", "diameter", 0, g.Solid().Face(uint32(end.H.V)))
}
