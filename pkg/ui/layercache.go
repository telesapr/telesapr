package ui

import (
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/render/soft"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// layerCanvas is a canvas that can save and restore what it has drawn,
// as the software renderer can.
type layerCanvas interface {
	SaveLayer(l *soft.Layer)
	RestoreLayer(l *soft.Layer) bool
}

// persistentKey is what the persistent part of a frame (the snap grid,
// then DrawPersistent) depends on beyond what persistentDirty tracks:
// C++ GL 3 keeps that part as a batch of 3d geometry, redrawn from any
// view, but the software renderer keeps its pixels. The show flags are
// here too, so that a frame never depends on their changes invalidating
// the batch.
type persistentKey struct {
	camera   render.Camera
	lighting render.Lighting

	sketch     *sketch.Sketch
	generation uint64

	activeGroup core.HGroup
	show        sketch.Visibility
	showShaded  bool
	showFaces   bool
	showMesh    bool
	occludedAs  DrawOccludedAs
	dimSolid    bool

	section   srf.HalfSpace
	sectionOn bool

	snapGrid    bool
	workplane   core.HEntity
	gridSpacing float64
}

// persistentLayer is the saved persistent part of the last frame.
type persistentLayer struct {
	layer    soft.Layer
	key      persistentKey
	valid    bool
	disabled bool   // for tests: always draw
	onHit    func() // for tests: told of each restored frame
}

func (gw *GraphicsWindow) persistentKeyFor(canvas render.Canvas, lighting render.Lighting) persistentKey {
	section, sectionOn := gw.sectionKeep()
	return persistentKey{
		section:     section,
		sectionOn:   sectionOn,
		camera:      *canvas.GetCamera(),
		lighting:    lighting,
		sketch:      gw.sk(),
		generation:  gw.sk().Generation,
		activeGroup: gw.activeGroupH(),
		show:        gw.show,
		showShaded:  gw.showShaded,
		showFaces:   gw.showFaces,
		showMesh:    gw.showMesh,
		occludedAs:  gw.drawOccludedAs,
		dimSolid:    gw.dimSolidModel,
		snapGrid:    gw.showSnapGrid,
		workplane:   gw.ActiveWorkplane(),
		gridSpacing: gw.ss.GridSpacing,
	}
}

// drawPersistentLayer draws the snap grid and DrawPersistent, or puts
// back their pixels from the last frame when nothing they depend on has
// changed.
func (gw *GraphicsWindow) drawPersistentLayer(canvas render.Canvas) {
	lc, ok := canvas.(layerCanvas)
	ok = ok && !gw.persistent.disabled
	var key persistentKey
	if ok {
		key = gw.persistentKeyFor(canvas, gw.lighting)
		if !gw.persistentDirty && gw.persistent.valid && gw.persistent.key == key &&
			lc.RestoreLayer(&gw.persistent.layer) {
			if gw.persistent.onHit != nil {
				gw.persistent.onHit()
			}
			return
		}
	}

	if gw.showSnapGrid {
		gw.DrawSnapGrid(canvas)
	}
	// Draw all the things that don't change when we rotate. The software
	// renderer has no batches, like the GL 1 one.
	gw.persistentDirty = false
	gw.DrawPersistent(canvas)

	if ok {
		lc.SaveLayer(&gw.persistent.layer)
		gw.persistent.key = key
		gw.persistent.valid = true
	}
}
