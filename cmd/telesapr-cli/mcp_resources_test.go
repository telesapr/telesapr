package main

import (
	"bytes"
	"encoding/json"
	"image/png"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/agent"
)

// read is a server's answer to resources/list, templates/list and
// resources/read, and to a tool's call with a link in it.
type read struct {
	Result struct {
		Resources []struct {
			URI   string `json:"uri"`
			Title string `json:"title"`
		} `json:"resources"`
		ResourceTemplates []struct {
			URITemplate string `json:"uriTemplate"`
		} `json:"resourceTemplates"`
		Contents []struct {
			URI      string `json:"uri"`
			MIMEType string `json:"mimeType"`
			Text     string `json:"text"`
			Blob     []byte `json:"blob"`
		} `json:"contents"`
		Content []struct {
			Type     string `json:"type"`
			URI      string `json:"uri"`
			Name     string `json:"name"`
			MIMEType string `json:"mimeType"`
		} `json:"content"`
		Structured struct {
			Path  string `json:"path"`
			Bytes int64  `json:"bytes"`
		} `json:"structuredContent"`
		Completion struct {
			Values []string `json:"values"`
		} `json:"completion"`
	} `json:"result"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// A client's recorded conversation with the resources, over the real
// protocol against a headless server in a folder of its own: they are
// listed, each reads, one there isn't is not found, a group's name
// completes, and export links the file it wrote by its absolute path.
func TestMCPResources(t *testing.T) {
	recorded, err := os.ReadFile(filepath.Join("..", "..", "testdata", "mcp", "resources.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	// The export's relative path lands in the folder: not parallel.
	dir := t.TempDir()
	t.Chdir(dir)
	raw, status, stderr := converseRaw(t, []string{"--headless"}, strings.Split(string(recorded), "\n"))
	if status != exitOK || stderr != "" {
		t.Fatalf("mcp --headless: status %d, stderr %q", status, stderr)
	}
	answers := map[int]read{}
	for id, b := range raw {
		var a read
		if err := json.Unmarshal(b, &a); err != nil {
			t.Fatal(err)
		}
		answers[id] = a
		if a.Error != nil && id != 11 {
			t.Errorf("request %d was answered with the error %q", id, a.Error.Message)
		}
	}

	titles := map[string]string{}
	for _, r := range answers[3].Result.Resources {
		titles[r.URI] = r.Title
	}
	for _, uri := range []string{"telesapr://drawing", "telesapr://drawing/script", "telesapr://drawing/picture",
		"telesapr://help/primer", "telesapr://examples/bolt"} {
		if titles[uri] == "" {
			t.Errorf("%s is not listed with a title: %v", uri, titles)
		}
	}
	if tm := answers[4].Result.ResourceTemplates; len(tm) != 1 || tm[0].URITemplate != "telesapr://drawing/group/{name}" {
		t.Errorf("the templates are %+v", tm)
	}

	text := func(id int, mime string, want ...string) {
		t.Helper()
		c := answers[id].Result.Contents
		if len(c) == 0 || c[0].MIMEType != mime {
			t.Fatalf("request %d reads as %+v", id, c)
		}
		for _, w := range want {
			if !strings.Contains(c[0].Text, w) {
				t.Errorf("request %d: want %q in\n%s", id, w, c[0].Text)
			}
		}
	}
	text(5, "text/plain", "g003-plate, extrude: solved")
	text(6, "text/x-go", "package main", `"plate"`)
	text(8, "text/markdown", agent.Primer())
	text(9, "text/plain", "g003-plate, extrude: solved, 1 DOF, volume 1000 mm³")
	text(10, "text/x-go", "func main()")
	if c := answers[7].Result.Contents; len(c) != 1 || c[0].MIMEType != "image/png" {
		t.Errorf("the picture reads as %+v", c)
	} else if img, err := png.Decode(bytes.NewReader(c[0].Blob)); err != nil || img.Bounds().Dx() != 800 {
		t.Errorf("the picture is no PNG 800 wide: %v", err)
	}
	if e := answers[11].Error; e == nil || !strings.Contains(e.Message, "not found") {
		t.Errorf("an example there isn't reads as %+v", answers[11])
	}
	if got := answers[12].Result.Completion.Values; len(got) != 1 || got[0] != "g003-plate" {
		t.Errorf("a group's name completes as %v", got)
	}

	// The path the server made absolute, from the directory it works in.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	abs := filepath.Join(wd, "plate.stl")
	fi, err := os.Stat(abs)
	if err != nil {
		t.Fatal(err)
	}
	ex := answers[13].Result
	if ex.Structured.Path != abs || ex.Structured.Bytes != fi.Size() {
		t.Errorf("export says it wrote %+v, want %s of %d bytes", ex.Structured, abs, fi.Size())
	}
	// A drive path takes a slash before it, file:///C:/x, as a Unix one
	// has its own.
	slashed := filepath.ToSlash(abs)
	if !strings.HasPrefix(slashed, "/") {
		slashed = "/" + slashed
	}
	want := (&url.URL{Scheme: "file", Path: slashed}).String()
	if len(ex.Content) != 2 || ex.Content[1].Type != "resource_link" || ex.Content[1].URI != want ||
		ex.Content[1].Name != "plate.stl" || ex.Content[1].MIMEType != "model/stl" {
		t.Errorf("export's content is %+v, want a link to %s", ex.Content, want)
	}
}
