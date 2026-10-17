# Scripting is Go, run by yaegi, and MCP exposes it

**Status:** Accepted
**Date:** 2026-10-01 (scripting), 2026-10-03 (MCP)

## Context

A CAD program needs a way to build models without the mouse: for tests, for batch work, and for
an AI model to draw with. The choice was between an embedded language of its own, Lua or
Python bindings, and Go itself.

## Decision

Scripts are Go, run by the yaegi interpreter (`github.com/traefik/yaegi`) against a facade
package, `pkg/script/api`, that is the same code a compiled Go program would call. The tables of
what a script may import are generated (`make generate`) from the packages and checked in. The
same engine serves the console in the window, `telesapr-cli run`, `repl` and `eval`, and the
MCP server (`telesapr-cli mcp`, `pkg/mcpserver`), which offers four tools, with typed inputs,
over the official `modelcontextprotocol/go-sdk`. Each script is one undo step; a script that
fails leaves the model as it was.

## Consequences

One language for the program, its tests and its users; a feature is scriptable the moment its
API method exists, and the definition of done requires that method. yaegi lags the Go release
(it reads up to the version it was built for, and has its own forms for a few constructs), so the
symbol tables pin what scripts see. A sandbox is not provided: a script can do what the program
can, which `doc/ai.md` says plainly.

## Addendum, 2026-10-10: the API follows the menus

`Doc` had grown to some 150 methods and `Group` to 100, one flat list in `help()`, in godoc and
in the primer a model reads, where nothing said what went with what. The calls that change a
drawing are now fields of `Doc`, one per menu of the window (`doc.Sketch.Line`,
`doc.Features.Extrude`), each in a file of `pkg/script/api` named for it; `Doc` keeps what finds
things in the drawing. What only one kind of group has is on a view of it
(`g.AsFillet().SetRadius`), and `g.History()` and `g.Analyze()` hold the Move Group and Analyze
commands. `api.MenuCalls` names the call of every menu entry, a test holds it to the menu both
ways, and `help("Features > Fillet Edges")` reads it. The flat names were dropped, not kept as
aliases, which would have kept the long lists: a script of 0.4 is brought up to date with
`doc/script-api-renames.md`.
