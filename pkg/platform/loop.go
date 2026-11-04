package platform

// LoopPoster is a Platform whose event loop other goroutines can ask
// to run something: all of the user interface, and the sketch with
// it, belongs to the goroutine of the loop. A platform without a loop,
// such as the headless one, doesn't implement it; there everything
// runs where it is called.
type LoopPoster interface {
	// PostToLoop has fn run on the loop's goroutine, soon. It may be
	// called from any goroutine, and returns without waiting for fn.
	PostToLoop(fn func())
	// LoopDone is closed once the loop has ended: what is posted then
	// never runs.
	LoopDone() <-chan struct{}
}

// Yielder is a Platform whose event loop's thread is the only one the
// program has, as a browser page's is: while the loop's goroutine
// regenerates a model, nothing else, the page's drawing included, runs.
// The application calls Yield between the groups of a regeneration, on
// the loop's goroutine; the platform may let the page draw and show the
// work's progress there before it returns. What comes from the user
// meanwhile waits, or is dropped, until the regeneration is done. A
// platform with threads of its own doesn't implement it.
type Yielder interface {
	Yield()
}
