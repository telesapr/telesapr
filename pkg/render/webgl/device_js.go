//go:build js && wasm

package webgl

import (
	"errors"
	"fmt"
	"syscall/js"
	"unsafe"

	"github.com/telesapr/telesapr/pkg/res"
)

// The WebGL enums the device uses.
const (
	glDepthBufferBit = 0x00000100
	glColorBufferBit = 0x00004000
	glTriangles      = 0x0004
	glArrayBuffer    = 0x8892
	glDynamicDraw    = 0x88E8
	glFloat          = 0x1406
	glUnsignedByte   = 0x1401
	glDepthTest      = 0x0B71
	glBlend          = 0x0BE2
	glLEqual         = 0x0203
	glGreater        = 0x0204
	glSrcAlpha       = 0x0302
	glOneMinusSrcA   = 0x0303
	glOne            = 1
	glFragmentShader = 0x8B30
	glVertexShader   = 0x8B31
	glCompileStatus  = 0x8B81
	glLinkStatus     = 0x8B82
	glTexture2D      = 0x0DE1
	glTexture0       = 0x84C0
	glTexMag         = 0x2800
	glTexMin         = 0x2801
	glTexWrapS       = 0x2802
	glTexWrapT       = 0x2803
	glNearest        = 0x2600
	glClampToEdge    = 0x812F
	glRGBA           = 0x1908
	glRGB            = 0x1907
	glRed            = 0x1903
	glR8             = 0x8229
	glRGBA8          = 0x8058
	glRGB8           = 0x8051
	glVendor         = 0x1F00
	glRenderer       = 0x1F01
	glVersion        = 0x1F02
	glPackAlignment  = 0x0D05
	glUnpackAlign    = 0x0CF5
)

// ErrUnavailable is returned by New where the browser has no WebGL 2.
var ErrUnavailable = errors.New("webgl: WebGL 2 is not available")

// New returns a renderer drawing into canvas, an HTMLCanvasElement
// that has no 2D context, at width by height device pixels, which it
// sets as the canvas's width and height attributes. It fails where
// the browser has no WebGL 2.
//
// The canvas is opaque and multisampled, and keeps its pixels after
// they are shown, so that ReadFrame and canvas.toDataURL see the frame.
func New(canvas js.Value, width, height int) (*Renderer, error) {
	d, err := newJSDevice(canvas, max(width, 1), max(height, 1))
	if err != nil {
		return nil, err
	}
	r := newRenderer(d)
	r.width, r.height = d.w, d.h
	return r, nil
}

// uniform names of the program, in the order of jsDevice.loc.
var uniformNames = []string{
	"uMVP", "uMode", "uPattern", "uTexMode", "uTex", "uClipOn", "uClip",
	"uLightDir0", "uLightDir1", "uLightI0", "uLightI1", "uAmbient", "uView", "uUp",
	"uSpec", "uShin", "uMetal", "uTwoSide", "uUseBack", "uBackCol",
}

// jsDevice is the device on a WebGL 2 context.
type jsDevice struct {
	canvas, gl js.Value
	w, h       int

	prog, vao, vbo js.Value
	loc            map[string]js.Value

	// scratch is a Uint8Array that vertices are copied through, grown
	// as the runs are.
	scratch js.Value
	scrLen  int

	textures map[*res.Pixmap]js.Value

	// applied is the state the context is in: the pipe and the scene
	// of the last run, valid when have is set.
	have    bool
	applied pipe
	scene   scene
	haveSc  bool

	lose js.Value
}

func newJSDevice(canvas js.Value, w, h int) (*jsDevice, error) {
	canvas.Set("width", w)
	canvas.Set("height", h)
	attrs := js.ValueOf(map[string]any{
		"antialias":             true,
		"alpha":                 false,
		"depth":                 true,
		"stencil":               false,
		"preserveDrawingBuffer": true,
		"powerPreference":       "high-performance",
	})
	gl := canvas.Call("getContext", "webgl2", attrs)
	if gl.IsNull() || gl.IsUndefined() {
		return nil, ErrUnavailable
	}
	d := &jsDevice{canvas: canvas, gl: gl, w: w, h: h, textures: map[*res.Pixmap]js.Value{}}
	if err := d.setup(); err != nil {
		return nil, err
	}
	d.lose = gl.Call("getExtension", "WEBGL_lose_context")
	return d, nil
}

