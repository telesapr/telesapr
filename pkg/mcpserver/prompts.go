package mcpserver

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/telesapr/telesapr/pkg/script/agent"
)

// The names of the prompts.
const (
	PromptDraw        = "draw"
	PromptCheck       = "check"
	PromptExplain     = "explain"
	PromptParametrize = "parametrize"
	PromptSheet       = "sheet"
)

// The descriptions of the prompts and their arguments, as a user reads
// them in a client's menu.
const (
	drawDescription        = `Draws what you describe in the open drawing: sketches held by dimensions, the features on them, and a picture to check it by.`
	drawWhatDescription    = `what to draw, with its sizes, as "a plate 60 by 20 by 4 mm with three 5 mm holes"; it may as well be said in the next message`
	checkDescription       = `Reports what is wrong with the open drawing: sketches not held, constraints that conflict or say a thing twice, groups that made no solid, a solid that isn't sound. It changes nothing.`
	checkGroupDescription  = `a group to look at closely, by its name or as g003; the active group if left out`
	explainDescription     = `Says how the open drawing is built, group by group, from the script that builds it and a picture of it. It changes nothing.`
	parametrizeDescription = `Names the sizes of the open drawing that matter as parameters and puts its dimensions on them, keeping the shape.`
	sheetDescription       = `Makes a drawing sheet of the solid: the views, a section where it helps, and the main dimensions.`
	sheetPaperDescription  = `the sheet: A4, A3, A2, A1, A0, Letter or Tabloid, and portrait if it is to stand; the one that fits if left out`
)

// The instructions the prompts give the model. The ones with a %s take
// the prompt's argument.
const (
	// drawWhat is said of the user's words, which a client may have
	// cut at the first space; drawAsk when there are none.
	drawWhat = ` They wrote, with the prompt: "%s". If that is not the whole of it (a word or two, cut short), take what to draw from their next message, or ask.`
	drawAsk  = ` Take what to draw from their next message; if there is none, ask what to draw.`

	drawInstruction = `Draw in TeleSAPR what the user asks for.%s

The primer of the scripting interface (what api_help gives with no name) and the drawing as it is now (what get_drawing gives) follow: there is no need to call those first.

Work by the instructions: write a script, run it with run_script, and read the status it returns. Hold each sketch with dimensions and a point on the origin until it has 0 DOF, see that it is "solved", and that no group has a problem. Then call render and look at the picture before saying it is done. If something is not as asked, mend it with another script.

Draw what was asked for and no more. Leave what is already in the drawing as it is. Do not save or export.`

	checkInstruction = `Check the drawing that is open in TeleSAPR%s, and report what is wrong with it. The drawing (what get_drawing gives) and the measurement of its solid (what measure gives) follow.

Look for:
- sketches that are not held: more than 0 DOF. Say which points or curves can still move ("Entity.Free" tells, as doc.Request("r004").Entity().Free() does) and what dimension or constraint would hold them.
- sketches that do not solve: the constraints that conflict (g.Conflicts()) or say the same thing twice (g.Redundancies()). Say which one to take away or change.
- groups with a problem, which made no solid, and why.
- a solid that is not sound (not closed, crossing itself, a Boolean that failed): which faces and edges, and what in the model makes them.

For each, say where it is, by the names the description gives, and what would mend it. If nothing is wrong, say so. Change nothing: a script may read the drawing (doc.Group("g002").Describe(), g.Conflicts()), but nothing it runs may change it.`

	explainInstruction = `Explain how the drawing that is open in TeleSAPR is built. The drawing as the Go script that builds it again (what get_drawing gives with script) follows. Call render once, and look at the picture, before you begin.

Go group by group, in the order the model is built: what each sketch draws and how its dimensions and constraints hold it; what each feature makes of it (extrude, lathe, fillet, ...) and how it combines with the solid before it; which parameters drive which sizes. Name the groups as the script does. Then say in a sentence or two what the part is and what its main sizes are.

Change nothing.`

	parametrizeInstruction = `Make the drawing that is open in TeleSAPR driven by parameters. The drawing (what get_drawing gives, with the parameters it has) and the measurement of its solid (what measure gives) follow.

Find the sizes that matter to the part: its length, width, height, wall thickness, the diameters of holes, their spacing. Give each a parameter with a short name and its value now, by doc.Params.Set("wall", "2"), and put the dimension on it with its expression, as doc.Constraint("c00a").SetExpr("wall"); a size made of others is an expression of them ("wall*4"). An extrusion's depth that the group holds takes one by g.SetExpr(sketch.GroupExtent, "height"), as do a step and repeat's copies (sketch.GroupCopies). Use the parameters that are there before making new ones.

Keep the shape: every value is the one it had, so nothing moves. Read the status after each script: every sketch stays solved with the DOF it had. Say the volume before (in what follows) and after (measure, or doc.Active().Analyze().Volume()); they are to be the same. Then list the parameters, with what each drives.`

	sheetInstruction = `Make a drawing sheet of the solid in the drawing that is open in TeleSAPR%s. The drawing (what get_drawing gives) and the faces and edges of its solid (what get_drawing gives with solid) follow.

The sheet is doc.Drawing.Get(), changed and given back with doc.Drawing.Set: Sheet names the paper and Portrait turns it, a Scale of 0 fits the views, and no Views places the standard ones. Put on the main dimensions: the overall sizes, the holes' diameters, the thickness of walls. doc.Drawing.AddDimension measures edges (length, distance, diameter, radius), doc.Drawing.AddFaceDimension faces and doc.Drawing.AddEdgeFaceDistance an edge from a face, each on a view (front, top, right, ...) by the names the solid's description gives, as g.Solid().Edge and g.Solid().Face take them. Put each size on the view where it shows best, and each once. Where a hole or a pocket is hidden, add a section with doc.Drawing.AddSection.

Then check each dimension with doc.Drawing.Dimensions(): every one has a value and no error. Change nothing of the model. Do not export the sheet unless asked.`
)

