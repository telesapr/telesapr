package check

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// touchSamples is how many points along a problem edge touchAt looks at
// for where it comes nearest each part.
const touchSamples = 32

// touchAt reports whether the solid of a group is not sound only because
// its part and the solid before it touch at a point or along a line
// (roadmap K3), as a ball resting on a plate or a rod lying on one:
// every naked or self-intersecting edge of the mesh comes within the
// chord tolerance of both, and the solid's volume is theirs added, so
// they overlap nowhere. Their union is then the true result, not a
// manifold where they touch, and not a Boolean that failed. It gives a
// point of the contact, and false when the Boolean failed, when the
// group added no shell, or when either test fails.
func touchAt(sk *sketch.Sketch, src *sketch.Group, problems *srf.SEdgeList, volume float64) (geom.Vector, bool) {
	if len(problems.Edges) == 0 || src.RunningShell.BooleanFailed || len(src.ThisShell.Surface) == 0 {
		return geom.Vector{}, false
	}
	prev := src.RunningMeshGroup(sk)
	if prev == nil || len(prev.RunningShell.Surface) == 0 {
		return geom.Vector{}, false
	}
	var mine, before srf.SMesh
	src.ThisShell.TriangulateInto(&mine)
	prev.RunningShell.TriangulateInto(&before)
	if mine.IsEmpty() || before.IsEmpty() {
		return geom.Vector{}, false
	}
	sum := mine.CalculateVolume() + before.CalculateVolume()
	if math.Abs(volume-sum) > 1e-3*sum {
		return geom.Vector{}, false
	}
	tm, tb := srf.NewMeshTree(&mine), srf.NewMeshTree(&before)
	tol := src.RunningShell.Tol.ChordTolMm() + geom.LengthEps
	var at geom.Vector
	for k, e := range problems.Edges {
		// Where along the edge it comes nearest both parts at once.
		best, bestP := math.Inf(1), geom.Vector{}
		for i := range touchSamples + 1 {
			p := e.A.Plus(e.B.Minus(e.A).ScaledBy(float64(i) / touchSamples))
			dm, _, _ := tm.NearestPoint(p)
			db, _, _ := tb.NearestPoint(p)
			if d := max(dm, db); d < best {
				best, bestP = d, p
			}
		}
		if best > tol {
			return geom.Vector{}, false
		}
		if k == 0 {
			at = bestP
		}
	}
	return at, true
}
