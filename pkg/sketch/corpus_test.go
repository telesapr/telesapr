package sketch_test

import (
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/fixtures"
	"github.com/telesapr/telesapr/pkg/persist/tlspr"
)

// corpusDir holds the corpus: the models of testdata as .tlspr files,
// the imports of the .slvs fixtures pkg/persist/convert keeps.
const corpusDir = "tlspr/golden/"

// corpus lists the models of the corpus, as slash-separated paths from
// corpusDir, in lexical order.
func corpus(t testing.TB) []string {
	t.Helper()
	all, err := fixtures.Walk(".tlspr")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, f := range all {
		if rel, ok := strings.CutPrefix(f, corpusDir); ok {
			out = append(out, rel)
		}
	}
	if len(out) == 0 {
		t.Fatal("no models under testdata/" + corpusDir)
	}
	return out
}

// corpusPath is the file of a model of the corpus.
func corpusPath(rel string) string { return fixtures.Path(corpusDir + rel) }

// openCorpus opens a model of the corpus, regenerated without solving,
// as a file is opened.
func openCorpus(rel string) (*tlspr.Document, error) {
	return tlspr.LoadFile(corpusPath(rel))
}
