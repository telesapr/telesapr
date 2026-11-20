# The MCP server

Refreshed 2026-11-20: prompts, resources, export's link; the drawing changed behind the model's
back; what the evaluation found.

`telesapr-cli mcp` lets an AI model's program draw in TeleSAPR. The user's page is
`doc/guide/ai.md`; this is how it is made.

## The pieces

- `cmd/telesapr-cli/mcp.go`: the command. It speaks MCP on stdin and stdout and parses its flags
  by hand (`--socket`, `--headless`, `--lib`, `--allow`, `--unguarded`, `--record`).
- `pkg/mcpserver`: the tools (`api_help`, `get_drawing`, `run_script`, `measure`, `render`,
  `list_files`, `export`, `open`), the prompts (`prompts.go`) and the resources (`resources.go`)
  on the official Go SDK. It
  knows only an `agent.Backend`.
- `pkg/script/agent`: the backends. `Remote` sends each call over the socket to the window;
  `Session` runs it on an engine of its own. Also the primer (`api_help` with no name), the
  guard on files, and the socket protocol.
- `pkg/ui/agent.go`: the window's end of the socket.

## The example scripts

`api_help` with `examples` lists the scripts of `examples/scripts`, a line each with the first
sentence of its comment, and with `example <name>` returns one's source and its `lib` files, after
a line on how a whole program differs from run_script's statements. The files are built into the
binary by package `examples` (`examples/embed.go`, an `embed.FS`; embed reaches only below its
package, so it sits there and not under `pkg/`), a leaf that imports nothing of the program.
`script.Help` answers both names, so the console's `help()` and `telesapr-cli repl` have them as
well; `Remote.Help` answers locally, and the socket protocol did not change. "example" and
"examples" are no package, type or menu (`TestHelpExampleNamesAreFree`), and a name with a space
is no name of Go. An unknown name gets the nearest by spelling, then by a word of the titles
(`examples.Nearest`). The list is alphabetical: the titles carry the words a model looks for
(thread, drawing, assembly), and a hand-kept grouping would rot. A script without a title line
fails `TestListHasEveryExample`.

The embedded text costs about 330 KB in each binary (`telesapr-cli` 46.13 to 46.47 MB, the
window 42.93 to 43.27 MB) and 350 KB in the browser's `.wasm`, about 90 KB gzipped. It stays in
the browser too, where the console's `help()` and the page's agent connection reach it.

## The socket protocol

The window listens on a Unix socket in a folder of the user's (`agent.SocketDir`), turned on by
Script > Allow AI Connections or `--agent`. A client sends one JSON object per line (a call: its
name, its arguments) and reads one JSON line back (the text, the images, whether it failed). Each answer carries the
drawing's revision (see above). The protocol has a version, 8 since the revision, and the command
and the window must have the same one. The messages are
in `pkg/script/agent/wire.go` (`socket_js.go` is the browser's listener, over postMessage).

## The drawing changed behind the model's back

The window is shared with the user, and with any other program connected to it: between two
calls the drawing may be drawn on, undone, re-dimensioned or replaced by another file. A tool
that reads or changes the drawing (`run_script`, `get_drawing`, `render`, `measure`, `export`,
`open`) then starts its result with a line that says so ("The drawing was changed in TeleSAPR
since your last call, by the user or another program: get_drawing shows it as it is now.", or
"Another drawing was opened ..."); `get_drawing`'s says the same without sending the model to
get_drawing. `run_script`'s structured result has `drawingChanged`. `api_help` and `list_files`
don't read the drawing and are not compared. The first call has nothing to compare with.

What was found of the candidates for "the drawing changed":

- `Sketch.Generation` counts regenerations. An activation regenerates (`groupActivate`,
  `MarkGroupDirty`), and so does a read of a group past the active one (`MakeThrough`) and a
  render with `group`: it would cry wolf at the model's own reads.
- `Sketch.Unsaved` is set by `MarkGroupDirty`, so by any activation (the file keeps which
  group is active), and a save clears it without a change. The model's `render` with `group`
  activates the group and puts the old one back; `Session.Render` then leaves a saved drawing
  saved, so that an `open` after the picture is not refused (`TestAgentReadsKeepSaved`).
- The undo stack moves at every remembered step, undo and redo, but its count stops at
  `MaxUndo`, a refused edit drops its step, and a new file or a load clears it.

So `pkg/sketch/revision.go` keeps a counter of its own, `Sketch.Revision`: `Change` moves where
a state is remembered for undo (`UndoRemember`, `UndoRememberFor` when the edit is kept), at
undo and redo, at a drag's end (`NoteChange`, since a drag remembers its state as it begins) and
at each change of a script after the first of its step (`Changing`); `Drawing` and `Change`
move where the sketch is cleared (`ClearExisting`) or replaced by one read from a file or a new
one, which takes its predecessor's revision (`Follow`, in `Application.setSketch` and
`SketchHost.adopt`). Folding steps into a script's one (`UndoDropNewest`) doesn't move it. The
revision is not saved.

