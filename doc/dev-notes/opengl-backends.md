# OpenGL contexts on each system

Refreshed 2026-11-23: `cgl` has an offscreen context.

`pkg/render/gl` draws with fixed-function GL 2.1 into a context it is given. The context comes
from `gl.NewWindowContext(id)`, which makes one on the toolkit window with the platform handle
`id` (`window.Window.PlatformID`: an X Window, an HWND, and on macOS takigo's own view ID, which cgl looks up among the application's views by their `windowID` property). It returns a
`gl.WindowContext`; any error, or a build without the backend, sends the window to the software
renderer (`pkg/render/soft`) with one log line.

| System | Package | Built with | Notes |
|---|---|---|---|
| Linux | `glx` | cgo, Xlib, libGL | second Xlib connection; has an offscreen context for tests |
| Windows | `wgl` | cgo and a C compiler (amd64 only in the release) | pixel format on the window's DC; a GDI generic format is refused, so a machine with no GL driver draws in software |
| macOS | `cgl` | cgo (takigo needs it there anyway) | `NSOpenGLContext`, legacy profile, in a view over the canvas view; a Retina-resolution surface, viewport scaled by `PixelRatio` |

Every backend has a stub for builds without cgo or with `-tags nogl`.

Open work is in ROADMAP "OpenGL leftovers". Neither window backend runs in CI: both were tried by hand on a real machine.

## Offscreen contexts, and the GL tests

The renderer tests (`pkg/render/gl/renderer_test.go`) make their context through `offscreen`, which each system has in a file of its own (`offscreen_linux_test.go`, `offscreen_darwin_test.go`, `offscreen_other_test.go`) and which skips the test where no context can be made.

- Linux: `glx.NewOffscreen`, a pixel buffer on the X server (Xvfb in CI).
- macOS: `cgl.NewOffscreen`, plain CGL with no AppKit, no window and no main thread. It tries a legacy-profile pixel format with `kCGLPFAAccelerated` (offline GPUs allowed, since nothing is shown), then one with `kCGLPFARendererID` `kCGLRendererGenericFloatID`, Apple's software renderer, which is what a GitHub macOS runner without a GPU would give; `Context.Software` says which served. A context with no drawable has no default framebuffer, so the context makes a framebuffer object (RGBA8 colour, packed depth and stencil, EXT_framebuffer_object, which the legacy profile has) that stays bound. `ReadFrame` reads its colour attachment instead of the front or back buffer (`gl.Framebuffered`).
- Windows: `wgl.NewOffscreen` exists; no test uses it yet.

The macOS part was written on Linux and has never run: the nightly and Release workflows run `scripts/test-os.sh` on `macos-latest`, which includes `pkg/render/gl`, and the first run there shows whether the tests pass or skip.

