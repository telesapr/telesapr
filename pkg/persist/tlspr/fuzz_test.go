package tlspr

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// FuzzRead reads any bytes as a file. They decode or give an error,
// never a panic. What decodes encodes to a file that decodes again,
// without warnings and with the same expression errors, to a document
// that encodes to the same bytes.
func FuzzRead(f *testing.F) {
	for _, name := range []string{"sketch.tlspr", "fields.tlspr", "text.tlspr", "assembly/assembly.tlspr", "image/image.tlspr"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "tlspr", "v1", filepath.FromSlash(name)))
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	f.Add([]byte(""))
	f.Add([]byte("telesapr 1\n"))
	f.Add([]byte("telesapr 2\n"))
	f.Add([]byte("telesapr 1\nparam a = b*2 # c\nfuture {\n  x \"y\\n\" = 1\n}\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		doc, err := Decode(data)
		if err != nil {
			return
		}
		first := doc.Encode()
		again, err := Decode(first)
		if err != nil {
			t.Fatalf("encoded file does not decode: %v\n%s", err, first)
		}
		if len(again.Warnings) > 0 {
			t.Fatalf("encoded file has warnings %v\n%s", again.Warnings, first)
		}
		if len(again.ExprErrors) != len(doc.ExprErrors) {
			t.Fatalf("expression errors %v, then %v\n%s", doc.ExprErrors, again.ExprErrors, first)
		}
		if second := again.Encode(); !bytes.Equal(first, second) {
			t.Fatalf("encoding is not stable:\n%s\nthen\n%s", first, second)
		}
	})
}
