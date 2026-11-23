package topo

import (
	"errors"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/internal/par"
	"github.com/telesapr/telesapr/pkg/srf"
)

// faceToAdd is a face for FaceAdder.addAll: a surface and its loops.
type faceToAdd struct {
	s     srf.SSurface
	loops [][]srf.SBezier
}

// errNoLoop refuses a face given no loop.
var errNoLoop = errors.New("topo: a face needs a loop")

// addAll puts the faces in, in order, as Add one after another would,
// and gives their handles; on a face with no loop, the error and the
// index of that face, those before it put in. The faces are prepared in
// order (prepare: the open edges each takes or leaves are the next's to
// take), each turned over or not at once (orient, which projects only
// on the face's own copy of its surface, the most of Add's time on a
// fillet's patches), and put in in order. The handles are those Add
// would give: each face's the one after the last's, its fresh curves'
// on from those of the faces before.
func (fa *FaceAdder) addAll(faces []faceToAdd) ([]core.HSSurface, int, error) {
	n := len(faces)
	for i, f := range faces {
		if len(f.loops) == 0 || len(f.loops[0]) == 0 {
			n = i
			break
		}
	}
	pfs := make([]*addFace, n)
	h, nextH := newSurfaceHandle(fa.sh), newCurveHandle(fa.sh)
	for i := range n {
		pfs[i] = fa.prepare(faces[i].s, faces[i].loops, h, nextH)
		h.V++
		nextH = pfs[i].nextH
	}
	par.For(n, func(i int) { pfs[i].orient() })
	hs := make([]core.HSSurface, n)
	for i, pf := range pfs {
		fa.commit(pf)
		hs[i] = pf.s.H
	}
	if n < len(faces) {
		return hs, n, errNoLoop
	}
	return hs, -1, nil
}
