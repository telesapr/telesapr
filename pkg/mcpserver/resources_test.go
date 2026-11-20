package mcpserver

import (
	"bytes"
	"context"
	"errors"
	"image/png"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/telesapr/telesapr/examples"
	"github.com/telesapr/telesapr/pkg/script/agent"
)

// readResource reads uri, and fails the test if it can't be read.
func readResource(t *testing.T, cs *mcp.ClientSession, uri string) []*mcp.ResourceContents {
	t.Helper()
	res, err := cs.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: uri})
	if err != nil {
		t.Fatalf("reading %s: %v", uri, err)
	}
	if len(res.Contents) == 0 {
		t.Fatalf("%s has no contents", uri)
	}
	return res.Contents
}

// The drawing's resources, the primer and every example are listed,
// each with a name, a title and a description a user reads, and the
// group's template is listed apart.
func TestResourcesListed(t *testing.T) {
	t.Parallel()
	cs, _ := connect(t)
	ctx := context.Background()
	list, err := cs.ListResources(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	ex, err := examples.List()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{ResourceDrawing: "text/plain", ResourceScript: "text/x-go",
		ResourcePicture: "image/png", ResourcePrimer: "text/markdown"}
	for _, e := range ex {
		want[ResourceExamples+e.Name] = "text/x-go"
	}
	if len(list.Resources) != len(want) {
		t.Errorf("%d resources are listed, want %d", len(list.Resources), len(want))
	}
	for _, r := range list.Resources {
		mime, known := want[r.URI]
		if !known {
			t.Errorf("a resource %q is listed", r.URI)
			continue
		}
		if r.MIMEType != mime || r.Name == "" || r.Title == "" || r.Description == "" || strings.Contains(r.Description, "\n") {
			t.Errorf("%s: %+v", r.URI, r)
		}
	}
	if i := slices.IndexFunc(list.Resources, func(r *mcp.Resource) bool { return r.URI == ResourceExamples+"bolt" }); i < 0 ||
		list.Resources[i].Title != "Example bolt" || !strings.Contains(list.Resources[i].Description, "bolt") {
		t.Errorf("the bolt example is not listed as a user would find it")
	}

	templates, err := cs.ListResourceTemplates(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(templates.ResourceTemplates) != 1 {
		t.Fatalf("the templates are %+v", templates.ResourceTemplates)
	}
	if tm := templates.ResourceTemplates[0]; tm.URITemplate != "telesapr://drawing/group/{name}" || tm.Title == "" || tm.Description == "" {
		t.Errorf("the group's template is %+v", tm)
	}
	if caps := cs.InitializeResult().Capabilities.Resources; caps == nil || caps.Subscribe {
		t.Errorf("the resources' capabilities are %+v", caps)
	}
}

// Each resource reads as the tool that gives the same thing gives it.
func TestResourcesRead(t *testing.T) {
	t.Parallel()
	cs, painter := connect(t)
	call(t, cs, ToolRun, map[string]any{"code": "doc.Sketch.Circle(V(0, 0, 0), 5)\ndoc.Features.Extrude(3, \"disc\")"})

	_, described := call(t, cs, ToolDescribe, nil)
	_, script := call(t, cs, ToolDescribe, map[string]any{"script": true})
	_, group := call(t, cs, ToolDescribe, map[string]any{"group": "g002"})
	for _, c := range []struct{ uri, mime, want string }{
		{ResourceDrawing, "text/plain", described},
		{ResourceScript, "text/x-go", script},
		{ResourcePrimer, "text/markdown", agent.Primer()},
		{ResourceGroup + "g002", "text/plain", group},
		{ResourceGroup + "g002-sketch-in-plane", "text/plain", group},
	} {
		got := readResource(t, cs, c.uri)
		if len(got) != 1 || got[0].URI != c.uri || got[0].MIMEType != c.mime || got[0].Text != c.want {
			t.Errorf("%s reads as %+v, want %q", c.uri, got, c.want)
		}
	}
	if !strings.Contains(described, "g003-disc") || !strings.Contains(script, "package main") ||
		!strings.Contains(group, "r004-circle") {
		t.Errorf("the tools gave\n%s\n%s\n%s", described, script, group)
	}

	pic := readResource(t, cs, ResourcePicture)
	if len(pic) != 1 || pic[0].MIMEType != "image/png" || pic[0].Text != "" {
		t.Fatalf("the picture is %+v", pic)
	}
	if img, err := png.Decode(bytes.NewReader(pic[0].Blob)); err != nil || img.Bounds().Dx() != 4 {
		t.Errorf("the picture is no PNG of the painter's: %v", err)
	}
	if painter.asked != (agent.Picture{}) {
		t.Errorf("the picture was asked for as %+v, not at the defaults", painter.asked)
	}

	// An example with a library: the script, then its library's files.
	lib := readResource(t, cs, ResourceExamples+"library")
	files, err := examples.Files("library")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 2 || len(lib) != len(files) || lib[0].URI != ResourceExamples+"library" || lib[0].Text != files[0].Text {
		t.Fatalf("the library example reads as %d contents, want %d", len(lib), len(files))
	}
	for i, f := range files[1:] {
		if c := lib[i+1]; c.URI != ResourceExamples+"library/"+f.Path || c.Text != f.Text || c.MIMEType != "text/x-go" {
			t.Errorf("the library's file %s reads as %s", f.Path, c.URI)
		}
	}
}

// A resource there isn't, or a group there isn't, is not read, and
// says why.
func TestResourcesNotFound(t *testing.T) {
	t.Parallel()
	cs, _ := connect(t)
	for _, uri := range []string{"telesapr://nothing", ResourceExamples + "nothing", ResourceGroup} {
		_, err := cs.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: uri})
		// The SDK's resource-not-found, by its message: its code is to change.
		if rpc, ok := errors.AsType[*jsonrpc.Error](err); !ok || rpc.Message != "Resource not found" {
			t.Errorf("reading %s: %v, want resource not found", uri, err)
		}
	}
	_, err := cs.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: ResourceGroup + "g099"})
	if err == nil || !strings.Contains(err.Error(), `reading telesapr://drawing/group/g099: no group is called "g099"`) {
		t.Errorf("reading a group there isn't: %v", err)
	}
}

