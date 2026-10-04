package link

import (
	"path/filepath"
	"strings"
	"sync"

	"github.com/telesapr/telesapr/pkg/sketch"
)

// SketchReader reads the sketch of a model file at path, as saved and
// not regenerated, for counting the parts it links (roadmap 29.3).
type SketchReader func(path string) (*sketch.Sketch, error)

var (
	readersMu sync.RWMutex
	readers   = map[string]SketchReader{}
)

// RegisterSketchReader makes r the reader of model files with
// extension ext, given without its dot and matched ignoring case, so
// that a bill of materials counts the parts those files link.
func RegisterSketchReader(ext string, r SketchReader) {
	readersMu.Lock()
	defer readersMu.Unlock()
	readers[strings.ToLower(ext)] = r
}

func readerFor(path string) SketchReader {
	readersMu.RLock()
	defer readersMu.RUnlock()
	return readers[strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))]
}

// maxPartsDepth bounds how deep links inside linked models are
// followed; a file that links itself, directly or not, stops sooner.
const maxPartsDepth = 16

// LinkedParts is the bill of materials of the model file linkFile: the
// parts it places by linking files, and those the linked models place in
// turn, with each file named as saveFile, the file holding the link,
// would name it. It is nil for a file that isn't a model or can't be
// read.
func LinkedParts(saveFile, linkFile string) []sketch.BOMItem {
	return linkedParts(saveFile, linkFile, map[string]bool{}, 0)
}

func linkedParts(saveFile, linkFile string, open map[string]bool, depth int) []sketch.BOMItem {
	read := readerFor(linkFile)
	if read == nil || depth >= maxPartsDepth {
		return nil
	}
	abs, err := filepath.Abs(linkFile)
	if err != nil {
		abs = linkFile
	}
	if open[abs] {
		return nil
	}
	sk, err := read(linkFile)
	if err != nil || sk == nil {
		return nil
	}
	open[abs] = true
	defer delete(open, abs)
	for i := range sk.Groups {
		g := &sk.Groups[i]
		if g.Type != sketch.GroupLinked || g.LinkFile == "" {
			continue
		}
		file := Path(linkFile, g.LinkFile)
		g.LinkParts = linkedParts(saveFile, file, open, depth+1)
		g.LinkFile = RelativeLink(saveFile, file)
	}
	return sk.BillOfMaterialsAsRead()
}
