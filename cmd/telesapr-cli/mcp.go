package main

import (
	"context"
	"fmt"
	"io"
	"net"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/telesapr/telesapr/pkg/mcpserver"
	"github.com/telesapr/telesapr/pkg/script"
	"github.com/telesapr/telesapr/pkg/script/agent"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/version"
)

// This file is the tool as a Model Context Protocol server: what an
// AI model's program starts to draw in TeleSAPR. The protocol is
// spoken here, on standard input and output; the drawing is the one
// open in the application, reached through its socket, or with
// --headless one of this process's own.

const mcpUsage = `
AI commands:
    mcp [--socket <path>]
        Serves the drawing that is open in TeleSAPR to a Model Context
        Protocol client, such as an AI model's program, on standard input
        and output: the client runs scripts on the drawing, reads what is
        in it, and looks at pictures of it. TeleSAPR must have
        Script > Allow AI Connections turned on, or be started with
        --agent; it may be started after this command. <path> is the
        socket of the TeleSAPR to talk to, if not the last one started.
        A client is told to start this command, for example:
            claude mcp add telesapr -- telesapr-cli mcp
    mcp --headless [--lib <dir>] [<sketch.tlspr>]
        Serves a drawing of its own instead, with no window: the sketch
        file, or a new sketch. Nothing is saved unless a script does it.
        <dir> is the script library, as for run; the window's is the one
        set in its configuration.
`

// nopWriteCloser is a writer that isn't closed with the transport.
type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }

// runMCP performs the mcp command, and returns the tool's exit
// status. It returns when the client closes its end.
func runMCP(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	headless, sketchFile, socket, library := false, "", "", ""
	rest := args[2:]
	for i := 0; i < len(rest); i++ {
		switch arg := rest[i]; {
		case arg == "--headless":
			headless = true
		case arg == "--socket":
			if i+1 >= len(rest) {
				_, _ = fmt.Fprintln(stderr, "--socket needs the socket's path.")
				return exitUsage
			}
			i++
			socket = rest[i]
		case arg == "--lib":
			if i+1 >= len(rest) {
				_, _ = fmt.Fprintln(stderr, "--lib needs the library's folder.")
				return exitUsage
			}
			i++
			library = rest[i]
		case headless && sketchFile == "" && isSketch(arg):
			sketchFile = arg
		default:
			_, _ = fmt.Fprintf(stderr, "Unrecognized option '%s'.\n", arg)
			return exitUsage
		}
	}
	if headless && socket != "" {
		_, _ = fmt.Fprintln(stderr, "--socket is of a TeleSAPR that is running; with --headless there is none.")
		return exitUsage
	}
	if !headless && library != "" {
		_, _ = fmt.Fprintln(stderr, "--lib is for --headless; the window's library is set in its configuration.")
		return exitUsage
	}
	if library == "" {
		library = script.DefaultLibrary()
	}

	var backend agent.Backend
	if headless {
		// What the scripts print, and what the program has to say, is
		// returned to the client: standard output is the protocol's.
		out := &agent.Capture{}
		host, err := scriptSession(sketchFile, out)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "%s\n", capitalized(err.Error()))
			return exitLoad
		}
		engine, err := script.New(script.Options{
			Host:    host,
			Stdout:  out,
			Stderr:  out,
			Library: library,
			Open:    func(path string) (api.Host, error) { return scriptSession(path, out) },
			New: func() api.Host {
				h, _ := scriptSession("", out)
				return h
			},
		})
		if err != nil {
			_, _ = fmt.Fprintln(stderr, err)
			return exitScript
		}
		backend = agent.NewSession(engine, out)
	} else {
		remote := &agent.Remote{Connect: func() (net.Conn, error) {
			if socket != "" {
				return agent.ConnectTo(socket)
			}
			dir, err := agent.SocketDir()
			if err != nil {
				return nil, err
			}
			return agent.Connect(dir)
		}}
		defer remote.Close()
		backend = remote
	}

	server := mcpserver.New(backend, version.Version)
	transport := &mcp.IOTransport{Reader: io.NopCloser(stdin), Writer: nopWriteCloser{stdout}}
	if err := server.Run(ctx, transport); err != nil && ctx.Err() == nil {
		_, _ = fmt.Fprintln(stderr, err)
		return exitScript
	}
	return exitOK
}
