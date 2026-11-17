package topo_test

import (
	"bytes"
	"fmt"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/export"
	"github.com/telesapr/telesapr/pkg/persist/step"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// stepEdgesOffFaces writes sh as STEP and reads the file's plane faces
// back as a reader takes them: each bounded by the surface it is
// written on, over that surface's domain. It returns how far a point of
// a plane face's edges lies off the face's surface so bounded, the
// farthest, and where: what of the face lies past its surface. A STEP
// file gives the edges no curves in the surfaces' parameters, so a
// reader such as OpenCASCADE finds them by projecting the edges on the
// surface, and what of a face lies past the surface it cuts away.
func stepEdgesOffFaces(t *testing.T, sh *srf.SShell) (worst float64, where string) {
	t.Helper()
	sk := sketch.NewSketch()
	sk.NewFile()
	sk.GenerateAll()
	g := &sk.Groups[len(sk.Groups)-1]
	g.RunningShell = *sh
	var buf bytes.Buffer
	if err := export.WriteStep(&buf, sk, g, export.Options{Scale: 1}); err != nil {
		t.Fatal(err)
	}
	f, err := step.Parse(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range step.ReadModel(f).Shapes {
		for _, face := range s.Faces {
			u0, u1, v0, v1 := face.Surf.Domain()
			o, su, sv := face.Surf.Derivs(u0, v0)
			_, su1, sv1 := face.Surf.Derivs(u1, v1)
			if !su.Equals(su1) || !sv.Equals(sv1) {
				continue // not a plane
			}
			n := su.Cross(sv).WithMagnitude(1)
			for _, b := range face.Bounds {
				for _, oe := range b.Edges {
					lo, hi := oe.E.Curve.Range()
					for k := range 9 {
						p := oe.E.Curve.Eval(lo + (hi-lo)*float64(k)/8)
						u, v := face.Surf.Project(p, nil)
						u, v = math.Max(u0, math.Min(u1, u)), math.Max(v0, math.Min(v1, v))
						on := math.Abs(p.Minus(o).Dot(n))
						if d := face.Surf.Eval(u, v).Minus(p).Magnitude() - on; d > worst {
							worst, where = d, fmt.Sprintf("face #%d at %v", face.ID, p)
						}
					}
				}
			}
		}
	}
	return worst, where
}