func (d *jsDevice) compile(kind int, src string) (js.Value, error) {
	gl := d.gl
	sh := gl.Call("createShader", kind)
	gl.Call("shaderSource", sh, src)
	gl.Call("compileShader", sh)
	if !gl.Call("getShaderParameter", sh, glCompileStatus).Bool() {
		return sh, fmt.Errorf("webgl: shader: %s", gl.Call("getShaderInfoLog", sh).String())
	}
	return sh, nil
}

// setup makes the program, the vertex array and the fixed state.
func (d *jsDevice) setup() error {
	gl := d.gl
	vs, err := d.compile(glVertexShader, vertexSource)
	if err != nil {
		return err
	}
	fs, err := d.compile(glFragmentShader, fragmentSource)
	if err != nil {
		return err
	}
	d.prog = gl.Call("createProgram")
	gl.Call("attachShader", d.prog, vs)
	gl.Call("attachShader", d.prog, fs)
	gl.Call("linkProgram", d.prog)
	if !gl.Call("getProgramParameter", d.prog, glLinkStatus).Bool() {
		return fmt.Errorf("webgl: the program doesn't link: %s", gl.Call("getProgramInfoLog", d.prog).String())
	}
	gl.Call("deleteShader", vs)
	gl.Call("deleteShader", fs)
	gl.Call("useProgram", d.prog)
	d.loc = make(map[string]js.Value, len(uniformNames))
	for _, name := range uniformNames {
		d.loc[name] = gl.Call("getUniformLocation", d.prog, name)
	}
	gl.Call("uniform1i", d.loc["uTex"], 0)

	d.vao = gl.Call("createVertexArray")
	gl.Call("bindVertexArray", d.vao)
	d.vbo = gl.Call("createBuffer")
	gl.Call("bindBuffer", glArrayBuffer, d.vbo)
	stride := vertexFloats * 4
	for i, a := range []struct{ size, offset int }{{3, 0}, {3, 12}, {4, 24}, {2, 40}} {
		gl.Call("enableVertexAttribArray", i)
		gl.Call("vertexAttribPointer", i, a.size, glFloat, false, stride, a.offset)
	}

	gl.Call("enable", glDepthTest)
	gl.Call("enable", glBlend)
	gl.Call("blendFunc", glSrcAlpha, glOneMinusSrcA)
	gl.Call("pixelStorei", glPackAlignment, 1)
	gl.Call("viewport", 0, 0, d.w, d.h)
	return nil
}

func (d *jsDevice) resize(w, h int) {
	if w == d.w && h == d.h {
		return
	}
	d.w, d.h = w, h
	d.canvas.Set("width", w)
	d.canvas.Set("height", h)
	d.gl.Call("viewport", 0, 0, w, h)
}

func (d *jsDevice) clear(color [4]float32, colorToo bool) {
	gl := d.gl
	gl.Call("depthMask", true)
	gl.Call("depthRange", 0, 1)
	gl.Call("clearDepth", 1)
	bits := glDepthBufferBit
	if colorToo {
		gl.Call("colorMask", true, true, true, true)
		gl.Call("clearColor", color[0], color[1], color[2], color[3])
		bits |= glColorBufferBit
	}
	gl.Call("clear", bits)
	d.have = false
}

// upload copies b to the vertex buffer through the scratch array.
func (d *jsDevice) upload(b []byte) {
	if len(b) > d.scrLen {
		d.scrLen = max(len(b), 2*d.scrLen)
		d.scratch = js.Global().Get("Uint8Array").New(d.scrLen)
	}
	js.CopyBytesToJS(d.scratch, b)
	d.gl.Call("bufferData", glArrayBuffer, d.scratch, glDynamicDraw, 0, len(b))
}

