package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/telesapr/telesapr/examples"
	"github.com/telesapr/telesapr/pkg/script/agent"
)

// The addresses of the resources, and of the templates' resources up
// to their argument.
const (
	ResourceDrawing  = "telesapr://drawing"
	ResourceScript   = "telesapr://drawing/script"
	ResourcePicture  = "telesapr://drawing/picture"
	ResourcePrimer   = "telesapr://help/primer"
	ResourceGroup    = "telesapr://drawing/group/"
	ResourceExamples = "telesapr://examples/"

	// groupTemplate is the template of a group's resource; name is its
	// argument, which completion offers.
	groupTemplate = ResourceGroup + "{name}"
)

// The descriptions of the resources, as a user reads them where a
// client offers them to attach.
const (
	drawingResDescription = `The open drawing in words, as get_drawing gives it: the groups, how each solved, the active group's requests and constraints, and the selection.`
	scriptResDescription  = `The open drawing as the Go script that builds it again, as File > Export > Script (.go) writes it.`
	pictureResDescription = `A picture (PNG) of the open drawing from the isometric view, as render gives it.`
	primerResDescription  = `The primer of TeleSAPR's scripting interface, as api_help gives it with no name: how scripts work, and every call there is.`
	groupResDescription   = `One group of the open drawing, by its name or as g003: its requests and constraints, as get_drawing gives them with group.`
)

// read is a resource's handler that gives one text, or the error of
// getting it.
func read(mime string, get func(context.Context, *mcp.ReadResourceRequest) (string, error)) mcp.ResourceHandler {
	return func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		s, err := get(ctx, req)
		if err != nil {
			return nil, readError(req.Params.URI, err)
		}
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: req.Params.URI, MIMEType: mime, Text: s}}}, nil
	}
}

// readError is the protocol's error for a resource that could not be
// read: the back end's sentence, as "TeleSAPR isn't running, ...", said
// to be of the resource. A resource not found is left as it is.
func readError(uri string, err error) error {
	if _, ok := errors.AsType[*jsonrpc.Error](err); ok {
		return err
	}
	return fmt.Errorf("reading %s: %w", uri, err)
}

// addResources offers b's drawing, the primer and the example scripts
// to s as resources. Nothing is subscribed to: a resource is read again
// when it is wanted.
func addResources(s *mcp.Server, b agent.Backend) {
	describe := func(d agent.Describing) func(context.Context, *mcp.ReadResourceRequest) (string, error) {
		return func(ctx context.Context, _ *mcp.ReadResourceRequest) (string, error) {
			got, err := b.Describe(ctx, d)
			return got.Text, err
		}
	}
	s.AddResource(&mcp.Resource{URI: ResourceDrawing, Name: "drawing", Title: "The drawing",
		Description: drawingResDescription, MIMEType: "text/plain"},
		read("text/plain", describe(agent.Describing{})))
	s.AddResource(&mcp.Resource{URI: ResourceScript, Name: "drawing-script", Title: "The drawing as a script",
		Description: scriptResDescription, MIMEType: "text/x-go"},
		read("text/x-go", describe(agent.Describing{Script: true})))
	s.AddResource(&mcp.Resource{URI: ResourcePicture, Name: "drawing-picture", Title: "A picture of the drawing",
		Description: pictureResDescription, MIMEType: "image/png"},
		func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			pic, err := b.Render(ctx, agent.Picture{})
			if err != nil {
				return nil, readError(req.Params.URI, err)
			}
			return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: req.Params.URI, MIMEType: "image/png", Blob: pic.PNG}}}, nil
		})
	s.AddResource(&mcp.Resource{URI: ResourcePrimer, Name: "primer", Title: "The scripting primer",
		Description: primerResDescription, MIMEType: "text/markdown"},
		read("text/markdown", func(ctx context.Context, _ *mcp.ReadResourceRequest) (string, error) {
			return b.Help(ctx, "")
		}))
	s.AddResourceTemplate(&mcp.ResourceTemplate{URITemplate: groupTemplate, Name: "group", Title: "A group of the drawing",
		Description: groupResDescription, MIMEType: "text/plain"},
		read("text/plain", func(ctx context.Context, req *mcp.ReadResourceRequest) (string, error) {
			name, err := url.PathUnescape(strings.TrimPrefix(req.Params.URI, ResourceGroup))
			if err != nil || name == "" {
				return "", mcp.ResourceNotFoundError(req.Params.URI)
			}
			got, err := b.Describe(ctx, agent.Describing{Group: name})
			return got.Text, err
		}))
	addExamples(s)
}

// addExamples offers each example script as a resource of its own. A
// listed resource, not a template: a client that offers resources to
// attach lists those it is given, and finds one by a word of its name
// or title, where what it does with a template is not to be relied
// on. Its contents are the script, then each file of its library, if
// it has one, at the example's address followed by the file's path.
func addExamples(s *mcp.Server) {
	// The examples are built in: there is no list only if the tests failed.
	list, _ := examples.List()
	for _, e := range list {
		uri := ResourceExamples + e.Name
		s.AddResource(&mcp.Resource{URI: uri, Name: "example-" + e.Name, Title: "Example " + e.Name,
			Description: e.Title, MIMEType: "text/x-go"},
			func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
				files, err := examples.Files(e.Name)
				if err != nil {
					return nil, readError(uri, err)
				}
				res := &mcp.ReadResourceResult{}
				for i, f := range files {
					at := uri
					if i > 0 {
						at += "/" + f.Path
					}
					res.Contents = append(res.Contents, &mcp.ResourceContents{URI: at, MIMEType: "text/x-go", Text: f.Text})
				}
				return res, nil
			})
	}
}
