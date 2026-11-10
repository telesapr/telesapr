# OpenGL contexts on each system

`pkg/render/gl` draws with fixed-function GL 2.1 into a context it is given. The context comes
from `gl.NewWindowContext(id)`, which makes one on the toolkit window with the platform handle
`id` (`window.Window.PlatformID`: an X Window, an HWND, an NSView). It returns a
`gl.WindowContext`; any error, or a build without the backend, sends the window to the software
renderer (`pkg/render/soft`) with one log line.

| System | Package | Built with | Notes |
|---|---|---|---|
| Linux | `glx` | cgo, Xlib, libGL | second Xlib connection; has an offscreen context for tests |
| Windows | `wgl` | cgo and a C compiler (amd64 only in the release) | pixel format on the window's DC; a GDI generic format is refused, so a machine with no GL driver draws in software |
| macOS | `cgl` | cgo (takigo needs it there anyway) | `NSOpenGLContext`, legacy profile, in a view over the canvas view; points, not Retina pixels |

Every backend has a stub for builds without cgo or with `-tags nogl`.

Open on Windows and macOS: no offscreen context (the GL tests skip there), no arm64 GL build on
Windows, no Retina-resolution surface on macOS, and neither backend has run on a real machine in
CI yet.
