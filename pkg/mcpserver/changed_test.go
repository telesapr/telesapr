package mcpserver

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// firstText is the text of a result's first item, "" if it isn't text.
func firstText(res *mcp.CallToolResult) string {
	if len(res.Content) == 0 {
		return ""
	}
	tc, _ := res.Content[0].(*mcp.TextContent)
	if tc == nil {
		return ""
	}
	return tc.Text
}

// A change made in the drawing behind the server's back is told at the
// next call that reads the drawing, once; the model's own scripts, one
// rolled back, and a file it opened are not; another file opened
// behind its back is said to be another drawing.
func TestChangedNotice(t *testing.T) {
	t.Parallel()
	cs, backend := connect(t)
	doc := backend.Engine().Doc()
	notice := func(what string, tool string, args map[string]any, want string) {
		t.Helper()
		res, _ := call(t, cs, tool, args)
		if got := firstText(res); (want == "") == (strings.HasPrefix(got, "The drawing was changed") || strings.HasPrefix(got, "Another drawing")) ||
			(want != "" && got != want) {
			t.Errorf("%s: %s starts with %q, want %q", what, tool, got, want)
		}
	}
	none := ""

	notice("the first call", ToolDescribe, nil, none)
	notice("the model's script", ToolRun, map[string]any{"code": "doc.Sketch.Circle(V(0, 0, 0), 5)"}, none)
	notice("after the model's script", ToolMeasure, nil, none)
	notice("a script rolled back", ToolRun, map[string]any{"code": "doc.Sketch.Circle(V(0, 0, 0), 3)\nundefined()"}, none)
	notice("after a script rolled back", ToolDescribe, nil, none)

	doc.Sketch.Line(api.V(0, 0, 0), api.V(10, 0, 0))
	res, _ := call(t, cs, ToolRun, map[string]any{"code": "1 + 1"})
	if got := firstText(res); got != changedNotice {
		t.Errorf("a script after a change behind its back starts with %q", got)
	}
	var out RunOutput
	structured(t, res, &out)
	if !out.DrawingChanged {
		t.Errorf("the structured result doesn't say the drawing changed: %+v", out)
	}
	res, _ = call(t, cs, ToolRun, map[string]any{"code": "2 + 2"})
	if got := firstText(res); got == changedNotice {
		t.Error("the change is told twice")
	}
	out = RunOutput{}
	if structured(t, res, &out); out.DrawingChanged {
		t.Errorf("the second script's structured result says the drawing changed: %+v", out)
	}

	doc.Sketch.Line(api.V(0, 0, 0), api.V(0, 10, 0))
	notice("get_drawing after a change", ToolDescribe, nil, changedNoticeDescribe)
	// list_files and api_help don't read the drawing, and leave the
	// notice to the next call that does.
	doc.Sketch.Line(api.V(0, 0, 0), api.V(5, 5, 0))
	notice("api_help after a change", ToolHelp, map[string]any{"name": "Group"}, none)
	notice("list_files after a change", ToolFiles, nil, none)
	notice("export after a change", ToolExport, map[string]any{"path": filepath.Join(t.TempDir(), "drawing.go")}, changedNotice)

	file := filepath.Join(t.TempDir(), "drawing.tlspr")
	if err := doc.File.Save(file); err != nil {
		t.Fatal(err)
	}
	notice("after a save", ToolMeasure, nil, none)
	notice("the model's open", ToolOpen, map[string]any{"path": file}, none)
	notice("after the model's open", ToolDescribe, nil, none)
	if err := doc.File.Load(file); err != nil {
		t.Fatal(err)
	}
	notice("a file opened behind its back", ToolMeasure, nil, anotherNotice)
	if err := doc.File.Load(file); err != nil {
		t.Fatal(err)
	}
	notice("get_drawing after a file opened", ToolDescribe, nil, anotherNoticeDescribe)
}
