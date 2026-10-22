package soft

// rowState is what is owed to a row of the frame, and what it holds.
// Clears are kept per row and done at the row's first fragment, so
// that the bands of a frame drawn at once each settle their own rows,
// and so that a frame which puts a saved layer back copies only the
// rows drawn on since: a hover frame is the persistent layer, a few
// rows of highlight and the toolbar, not 16 MB of copying.
type rowState uint8

const (
	// rowColorStale: the row is to be filled with the background.
	rowColorStale rowState = 1 << iota
	// rowDepthStale: the row's depth is to be cleared to the far plane.
	rowDepthStale
	// rowDiffers: the row may differ from the layer the frame was last
	// saved to or restored from.
	rowDiffers
)

// settleRow does the clears row y waits for, and marks it as differing
// from the layer, as it is about to be drawn on. Only the band drawing
// row y calls it, so bands drawn at once don't share a row's state.
func (r *Renderer) settleRow(y int) {
	st := r.rows[y]
	if st&rowColorStale != 0 {
		fill(r.frame.Pix[y*r.width*4:(y+1)*r.width*4], r.bg[:])
	}
	if st&rowDepthStale != 0 {
		d := r.depth[y*r.width : (y+1)*r.width]
		d[0] = 1
		for n := 1; n < len(d); n *= 2 {
			copy(d[n:], d[:n])
		}
	}
	r.rows[y] = rowDiffers
}

// settleRows settles the rows from y0 to y1, both included, that a
// primitive may draw on.
func (r *Renderer) settleRows(y0, y1 int) {
	for y := max(y0, 0); y <= min(y1, r.height-1); y++ {
		if r.rows[y] != rowDiffers {
			r.settleRow(y)
		}
	}
}

// settleAll does every clear waiting, before the frame is read or
// copied whole.
func (r *Renderer) settleAll() {
	for y, st := range r.rows {
		if st&(rowColorStale|rowDepthStale) != 0 {
			r.settleRow(y)
		}
	}
}
