package sketch

import (
	"errors"
	"fmt"
	"slices"

	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// FaceEdges are the edges of the solid at the active group that bound
// any of faces, where the face meets another: what the menu's Select
// Face Edges and a script's SolidFace.SelectEdges select. The seams
// inside a face, and the edges between two pieces of one face, are none
// of them. They come in the order their names number them, each once.
func (sk *Sketch) FaceEdges(faces []srf.FaceID) ([]topo.Name, error) {
	solid := sk.solidAt(sk.ActiveGroup())
	switch {
	case solid != nil && solid.IsForcedToMesh(sk):
		return nil, errors.New("the solid is a triangle mesh, which has no faces and edges to select")
	case solid == nil || len(solid.RunningShell.Surface) == 0:
		return nil, errors.New("there is no solid with faces to select the edges of")
	}
	tp, err := solid.Topology()
	if err != nil {
		return nil, fmt.Errorf("finding the solid's edges: %w", err)
	}
	var out []topo.Name
	for _, id := range faces {
		sheets := tp.SheetsWithID(id)
		if len(sheets) == 0 {
			return nil, fmt.Errorf("%s is not a face of the solid", sk.FaceName(id))
		}
		for _, sh := range sheets {
			for _, e := range tp.SheetEdges(sh) {
				n, err := tp.NameOf(e)
				if err != nil || n.A == n.B || slices.Contains(out, n) {
					continue
				}
				out = append(out, n)
			}
		}
	}
	slices.SortFunc(out, func(a, b topo.Name) int {
		switch {
		case a.Less(b):
			return -1
		case b.Less(a):
			return 1
		}
		return 0
	})
	return out, nil
}
