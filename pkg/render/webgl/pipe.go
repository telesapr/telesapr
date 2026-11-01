package webgl

import (
	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/res"
)

// lightMode says how the fragment shader colours a run.
type lightMode int

const (
	// modeFlat draws the vertex colour as it is.
	modeFlat lightMode = iota
	// modeLit is GL's fixed-function lighting: the model ambient and
	// each light's diffuse term, no highlight.
	modeLit
	// modeShaded is render.Shade, with the run's material.
	modeShaded
)

// depthTest is the depth comparison of a layer.
type depthTest int

const (
	depthLEqual depthTest = iota
	depthGreater
)

// pipe is the state a run of triangles is drawn in. Changing any of it
// ends the run; the device applies only what differs from the run
// before.
type pipe struct {
	test       depthTest
	depthWrite bool
	colorWrite bool
	// near and far are glDepthRange.
	near, far float32

	pattern render.FillPattern
	// tex is the texture of a textured quad, nil for none.
	tex *res.Pixmap

	mode lightMode
	mat  render.Material
	// twoSide lights a back face with the normal reversed, as GL's
	// two-sided lighting does; useBack says the back faces take backCol and not the vertex
	// colour.
	twoSide, useBack bool
	backCol          [4]float32
}

// withLayer sets what layer and zIndex say about the depth buffer
// (ssglDepthRange): which fragments pass, whether depth and colour are
// written, and the slice of the depth range the geometry takes. A
// z-index steps the range by a little less than a 16-bit buffer's
// resolution, so geometry of a higher index wins a tie.
func (p pipe) withLayer(layer render.Layer, zIndex int) pipe {
	switch layer {
	case render.LayerNormal, render.LayerFront, render.LayerBack:
		p.test, p.depthWrite, p.colorWrite = depthLEqual, true, true
	case render.LayerDepthOnly:
		p.test, p.depthWrite, p.colorWrite = depthLEqual, true, false
	case render.LayerOccluded:
		p.test, p.depthWrite, p.colorWrite = depthGreater, false, true
	}
	switch layer {
	case render.LayerFront:
		p.near, p.far = 0, 0
	case render.LayerBack:
		p.near, p.far = 1, 1
	default:
		offset := 1.0 / (65535 * 0.8) * float64(zIndex)
		p.near = float32(clamp01(0.1 - offset))
		p.far = float32(clamp01(1.0 - offset))
	}
	return p
}

func clamp01(x float64) float64 { return max(0, min(1, x)) }
