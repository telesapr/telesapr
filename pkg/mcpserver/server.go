// Package mcpserver offers a drawing to a Model Context Protocol
// client, such as an AI model's program, as tools: run a script,
// describe the drawing, render a picture of it, measure it, export it,
// open a file, list a directory, and the scripting interface's help.
// What the tools do is an agent.Backend's; this package is the
// protocol around it.
package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/telesapr/telesapr/pkg/script"
	"github.com/telesapr/telesapr/pkg/script/agent"
)

// The names of the tools.
const (
	ToolRun      = "run_script"
	ToolDescribe = "get_drawing"
	ToolRender   = "render"
	ToolHelp     = "api_help"
	ToolMeasure  = "measure"
	ToolExport   = "export"
	ToolOpen     = "open"
	ToolFiles    = "list_files"
)

// The descriptions of the tools, as a model reads them. Clients cut
// long ones short: each stays under agent.MaxInstructions.
const (
	runDescription = `Runs a Go script on the drawing that is open in TeleSAPR, and returns what it printed, the value of its last expression, and how the drawing is afterwards (the active group, whether it solved, its degrees of freedom, and any group in trouble).

The code is statements, as in a function's body: doc is the drawing, V(x, y, z) a point in mm, and fmt, math and the like need no import. Variables stay defined for the next call. A whole program with "package main" also runs, but sees no such variables.

A script that declares a form (var Form = api.Form{...}, see api_help "Form") takes values for its fields from form, by field name, as the user's dialog gives them.

Each call is one step of undo for the user. If the script stops at an error, everything it changed is taken back and the drawing is as before the call: correct the script and send all of it again.

Call api_help with no name before the first script.`

	describeDescription = `Returns the drawing that is open in TeleSAPR, in words: the unit, the parameters, every group in the order the model is built with how it solved, its degrees of freedom, what is wrong with it, and the volume and bounds of the solid up to it; then the requests of the active group with where they are, in mm, its constraints with their values and what they are on, and what the user has selected.

The names it gives (g003-plate, r004-line-segment, c00a-pt-pt-distance) are the ones doc.Group, doc.Request and doc.Constraint take in a script. With group, it lists the requests and constraints of that group instead of the active one's.`

	renderDescription = `Returns a picture (PNG) of the drawing that is open in TeleSAPR, fitted to the picture: the solid shaded, with the sketch of the active group and its dimensions. Look at it to check that what was drawn is what was asked for.

The view is "isometric" (the default), "front" (the XY plane, where a new drawing's first sketch is: x to the right, y up), "back", "top", "bottom", "left" or "right", fitted; or "window", the view the user has in the window, as they see it. With group, the drawing is drawn as it is up to that group, as if it were active.

With section ("XY", "YZ" or "ZX"), the solid is drawn cut by a plane parallel to that workplane, to see inside it: section_offset moves the plane along the workplane's normal (a number or an expression, in the drawing's unit), and the side the normal points to is taken away, or the other side with section_flip. The user's own section view is left as it was.`

	measureDescription = `Measures the drawing that is open in TeleSAPR, as Analyze > Check Solid does: the volume, the area of the surface, the centre of mass and the bounding box of the solid, in mm, and whether it is sound (closed, not crossing itself, its Boolean worked), with the faces and edges in trouble if not. With group, the model as it is up to that group; the active group's if left out.

With selected, it measures what the user has selected instead, as Analyze > Measure does: a line's length, a circle's radius, the distance between two points, from a point to a line or a face, or the angle between two lines or faces. To measure other things, use run_script with doc.Analyze.`

	exportDescription = `Writes the drawing that is open in TeleSAPR to a file, in the format the file's extension names: a mesh of the solid (.stl, .3mf, .glb, .obj, .wrl), its exact surfaces (.step), a 2d view (.svg, .dxf, .pdf, .eps), a picture (.png), or a Go program that builds the model again (.go), with what it couldn't write listed. It says where it wrote and how many bytes. A relative path is taken from the directory TeleSAPR works in; list_files shows which that is.

Export only when the user asks for a file. The user may be asked to allow it.`

	openDescription = `Opens a file in TeleSAPR in place of the drawing that is open, and describes the drawing it opened as get_drawing does. The file is a TeleSAPR drawing (.tlspr). A drawing with changes that aren't saved is not replaced: ask the user to save it or let it go. A relative path is taken from the directory TeleSAPR works in.

Open a file only when the user asks for it. The user may be asked to allow it.`

	filesDescription = `Lists a directory on the computer TeleSAPR runs on: the directories and files in it, with their sizes, but for hidden ones. With no dir, the directory TeleSAPR works in, which is where export and open take a relative path from. Use it to find a drawing the user names, or where to export.`

	helpDescription = `Returns the documentation of TeleSAPR's scripting interface. With no name it is the primer: how scripts and drawings work, what the interpreter can't do, an example, and every call there is; read it once before writing the first script. With a name it is the documentation of that type, method or function: "Features.Extrude", "Group", "Group.SetCombine", "Entity", "ThreadOptions", "math.Hypot"; or of what a script calls in place of an entry of the window's menu, as "Features > Fillet Edges". The calls that change the drawing are grouped by menu: "Features" lists them all.`
)

