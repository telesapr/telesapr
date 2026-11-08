---
paths:
  - "pkg/script/**"
  - "pkg/mcpserver/**"
  - "cmd/telesapr-cli/**"
  - "examples/**"
---

# Scripting, the command line and MCP

- A feature has a method on `pkg/script/api`, on the menu it belongs to (`doc.Features.Extrude`
  is Features > Extrude), to create it, to read and change every parameter the text window shows
  and to query the result, with a doc comment: `help()`, `docs.json` and the reference page come
  from it. Then `make generate` (the yaegi symbol tables in `pkg/script/symbols`, through
  `scripts/generate-symbols.sh`) and `make generate-check` passing.
- An example under `examples/scripts/<name>` runs with `telesapr-cli run` and no arguments in a
  directory of its own (`TestExamplesRun` in `cmd/telesapr-cli`); a `lib` folder beside it is
  its script library. Each guide page is built on one.
- Scripts run in yaegi: no range over an integer or a function in `examples/`, which the lint
  exclusions allow; lengths are mm, angles degrees. A test in `pkg/script/api` covers misuse as
  well as use, as `TestMisuse` does.
- A model written back as the script that builds it is `TestScriptRoundTrip` (`pkg/script`);
  `TELESAPR_SCRIPT_OUT=dir` keeps the scripts that fail to come back.
- `telesapr-cli regenerate` writes its result over the input file: work on a copy.
- The MCP server is `telesapr-cli mcp` (`pkg/mcpserver`, backends in `pkg/script/agent`,
  `doc/dev-notes/mcp.md`); the primer an AI model reads first is `api_help` with no name, and
  the user's page is `doc/guide/ai.md`. The socket protocol has a version: a change to a message
  bumps it in the command and the window together. Recorded sessions under `testdata/mcp` replay
  in tests.
