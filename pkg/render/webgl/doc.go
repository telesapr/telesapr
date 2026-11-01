// Package webgl is a WebGL 2 renderer: a render.ViewportCanvas drawing
// into an HTML canvas from a Go program built for js/wasm, with the
// semantics of the OpenGL renderer (package gl) and the software one
// (package soft): depth ranges per layer and z-index, checkered fills,
// lines and points as screen-space quads, textured quads, alpha
// blending, the section view's clip plane, and lit meshes with either
// GL's fixed-function lighting or render.Shade, each pixel on the GPU.
//
// Everything that decides what is drawn (the vertex stream, the state
// each run of it is drawn in, the projection) is plain Go that builds
// on every platform and is tested natively against a recording device;
// only the device that turns runs into WebGL calls through syscall/js
// is built for js/wasm, and New is there too.
//
// The renderer draws straight into the canvas, so a frame is visible
// once FinishFrame returns and nothing has to present it. WebGL has no
// wide or smooth lines, so a stroke of any width is a quad, which the
// context's multisampling smooths. GL output depends on the browser's
// driver, so goldens use the software renderer; this one is pinned by
// the tests here and by drawing in a browser.
package webgl
