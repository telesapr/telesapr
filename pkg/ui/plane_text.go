package ui

import (
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/render"
)

// readableInPlane places text of width w and height h in the box of a
// plane whose lower left corner, along the plane's u and v, is corner:
// the origin and basis to draw it with so that it reads left to right
// and upright on the screen. Drawn along u and v as they are, the text
// is mirrored where the plane faces away from the view, as a thread's
// workplane does seen from above, and upside down where v points down
// the screen; one of the four ways of flipping u and v reads right, and
// the box stays where it was. Unit u and v are taken, as a workplane's
// are.
func readableInPlane(camera *render.Camera, corner, u, v geom.Vector, w, h float64) (o, uu, vv geom.Vector) {
	right, up := camera.ProjVectors()
	su, sv := 1.0, 1.0
	ux, uy := u.Dot(right), u.Dot(up)
	vx, vy := v.Dot(right), v.Dot(up)
	switch {
	case ux*vy-uy*vx < -readableEps:
		// Mirrored: flip whichever of u and v leaves v the further up
		// the screen.
		if -vy > vy {
			sv = -1
		} else {
			su = -1
		}
	case vy < -readableEps:
		// Upside down: turn it half round.
		su, sv = -1, -1
	}
	o = corner
	if su < 0 {
		o = o.Plus(u.ScaledBy(w))
	}
	if sv < 0 {
		o = o.Plus(v.ScaledBy(h))
	}
	return o, u.ScaledBy(su), v.ScaledBy(sv)
}

// readableEps is how far from edge on, or from level, a plane's basis
// must be on the screen for text in it to be turned: a plane seen edge
// on, as the side planes are from above, keeps its text as it was.
const readableEps = 1e-6
