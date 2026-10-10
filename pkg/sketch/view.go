package sketch

import "github.com/telesapr/telesapr/pkg/geom"

// View is the map from model space to screen pixels: the window's
// offset, projection vectors and scale; Tangent is the perspective
// factor, zero for an orthographic view.
type View struct {
	Offset    geom.Vector
	ProjRight geom.Vector
	ProjUp    geom.Vector
	Scale     float64
	Tangent   float64
}

// DefaultView is the view GraphicsWindow::Init sets up: identity
// projection at 5 pixels per mm, orthographic.
func DefaultView() View {
	return View{
		ProjRight: geom.Vector{X: 1},
		ProjUp:    geom.Vector{Y: 1},
		Scale:     5,
	}
}

// view is the sketch's view, or the default one when the sketch has
// none (a sketch without a window).
func (sk *Sketch) view() *View {
	if sk.View != nil {
		return sk.View
	}
	v := DefaultView()
	return &v
}

// ProjectPoint4 projects p halfway into screen space: no scale and no
// perspective divide; w is returned.
func (v *View) ProjectPoint4(p geom.Vector) (r geom.Vector, w float64) {
	p = p.Plus(v.Offset)
	r.X = p.Dot(v.ProjRight)
	r.Y = p.Dot(v.ProjUp)
	r.Z = p.Dot(v.ProjUp.Cross(v.ProjRight))
	w = 1 + r.Z*v.Tangent*v.Scale
	return r, w
}

// ProjectPoint3 projects p to screen pixels, z included.
func (v *View) ProjectPoint3(p geom.Vector) geom.Vector {
	r, w := v.ProjectPoint4(p)
	return r.ScaledBy(v.Scale / w)
}

// ProjectPoint projects p to screen pixels.
func (v *View) ProjectPoint(p geom.Vector) geom.Point2d {
	p3 := v.ProjectPoint3(p)
	return geom.Point2d{U: p3.X, V: p3.Y}
}

// UnProjectPoint is the point in the plane through the offset, parallel
// to the screen, that projects onto p.
func (v *View) UnProjectPoint(p geom.Point2d) geom.Vector {
	orig := v.Offset.ScaledBy(-1)
	return orig.Plus(v.ProjRight.ScaledBy(p.U / v.Scale)).Plus(
		v.ProjUp.ScaledBy(p.V / v.Scale))
}

// UnProjectPoint3 inverts ProjectPoint3.
func (v *View) UnProjectPoint3(p geom.Vector) geom.Vector {
	p.Z /= v.Scale - p.Z*v.Tangent*v.Scale
	w := 1 + p.Z*v.Tangent*v.Scale
	p.X *= w / v.Scale
	p.Y *= w / v.Scale

	orig := v.Offset.ScaledBy(-1)
	return orig.Plus(v.ProjRight.ScaledBy(p.X)).Plus(
		v.ProjUp.ScaledBy(p.Y).Plus(
			v.ProjRight.Cross(v.ProjUp).ScaledBy(p.Z)))
}

// NormalizeProjectionVectors makes ProjRight and ProjUp orthonormal.
func (v *View) NormalizeProjectionVectors() {
	if v.ProjRight.Magnitude() < geom.LengthEps {
		v.ProjRight = geom.Vector{X: 1}
	}
	norm := v.ProjRight.Cross(v.ProjUp)
	if norm.Magnitude() < geom.LengthEps {
		norm = v.ProjRight.NormalDir(0)
	}
	v.ProjUp = norm.Cross(v.ProjRight)
	v.ProjUp = v.ProjUp.WithMagnitude(1)
	v.ProjRight = v.ProjRight.WithMagnitude(1)
}

// VectorFromProjs is the model-space vector with the given right, up
// and forward screen components.
func (v *View) VectorFromProjs(rightUpForward geom.Vector) geom.Vector {
	n := v.ProjRight.Cross(v.ProjUp)
	r := v.ProjRight.ScaledBy(rightUpForward.X)
	r = r.Plus(v.ProjUp.ScaledBy(rightUpForward.Y))
	return r.Plus(n.ScaledBy(rightUpForward.Z))
}

// ProjectPoint projects p to the screen in the sketch's view.
func (sk *Sketch) ProjectPoint(p geom.Vector) geom.Point2d {
	return sk.view().ProjectPoint(p)
}

// Visibility holds the view's show flags the engine reads:
// Entity::IsVisible reads the first four, Group::GenerateDisplayItems
// the last two.
type Visibility struct {
	Points, Normals, Construction, Workplanes bool
	Edges, Outlines                           bool
	// References and Comments show reference dimensions and comments,
	// when constraints are shown at all.
	References, Comments bool
}

// DefaultVisibility is what GraphicsWindow::Init sets: points, normals,
// construction, edges, reference dimensions and comments shown,
// workplanes and outlines not.
func DefaultVisibility() Visibility {
	return Visibility{Points: true, Normals: true, Construction: true, Edges: true, References: true, Comments: true}
}

func (sk *Sketch) visibility() Visibility {
	if sk.Show != nil {
		return *sk.Show
	}
	return DefaultVisibility()
}

// DistanceLabelOffset is where the label of a new distance between
// points at a and b goes, as Constrain > Distance puts it: 50 pixels
// from the middle, across the line between them in the view.
func (sk *Sketch) DistanceLabelOffset(a, b geom.Vector) geom.Vector {
	v := sk.view()
	n := v.ProjRight.Cross(v.ProjUp)
	return n.Cross(a.Minus(b)).WithMagnitude(50 / v.Scale)
}
