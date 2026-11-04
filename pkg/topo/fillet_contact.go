package topo

import (
	"errors"
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
)

// contactSamples is how many points along a circular edge its tangent
// curves are checked at.
const contactSamples = 64

// contactsClear checks that a circular edge's fillet, or chamfer, lies on
// its faces: that each tangent curve stays inside its face, clear of the
// face's edges other than those the fillet rounds or meets at its ends.
// A closed fillet round a rod through a plate whose foot reaches the
// plate's sides ran off the plate's top unseen: the torus was made and
// trimmed against the sides, the solid left open (69 naked edges at
// r 2 for a rod of 4 in a plate 12 wide). Running off the face is
// refused here; where every point that runs off goes past a straight
// edge into a flat side square to the face, the refusal says which
// sides and how far past them (runOff), and filletPastSides makes the
// fillet cut back by their planes.
func (t *Topology) contactsClear(fe *filletEdge, rounded map[int]int, flat bool) error {
	// The edges at the chain's vertices: a corner's own, or the next
	// piece of the outline, which the fillet meets at its ends.
	near := map[int]bool{}
	for e := range rounded {
		for _, v := range t.Edges[e].V {
			for _, x := range t.Vertices[v].Edges {
				near[x] = true
			}
		}
	}
	margin := 2*t.shell.Tol.ChordTolMm() + 100*geom.LengthEps
	what := "fillet"
	if flat {
		what = "chamfer"
	}
	for k := range 2 {
		f := fe.f[k]
		var others [][]geom.Vector
		var otherEdges []int
		for _, l := range t.Faces[f].Loops {
			for _, ce := range t.Loops[l].CoEdges {
				e := t.CoEdges[ce].Edge
				if _, ok := rounded[e]; ok || near[e] {
					continue
				}
				others = append(others, t.Edges[e].Points)
				otherEdges = append(otherEdges, e)
			}
		}
		n, flat := t.planarNormal(f)
		var first *geom.Vector
		off := &runOff{face: f, k: k, n: n, sides: map[int]*runOffSide{}}
		if flat {
			off.color = t.Surface(f).Color
			off.at = t.Vertices[t.Edges[t.CoEdges[t.Loops[t.Faces[f].Loops[0]].CoEdges[0]].Edge].V[0]].P
		}
		for i := range contactSamples {
			u := (float64(i) + 0.5) / contactSamples
			q := fe.curve.PointAt(u)
			p := q.Plus(fe.into(k, q).ScaledBy(fe.ss[k] * fe.lawAt(u)))
			inside := !flat || t.inFace(f, p, n)
			apart := true
			for _, pts := range others {
				if distanceToPolyline(p, pts) <= margin {
					apart = false
					break
				}
			}
			if inside && apart {
				continue
			}
			if first == nil {
				first = &p
			}
			if off != nil && flat {
				off = t.offInto(off, p, n, others, otherEdges)
			} else {
				off = nil
			}
		}
		if first != nil {
			err := &FilletError{Msg: fmt.Sprintf("the %s runs off the face on side %d here, past its edge: make it smaller", what, k+1), At: *first}
			if off != nil && len(off.sides) > 0 {
				err.off = off
			}
			return err
		}
	}
	return nil
}

// lawAt is the share of the edge's radius at its start that its
// radius is at u along it: 1 where it doesn't vary, else the law of
// varyingRows, from 1 at its start to r1/r at its end.
func (fe *filletEdge) lawAt(u float64) float64 {
	if fe.r1 == 0 {
		return 1
	}
	a, v := fe.r1/fe.r, 1-u
	return v*v*v + 3*u*v*v + a*(3*u*u*v+u*u*u)
}

// clearOfFaces checks every circular edge of the network with
// contactsClear. Where the foot runs off its flat face into flat sides
// only, the refusal says which sides, of all the pieces of the circle.
func (t *Topology) clearOfFaces(edges []filletEdge, byEdge map[int]int, flat bool) error {
	var first *FilletError
	for i := range edges {
		if !edges[i].circ {
			continue
		}
		err := t.contactsClear(&edges[i], byEdge, flat)
		if err == nil {
			continue
		}
		fx, ok := errors.AsType[*FilletError](err)
		if !ok || fx.off == nil {
			return err
		}
		if first == nil {
			first = fx
			continue
		}
		if first.off.face != fx.off.face {
			first.off = nil
			return first
		}
		for s, side := range fx.off.sides {
			if have := first.off.sides[s]; have != nil {
				have.past = math.Max(have.past, side.past)
			} else {
				first.off.sides[s] = side
			}
		}
	}
	if first != nil {
		return first
	}
	return nil
}
