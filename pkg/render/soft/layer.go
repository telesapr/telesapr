package soft

// Layer is a saved copy of a frame's colour and depth, for redrawing a
// frame whose first part hasn't changed. GL 1 has no such thing; it is
// the software renderer's stand-in for GL 3's persistent batch.
type Layer struct {
	pix           []uint8
	depth         []float32
	width, height int
	// gen counts the saves into the layer, so that a renderer that
	// restored it before can tell it was saved again since.
	gen uint64
}

// SaveLayer copies the frame drawn so far into l, reusing its buffers.
func (r *Renderer) SaveLayer(l *Layer) {
	if r.frame == nil {
		l.width, l.height = 0, 0
		return
	}
	r.settleAll()
	l.pix = append(l.pix[:0], r.frame.Pix...)
	l.depth = append(l.depth[:0], r.depth...)
	l.width, l.height = r.width, r.height
	l.gen++
	r.layer, r.layerGen = l, l.gen
	for y := range r.rows {
		r.rows[y] = 0
	}
}

// RestoreLayer replaces the frame drawn so far with l, and reports
// whether it could: l must have been saved at the frame's size. Where
// the frame was last saved to or restored from l, only the rows drawn
// on since are copied, and the clears waiting are dropped, l being
// what they would have been drawn over.
func (r *Renderer) RestoreLayer(l *Layer) bool {
	if r.frame == nil || l.width != r.width || l.height != r.height ||
		len(l.pix) != len(r.frame.Pix) || len(l.depth) != len(r.depth) {
		return false
	}
	if r.layer != l || r.layerGen != l.gen {
		copy(r.frame.Pix, l.pix)
		copy(r.depth, l.depth)
		r.layer, r.layerGen = l, l.gen
		for y := range r.rows {
			r.rows[y] = 0
		}
		return true
	}
	w := r.width
	for y, st := range r.rows {
		if st&rowDiffers != 0 {
			copy(r.frame.Pix[y*w*4:(y+1)*w*4], l.pix[y*w*4:(y+1)*w*4])
			copy(r.depth[y*w:(y+1)*w], l.depth[y*w:(y+1)*w])
		}
		r.rows[y] = 0
	}
	return true
}
