package sketch

import (
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// ClearExisting drops the whole sketch and the undo history, leaving
// it without even the references; the caller must fix that (C++
// SolveSpaceUI::ClearExisting).
func (sk *Sketch) ClearExisting() {
	sk.UndoClearStack(&sk.Redo)
	sk.UndoClearStack(&sk.Undo)

	for _, hg := range sk.GroupOrder {
		if g := sk.Group(hg); g != nil {
			g.Clear()
		}
	}
	sk.Clear()
	sk.Images = nil
}

// CreateDefaultDrawingGroup adds the empty group in the XY plane that
// a new file starts in (C++ SolveSpaceUI::CreateDefaultDrawingGroup).
func (sk *Sketch) CreateDefaultDrawingGroup() core.HGroup {
	hg := sk.AddGroup(Group{
		Visible: true,
		Name:    "sketch-in-plane",
		Type:    GroupDrawingWorkplane,
		Subtype: GroupWorkplaneByPointOrtho,
		Order:   1,
	})
	g := sk.Group(hg)
	g.Predef.Q = geom.Quaternion{W: 1}
	g.Predef.Origin = HRequestReferenceXY.Entity(1)
	g.ActiveWorkplane = hg.Entity(0)
	return hg
}

// NewFile makes the sketch a new, empty file: the references group
// with the three reference workplanes, and a drawing group (C++
// SolveSpaceUI::NewFile). The caller then generates it, as C++
// AfterNewFile does.
func (sk *Sketch) NewFile() {
	sk.ClearExisting()
	sk.Groups = append(sk.Groups, Group{
		H:       core.HGroup{V: core.HGroupReferences},
		Type:    GroupDrawing3D,
		Order:   0,
		Name:    "#references",
		Visible: true,
	})
	for _, hr := range []core.HRequest{HRequestReferenceXY, HRequestReferenceYZ, HRequestReferenceZX} {
		sk.Requests = append(sk.Requests, Request{
			H:         hr,
			Type:      RequestWorkplane,
			Group:     core.HGroup{V: core.HGroupReferences},
			Workplane: core.FreeIn3D,
		})
	}
	sk.CreateDefaultDrawingGroup()
}
