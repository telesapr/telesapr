package sketch_test

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/srf"
)

// faceEdgeCount is how many edges Sketch.FaceEdges gives for one face.
func faceEdgeCount(t *testing.T, d *api.Doc, f api.SolidFace) int {
	t.Helper()
	names, err := d.Host().Sketch().FaceEdges([]srf.FaceID{srf.FaceID(f.ID())})
	if err != nil {
		t.Fatal(err)
	}
	return len(names)
}

// The edges of a block's top are its four, of a cylinder's side its two
// rims (the seam between the patches is none), and of a block with a
// bore the outer four and the bore's rim; each is also an edge of the
// face the API's Edges lists.
func TestFaceEdges(t *testing.T) {
	t.Parallel()
	d := api.New()
	ex := block(d, 0, 0, 20, 10, 5)
	s := ex.Solid()
	top := s.FaceAt(api.V(10, 5, 5))
	if n := faceEdgeCount(t, d, top); n != 4 {
		t.Errorf("top of a block: %d edges, want 4", n)
	}
	if n := faceEdgeCount(t, d, s.FaceAt(api.V(0, 5, 2))); n != 4 {
		t.Errorf("side of a block: %d edges, want 4", n)
	}
	if got, want := len(top.Edges()), 4; got != want {
		t.Errorf("SolidFace.Edges: %d, want %d", got, want)
	}

	// Two faces: the edge they share counts once.
	names, err := d.Host().Sketch().FaceEdges([]srf.FaceID{srf.FaceID(top.ID()), srf.FaceID(s.FaceAt(api.V(0, 5, 2)).ID())})
	if err != nil || len(names) != 7 {
		t.Errorf("top and a side: %d edges, error %v, want 7", len(names), err)
	}
}

// A cylinder's side is one face of patches joined by seams, which are
// no edges: it has its two rims.
func TestFaceEdgesCylinder(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Sketch.Circle(api.V(0, 0, 0), 5)
	s := d.Features.Extrude(10, "").Solid()
	if n := faceEdgeCount(t, d, s.FaceAt(api.V(5, 0, 5))); n != 2 {
		t.Errorf("side of a cylinder: %d edges, want 2", n)
	}
	if n := faceEdgeCount(t, d, s.FaceAt(api.V(1, 1, 10))); n != 1 {
		t.Errorf("top of a cylinder: %d edges, want 1", n)
	}
}

// Without a solid, or for a face it lacks, the edges are an error.
func TestFaceEdgesErrors(t *testing.T) {
	t.Parallel()
	d := api.New()
	if _, err := d.Host().Sketch().FaceEdges([]srf.FaceID{1}); err == nil {
		t.Error("no solid: no error")
	}
	block(d, 0, 0, 20, 10, 5)
	if _, err := d.Host().Sketch().FaceEdges([]srf.FaceID{1}); err == nil {
		t.Error("a face the solid doesn't have: no error")
	}
}
