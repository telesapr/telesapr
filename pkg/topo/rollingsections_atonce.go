package topo

import "github.com/telesapr/telesapr/pkg/internal/par"

// A chain's sections found at once. A section is the ball at an arc
// length (ballSection), found from the faces alone while the shell's
// surfaces are projected on independently (srf.SShell.ProjectIndependently)
// and the chain is traced and mitred (blendingAtOnce): the searches that
// find many of them, each from its own start (a mitre's arc lengths, the
// points a fitted end section passes through, the scan for seams, the
// sections a blend is fitted through and its misfits), find them on
// several goroutines and keep what one loop would, in its order. Outside
// that, where each projection starts where the one before on the
// surface ended (SSurface.Cached), they run in order as before.

// each calls fn(i) for every i in [0, n): at once where the chain's
// sections may be found so (atOnce), else in order. fn writes only to
// what iteration i owns.
func (rb *rollingBall) each(n int, fn func(i int)) {
	rb.eachWhile(n, func(i int) bool {
		fn(i)
		return true
	})
}

// eachWhile is each, but in order it stops after the first iteration
// for which fn says no more, as the loop it stands for returned there:
// what it would not have projected is not projected, which in order
// moves where the next projection starts. At once every iteration runs,
// and the caller takes their results in order up to that one.
func (rb *rollingBall) eachWhile(n int, fn func(i int) bool) {
	if rb.atOnce {
		par.For(n, func(i int) { fn(i) })
		return
	}
	for i := range n {
		if !fn(i) {
			return
		}
	}
}
