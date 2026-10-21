package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"reflect"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/telesapr/telesapr/pkg/script"
	"github.com/telesapr/telesapr/pkg/script/agent"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// painter is a session whose pictures are made here: a sketch with no
// program around it has none.
type painter struct {
	*agent.Session
	asked struct {
		view, group   string
		width, height int
	}
}

func (p *painter) Render(_ context.Context, view, group string, width, height int) ([]byte, error) {
	p.asked.view, p.asked.group, p.asked.width, p.asked.height = view, group, width, height
	if view == "inside" {
		return nil, errors.New("no view 'inside'")
	}
	img := image.NewRGBA(image.Rect(0, 0, 4, 3))
	img.Set(1, 1, color.RGBA{R: 200, A: 255})
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// connect is a client's session with a server on a new sketch.
func connect(t *testing.T) (*mcp.ClientSession, *painter) {
	t.Helper()
	sk := sketch.NewSketch()
	sk.NewFile()
	session, _, err := agent.Open(script.Options{Host: api.NewSketchHost(sk, "")})
	if err != nil {
		t.Fatal(err)
	}
	backend := &painter{Session: session}
	ctx := context.Background()
	clientSide, serverSide := mcp.NewInMemoryTransports()
	served, err := New(backend, "1.2").Connect(ctx, serverSide, nil)
	if err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := client.Connect(ctx, clientSide, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cs.Close()
		_ = served.Wait()
	})
	return cs, backend
}

// call calls a tool, and returns its result with the text of it.
func call(t *testing.T, cs *mcp.ClientSession, tool string, args map[string]any) (*mcp.CallToolResult, string) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", tool, err)
	}
	var parts []string
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}
	return res, strings.Join(parts, "\n")
}

// A client is told what the server is for, and offered the four
// tools, each described within what clients keep.
func TestToolsAndInstructions(t *testing.T) {
	t.Parallel()
	cs, _ := connect(t)
	init := cs.InitializeResult()
	if init.Instructions != agent.Instructions || init.ServerInfo.Name != "telesapr" || init.ServerInfo.Version != "1.2" {
		t.Errorf("the server presents itself as %+v with instructions %q", init.ServerInfo, init.Instructions)
	}
	list, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	readOnly := map[string]bool{ToolRun: false, ToolDescribe: true, ToolRender: true, ToolHelp: true}
	if len(list.Tools) != len(readOnly) {
		t.Errorf("%d tools are offered, want %d", len(list.Tools), len(readOnly))
	}
	for _, tool := range list.Tools {
		want, known := readOnly[tool.Name]
		if !known {
			t.Errorf("a tool %q is offered", tool.Name)
			continue
		}
		if tool.Annotations == nil || tool.Annotations.ReadOnlyHint != want {
			t.Errorf("%s: read only is %v, want %v", tool.Name, tool.Annotations, want)
		}
		if n := len(tool.Description); n == 0 || n > agent.MaxInstructions {
			t.Errorf("%s: the description is %d bytes, want some and at most %d", tool.Name, n, agent.MaxInstructions)
		}
		schema, _ := tool.InputSchema.(map[string]any)
		if schema["type"] != "object" {
			t.Errorf("%s: the input's schema is %v", tool.Name, tool.InputSchema)
		}
	}
	// Each tool the instructions name is one there is.
	for name := range readOnly {
		if !strings.Contains(agent.Instructions, name) { //nolint:gocritic // the instructions are the haystack
			t.Errorf("the instructions don't say when to call %s", name)
		}
	}
}

