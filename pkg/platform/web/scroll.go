package web

import "math"

// The scrollbar of a window is a scrolling box with a spacer in it: the
// application's range lo to hi, shown a page at a time, is the spacer's
// height against the track's, and the box's scrollTop the value. These
// are the conversions.

// scrollSpacer is the height of the spacer in a track of the given
// height, so that the box scrolls through hi-lo as pages of pageSize; a
// range that fits the page leaves nothing to scroll.
func scrollSpacer(track, lo, hi, pageSize float64) float64 {
	span := hi - lo
	if span <= pageSize || pageSize <= 0 {
		return track
	}
	return track * span / pageSize
}

// scrollTopFor is the box's scrollTop for a value.
func scrollTopFor(value, lo, hi, spacer float64) float64 {
	span := hi - lo
	if span <= 0 {
		return 0
	}
	return (value - lo) * spacer / span
}

// valueForScrollTop is the value for a scrollTop.
func valueForScrollTop(top, lo, hi, spacer float64) float64 {
	if spacer <= 0 {
		return lo
	}
	return lo + top*(hi-lo)/spacer
}

// clampScroll keeps a value between lo and hi-pageSize, as an
// adjustment does.
func clampScroll(value, lo, hi, pageSize float64) float64 {
	return math.Max(math.Min(value, hi-pageSize), lo)
}
