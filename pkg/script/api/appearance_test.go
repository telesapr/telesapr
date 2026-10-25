package api

import "testing"

// A face of the solid may have an appearance of its own, which the
// window's context menu sets too, and takes back with "".
func TestFaceAppearance(t *testing.T) {
	t.Parallel()
	d := New()
	d.Sketch.Rect(V(0, 0, 0), V(20, 10, 0))
	box := d.Features.Extrude(10, "box")
	box.SetAppearance("matte")
	faces := box.Solid().Faces()
	if len(faces) == 0 {
		t.Fatal("the box has no face")
	}
	face := faces[0]
	if got := d.View.FaceAppearance(face); got != "matte" {
		t.Errorf("the face has the appearance %q, want its group's matte", got)
	}
	d.View.SetFaceAppearance(face, "glossy")
	if got := d.View.FaceAppearance(face); got != "glossy" {
		t.Errorf("the face has the appearance %q, want glossy", got)
	}
	d.View.SetFaceAppearance(face, "")
	if got := d.View.FaceAppearance(face); got != "matte" {
		t.Errorf("with its own taken away, the face has the appearance %q", got)
	}
	// The face is kept by its identity: the solid keeps its own
	// appearance through the file.
	if got := box.Appearance(); got != "matte" {
		t.Errorf("the group's appearance is %q", got)
	}
	func() {
		defer func() {
			if _, ok := recover().(error); !ok {
				t.Error("an appearance that is not one: no error")
			}
		}()
		d.View.SetFaceAppearance(face, "chrome")
	}()
}
