package platform

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// SystemFontFiles lists the font files in the usual font directories, as
// the GTK build lists the ones fontconfig knows.
func SystemFontFiles() []string {
	var dirs []string
	switch runtime.GOOS {
	case "windows":
		dirs = []string{filepath.Join(os.Getenv("WINDIR"), "Fonts")}
	case "darwin":
		dirs = []string{"/System/Library/Fonts", "/Library/Fonts"}
		if home, err := os.UserHomeDir(); err == nil {
			dirs = append(dirs, filepath.Join(home, "Library", "Fonts"))
		}
	default:
		dirs = []string{"/usr/share/fonts", "/usr/local/share/fonts"}
		if dataHome := os.Getenv("XDG_DATA_HOME"); dataHome != "" {
			dirs = append(dirs, filepath.Join(dataHome, "fonts"))
		}
		if home, err := os.UserHomeDir(); err == nil {
			dirs = append(dirs, filepath.Join(home, ".local", "share", "fonts"), filepath.Join(home, ".fonts"))
		}
	}
	var files []string
	for _, dir := range dirs {
		_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil //nolint:nilerr // an entry that can't be read is skipped, the walk goes on
			}
			switch strings.ToLower(filepath.Ext(path)) {
			case ".ttf", ".otf", ".ttc", ".pfa", ".pfb", ".woff", ".woff2":
				files = append(files, path)
			}
			return nil
		})
	}
	return files
}