func TestRunScript(t *testing.T) {
	t.Parallel()
	cs, _ := connect(t)
	res, text := call(t, cs, ToolRun, map[string]any{
		"code": "c := doc.Sketch.Circle(V(0, 0, 0), 5)\nfmt.Println(\"drawn\")\nc.Radius()",
	})
	want := "output:\ndrawn\nvalue: 5\nstatus:\nactive group g002-sketch-in-plane, sketch-in-plane: solved, 3 DOF\n"
	if res.IsError || text != want {
		t.Errorf("the script's result is an error (%v) or reads\n%s\nwant\n%s", res.IsError, text, want)
	}

	// A script that fails is the tool's error, with what became of
	// the drawing.
	res, text = call(t, cs, ToolRun, map[string]any{"code": "doc.Features.Extrude(2, \"disc\")\nc.Length()\nc.Start()"})
	if !res.IsError || !strings.Contains(text, "error: script:3:") || !strings.Contains(text, "taken back") ||
		!strings.Contains(text, "active group g002-sketch-in-plane") {
		t.Errorf("the failing script's result is no error (%v) or reads\n%s", !res.IsError, text)
	}
	if res, text = call(t, cs, ToolRun, map[string]any{"code": "nothing()"}); !res.IsError || !strings.Contains(text, "changed nothing") {
		t.Errorf("a script that doesn't compile: error %v, text\n%s", res.IsError, text)
	}
	if res, text = call(t, cs, ToolRun, map[string]any{"code": " "}); !res.IsError || !strings.Contains(text, "no code to run") {
		t.Errorf("no code: error %v, text %q", res.IsError, text)
	}
	// What the schema doesn't allow never reaches the drawing.
	bad, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: ToolRun, Arguments: map[string]any{"source": "doc.Sketch.Point(V(1, 1, 0))"}})
	if err == nil && !bad.IsError {
		t.Errorf("a call without code was taken")
	}
	if _, text = call(t, cs, ToolDescribe, nil); strings.Contains(text, "r005") {
		t.Errorf("a call without code drew something:\n%s", text)
	}
}

func TestDescribeHelpRender(t *testing.T) {
	t.Parallel()
	cs, backend := connect(t)
	call(t, cs, ToolRun, map[string]any{"code": "doc.Sketch.Circle(V(0, 0, 0), 5)\ndoc.Features.Extrude(3, \"disc\")"})

	res, text := call(t, cs, ToolDescribe, nil)
	if res.IsError || !strings.HasPrefix(text, "sketch: unit mm") || !strings.Contains(text, "g003-disc, extrude: solved") {
		t.Errorf("the drawing is described as\n%s", text)
	}
	if res, text = call(t, cs, ToolDescribe, map[string]any{"group": "g002"}); res.IsError || !strings.Contains(text, "r004-circle: circle at (0, 0, 0), radius 5") {
		t.Errorf("the sketch is described as\n%s", text)
	}
	if res, text = call(t, cs, ToolDescribe, map[string]any{"group": "disk"}); !res.IsError || !strings.Contains(text, "g003-disc") {
		t.Errorf("a group that isn't there: error %v, text %q", res.IsError, text)
	}

	if res, text = call(t, cs, ToolHelp, nil); res.IsError || text != agent.Primer() {
		t.Errorf("help with no name isn't the primer")
	}
	if res, text = call(t, cs, ToolHelp, map[string]any{"name": "Group.SetCombine"}); res.IsError || !strings.Contains(text, "func (g Group) SetCombine(") {
		t.Errorf("help of Group.SetCombine reads %q", text)
	}
	if res, _ = call(t, cs, ToolHelp, map[string]any{"name": "Group.NoSuchCall"}); !res.IsError {
		t.Errorf("help of a name nothing has is no error")
	}

	res, _ = call(t, cs, ToolRender, map[string]any{"view": "front", "group": "g002", "width": 320, "height": 200})
	if res.IsError || len(res.Content) != 1 {
		t.Fatalf("the picture's result is %+v", res)
	}
	img, ok := res.Content[0].(*mcp.ImageContent)
	if !ok || img.MIMEType != "image/png" {
		t.Fatalf("the picture is %+v", res.Content[0])
	}
	decoded, err := png.Decode(bytes.NewReader(img.Data))
	if err != nil || decoded.Bounds().Dx() != 4 {
		t.Errorf("the picture doesn't decode as the PNG that was made: %v", err)
	}
	if backend.asked.view != "front" || backend.asked.group != "g002" || backend.asked.width != 320 || backend.asked.height != 200 {
		t.Errorf("the picture asked for was %+v", backend.asked)
	}
	if call(t, cs, ToolRender, nil); backend.asked.view != "" || backend.asked.width != 0 {
		t.Errorf("a picture with nothing said was asked for as %+v, want the backend's defaults", backend.asked)
	}
	if res, text = call(t, cs, ToolRender, map[string]any{"view": "inside"}); !res.IsError || !strings.Contains(text, "no view") {
		t.Errorf("a picture from a view there isn't: error %v, text %q", res.IsError, text)
	}
}

