// Package render is SolveSpace's backend-agnostic rendering interface:
// the camera, lighting, strokes and fills, the Canvas interfaces, and
// the canvases every backend shares (UiCanvas, ObjectPicker). It ports
// render/render.h and render/render.cpp.
package render

import (
	"math"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/srf"
)

// Camera maps 3d sketch coordinates to 2d screen coordinates, using an
// axonometric projection; Tangent is non-zero for perspective (C++
// Camera).
type Camera struct {
	Width, Height float64
	PixelRatio    float64
	GridFit       bool
	Offset        geom.Vector
	ProjRight     geom.Vector
	ProjUp        geom.Vector
	Scale         float64
	Tangent       float64
}

// IsPerspective reports whether the camera has perspective.
func (c *Camera) IsPerspective() bool { return c.Tangent != 0.0 }

// ProjectPoint is p on the screen, in pixels from the centre, y up.
func (c *Camera) ProjectPoint(p geom.Vector) geom.Point2d {
	p3 := c.ProjectPoint3(p)
	return geom.Point2d{U: p3.X, V: p3.Y}
}

// ProjectPoint3 is p in screen pixels, depth included.
func (c *Camera) ProjectPoint3(p geom.Vector) geom.Vector {
	r, w := c.ProjectPoint4(p)
	return r.ScaledBy(c.Scale / w)
}

// ProjectPoint4 is p rotated into screen axes, before scale and the
// perspective divide by w.
func (c *Camera) ProjectPoint4(p geom.Vector) (r geom.Vector, w float64) {
	p = p.Plus(c.Offset)
	r.X = p.Dot(c.ProjRight)
	r.Y = p.Dot(c.ProjUp)
	r.Z = p.Dot(c.ProjUp.Cross(c.ProjRight))
	w = 1 + r.Z*c.Tangent*c.Scale
	return r, w
}

// UnProjectPoint is the point in the plane through the offset,
// parallel to the screen, that projects onto p. Since that point has
// the same component normal to the screen as the offset, it has z = 0
// after the rotation, thus w = 1, so perspective doesn't matter.
func (c *Camera) UnProjectPoint(p geom.Point2d) geom.Vector {
	orig := c.Offset.ScaledBy(-1)
	return orig.Plus(c.ProjRight.ScaledBy(p.U / c.Scale)).Plus(
		c.ProjUp.ScaledBy(p.V / c.Scale))
}

// UnProjectPoint3 inverts ProjectPoint3.
func (c *Camera) UnProjectPoint3(p geom.Vector) geom.Vector {
	p.Z = p.Z / (c.Scale - p.Z*c.Tangent*c.Scale)
	w := 1 + p.Z*c.Tangent*c.Scale
	p.X *= w / c.Scale
	p.Y *= w / c.Scale

	orig := c.Offset.ScaledBy(-1)
	return orig.Plus(c.ProjRight.ScaledBy(p.X)).Plus(
		c.ProjUp.ScaledBy(p.Y).Plus(
			c.ProjUp.Cross(c.ProjRight).ScaledBy(p.Z)))
}

// VectorFromProjs is the vector with the given right, up and forward
// components.
func (c *Camera) VectorFromProjs(rightUpForward geom.Vector) geom.Vector {
	n := c.ProjRight.Cross(c.ProjUp)
	r := c.ProjRight.ScaledBy(rightUpForward.X)
	r = r.Plus(c.ProjUp.ScaledBy(rightUpForward.Y))
	return r.Plus(n.ScaledBy(rightUpForward.Z))
}

// AlignToPixelGrid moves v to the centre of its pixel when GridFit is
// set.
func (c *Camera) AlignToPixelGrid(v geom.Vector) geom.Vector {
	if !c.GridFit {
		return v
	}
	v = c.ProjectPoint3(v)
	v.X = math.Floor(v.X) + 0.5
	v.Y = math.Floor(v.Y) + 0.5
	return c.UnProjectPoint3(v)
}

// IsGridFit and ProjVectors let the camera grid-fit vector text
// (res.TraceCamera).
func (c *Camera) IsGridFit() bool { return c.GridFit }

// ProjVectors returns ProjRight and ProjUp.
func (c *Camera) ProjVectors() (right, up geom.Vector) { return c.ProjRight, c.ProjUp }

// ProjectBezier is b in screen coordinates, perspective included.
func (c *Camera) ProjectBezier(b srf.SBezier) srf.SBezier {
	q := geom.QuaternionFromBasis(c.ProjRight, c.ProjUp)
	q = q.Inverse()
	// we want Q*(p - o) = Q*p - Q*o
	b.TransformedBy(q.Rotate(c.Offset).ScaledBy(c.Scale), q, c.Scale)
	for i := 0; i <= b.Degree; i++ {
		ct := geom.Vector4FromWeighted(b.Weight[i], b.Ctrl[i])
		// The desired curve, before perspective, is (x/w, y/w, z/w),
		// and after it ((x/w)/(1 - (z/w)*tangent), ...) = (x/(w -
		// z*tangent), ...), so w' = w - z*tangent.
		ct.W = ct.W - ct.Z*c.Tangent
		b.Ctrl[i] = ct.PerspectiveProject()
		b.Weight[i] = ct.W
	}
	return b
}

// LoadIdentity resets the camera to no offset, identity rotation,
// scale 1 and no perspective.
func (c *Camera) LoadIdentity() {
	c.Offset = geom.Vector{}
	c.ProjRight = geom.Vector{X: 1}
	c.ProjUp = geom.Vector{Y: 1}
	c.Scale = 1.0
	c.Tangent = 0.0
}

// NormalizeProjectionVectors makes ProjRight and ProjUp orthonormal.
func (c *Camera) NormalizeProjectionVectors() {
	if c.ProjRight.Magnitude() < geom.LengthEps {
		c.ProjRight = geom.Vector{X: 1}
	}
	norm := c.ProjRight.Cross(c.ProjUp)
	// If projRight and projUp somehow ended up parallel, then pick an
	// arbitrary projUp normal to projRight.
	if norm.Magnitude() < geom.LengthEps {
		norm = c.ProjRight.NormalDir(0)
	}
	c.ProjUp = norm.Cross(c.ProjRight)
	c.ProjUp = c.ProjUp.WithMagnitude(1)
	c.ProjRight = c.ProjRight.WithMagnitude(1)
}

// Lighting describes the scene's lights, in right/up/forward screen
// coordinates (C++ Lighting).
type Lighting struct {
	BackgroundColor  core.RgbaColor
	AmbientIntensity float64
	LightIntensity   [2]float64
	LightDirection   [2]geom.Vector
}
