package web

import (
	"path"
	"strings"

	"github.com/telesapr/telesapr/pkg/platform"
)

// homeDir is where the files a user brings in are put and where saves
// go, in the page's in-memory file system.
const homeDir = "/home/user"

// acceptAttribute is the filters as an <input type=file>'s accept
// attribute: every extension of every filter. A filter with an empty
// extension takes any file, which is an empty attribute.
func acceptAttribute(filters []platform.FileFilter) string {
	var exts []string
	for _, f := range filters {
		for _, e := range f.Extensions {
			if e == "" || e == "*" {
				return ""
			}
			exts = append(exts, "."+strings.TrimPrefix(e, "."))
		}
	}
	return strings.Join(exts, ",")
}

// withExtension adds ext to a name typed without an extension, as the
// other save dialogs do.
func withExtension(name, ext string) string {
	if ext == "" || path.Ext(name) != "" {
		return name
	}
	return name + "." + ext
}

// stem is a path's file name without its extension.
func stem(p string) string {
	base := path.Base(p)
	if dot := strings.LastIndexByte(base, '.'); dot > 0 {
		return base[:dot]
	}
	return base
}

// firstExtension is the first extension of filter i, or of the first
// filter when i is out of range; empty if there is none.
func firstExtension(filters []platform.FileFilter, i int) string {
	if len(filters) == 0 {
		return ""
	}
	if i < 0 || i >= len(filters) {
		i = 0
	}
	if len(filters[i].Extensions) == 0 {
		return ""
	}
	return filters[i].Extensions[0]
}

// folderOrHome is the folder of a file name if it names one, else the
// home folder: a model saved on a desktop names paths that mean nothing
// here.
func folderOrHome(p string) string {
	if strings.HasPrefix(p, "/") {
		return path.Dir(p)
	}
	return homeDir
}
