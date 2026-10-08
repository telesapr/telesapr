package ui

import (
	"github.com/telesapr/telesapr/pkg/res"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// imagePixmap is the pixmap of an image request's file, or nil if it
// couldn't be read.
func (ss *Application) imagePixmap(file string) *res.Pixmap {
	img := ss.SK.Images[file]
	if img == nil || img.Pixels == nil {
		return nil
	}
	if ss.pixmaps == nil {
		ss.pixmaps = make(map[*sketch.Image]*res.Pixmap)
	}
	pm := ss.pixmaps[img]
	if pm == nil {
		pm = res.FromImage(img.Pixels, false)
		ss.pixmaps[img] = pm
	}
	return pm
}
