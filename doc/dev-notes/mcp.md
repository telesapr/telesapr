# The MCP server

`telesapr-cli mcp` lets an AI model's program draw in TeleSAPR. The user's page is
`doc/guide/ai.md`; this is how it is made.

## The pieces

- `cmd/telesapr-cli/mcp.go`: the command. It speaks MCP on stdin and stdout and parses its flags
  by hand (`--socket`, `--headless`, `--lib`, `--allow`, `--unguarded`, `--record`).
- `pkg/mcpserver`: the tools (`api_help`, `get_drawing`, `run_script`, `measure`, `render`,
  `list_files`, `export`, `open`) on the official Go SDK. It knows only an `agent.Backend`.
- `pkg/script/agent`: the backends. `Remote` sends each call over the socket to the window;
  `Session` runs it on an engine of its own. Also the primer (`api_help` with no name), the
  guard on files, and the socket protocol.
- `pkg/ui/agent.go`: the window's end of the socket.

## The socket protocol

The window listens on a Unix socket in a folder of the user's (`agent.SocketDir`), turned on by
Script > Allow AI Connections or `--agent`. A client sends one JSON object per line (a call: its
name, its arguments) and reads one JSON line back (the text, the images, whether it failed). The
protocol has a version, and the command and the window must have the same one. The messages are
in `pkg/script/agent/socket_js.go`.

## Headless

`mcp --headless` builds a `script.Engine` over a sketch of its own (the file named, or a new one)
and wraps it in `agent.NewSession`. What the scripts print is returned to the client, because
standard output is the protocol's. Files are guarded unless `--unguarded`.

## Recorded sessions

`--record <file>` puts a reader (`recorder` in `mcp.go`) between stdin and the SDK. Each complete
line the client sends is appended to the file as it arrives, so a real session with a model is
a replay fixture of the shape of `testdata/mcp/bracket.jsonl`. It works in both modes.

`TestMCPRecorded` (`cmd/telesapr-cli/mcp_recorded_test.go`) replays every
`testdata/mcp/recorded/<name>.jsonl` headless in a temporary folder, with the files of
`<name>.files/` copied in if there is such a folder, and checks:

- every `tools/call` answer is not an error, except the ids listed in `<name>.errors` (one per
  line; a deliberate mistake of the model's);
- after the replay, `get_drawing` and `measure` give what `<name>.want` holds: the groups, and
  the measure's soundness, `Volume:` and `Extent:` lines. `TELESAPR_REGEN_GOLDEN=1` writes it.

To keep a session: record it, remove calls that save or export (a fixture must not hold absolute
paths), list the ids of the calls that fail on purpose in `.errors`, and generate `.want` and read
it before committing.

This is the model-free half of the parked `make ai-eval` (ROADMAP "Parked"): it checks that what
a model once did still works, not what a model does now.
