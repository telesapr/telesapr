# Drawing with an AI model

spacesolve can be driven by an AI model: you say in plain English what to draw, and the model
draws it in the window you have open. spacesolve has no model of its own and needs no API key.
It offers the drawing to any program that speaks the Model Context Protocol (MCP), such as
Claude Code or Claude Desktop, and that program brings the model.

## Setting up

1. In spacesolve, turn on **Script > Allow AI Connections**. The setting is kept. To allow
   connections for one run only, start it as `spacesolve --agent`.
2. Tell your MCP client to start `spacesolve-cli mcp`. You never run that command yourself.

   Claude Code:

   ```bash
   claude mcp add spacesolve -- spacesolve-cli mcp
   ```

   Claude Desktop, and most other clients, in their MCP configuration file:

   ```json
   {
     "mcpServers": {
       "spacesolve": { "command": "spacesolve-cli", "args": ["mcp"] }
     }
   }
   ```

   Give the full path of `spacesolve-cli` if it is not on the `PATH`.
3. Ask for something: "draw a 60 by 20 mm plate, 4 mm thick, with three 5 mm holes in a row".

The order of starting does not matter. The client may be started before spacesolve, and
spacesolve may be closed and opened again in the middle of a conversation.

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
  spacesolve shows each script and runs it only if you agree. Most clients also ask before
  they call a tool.

## What the model can do

The model gets four tools.

| Tool | What it does |
|---|---|
| `run_script` | Runs Go on the drawing, as the script console does. It returns what the script printed, the value of its last expression, and how the drawing solved afterwards. |
| `get_drawing` | Describes the drawing in words: groups, requests with their positions, constraints, parameters, and what you have selected. |
| `render` | Returns a picture of the drawing from a named view, so the model can check its work. |
| `api_help` | Returns the documentation of the scripting interface: a primer, or one type or method. |

A script can do whatever a script you type in the console can, which includes reading and
writing files with your permissions. The model is told not to save, export or load unless you
ask for it. Turn on the confirmation setting if you want to read every script first.

Only programs running as you can connect. The application listens on a socket file in a
directory that is yours alone: `$XDG_RUNTIME_DIR/spacesolve` on Linux, `$TMPDIR/spacesolve`
on macOS, `%LOCALAPPDATA%\spacesolve` on Windows. Nothing listens on the network.

## Without a window

```bash
spacesolve-cli mcp --headless [sketch.spcs]
```

serves a drawing of its own instead of the application's: the sketch file, or a new sketch.
The model works as above, with pictures drawn in software. Nothing is saved unless you ask
the model to save.

## If it does not work

- **"spacesolve isn't running, or Script > Allow AI Connections is off"**: start spacesolve and
  check the menu item. With several spacesolve windows open, the one started last is used;
  `spacesolve-cli mcp --socket <path>` names another.
- **"another script is running"**: a script of yours runs in the console. Wait, or stop it.
- **The client shows no spacesolve tools**: check that the client can find `spacesolve-cli`;
  run `spacesolve-cli mcp --headless` in a terminal to see whether it starts. It then waits for
  a client on standard input, which is what it should do.
- **"install the same version of both"**: `spacesolve` and `spacesolve-cli` come from different
  builds. Build or install them together.

## How it is built

```
MCP client ──stdin/stdout, MCP──> spacesolve-cli mcp ──socket file, JSON lines──> spacesolve
```

`spacesolve-cli mcp` speaks MCP (`pkg/mcpserver`, on the official Go SDK) and passes each
call to the application over the socket (`pkg/script/agent`), where it runs in the script
console's interpreter (`pkg/ui/agent.go`). The model writes scripts against the same
`pkg/script/api` that scripts and the console use, so every feature with a scripting method
is available to it with nothing added.
