package srf

// SinkRedone is the number of exact intersection curves that were made
// over when the lists of surfaces intersected at once were added.
func SinkRedone() int64 { return sinkRedone.Load() }
