package mcpserver

import (
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ExportOutput is the part of export's result a program can read
// without reading the text: the file written and its size.
type ExportOutput struct {
	Path       string   `json:"path,omitempty" jsonschema:"the file written, as an absolute path"`
	Bytes      int64    `json:"bytes,omitempty" jsonschema:"the size of the file written, in bytes"`
	NotWritten []string `json:"notWritten,omitempty" jsonschema:"what the file couldn't hold, for a script (.go)"`
}

// exportTypes are the MIME types of the files export writes, by
// extension; a file of another is linked without one.
var exportTypes = map[string]string{
	".stl": "model/stl", ".3mf": "model/3mf", ".glb": "model/gltf-binary", ".obj": "model/obj", ".wrl": "model/vrml",
	".js": "text/javascript", ".html": "text/html",
	".step": "model/step", ".stp": "model/step",
	".svg": "image/svg+xml", ".dxf": "image/vnd.dxf", ".pdf": "application/pdf", ".eps": "application/postscript",
	".ps": "application/postscript", ".txt": "text/plain", ".png": "image/png", ".go": "text/x-go",
}

// exportResult is export's result from what the back end said of it.
// Both back ends say "wrote <absolute path>, <n> bytes" first, the path
// as the program with the drawing made it absolute, from the directory
// it works in, so the link is to the file it wrote even where that
// directory is not the command's. A sentence not of that shape is given
// as it is, with no link.
func exportResult(said string) (*mcp.CallToolResult, ExportOutput) {
	res := text(said)
	first, rest, _ := strings.Cut(said, "\n")
	out, ok := wrote(first)
	if !ok {
		return res, ExportOutput{}
	}
	if notes, found := strings.CutPrefix(rest, "not written:\n"); found {
		for line := range strings.Lines(notes) {
			if note, ok := strings.CutPrefix(strings.TrimRight(line, "\n"), "- "); ok {
				out.NotWritten = append(out.NotWritten, note)
			}
		}
	}
	size := out.Bytes
	res.Content = append(res.Content, &mcp.ResourceLink{URI: fileURI(out.Path), Name: filepath.Base(out.Path),
		MIMEType: exportTypes[strings.ToLower(filepath.Ext(out.Path))], Size: &size})
	return res, out
}

// wrote reads "wrote <path>, <n> bytes"; the path may hold commas.
func wrote(line string) (ExportOutput, bool) {
	rest, ok := strings.CutPrefix(line, "wrote ")
	if !ok {
		return ExportOutput{}, false
	}
	path, size, found := strings.CutLast(rest, ", ")
	if !found {
		return ExportOutput{}, false
	}
	count, ok := strings.CutSuffix(size, " bytes")
	n, err := strconv.ParseInt(count, 10, 64)
	if !ok || err != nil || !filepath.IsAbs(path) {
		return ExportOutput{}, false
	}
	return ExportOutput{Path: path, Bytes: n}, true
}

// fileURI is the file:// URI of an absolute path; on Windows, C:\x is
// file:///C:/x.
func fileURI(path string) string {
	p := filepath.ToSlash(path)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return (&url.URL{Scheme: "file", Path: p}).String()
}