// With no window running, a resource of the drawing is not read, and
// the error says what the user is to do; the primer and the examples,
// which need no drawing, still read.
func TestResourcesWithoutTheDrawing(t *testing.T) {
	t.Parallel()
	cs := connectTo(t, notRunning{})
	for _, uri := range []string{ResourceDrawing, ResourceScript, ResourcePicture, ResourceGroup + "g002"} {
		_, err := cs.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: uri})
		if err == nil || !strings.Contains(err.Error(), "reading "+uri+": "+agent.ErrNotRunning.Error()) {
			t.Errorf("reading %s with no window: %v", uri, err)
		}
	}
	if got := readResource(t, cs, ResourcePrimer); got[0].Text != agent.Primer() {
		t.Errorf("the primer with no window is %q", got[0].Text)
	}
	readResource(t, cs, ResourceExamples+"bolt")
	if got := resourceCompletions(t, cs, ""); len(got) != 0 {
		t.Errorf("the groups of no drawing are %v", got)
	}
}

// resourceCompletions are the values offered for the group template's
// name.
func resourceCompletions(t *testing.T, cs *mcp.ClientSession, typed string) []string {
	t.Helper()
	res, err := cs.Complete(context.Background(), &mcp.CompleteParams{
		Ref:      &mcp.CompleteReference{Type: "ref/resource", URI: groupTemplate},
		Argument: mcp.CompleteParamsArgument{Name: "name", Value: typed}})
	if err != nil {
		t.Fatalf("completing a group's name: %v", err)
	}
	return res.Completion.Values
}

// The group template's name completes from the drawing's groups, as
// check's group does.
func TestResourceTemplateCompletes(t *testing.T) {
	t.Parallel()
	cs, _ := connect(t)
	call(t, cs, ToolRun, map[string]any{"code": "doc.Sketch.Circle(V(0, 0, 0), 5)\ndoc.Features.Extrude(3, \"disc\")"})
	for typed, want := range map[string][]string{"": {"g002-sketch-in-plane", "g003-disc"}, "di": {"g003-disc"}, "x": {}} {
		if got := resourceCompletions(t, cs, typed); !slices.Equal(got, want) {
			t.Errorf("a group's name from %q: %v, want %v", typed, got, want)
		}
	}
	res, err := cs.Complete(context.Background(), &mcp.CompleteParams{
		Ref:      &mcp.CompleteReference{Type: "ref/resource", URI: ResourceDrawing},
		Argument: mcp.CompleteParamsArgument{Name: "name", Value: ""}})
	if err != nil || len(res.Completion.Values) != 0 {
		t.Errorf("a resource with no argument completes as %v, %v", res, err)
	}
}
