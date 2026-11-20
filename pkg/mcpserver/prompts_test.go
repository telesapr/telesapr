package mcpserver

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/telesapr/telesapr/pkg/script/agent"
)

// prompt gets a prompt, and returns the text of each of its messages,
// all of them the user's.
func prompt(t *testing.T, cs *mcp.ClientSession, name string, args map[string]string) []string {
	t.Helper()
	res, err := cs.GetPrompt(context.Background(), &mcp.GetPromptParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	var texts []string
	for _, m := range res.Messages {
		c, ok := m.Content.(*mcp.TextContent)
		if m.Role != "user" || !ok {
			t.Fatalf("%s: a message of %s is %T", name, m.Role, m.Content)
		}
		texts = append(texts, c.Text)
	}
	if res.Description == "" || len(texts) < 2 {
		t.Fatalf("%s: described %q, with %d messages", name, res.Description, len(texts))
	}
	return texts
}

// The five prompts are offered, each with a title and a description a
// user reads, and at most one argument, none of them required.
func TestPromptsListed(t *testing.T) {
	t.Parallel()
	cs, _ := connect(t)
	list, err := cs.ListPrompts(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{PromptDraw: "what", PromptCheck: "group", PromptExplain: "",
		PromptParametrize: "", PromptSheet: "paper"}
	if len(list.Prompts) != len(want) {
		t.Errorf("%d prompts are offered, want %d", len(list.Prompts), len(want))
	}
	for _, p := range list.Prompts {
		arg, known := want[p.Name]
		if !known {
			t.Errorf("a prompt %q is offered", p.Name)
			continue
		}
		if p.Title == "" || p.Description == "" {
			t.Errorf("%s: title %q, description %q", p.Name, p.Title, p.Description)
		}
		var got string
		for _, a := range p.Arguments {
			got += a.Name
			if a.Required || a.Description == "" {
				t.Errorf("%s: the argument %+v", p.Name, a)
			}
		}
		if got != arg {
			t.Errorf("%s takes %q, want %q", p.Name, got, arg)
		}
	}
	if _, err := cs.GetPrompt(context.Background(), &mcp.GetPromptParams{Name: "nothing"}); err == nil {
		t.Errorf("a prompt there isn't came back")
	}
}

// Each prompt comes with the instruction first, then what it needs of
// the drawing as it is when the prompt is got.
func TestPromptsCarryTheDrawing(t *testing.T) {
	t.Parallel()
	cs, _ := connect(t)
	call(t, cs, ToolRun, map[string]any{"code": "doc.Sketch.Circle(V(0, 0, 0), 5)\ndoc.Features.Extrude(3, \"disc\")"})

	for _, c := range []struct {
		name string
		args map[string]string
		want [][]string // what each message holds, the instruction's first
	}{
		{PromptDraw, map[string]string{"what": "a washer 10 mm across"},
			[][]string{{"Draw in TeleSAPR", `"a washer 10 mm across"`, "0 DOF", "render"},
				{"The primer, as api_help gives it:\n\n" + agent.Primer()}, {"The drawing, as get_drawing gives it:", "g003-disc, extrude: solved"}}},
		{PromptDraw, nil, [][]string{{"next message"}, {"The primer"}, {"g003-disc"}}},
		{PromptCheck, nil, [][]string{{"Check the drawing that is open in TeleSAPR, and report", "Change nothing"},
			{"g003-disc"}, {"The measurement of the solid, as measure gives it:", "the model up to g003-disc"}}},
		{PromptCheck, map[string]string{"group": "g002"}, [][]string{{"above all the group g002"},
			{"g003-disc"}, {"The group g002, as get_drawing with group g002 gives it:", "r004-circle: circle at (0, 0, 0), radius 5"},
			{"measure with group g002 gives it:"}}},
		{PromptExplain, nil, [][]string{{"group by group", "Call render"},
			{"The drawing as a script, as get_drawing with script gives it:", "package main", `"disc"`}}},
		{PromptParametrize, nil, [][]string{{"doc.Params.Set", "SetExpr", "volume before"},
			{"g003-disc"}, {"The measurement of the solid"}}},
		{PromptSheet, map[string]string{"paper": "A4"}, [][]string{{"TeleSAPR, on A4 paper.", "doc.Drawing.AddDimension"},
			{"g003-disc"}, {"The faces and edges of the solid, as get_drawing with solid gives it:", "3 faces, 2 edges"}}},
		{PromptSheet, nil, [][]string{{"open in TeleSAPR. The drawing"}, {"g003-disc"}, {"3 faces"}}},
	} {
		texts := prompt(t, cs, c.name, c.args)
		if len(texts) != len(c.want) {
			t.Errorf("%s %v: %d messages, want %d:\n%s", c.name, c.args, len(texts), len(c.want), strings.Join(texts, "\n---\n"))
			continue
		}
		for i, parts := range c.want {
			for _, part := range parts {
				if !strings.Contains(texts[i], part) {
					t.Errorf("%s %v: message %d has no %q:\n%s", c.name, c.args, i, part, texts[i])
				}
			}
		}
	}
	// A group there isn't is said so, with the groups there are.
	texts := prompt(t, cs, PromptCheck, map[string]string{"group": "g099"})
	if !strings.Contains(texts[2], "could not be got: no group is called \"g099\"") {
		t.Errorf("checking a group there isn't:\n%s", strings.Join(texts, "\n---\n"))
	}
}

// notRunning is the window's back end with no window running.
type notRunning struct{ agent.Backend }

func (notRunning) Describe(context.Context, agent.Describing) (agent.Described, error) {
	return agent.Described{}, agent.ErrNotRunning
}

func (notRunning) Measure(context.Context, agent.Measuring) (agent.Measured, error) {
	return agent.Measured{}, agent.ErrNotRunning
}

func (notRunning) Render(context.Context, agent.Picture) (agent.Rendered, error) {
	return agent.Rendered{}, agent.ErrNotRunning
}

func (notRunning) Help(_ context.Context, name string) (string, error) { return agent.Help(name) }

// With no drawing to read, a prompt still comes back, with its
// instruction and why the drawing isn't there, said once; and no group
// is offered to complete.
func TestPromptsWithoutTheDrawing(t *testing.T) {
	t.Parallel()
	cs := connectTo(t, notRunning{})
	texts := prompt(t, cs, PromptCheck, nil)
	if len(texts) != 3 || !strings.HasPrefix(texts[0], "Check the drawing") ||
		texts[1] != "The drawing (get_drawing) could not be got: "+agent.ErrNotRunning.Error() ||
		texts[2] != "The measurement of the solid (measure) could not be got either, for the same reason." {
		t.Errorf("checking with no window:\n%s", strings.Join(texts, "\n---\n"))
	}
	if texts = prompt(t, cs, PromptDraw, nil); texts[1] != "The primer, as api_help gives it:\n\n"+agent.Primer() ||
		!strings.Contains(texts[2], "Allow AI Connections") {
		t.Errorf("drawing with no window:\n%s", strings.Join(texts, "\n---\n"))
	}
	if got := completions(t, cs, PromptCheck, "group", ""); len(got) != 0 {
		t.Errorf("the groups of no drawing are %v", got)
	}
}

// completions are the values offered for a prompt's argument.
func completions(t *testing.T, cs *mcp.ClientSession, prompt, arg, typed string) []string {
	t.Helper()
	res, err := cs.Complete(context.Background(), &mcp.CompleteParams{
		Ref:      &mcp.CompleteReference{Type: "ref/prompt", Name: prompt},
		Argument: mcp.CompleteParamsArgument{Name: arg, Value: typed}})
	if err != nil {
		t.Fatalf("completing %s's %s: %v", prompt, arg, err)
	}
	if res.Completion.Total != len(res.Completion.Values) || res.Completion.HasMore {
		t.Errorf("completing %s's %s: %+v", prompt, arg, res.Completion)
	}
	return res.Completion.Values
}

// check's group completes from the drawing's groups, by the start of
// the name or of the part after its number; sheet's paper from the
// sheets; the rest to nothing.
func TestPromptArgumentsComplete(t *testing.T) {
	t.Parallel()
	cs, _ := connect(t)
	call(t, cs, ToolRun, map[string]any{"code": "doc.Sketch.Circle(V(0, 0, 0), 5)\ndoc.Features.Extrude(3, \"disc\")"})
	for _, c := range []struct {
		prompt, arg, typed string
		want               []string
	}{
		{PromptCheck, "group", "", []string{"g002-sketch-in-plane", "g003-disc"}},
		{PromptCheck, "group", "Di", []string{"g003-disc"}},
		{PromptCheck, "group", "g002", []string{"g002-sketch-in-plane"}},
		{PromptCheck, "group", "x", []string{}},
		{PromptSheet, "paper", "a", []string{"A4", "A3", "A2", "A1", "A0"}},
		{PromptSheet, "paper", "let", []string{"Letter"}},
		{PromptDraw, "what", "a", []string{}},
	} {
		if got := completions(t, cs, c.prompt, c.arg, c.typed); !slices.Equal(got, c.want) {
			t.Errorf("%s's %s from %q: %v, want %v", c.prompt, c.arg, c.typed, got, c.want)
		}
	}
}
