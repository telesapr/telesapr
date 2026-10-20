package soft

// Layer is a saved copy of a frame's colour and depth, for redrawing a
// frame whose first part hasn't changed. GL 1 has no such thing; it is
// the software renderer's stand-in for GL 3's persistent batch.
type Layer struct {
	pix           []uint8
	depth         []float32
	width, height int
}

// SaveLayer copies the frame drawn so far into l, reusing its buffers.
func (r *Renderer) SaveLayer(l *Layer) {
	if r.frame == nil {
		l.width, l.height = 0, 0
		return
	}
	r.readyDepth()
	l.pix = append(l.pix[:0], r.frame.Pix...)
	l.depth = append(l.depth[:0], r.depth...)
	l.width, l.height = r.width, r.height
}

// RestoreLayer replaces the frame drawn so far with l, and reports
// whether it could: l must have been saved at the frame's size.
func (r *Renderer) RestoreLayer(l *Layer) bool {
	if r.frame == nil || l.width != r.width || l.height != r.height ||
		len(l.pix) != len(r.frame.Pix) || len(l.depth) != len(r.depth) {
		return false
	}
	copy(r.frame.Pix, l.pix)
	copy(r.depth, l.depth)
	r.depthStale = false
	return true
}