func floats(v []float32) []byte {
	return unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(v))), len(v)*4)
}

func (d *jsDevice) draw(c *drawCall) {
	d.applyScene(&c.scene)
	if !d.applyPipe(&c.pipe) {
		return
	}
	d.upload(floats(c.verts))
	d.gl.Call("drawArrays", glTriangles, 0, len(c.verts)/vertexFloats)
}

func (d *jsDevice) applyScene(s *scene) {
	if d.haveSc && d.scene == *s {
		return
	}
	gl, loc := d.gl, d.loc
	old := d.scene
	all := !d.haveSc
	d.scene, d.haveSc = *s, true
	if all || old.mvp != s.mvp {
		m := make([]any, 16)
		for i, v := range s.mvp {
			m[i] = float64(v)
		}
		gl.Call("uniformMatrix4fv", loc["uMVP"], false, js.ValueOf(m))
	}
	if all || old.lightDir[0] != s.lightDir[0] {
		v := s.lightDir[0]
		gl.Call("uniform3f", loc["uLightDir0"], v[0], v[1], v[2])
	}
	if all || old.lightDir[1] != s.lightDir[1] {
		v := s.lightDir[1]
		gl.Call("uniform3f", loc["uLightDir1"], v[0], v[1], v[2])
	}
	if all || old.lightI != s.lightI {
		gl.Call("uniform1f", loc["uLightI0"], s.lightI[0])
		gl.Call("uniform1f", loc["uLightI1"], s.lightI[1])
	}
	if all || old.ambient != s.ambient {
		gl.Call("uniform1f", loc["uAmbient"], s.ambient)
	}
	if all || old.view != s.view {
		gl.Call("uniform3f", loc["uView"], s.view[0], s.view[1], s.view[2])
	}
	if all || old.up != s.up {
		gl.Call("uniform3f", loc["uUp"], s.up[0], s.up[1], s.up[2])
	}
	if all || old.clipOn != s.clipOn {
		gl.Call("uniform1i", loc["uClipOn"], s.clipOn)
	}
	if all || old.clip != s.clip {
		gl.Call("uniform4f", loc["uClip"], s.clip[0], s.clip[1], s.clip[2], s.clip[3])
	}
}

// applyPipe sets the context to p's state, changing only what differs
// from the state before, and reports false when p can't be drawn.
func (d *jsDevice) applyPipe(p *pipe) bool {
	gl, loc := d.gl, d.loc
	tex := js.Undefined()
	texMode := 0
	if p.tex != nil {
		var ok bool
		if tex, texMode, ok = d.texture(p.tex); !ok {
			return false
		}
	}
	old := d.applied
	all := !d.have
	d.applied, d.have = *p, true
	if all || old.test != p.test {
		fn := glLEqual
		if p.test == depthGreater {
			fn = glGreater
		}
		gl.Call("depthFunc", fn)
	}
	if all || old.depthWrite != p.depthWrite {
		gl.Call("depthMask", p.depthWrite)
	}
	if all || old.colorWrite != p.colorWrite {
		gl.Call("colorMask", p.colorWrite, p.colorWrite, p.colorWrite, p.colorWrite)
	}
	if all || old.near != p.near || old.far != p.far {
		gl.Call("depthRange", p.near, p.far)
	}
	if all || old.pattern != p.pattern {
		gl.Call("uniform1i", loc["uPattern"], int(p.pattern))
	}
	if p.tex != nil {
		gl.Call("activeTexture", glTexture0)
		gl.Call("bindTexture", glTexture2D, tex)
	}
	if all || old.tex != p.tex {
		gl.Call("uniform1i", loc["uTexMode"], texMode)
	}
	if all || old.mode != p.mode {
		gl.Call("uniform1i", loc["uMode"], int(p.mode))
	}
	if p.mode == modeShaded && (all || old.mat != p.mat) {
		gl.Call("uniform1f", loc["uSpec"], p.mat.Specular)
		gl.Call("uniform1f", loc["uShin"], p.mat.Shininess)
		gl.Call("uniform1f", loc["uMetal"], p.mat.Metallic)
	}
	if all || old.twoSide != p.twoSide {
		gl.Call("uniform1i", loc["uTwoSide"], p.twoSide)
	}
	if all || old.useBack != p.useBack {
		gl.Call("uniform1i", loc["uUseBack"], p.useBack)
	}
	if p.useBack && (all || old.backCol != p.backCol) {
		gl.Call("uniform4f", loc["uBackCol"], p.backCol[0], p.backCol[1], p.backCol[2], p.backCol[3])
	}
	return true
}

