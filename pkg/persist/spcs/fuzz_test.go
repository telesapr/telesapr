package spcs

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// FuzzRead reads any bytes as a file. They decode or give an error,
// never a panic. What decodes encodes to a file that decodes again,
// without warnings, to a document that encodes to the same bytes.
func FuzzRead(f *testing.F) {
	for _, name := range []string{"sketch.spcs", "fields.spcs", "text.spcs", "assembly/assembly.spcs", "image/image.spcs"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "spcs", "v1", filepath.FromSlash(name)))
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	f.Add([]byte(""))
	f.Add([]byte("spacesolve 1\n"))
	f.Add([]byte("spacesolve 2\n"))
	f.Add([]byte("spacesolve 1\nparam a = b*2 # c\nfuture {\n  x \"y\\n\" = 1\n}\n"))
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
		if second := again.Encode(); !bytes.Equal(first, second) {
			t.Fatalf("encoding is not stable:\n%s\nthen\n%s", first, second)
		}
	})
}
