package topo

import (
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A drafted prism is an extrusion whose sides lean into its material,
// as a moulded part's do so that it comes out of the mould (roadmap
// 15.4, E13). The profile's plane is the neutral one: the sides pass
// through the profile there, and the far end's outline is the profile
// offset into the material by the length times the angle's tangent, as
// a shell offsets an end (shellprism.go). A line's side is a plane, an
// arc's a cone about the arc's axis. Each piece of the profile is joined
// to its offset copy by a ruled patch, and the ends close it, face by
// face, with no Boolean. Where an arc meets a line at a sharp corner,
// their sides meet in a conic, and where two arcs do, in a curve of
// degree four that a quintic follows (draftsides.go).

// DraftedPrism is the solid of a profile, loops in one plane square to
// along, the first the outer one and the rest holes, extruded by along
// with its sides leaning in by angle radians (out when negative). The
// faces get color and origins from origin up, 0 leaving them to the
// caller.
func DraftedPrism(loops [][]srf.SBezier, along geom.Vector, angle float64, color core.RgbaColor, origin uint32) (*srf.SShell, error) {
	if math.IsNaN(angle) || math.Abs(angle) >= math.Pi/2-1e-6 {
		return nil, fmt.Errorf("a draft of %.6g° leans the sides flat", angle*180/math.Pi)
	}
	angles := make([][]float64, len(loops))
	for k, l := range loops {
		for range l {
			angles[k] = append(angles[k], angle)
		}
	}
	return DraftedPrismAngles(loops, angles, along, color, origin)
}

// pieceTangentAt is an arc's direction of travel where its radial
// direction is radial.
func pieceTangentAt(p profilePiece, radial, n geom.Vector) geom.Vector {
	t := n.WithMagnitude(1).Cross(radial)
	if p.angle < 0 {
		t = t.ScaledBy(-1)
	}
	return t
}