// RunInput is what run_script takes.
type RunInput struct {
	Code string            `json:"code" jsonschema:"Go statements to run on the drawing, or a whole Go program"`
	Form map[string]string `json:"form,omitempty" jsonschema:"values for the fields of the form the code declares, by field name, as \"teeth\": \"30\"; the values written in the code if left out"`
}

// DescribeInput is what get_drawing takes.
type DescribeInput struct {
	Group string `json:"group,omitempty" jsonschema:"a group whose requests and constraints to list, by its name or as g003; the whole drawing if left out"`
}

// RenderInput is what render takes.
type RenderInput struct {
	View          string `json:"view,omitempty" jsonschema:"isometric (the default), front, back, top, bottom, left, right, or window for what the user sees"`
	Group         string `json:"group,omitempty" jsonschema:"a group to draw the drawing up to, as if it were active, by its name or as g003; the active group if left out"`
	Width         int    `json:"width,omitempty" jsonschema:"the picture's width in pixels, 800 if left out"`
	Height        int    `json:"height,omitempty" jsonschema:"the picture's height in pixels, 600 if left out"`
	Section       string `json:"section,omitempty" jsonschema:"XY, YZ or ZX: draw the solid cut by a plane parallel to that workplane; no cut if left out"`
	SectionOffset string `json:"section_offset,omitempty" jsonschema:"the cutting plane's distance from the workplane along its normal, a number or an expression in the drawing's unit; 0 if left out"`
	SectionFlip   bool   `json:"section_flip,omitempty" jsonschema:"take away the side the normal points away from, instead of the side it points to"`
}

// MeasureInput is what measure takes.
type MeasureInput struct {
	Group    string `json:"group,omitempty" jsonschema:"a group to measure the model up to, by its name or as g003; the active group if left out"`
	Selected bool   `json:"selected,omitempty" jsonschema:"measure what the user has selected instead of the solid"`
}

// PathInput is what export and open take.
type PathInput struct {
	Path string `json:"path" jsonschema:"the file, absolute or relative to the directory TeleSAPR works in"`
}

// FilesInput is what list_files takes.
type FilesInput struct {
	Dir string `json:"dir,omitempty" jsonschema:"the directory, absolute or relative to the directory TeleSAPR works in; that directory if left out"`
}

// RunOutput is the part of run_script's result a program can read
// without reading the text: whether the script failed and how the
// groups came out.
type RunOutput struct {
	Error            string              `json:"error,omitempty" jsonschema:"why the script stopped, with its line and column; empty if it ran to its end"`
	RolledBack       bool                `json:"rolledBack,omitempty" jsonschema:"whether what the script changed was taken back"`
	Groups           []agent.GroupStatus `json:"groups" jsonschema:"the active group, then every group in trouble: not solved, or with a problem"`
	ExpressionErrors []string            `json:"expressionErrors,omitempty" jsonschema:"expressions that can't be worked out"`
}

