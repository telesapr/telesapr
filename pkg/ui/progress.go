package ui

import (
	"fmt"
	"path/filepath"

	"github.com/telesapr/telesapr/pkg/platform"
)

// Opening a file, saving one and exporting one run on the loop's
// goroutine, and a large model takes them long: a progress window says how far they
// have got (platform.Progress). It goes up once the dialogs that asked
// for the file and its options have closed, and down when the work
// ends, before what it has to say (an error, a warning) is shown.

// withProgress runs work with a progress window titled title over the
// graphics window, on a platform that shows one. Work started inside
// one already up shows in it.
func (ss *Application) withProgress(title string, work func()) {
	ps, ok := ss.Platform.(platform.ProgressShower)
	if !ok || ss.progress != nil || ss.GW.window == nil {
		work()
		return
	}
	pr := ps.ShowProgress(ss.GW.window, title)
	ss.progress = pr
	defer func() {
		ss.progress = nil
		pr.Close()
	}()
	pr.Update(title, -1)
	work()
}

// progressPhase says, in the progress window if one is up, what the
// work does now, not knowing how far it has got.
func (ss *Application) progressPhase(text string) {
	if ss.progress != nil {
		ss.progress.Update(text, -1)
	}
}

// progressRegenerating shows in the progress window, if one is up, the
// group a regeneration begins: before groups of total are done.
func (ss *Application) progressRegenerating(group string, before, total int) {
	if ss.progress == nil || total <= 0 {
		return
	}
	ss.progress.Update(fmt.Sprintf("Regenerating %s (group %d of %d)", group, before+1, total),
		float64(before)/float64(total))
}

// fileTitle is the title of a progress window for doing what to the
// file path: "Opening part.tlspr".
func fileTitle(what, path string) string { return what + " " + filepath.Base(path) }