// promptText is a user message of text.
func promptText(s string) *mcp.PromptMessage {
	return &mcp.PromptMessage{Role: "user", Content: &mcp.TextContent{Text: s}}
}

// carrier gathers what a prompt carries from the back end. A call that
// fails leaves the error's sentence in place of what it would have
// carried, so the prompt still comes back and the model can say why;
// the same error again is said once.
type carrier struct {
	ctx  context.Context
	b    agent.Backend
	msgs []*mcp.PromptMessage
	errs []string
}

// failed puts the error of getting what, by tool, in the messages.
func (c *carrier) failed(what, tool string, err error) {
	if slices.Contains(c.errs, err.Error()) {
		c.msgs = append(c.msgs, promptText(fmt.Sprintf("%s (%s) could not be got either, for the same reason.", what, tool)))
		return
	}
	c.errs = append(c.errs, err.Error())
	c.msgs = append(c.msgs, promptText(fmt.Sprintf("%s (%s) could not be got: %v", what, tool, err)))
}

// carry puts text in the messages, said to be what tool gives.
func (c *carrier) carry(what, tool, text string) {
	c.msgs = append(c.msgs, promptText(fmt.Sprintf("%s, as %s gives it:\n\n%s", what, tool, text)))
}

func (c *carrier) primer() {
	s, err := c.b.Help(c.ctx, "")
	if err != nil {
		c.failed("The primer", ToolHelp, err)
		return
	}
	c.carry("The primer", ToolHelp, s)
}

func (c *carrier) describe(what string, d agent.Describing) {
	tool := ToolDescribe
	switch {
	case d.Script:
		tool += " with script"
	case d.Solid:
		tool += " with solid"
	case d.Group != "":
		tool += " with group " + d.Group
	}
	got, err := c.b.Describe(c.ctx, d)
	if err != nil {
		c.failed(what, tool, err)
		return
	}
	c.carry(what, tool, got.Text)
}

func (c *carrier) measure(group string) {
	tool := ToolMeasure
	if group != "" {
		tool += " with group " + group
	}
	got, err := c.b.Measure(c.ctx, agent.Measuring{Group: group})
	if err != nil {
		c.failed("The measurement of the solid", tool, err)
		return
	}
	c.carry("The measurement of the solid", tool, got.Text)
}

// result is the prompt's messages, the instruction first, or the
// context's error if the client gave up meanwhile.
func (c *carrier) result(description, instruction string) (*mcp.GetPromptResult, error) {
	if err := c.ctx.Err(); err != nil {
		return nil, err
	}
	return &mcp.GetPromptResult{Description: description,
		Messages: append([]*mcp.PromptMessage{promptText(instruction)}, c.msgs...)}, nil
}

// argument is the prompt's argument name, trimmed.
func argument(req *mcp.GetPromptRequest, name string) string {
	return strings.TrimSpace(req.Params.Arguments[name])
}