// HelpInput is what api_help takes.
type HelpInput struct {
	Name string `json:"name,omitempty" jsonschema:"a menu, type, method or function, as Features, Features.Extrude or Group, or a menu entry, as Features > Fillet Edges; the primer if left out"`
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
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in RunInput) (*mcp.CallToolResult, RunOutput, error) {
		code, err := script.ApplyForm(in.Code, in.Form)
		if err != nil {
			out := text("error: " + err.Error() + "\nThe script did not run.\n")
			out.IsError = true
			// The values don't fit the form: the tool's error, as a script's is, not the protocol's.
			return out, RunOutput{Error: err.Error(), Groups: []agent.GroupStatus{}}, nil //nolint:nilerr // reported in the result
		}
		res, err := b.Run(ctx, code)
		if err != nil {
			return nil, RunOutput{}, err
		}
		out := text(RunText(res))
		out.IsError = res.Error != ""
		groups := res.Groups
		if groups == nil {
			groups = []agent.GroupStatus{}
		}
		return out, RunOutput{Error: res.Error, RolledBack: res.RolledBack, Groups: groups, ExpressionErrors: res.ExpressionErrors}, nil
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
		png, err := b.Render(ctx, agent.Picture{View: in.View, Group: in.Group, Width: in.Width, Height: in.Height,
			Section: in.Section, SectionOffset: in.SectionOffset, SectionFlip: in.SectionFlip})
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

	mcp.AddTool(s, &mcp.Tool{
		Name:        ToolMeasure,
		Title:       "Measure the drawing",
		Description: measureDescription,
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closed},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in MeasureInput) (*mcp.CallToolResult, any, error) {
		s, err := b.Measure(ctx, in.Group, in.Selected)
		if err != nil {
			return nil, nil, err
		}
		return text(s), nil, nil
	})

	// Writing a file replaces any of that name, so export is
	// destructive; it is idempotent, the same drawing writing the same
	// bytes.
	destructive := true
	mcp.AddTool(s, &mcp.Tool{
		Name:        ToolExport,
		Title:       "Export the drawing to a file",
		Description: exportDescription,
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive, IdempotentHint: true, OpenWorldHint: &closed},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in PathInput) (*mcp.CallToolResult, any, error) {
		s, err := b.Export(ctx, in.Path)
		if err != nil {
			return nil, nil, err
		}
		return text(s), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        ToolOpen,
		Title:       "Open a file in place of the drawing",
		Description: openDescription,
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive, OpenWorldHint: &closed},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in PathInput) (*mcp.CallToolResult, any, error) {
		s, err := b.Open(ctx, in.Path)
		if err != nil {
			return nil, nil, err
		}
		return text(s), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        ToolFiles,
		Title:       "List a directory",
		Description: filesDescription,
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closed},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in FilesInput) (*mcp.CallToolResult, agent.Listing, error) {
		l, err := b.Files(ctx, in.Dir)
		if err != nil {
			return nil, agent.Listing{}, err
		}
		return text(ListingText(l)), l, nil
	})
	return s
}

// ListingText is a directory's listing as a model reads it: one line
// an entry, a directory with a slash after its name, a file with its
// size.
func ListingText(l agent.Listing) string {
	var b strings.Builder
	b.WriteString(l.Dir + ":\n")
	if len(l.Entries) == 0 {
		b.WriteString("(empty)\n")
	}
	for _, e := range l.Entries {
		if e.Dir {
			b.WriteString(e.Name + "/\n")
		} else {
			fmt.Fprintf(&b, "%s  %d bytes\n", e.Name, e.Size)
		}
	}
	if l.More > 0 {
		fmt.Fprintf(&b, "… and %d more\n", l.More)
	}
	return b.String()
}