Whose change it was: a call of the model's own changes the drawing too (a script, one rolled
back, `open`). Each host counts the change made inside its `Run` calls, the outermost only
(`RunChanges`, `SketchHost` and the window's `ScriptHost`); a script runs every access to the
sketch through `Run`, and the user's edits in the window come between them, on the event loop.
`agent.Session` reads the revision and that count at the start and end of a call (`track`, in
`revision.go`), and says what it found as an `agent.Seen`: the revision at the end, and `Own`,
the changes made in `Run` during a call that runs a script or opens a file (0 for a read, so a
console script of the user's running beside a read is the user's). The server remembers the
last `Seen` and compares (`Seen.Since`): someone else changed the drawing when
`Change - Own` is not the last `Change`. So a change the user makes while a long script runs is
told on that script's result, not lost in it: "before" is the revision the model last saw, and
the call's own changes are what is taken off. Several `telesapr-cli mcp` processes on one window
each keep their own last `Seen`, and each is told of the others' changes.

`Seen` rides on the call's context (`agent.WatchRevision`; the window's run, which starts on a
job's context, takes the watch with `KeepWatch`), so the eight methods of `agent.Backend` kept
their signatures and a caller that doesn't ask, as the prompts, costs nothing. Over the socket
it is the `revision` of each answer (`agent.Protocol` 8): no request of its own. The browser's
page answers through the same `Serve`, so `agent-client.js`'s `call` has it on each answer too.

Not done: a window started again in the middle of a conversation starts its revision from zero,
and one that happens to land on the last revision seen is not told apart (the drawing is then a
new one, or the file opened at start). A menu entry made of script calls (`ScriptHost.onLoop`)
used while the model's script runs is counted as the script's. Activating a group is no change,
though the model's next script then works in the group the user made active; `get_drawing` and
the status of each run name the active group.

## Prompts

`pkg/mcpserver/prompts.go` offers five prompts (`draw`, `check`, `explain`, `parametrize`,
`sheet`). A prompt is got through the same `agent.Backend` as the tools, so it needs nothing of
the socket and no change of `agent.Protocol`. Its messages are all the user's: the instruction
first, then one message of text for each thing it carries, said to be what a tool gives
("The drawing, as get_drawing gives it:"), so the model's first turn holds the state and the
model needs no opening calls. `draw` carries the primer and the description, `check` the
description (and the group's, with `group`) and the measurement, `explain` the drawing as a
script, `parametrize` the description and the measurement (the volume before), `sheet` the
description and the solid's faces and edges.

What was decided:

- Carried content is plain text, not an embedded resource: every client shows text, and
  Claude Code documents nothing else in a prompt. For the same reason `explain` carries no
  picture; its text asks the model to call `render`.
- A back end that fails (no window running) does not fail the prompt: the message it would
  have filled says what could not be got and the error's sentence, and the same error again is
  said once ("could not be got either, for the same reason"). Only a cancelled request is the
  protocol's error.
- Each prompt has at most one argument, and none is required. Claude Code splits what is typed
  after the command at spaces, one word an argument, so `draw`'s `what` may come cut short or
  not at all: its text then has the model take what to draw from the user's next message, or
  ask.
- Completion (`ServerOptions.CompletionHandler`, `complete`) offers `check`'s `group` from the
  description's groups, by the start of the name or of the part after its number ("pl" finds
  `g003-plate`), and `sheet`'s `paper` from the sheets. A client that never asks loses nothing;
  no drawing to read is no completion, not an error.

Tests: `TestPromptsListed`, `TestPromptsCarryTheDrawing`, `TestPromptsWithoutTheDrawing` and
`TestPromptArgumentsComplete` (`pkg/mcpserver/prompts_test.go`) through `connect`;
`TestDescriptionsNameRealCalls` checks the calls the prompt texts name with
`agent.UnknownCalls`, which also reads `doc.Constraint("c00a").SetExpr` (a method of what a
method of `Doc` returns). `TestMCPPrompts` (`cmd/telesapr-cli`) replays
`testdata/mcp/prompts.jsonl` headless: prompts/list, prompts/get `check`, a completion; and
`TestMCPThroughSocket` gets `explain` with nothing listening.

## Resources

`pkg/mcpserver/resources.go` offers what a user attaches to a message (Claude Code's `@`
menu): `telesapr://drawing` (text/plain, `Describe`), `telesapr://drawing/script` (text/x-go,
`Describe` with `Script`), `telesapr://drawing/picture` (image/png as a blob, `Render` at its
defaults), `telesapr://help/primer` (text/markdown, `Help("")`), the template
`telesapr://drawing/group/{name}` (`Describe` with `Group`), and `telesapr://examples/<name>`
for each example script. Like the prompts, they use only the `agent.Backend` methods there are,
so the socket protocol did not change.

What was decided:

- The examples are listed resources, one each (about a hundred), not a template. Claude Code lists
  resources in its `@` menu and searches them by a word of the name or title, as
  `@telesapr:telesapr://examples/bolt`; what it does with templates and completion is not
  documented. So many entries are a lot, but the menu is searched, not read down, and each carries
  the example's title as its description. An example's contents are its script at its own
  address, then each file of its `lib` at `telesapr://examples/<name>/<path>`; those addresses
  are not resources of their own.
- A resource that can't be read is the protocol's error, carrying the back end's sentence
  ("reading telesapr://drawing: TeleSAPR isn't running, ..."); unlike a prompt, a read has no
  message to put it in. An address there isn't, an example there isn't and an empty group name
  are the SDK's resource-not-found; a group there isn't is the back end's "no group is called".
- Completion of the group template's `name` goes through the prompts' `complete`, with the
  same groups as `check`'s `group`.
- No subscriptions and no `resources/updated`: few clients act on them, and the window would
  need to send events on the socket, which it does not (ROADMAP "Parked").

## Export's link

`export` returns, after its text, a `resource_link` to the `file://` address of the file
written, with its name, MIME type (a table by extension, not the system's, which differs by
machine) and size, and as structured content `path`, `bytes` and `notWritten`. Both back ends
answer with `Session.Export`'s sentence, "wrote <absolute path>, <n> bytes" first, the path made
absolute by the program with the drawing from the directory it works in, so the window's
answer over the socket has its own directory, not the command's. `exportResult` reads the path
from that sentence (the last ", " before " bytes", so a path with a comma reads too); a sentence
not of that shape gives the text alone and no link (`TestExportResultReadsTheSentence`). The
sentence is the contract: a change to it in `pkg/script/agent` breaks `TestExportLinksTheFile`.

Tests: `TestResourcesListed`, `TestResourcesRead`, `TestResourcesNotFound`,
`TestResourcesWithoutTheDrawing`, `TestResourceTemplateCompletes` (`resources_test.go`),
`TestExportLinksTheFile` through `connect`; `TestMCPResources` (`cmd/telesapr-cli`) replays
`testdata/mcp/resources.jsonl` headless in a folder of its own, ending with an export to a
relative path linked by its absolute one.

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

To keep a session: record it, remove calls that save or export to an absolute path (a fixture
must not hold one; a relative path lands in the temporary folder), list the ids of the calls that
fail in `.errors`, and generate `.want` and read it before committing.

Besides `plate`, the sessions there are the six tasks of the evaluation of 2026-11-20, each done
by a model that knew only what the server told it, kept with their failed calls: `plate-holes`,
`hex-bolt` (a thread by `ThreadOnFace`), `l-bracket` (fillet and holes on a face),
`lengthen-angle` (a converted example, one dimension made 20 % longer), `plate-drawing` (a sheet
with two views and dimensions, exported as SVG) and `mend-broken` (a rectangle with a conflicting
and a redundant dimension, mended to 0 DOF). Refreshed 2026-11-20.

## What the evaluation of 2026-11-20 found and what was changed

Over the six tasks all ended right, but 9 `run_script` calls and 3 `api_help` calls failed, nearly
all from faults in the texts the server gives. By class, for the next evaluation to compare:

- A call of two results shown as one value (5 scripts, one in every task that made a solid): the
  primer showed `a.Bounds()` alone on a line, and models wrote `fmt.Println("...", a.Bounds())`.
  The primer shows `lo, hi := a.Bounds()` and `src, notes := doc.Script()`; `TestPrimerChecks`
  prints each line of the checks as a value, so a call of several results shown so fails it.
- Help without a struct's fields (3 help calls, in the thread task): `ThreadOptions` listed bare
  field names, `ThreadOptions.Length` found nothing. gendoc writes the fields of package api's
  and check's structs with their comments into `docs.json` (`-fields`; 827 to 863 KB, where every
  package's would have added a sixth); help lists them with their types and first sentences, and
  `Type.Field` gives the whole comment, then a method of another package's type of that name.
- Stale or internal words in doc comments (2 models puzzled, no call failed by itself):
  `SetDrawing`, `AddDrawingDimension`, roadmap numbers, `Drawing.Export` "needs the
  application", `Features.Extrude`'s depth. Mended, and `TestAPICommentsNameRealCalls`
  (`pkg/script`) checks that every CamelCase and `Type.Name` in package api's comments exists;
  it found `LinkInfo`, `LinkProblem`, `SetParameter`, `ShowAsRadius` and `PointOfRayCasting` too.
  A listing ends "Help for "X.Method" says more", right for the console and for api_help.
- What the interpreter lacks (1 script): `min`. The primer lists `min`, `max` and `clear`;
  `TestPrimerMissingBuiltins` runs them.
- An edge name without its ordinal (1 script): `Solid.Edge` now names the edges between the
  name's faces, or of either face, and points at `get_drawing` with solid and `EdgeAt(point)`.
- Of the rest, the recorded sessions hold two `run_script` calls sent with no code at all
  (`hex-bolt` 23 and 24), a slip of the client's, which no text can mend. From the models'
  remarks: render's and the
  primer's texts say which way each view looks and the workplanes' normals, how to see the sheet
  (`doc.Drawing.Show(true)`, view "window"), that export's .svg is the model's view and not the
  sheet, that the script of `get_drawing` names its variables its own way; and the description
  of a sketch that doesn't solve names the constraints at odds (`Group.Conflicts`, read from the
  last solve) and whether redundant ones are allowed.

The recorded `hex-bolt` session's three help calls on fields now succeed and left its `.errors`.

This is the model-free half of the parked `make ai-eval` (ROADMAP "Parked"): it checks that what
a model once did still works, not what a model does now.
