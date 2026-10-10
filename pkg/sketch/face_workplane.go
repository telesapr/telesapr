package sketch

import (
	"errors"
	"fmt"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// A sketch on a face (F1) is a sketch in a workplane whose plane is a
// flat face of the solid so far, SolidFaces[0], found again at every
// regeneration: when an edit upstream moves the face, the workplane and
// what is drawn in it move with it. The face is named by its identity,
// so a face no entity stands for, as a fillet's or a STEP solid's, is
// named as a shell names the faces it takes away. topo.FaceFrame puts
// the origin on a corner of the face and u along one of its edges.
//
// The workplane's entities are made before the solid of the same
// regeneration, from the solid of the groups before it, which are made
// by then. Where it was last found is kept in Predef, and saved: it
// holds the workplane while the face can't be found, as on loading,
// where the first pass, which finds the model's size, has no solids yet,
// and when the face is gone, which is then a problem on the group.

// NewSketchOnFace starts a sketch whose workplane lies in the flat face
// face of the solid at the active group, its normal out of the solid. It
// is an error when that solid has no such face, or the face is not flat.
// It is what the interface and scripts add with AddGroupAfterActive.
func (sk *Sketch) NewSketchOnFace(face srf.FaceID, name string) (Group, error) {
	g := NewGroup()
	g.Type = GroupDrawingWorkplane
	g.Subtype = GroupWorkplaneByFace
	g.Name = name
	if name == "" {
		g.Name = "sketch-on-face"
	}
	fr, err := sk.faceFrameAt(sk.solidAt(sk.Active), face)
	if err != nil {
		return g, err
	}
	g.SolidFaces = []srf.FaceID{face}
	g.Predef.Q, g.Predef.At = fr.Quaternion(), fr.Origin
	return g, nil
}

// faceFrameAt is the frame of face in the solid of group solid.
func (sk *Sketch) faceFrameAt(solid *Group, face srf.FaceID) (topo.FaceFrame, error) {
	switch {
	case solid != nil && solid.IsForcedToMesh(sk):
		return topo.FaceFrame{}, errors.New("the solid is a triangle mesh, which has no faces to sketch on")
	case solid == nil || len(solid.RunningShell.Surface) == 0:
		return topo.FaceFrame{}, errors.New("there is no solid with faces to sketch on")
	}
	t, err := solid.Topology()
	if err != nil {
		return topo.FaceFrame{}, fmt.Errorf("finding the solid's faces: %w", err)
	}
	return t.FaceFrame(face)
}

// facePlane is the orientation and origin of a sketch on a face: the
// face's, in the solid of the groups before it, when it is there, and
// otherwise where it was last found, with the reason kept for the
// group's problem.
func (g *Group) facePlane(sk *Sketch) (geom.Quaternion, geom.Vector) {
	g.faceProblem = ""
	if len(g.SolidFaces) == 0 {
		g.faceProblem = "the sketch names no face to lie in"
		return g.Predef.Q, g.Predef.At
	}
	var solid *Group
	if prev := g.PreviousGroup(sk); prev != nil {
		solid = sk.solidAt(prev.H)
	}
	fr, err := sk.faceFrameAt(solid, g.SolidFaces[0])
	if err != nil {
		g.faceProblem = err.Error() + "; the workplane stays where it was"
		return g.Predef.Q, g.Predef.At
	}
	g.Predef.Q, g.Predef.At = fr.Quaternion(), fr.Origin
	return g.Predef.Q, g.Predef.At
}

// FaceProblem is why a sketch on a face did not find its face at the
// last regeneration, or "": the workplane then stays where it was.
func (g *Group) FaceProblem() string { return g.faceProblem }

// facePlaneMoved reports whether g is a sketch on a face that no
// longer lies where q and at put it.
func (g *Group) facePlaneMoved(q geom.Quaternion, at geom.Vector) bool {
	return g.Type == GroupDrawingWorkplane && g.Subtype == GroupWorkplaneByFace &&
		(g.Predef.Q != q || g.Predef.At != at)
}
