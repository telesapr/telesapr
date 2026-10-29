// Package fixtures finds the models and other files under the
// repository's testdata for the tests of any package, and thins the
// loops over them under the race detector.
package fixtures

import (
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"
)

// raceEvery is how many fixtures and cases make one that runs under the
// race detector, which is about eight times slower. The race detector
// needs the code that runs on several goroutines, not every model; the
// plain and the coverage runs take them all.
const raceEvery = 5

// Dir is the repository's testdata folder, found from this file, so
// that it is the same whichever package's test asks.
func Dir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "testdata")
}

// Path is the file named by rel, a slash-separated path under testdata.
func Path(rel string) string {
	return filepath.Join(Dir(), filepath.FromSlash(rel))
}

// Walk lists the files under testdata whose names end in ext, as
// slash-separated paths relative to it, in lexical order.
func Walk(ext string) ([]string, error) {
	dir := Dir()
	var out []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ext) {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	return out, err
}

// Sample is every raceEvery-th of xs under the race detector, and all
// of them otherwise.
func Sample[T any](xs []T) []T {
	return sample(xs, RaceDetector)
}

// Thin is every raceEvery-th of xs, for a loop that decides itself when
// to thin, as a sweep does under the race detector unless its whole
// grid is asked for.
func Thin[T any](xs []T) []T {
	return sample(xs, true)
}

// sample is every raceEvery-th of xs when thin, else all of them.
func sample[T any](xs []T, thin bool) []T {
	if !thin {
		return xs
	}
	var out []T
	for i, x := range xs {
		if i%raceEvery == 0 {
			out = append(out, x)
		}
	}
	return out
}
