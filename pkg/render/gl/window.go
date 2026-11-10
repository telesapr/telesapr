package gl

// WindowContext is a GL context drawing on a toolkit window: the
// Context a renderer needs, and what the window's owner does with it.
// Each platform has one (package glx on X11, wgl on Windows, cgl on
// macOS); NewWindowContext picks the one of the running system.
type WindowContext interface {
	Context
	// SwapBuffers shows the frame drawn since the last swap.
	SwapBuffers()
	// Size is the drawable's size in pixels when the context was made.
	Size() (width, height int)
	// Destroy frees the context. The window stays.
	Destroy()
}
