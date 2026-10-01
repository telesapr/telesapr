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
