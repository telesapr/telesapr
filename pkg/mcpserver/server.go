// Package mcpserver offers a drawing to a Model Context Protocol
// client, such as an AI model's program, as four tools: run a script,
// describe the drawing, render a picture of it, and the scripting
// interface's help. What the tools do is an agent.Backend's; this
// package is the protocol around it.
package mcpserver

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/telesapr/telesapr/pkg/script/agent"
)

// The names of the tools.
const (
	ToolRun      = "run_script"
	ToolDescribe = "get_drawing"
	ToolRender   = "render"
	ToolHelp     = "api_help"
)

// The descriptions of the tools, as a model reads them. Clients cut
// long ones short: each stays under agent.MaxInstructions.
const (
	runDescription = `Runs a Go script on the drawing that is open in TeleSAPR, and returns what it printed, the value of its last expression, and how the drawing is afterwards (the active group, whether it solved, its degrees of freedom, and any group in trouble).

The code is statements, as in a function's body: doc is the drawing, V(x, y, z) a point in mm, and fmt, math and the like need no import. Variables stay defined for the next call. A whole program with "package main" also runs, but sees no such variables.

Each call is one step of undo for the user. If the script stops at an error, everything it changed is taken back and the drawing is as before the call: correct the script and send all of it again.

Call api_help with no name before the first script.`

	describeDescription = `Returns the drawing that is open in TeleSAPR, in words: the unit, the parameters, every group in the order the model is built with how it solved, its degrees of freedom, what is wrong with it, and the volume and bounds of the solid up to it; then the requests of the active group with where they are, in mm, its constraints with their values and what they are on, and what the user has selected.

The names it gives (g003-plate, r004-line-segment, c00a-pt-pt-distance) are the ones doc.Group, doc.Request and doc.Constraint take in a script. With group, it lists the requests and constraints of that group instead of the active one's.`

	renderDescription = `Returns a picture (PNG) of the drawing that is open in TeleSAPR, fitted to the picture: the solid shaded, with the sketch of the active group and its dimensions. Look at it to check that what was drawn is what was asked for.

The view is "isometric" (the default), "front" (the XY plane, where a new drawing's first sketch is: x to the right, y up), "back", "top", "bottom", "left" or "right", fitted; or "window", the view the user has in the window, as they see it. With group, the drawing is drawn as it is up to that group, as if it were active.`

	helpDescription = `Returns the documentation of TeleSAPR's scripting interface. With no name it is the primer: how scripts and drawings work, what the interpreter can't do, an example, and every call there is; read it once before writing the first script. With a name it is the documentation of that type, method or function: "Doc.Extrude", "Group", "Group.SetCombine", "Entity", "ThreadOptions", "math.Hypot".`
)

// RunInput is what run_script takes.
type RunInput struct {
	Code string `json:"code" jsonschema:"Go statements to run on the drawing, or a whole Go program"`
}

// DescribeInput is what get_drawing takes.
type DescribeInput struct {
	Group string `json:"group,omitempty" jsonschema:"a group whose requests and constraints to list, by its name or as g003; the whole drawing if left out"`
}

// RenderInput is what render takes.
type RenderInput struct {
	View   string `json:"view,omitempty" jsonschema:"isometric (the default), front, back, top, bottom, left, right, or window for what the user sees"`
	Group  string `json:"group,omitempty" jsonschema:"a group to draw the drawing up to, as if it were active, by its name or as g003; the active group if left out"`
	Width  int    `json:"width,omitempty" jsonschema:"the picture's width in pixels, 800 if left out"`
	Height int    `json:"height,omitempty" jsonschema:"the picture's height in pixels, 600 if left out"`
}

// HelpInput is what api_help takes.
type HelpInput struct {
	Name string `json:"name,omitempty" jsonschema:"a type, method or function, as Doc.Extrude or Group; the primer if left out"`
}

// text is a tool's result that is text.
func text(s string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: s}}}
}

// RunText is the result of a script as a model reads it.
func RunText(res agent.Result) string {
	var b strings.Builder
	if res.Output != "" {
		b.WriteString("output:\n" + res.Output)
		if !strings.HasSuffix(res.Output, "\n") {
			b.WriteString("\n")
		}
	}
	if res.Value != "" {
		b.WriteString("value: " + res.Value + "\n")
	}
	if res.Error != "" {
		b.WriteString("error: " + res.Error + "\n")
		if res.RolledBack {
			b.WriteString("What the script changed was taken back: the drawing is as it was before this call. Variables it defined before the error are still defined, but what they drew is gone.\n")
		} else {
			b.WriteString("The script changed nothing in the drawing.\n")
		}
	}
	if res.Status != "" {
		b.WriteString("status:\n" + res.Status + "\n")
	}
	return b.String()
}

// New is an MCP server that offers b's drawing; version is the
// program's.
func New(b agent.Backend, version string) *mcp.Server {
	s := mcp.NewServer(
		&mcp.Implementation{Name: "telesapr", Title: "TeleSAPR", Version: version},
		&mcp.ServerOptions{Instructions: agent.Instructions},
	)
	closed := false

	mcp.AddTool(s, &mcp.Tool{
		Name:        ToolRun,
		Title:       "Run a script on the drawing",
		Description: runDescription,
		Annotations: &mcp.ToolAnnotations{OpenWorldHint: &closed},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in RunInput) (*mcp.CallToolResult, any, error) {
		res, err := b.Run(ctx, in.Code)
		if err != nil {
			return nil, nil, err
		}
		out := text(RunText(res))
		out.IsError = res.Error != ""
		return out, nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        ToolDescribe,
		Title:       "Describe the drawing",
		Description: describeDescription,
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closed},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in DescribeInput) (*mcp.CallToolResult, any, error) {
		s, err := b.Describe(ctx, in.Group)
		if err != nil {
			return nil, nil, err
		}
		return text(s), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        ToolRender,
		Title:       "Render a picture of the drawing",
		Description: renderDescription,
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closed},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in RenderInput) (*mcp.CallToolResult, any, error) {
		png, err := b.Render(ctx, in.View, in.Group, in.Width, in.Height)
		if err != nil {
			return nil, nil, err
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.ImageContent{Data: png, MIMEType: "image/png"}}}, nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        ToolHelp,
		Title:       "Help on the scripting interface",
		Description: helpDescription,
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closed},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in HelpInput) (*mcp.CallToolResult, any, error) {
		s, err := b.Help(ctx, in.Name)
		if err != nil {
			return nil, nil, err
		}
		return text(s), nil, nil
	})
	return s
}