// addPrompts offers b's drawing to s through the prompts.
func addPrompts(s *mcp.Server, b agent.Backend) {
	s.AddPrompt(&mcp.Prompt{
		Name: PromptDraw, Title: "Draw a part", Description: drawDescription,
		Arguments: []*mcp.PromptArgument{{Name: "what", Title: "What to draw", Description: drawWhatDescription}},
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		// what is not required: a client may cut what the user typed
		// after the command at its spaces, or pass none, and the user
		// then says it in the next message.
		ask := drawAsk
		if what := argument(req, "what"); what != "" {
			ask = fmt.Sprintf(drawWhat, what)
		}
		c := &carrier{ctx: ctx, b: b}
		c.primer()
		c.describe("The drawing", agent.Describing{})
		return c.result(drawDescription, fmt.Sprintf(drawInstruction, ask))
	})

	s.AddPrompt(&mcp.Prompt{
		Name: PromptCheck, Title: "Check the drawing", Description: checkDescription,
		Arguments: []*mcp.PromptArgument{{Name: "group", Title: "Group", Description: checkGroupDescription}},
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		group := argument(req, "group")
		c := &carrier{ctx: ctx, b: b}
		c.describe("The drawing", agent.Describing{})
		where := ""
		if group != "" {
			where = ", above all the group " + group
			c.describe("The group "+group, agent.Describing{Group: group})
		}
		c.measure(group)
		return c.result(checkDescription, fmt.Sprintf(checkInstruction, where))
	})

	s.AddPrompt(&mcp.Prompt{
		Name: PromptExplain, Title: "Explain the drawing", Description: explainDescription,
	}, func(ctx context.Context, _ *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		c := &carrier{ctx: ctx, b: b}
		// The picture is left to the model's call of render: a picture
		// in a prompt is not shown by every client.
		c.describe("The drawing as a script", agent.Describing{Script: true})
		return c.result(explainDescription, explainInstruction)
	})

	s.AddPrompt(&mcp.Prompt{
		Name: PromptParametrize, Title: "Put the sizes on parameters", Description: parametrizeDescription,
	}, func(ctx context.Context, _ *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		c := &carrier{ctx: ctx, b: b}
		c.describe("The drawing", agent.Describing{})
		c.measure("")
		return c.result(parametrizeDescription, parametrizeInstruction)
	})

	s.AddPrompt(&mcp.Prompt{
		Name: PromptSheet, Title: "Make a drawing sheet", Description: sheetDescription,
		Arguments: []*mcp.PromptArgument{{Name: "paper", Title: "Paper", Description: sheetPaperDescription}},
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		on := ""
		if paper := argument(req, "paper"); paper != "" {
			on = ", on " + paper + " paper"
		}
		c := &carrier{ctx: ctx, b: b}
		c.describe("The drawing", agent.Describing{})
		c.describe("The faces and edges of the solid", agent.Describing{Solid: true})
		return c.result(sheetDescription, fmt.Sprintf(sheetInstruction, on))
	})
}

// maxCompletions is the most values a completion holds.
const maxCompletions = 100

// sheets are the papers a drawing sheet is on (sketch.SheetSize).
var sheets = []string{"A4", "A3", "A2", "A1", "A0", "Letter", "Tabloid"}

// complete offers the values of the prompts' and the resource
// templates' arguments that start as the client's: the groups of b's
// drawing for check's group and a group resource's name, the sheets for
// sheet's paper, and nothing for the rest.
func complete(b agent.Backend) func(context.Context, *mcp.CompleteRequest) (*mcp.CompleteResult, error) {
	return func(ctx context.Context, req *mcp.CompleteRequest) (*mcp.CompleteResult, error) {
		p := req.Params
		var all []string
		switch {
		case p.Ref.Type == "ref/resource" && p.Ref.URI == groupTemplate && p.Argument.Name == "name",
			p.Ref.Type == "ref/prompt" && p.Ref.Name == PromptCheck && p.Argument.Name == "group":
			// No drawing to read, as with no window running, is no completion.
			if d, err := b.Describe(ctx, agent.Describing{}); err == nil {
				for _, g := range d.Data.Groups {
					all = append(all, g.Name)
				}
			}
		case p.Ref.Type == "ref/prompt" && p.Ref.Name == PromptSheet && p.Argument.Name == "paper":
			all = sheets
		}
		typed := strings.ToLower(p.Argument.Value)
		values := []string{}
		for _, v := range all {
			// A group is g003-plate: typing "pl" finds it too.
			lower := strings.ToLower(v)
			_, name, _ := strings.Cut(lower, "-")
			if strings.HasPrefix(lower, typed) || strings.HasPrefix(name, typed) {
				values = append(values, v)
			}
		}
		// The protocol has a completion hold at most 100 values.
		total := len(values)
		return &mcp.CompleteResult{Completion: mcp.CompletionResultDetails{
			Values: values[:min(total, maxCompletions)], Total: total, HasMore: total > maxCompletions}}, nil
	}
}
