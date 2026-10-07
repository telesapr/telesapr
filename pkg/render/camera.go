// Package render is SolveSpace's backend-agnostic rendering interface:
// the camera, lighting, strokes and fills, the Canvas interfaces, and
// the canvases every backend shares (UiCanvas, ObjectPicker). It ports
// render/render.h and render/render.cpp.
package render

import (
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
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
	p.Z /= c.Scale - p.Z*c.Tangent*c.Scale
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
		ct.W -= ct.Z * c.Tangent
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
	BackgroundColor  core.RGBA
	AmbientIntensity float64
	LightIntensity   [2]float64
	LightDirection   [2]geom.Vector
	// Shaded lights meshes as roadmap 19.1 does, in place of GL's fixed
	// function: the ambient light from the sky above more than from the
	// ground below, each light's diffuse term, and a highlight by the
	// faces' Material (see Shade).
	Shaded bool
}

// Material is how a face takes light beyond its colour (roadmap 19.2):
// how strong its highlight is, how tight (a Blinn-Phong exponent), and
// how much it is a metal's, whose highlight takes the face's colour and
// whose diffuse light is less.
type Material struct {
	Specular, Shininess, Metallic float64
}

// Materials are the appearances by name; "" is plastic.
var Materials = map[string]Material{
	"matte":   {Specular: 0, Shininess: 1},
	"plastic": {Specular: 0.35, Shininess: 24},
	"glossy":  {Specular: 0.7, Shininess: 80},
	"metal":   {Specular: 0.8, Shininess: 48, Metallic: 1},
}

// MaterialNames are the appearances in the order the interface lists
// them.
var MaterialNames = []string{"matte", "plastic", "glossy", "metal"}

// MaterialCanvas is a canvas that can light a mesh's faces by their
// materials when the lighting is Shaded: of returns a face's, from the
// face of its triangles.
type MaterialCanvas interface {
	SetMaterials(of func(face uint32) Material)
}

// Shade is the shaded lighting of a point of normal n, both in model
// coordinates, of colour base (0 to 1): the ambient intensity weighted
// three quarters to all of it from below to above (up), each light's
// diffuse term, less for a metal, and a Blinn-Phong highlight toward
// view, white or, for a metal, the colour. The software renderer
// shades each vertex with it, the GL one each pixel.
func Shade(n geom.Vector, base [3]float64, ambient float64, lights []geom.Vector, intensity []float64,
	view, up geom.Vector, m Material) [3]float64 {
	n = n.WithMagnitude(1)
	var c [3]float64
	sky := ambient * (0.75 + 0.25*n.Dot(up))
	for k := range 3 {
		c[k] = sky * base[k]
	}
	for i, l := range lights {
		d := n.Dot(l)
		if d <= 0 {
			continue
		}
		h := l.Plus(view).WithMagnitude(1)
		s := intensity[i] * m.Specular * math.Pow(math.Max(n.Dot(h), 0), m.Shininess)
		for k := range 3 {
			spec := 1 + (base[k]-1)*m.Metallic
			c[k] += d*intensity[i]*base[k]*(1-0.5*m.Metallic) + s*spec
		}
	}
	for k := range 3 {
		c[k] = math.Max(0, math.Min(1, c[k]))
	}
	return c
}
