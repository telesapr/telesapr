package mcpserver

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// export links the file it wrote, by its absolute path, with its type
// and size, and says the same as data.
func TestExportLinksTheFile(t *testing.T) {
	t.Parallel()
	cs, _ := connect(t)
	call(t, cs, ToolRun, map[string]any{"code": "doc.Sketch.Rect(V(0, 0, 0), V(10, 20, 0))\ndoc.Features.Extrude(5, \"plate\")"})
	dir := t.TempDir()
	for _, c := range []struct{ name, mime string }{{"plate, one.stl", "model/stl"}, {"plate.glb", "model/gltf-binary"}, {"plate.go", "text/x-go"}} {
		path := filepath.Join(dir, c.name)
		res, text := call(t, cs, ToolExport, map[string]any{"path": path})
		if res.IsError {
			t.Fatalf("exporting %s: %s", c.name, text)
		}
		var out ExportOutput
		structured(t, res, &out)
		if out.Path != path || out.Bytes <= 0 {
			t.Errorf("%s: the data is %+v", c.name, out)
		}
		if len(res.Content) != 2 {
			t.Fatalf("%s: the content is %v", c.name, res.Content)
		}
		link, ok := res.Content[1].(*mcp.ResourceLink)
		if !ok || link.URI != fileURI(path) || link.Name != c.name || link.MIMEType != c.mime || link.Size == nil || *link.Size != out.Bytes {
			t.Errorf("%s: the link is %+v", c.name, res.Content[1])
		}
	}
}

// What the back end says of an export is read for the path and the
// size where it has the shape of both back ends' sentence, and not
// otherwise.
func TestExportResultReadsTheSentence(t *testing.T) {
	t.Parallel()
	abs, err := filepath.Abs(filepath.Join("out", "a, b.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		said string
		want ExportOutput
		link bool
	}{
		{"wrote " + abs + ", 12 bytes", ExportOutput{Path: abs, Bytes: 12}, true},
		{"wrote " + abs + ", 12 bytes\nnot written:\n- a thread\n- a link", ExportOutput{Path: abs, Bytes: 12, NotWritten: []string{"a thread", "a link"}}, true},
		{"wrote plate.stl, 12 bytes", ExportOutput{}, false},
		{"wrote " + abs + ", many bytes", ExportOutput{}, false},
		{"wrote " + abs, ExportOutput{}, false},
		{"exported", ExportOutput{}, false},
	} {
		res, out := exportResult(c.said)
		if out.Path != c.want.Path || out.Bytes != c.want.Bytes || !slices.Equal(out.NotWritten, c.want.NotWritten) {
			t.Errorf("%q reads as %+v, want %+v", c.said, out, c.want)
		}
		if got := len(res.Content) == 2; got != c.link {
			t.Errorf("%q: linked %v, want %v", c.said, got, c.link)
		}
	}
	if got := fileURI(filepath.FromSlash("/tmp/a b.stl")); filepath.Separator == '/' && got != "file:///tmp/a%20b.stl" {
		t.Errorf("the URI is %s", got)
	}
}
