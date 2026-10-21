# Drawing with an AI model

TeleSAPR can be driven by an AI model: you say in plain English what to draw, and the model
draws it in the window you have open. TeleSAPR has no model of its own and needs no API key.
It offers the drawing to any program that speaks the Model Context Protocol (MCP), such as
Claude Code or Claude Desktop, and that program brings the model.

## Setting up

1. In TeleSAPR, turn on **Script > Allow AI Connections**. The setting is kept. To allow
   connections for one run only, start it as `telesapr --agent`.
2. Tell your MCP client to start `telesapr-cli mcp`. You never run that command yourself.

   Claude Code:

   ```bash
   claude mcp add telesapr -- telesapr-cli mcp
   ```

   Claude Desktop, and most other clients, in their MCP configuration file:

   ```json
   {
     "mcpServers": {
       "telesapr": { "command": "telesapr-cli", "args": ["mcp"] }
     }
   }
   ```

   Give the full path of `telesapr-cli` if it is not on the `PATH`.
3. Ask for something: "draw a 60 by 20 mm plate, 4 mm thick, with three 5 mm holes in a row".

The order of starting does not matter. The client may be started before TeleSAPR, and
TeleSAPR may be closed and opened again in the middle of a conversation.

## What you see

- Each script the model runs is shown in the script console (**Script > Show Console**) after
  an `ai>` prompt, with what it printed. The variables it defined are there for you to use in
  the console.
- Each script is one step of undo. Ctrl+Z takes back what one request to the model did, a
  script at a time.
- A script that fails changes nothing: what it did before the error is taken back, and the
  model is told so that it can correct the script.
- **Script > Stop** stops a script that runs.
- With **ask before an AI program's script runs** checked on the configuration screen,
  TeleSAPR shows each script and runs it only if you agree. Most clients also ask before
  they call a tool.

## What the model can do

The model gets eight tools.

| Tool | What it does |
|---|---|
| `run_script` | Runs Go on the drawing, as the script console does. It returns what the script printed, the value of its last expression, and how the drawing solved afterwards. |
| `get_drawing` | Describes the drawing in words: groups, requests with their positions, constraints, parameters, what you have selected, and what your mouse is over, so that "this" can be pointed at. |
| `render` | Returns a picture of the drawing from a named view, fitted, or as your window shows it (`window`), so the model can check its work; with a group, as the drawing is up to that group; with a section, cut open by a plane parallel to the XY, YZ or ZX workplane to show the inside, leaving your own section view as it was. |
| `api_help` | Returns the documentation of the scripting interface: a primer, or one menu, type or method, or what a script calls in place of an entry of the window's menu ("Features > Fillet Edges"). |
| `measure` | Reports the solid as Analyze > Check Solid does: volume, surface area, centre of mass, bounding box, and whether it is sound; or measures what you have selected, as Analyze > Measure does. |
| `export` | Writes the drawing to a file in the format its extension names (`.stl`, `.3mf`, `.glb`, `.step`, `.svg`, `.png`, ...) and says where. |
| `open` | Opens a `.tlspr` drawing in place of the one on screen, and describes it. It never replaces a drawing with changes that aren't saved. |
| `list_files` | Lists a directory, so the model can find the drawing you name or where to export. |

`measure`, `render` and `list_files` only read. `export` and `open` are shown in the console as
the script that would do the same, and with the confirmation setting on, you are asked before
each, as for a script.

A script can do whatever a script you type in the console can, which includes reading and
writing files with your permissions. The model is told not to save, export or open files unless
you ask for it. Turn on the confirmation setting if you want to read every script first.

Only programs running as you can connect. The application listens on a socket file in a
directory that is yours alone: `$XDG_RUNTIME_DIR/telesapr` on Linux, `$TMPDIR/telesapr`
on macOS, `%LOCALAPPDATA%\telesapr` on Windows. Nothing listens on the network.

## Without a window

```bash
telesapr-cli mcp --headless [sketch.tlspr]
```

serves a drawing of its own instead of the application's: the sketch file, or a new sketch.
The model works as above, with pictures drawn in software. Nothing is saved unless you ask
the model to save.

## If it does not work

- **"TeleSAPR isn't running, or Script > Allow AI Connections is off"**: start TeleSAPR and
  check the menu item. With several TeleSAPR windows open, the one started last is used;
  `telesapr-cli mcp --socket <path>` names another.
- **"another script is running"**: a script of yours runs in the console. Wait, or stop it.
- **The client shows no TeleSAPR tools**: check that the client can find `telesapr-cli`;
  run `telesapr-cli mcp --headless` in a terminal to see whether it starts. It then waits for
  a client on standard input, which is what it should do.
- **"install the same version of both"**: `telesapr` and `telesapr-cli` come from different
  builds. Build or install them together.

## How it is built

```
MCP client ──stdin/stdout, MCP──> telesapr-cli mcp ──socket file, JSON lines──> telesapr
```

`telesapr-cli mcp` speaks MCP (`pkg/mcpserver`, on the official Go SDK) and passes each
call to the application over the socket (`pkg/script/agent`), where it runs in the script
console's interpreter (`pkg/ui/agent.go`). The model writes scripts against the same
`pkg/script/api` that scripts and the console use, so every feature with a scripting method
is available to it with nothing added.