// texture is the texture made of pm, uploaded the first time, and the
// mode the shader reads it in.
func (d *jsDevice) texture(pm *res.Pixmap) (tex js.Value, mode int, ok bool) {
	var internal, format int
	switch pm.Format {
	case res.FormatRGBA:
		internal, format, mode = glRGBA8, glRGBA, 1
	case res.FormatRGB:
		internal, format, mode = glRGB8, glRGB, 2
	case res.FormatA:
		internal, format, mode = glR8, glRed, 3
	default:
		// BGR formats are the platform's own, never drawn.
		return tex, 0, false
	}
	if t, have := d.textures[pm]; have {
		return t, mode, true
	}
	gl := d.gl
	tex = gl.Call("createTexture")
	gl.Call("bindTexture", glTexture2D, tex)
	gl.Call("texParameteri", glTexture2D, glTexMag, glNearest)
	gl.Call("texParameteri", glTexture2D, glTexMin, glNearest)
	gl.Call("texParameteri", glTexture2D, glTexWrapS, glClampToEdge)
	gl.Call("texParameteri", glTexture2D, glTexWrapT, glClampToEdge)
	// Rows are padded to four bytes, which is the default alignment.
	gl.Call("pixelStorei", glUnpackAlign, 4)
	data := js.Global().Get("Uint8Array").New(len(pm.Data))
	js.CopyBytesToJS(data, pm.Data)
	gl.Call("texImage2D", glTexture2D, 0, internal, pm.Width, pm.Height, 0, format, glUnsignedByte, data)
	d.textures[pm] = tex
	return tex, mode, true
}

func (d *jsDevice) invalidate(pm *res.Pixmap) {
	if t, ok := d.textures[pm]; ok {
		d.gl.Call("deleteTexture", t)
		delete(d.textures, pm)
	}
}

func (d *jsDevice) read() (pix []byte, width, height int) {
	u8 := js.Global().Get("Uint8Array").New(4 * d.w * d.h)
	d.gl.Call("readPixels", 0, 0, d.w, d.h, glRGBA, glUnsignedByte, u8)
	pix = make([]byte, 4*d.w*d.h)
	js.CopyBytesToGo(pix, u8)
	return pix, d.w, d.h
}

func (d *jsDevice) ident() (vendor, renderer, version string) {
	gl := d.gl
	vendor, renderer = gl.Call("getParameter", glVendor).String(), gl.Call("getParameter", glRenderer).String()
	// The unmasked names are the GPU's, where the others are the
	// browser's.
	if ext := gl.Call("getExtension", "WEBGL_debug_renderer_info"); ext.Truthy() {
		vendor = gl.Call("getParameter", ext.Get("UNMASKED_VENDOR_WEBGL")).String()
		renderer = gl.Call("getParameter", ext.Get("UNMASKED_RENDERER_WEBGL")).String()
	}
	return vendor, renderer, gl.Call("getParameter", glVersion).String()
}

func (d *jsDevice) flush()  { d.gl.Call("flush") }
func (d *jsDevice) finish() { d.gl.Call("finish") }

func (d *jsDevice) close() {
	gl := d.gl
	for pm, t := range d.textures {
		gl.Call("deleteTexture", t)
		delete(d.textures, pm)
	}
	gl.Call("deleteBuffer", d.vbo)
	gl.Call("deleteVertexArray", d.vao)
	gl.Call("deleteProgram", d.prog)
	if d.lose.Truthy() {
		d.lose.Call("loseContext")
	}
}