func TestRunText(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		res  agent.Result
		want string
	}{
		{agent.Result{}, ""},
		{agent.Result{Output: "no newline", Status: "s"}, "output:\nno newline\nstatus:\ns\n"},
		{agent.Result{Error: "1:1: bad"}, "error: 1:1: bad\nThe script changed nothing in the drawing.\n"},
	} {
		if got := RunText(c.res); got != c.want {
			t.Errorf("%+v reads %q, want %q", c.res, got, c.want)
		}
	}
}

// A sketch that crosses itself makes no solid, and the status and the
// drawing say why and where (roadmap 23.1).
func TestSketchProblemShown(t *testing.T) {
	t.Parallel()
	cs, _ := connect(t)
	res, text := call(t, cs, ToolRun, map[string]any{
		"code": "doc.Sketch.Polyline(true, V(0, 0, 0), V(10, 10, 0), V(10, 0, 0), V(0, 10, 0))\ndoc.Features.Extrude(5, \"bow\")",
	})
	if res.IsError || !strings.Contains(text, "active group g003-bow, extrude: solved, 1 DOF, problem: the sketch g002-sketch-in-plane crosses itself at (5, 5, 0)") ||
		!strings.Contains(text, "in trouble: g002-sketch-in-plane, sketch-in-plane: solved, 8 DOF, problem: the sketch crosses itself at (5, 5, 0)") {
		t.Errorf("the status reads\n%s", text)
	}
	// The same, for a program that reads the structured result.
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var out RunOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	want := []agent.GroupStatus{
		{Name: "bow", Kind: "extrude", Active: true, Solved: true, DOF: 1, Problem: "the sketch g002-sketch-in-plane crosses itself at (5, 5, 0)"},
		{Name: "sketch-in-plane", Kind: "sketch-in-plane", Solved: true, DOF: 8, Problem: "the sketch crosses itself at (5, 5, 0)"},
	}
	if out.Error != "" || !reflect.DeepEqual(out.Groups, want) {
		t.Errorf("the structured result is %s", raw)
	}
	if _, text = call(t, cs, ToolDescribe, nil); !strings.Contains(text, "problem: the sketch g002-sketch-in-plane crosses itself") {
		t.Errorf("the drawing is described as\n%s", text)
	}
	if _, text = call(t, cs, ToolRun, map[string]any{"code": "doc.Active().Analyze().Check().Sound()"}); !strings.Contains(text, "value: false") {
		t.Errorf("the solid that isn't there is sound:\n%s", text)
	}
}

// run_script's form gives the fields of the form the code declares
// their values; one that doesn't fit stops the script before it runs.
func TestRunScriptForm(t *testing.T) {
	t.Parallel()
	cs, _ := connect(t)
	code := "var form = Form{Fields: []Field{{Name: \"r\", Value: 5.0}}}\ndoc.Sketch.Circle(V(0, 0, 0), form.Float(\"r\")).Radius()"
	if _, text := call(t, cs, ToolRun, map[string]any{"code": code}); !strings.Contains(text, "value: 5\n") {
		t.Errorf("as written:\n%s", text)
	}
	if _, text := call(t, cs, ToolRun, map[string]any{"code": code, "form": map[string]any{"r": "7.5"}}); !strings.Contains(text, "value: 7.5\n") {
		t.Errorf("with r 7.5:\n%s", text)
	}
	res, text := call(t, cs, ToolRun, map[string]any{"code": code, "form": map[string]any{"r": "big"}})
	if !res.IsError || !strings.Contains(text, "r is a number") || !strings.Contains(text, "did not run") {
		t.Errorf("with r big: error %v,\n%s", res.IsError, text)
	}
	var out RunOutput
	if b, err := json.Marshal(res.StructuredContent); err != nil || json.Unmarshal(b, &out) != nil || !strings.Contains(out.Error, "r is a number") {
		t.Errorf("the structured result is %v", res.StructuredContent)
	}
}
