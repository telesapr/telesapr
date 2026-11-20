package mcpserver

import (
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/telesapr/telesapr/pkg/script/agent"
)

// The drawing in the window is the user's too: between two calls of
// the model's, the user may draw, undo or open another file, and so may
// another program connected to the window. The server remembers what
// its last answer found of the drawing's revision (agent.Seen) and
// compares at the next call of a tool that reads or changes the
// drawing; the model's own changes, a script's or a file it opened,
// are told apart by the call that made them. api_help and list_files
// don't read the drawing and aren't compared; the first call of a
// session has nothing to compare with.

// The notices a tool's result starts with when the drawing changed by
// someone else's doing since the model's last call. get_drawing's say
// so without sending the model to get_drawing.
const (
	changedNotice         = "The drawing was changed in TeleSAPR since your last call, by the user or another program: get_drawing shows it as it is now."
	changedNoticeDescribe = "The drawing was changed in TeleSAPR since your last call, by the user or another program: what you knew of it may be out of date; this is how it is now."
	anotherNotice         = "Another drawing was opened in TeleSAPR since your last call, in place of the one you worked on: get_drawing shows what it holds."
	anotherNoticeDescribe = "Another drawing was opened in TeleSAPR since your last call, in place of the one you worked on: this is the one open now."
)

// tracker is what the server's last answer found of the drawing's
// revision.
type tracker struct {
	mu    sync.Mutex
	last  agent.Seen
	known bool
}

// notice remembers what a call that succeeded found, and is the notice
// its result starts with, "" for none: when the drawing changed by
// someone else since the last call, of tool, that found it.
func (t *tracker) notice(w *agent.Watch, tool string) string {
	seen, ok := w.Seen()
	if !ok {
		return ""
	}
	t.mu.Lock()
	last, known := t.last, t.known
	t.last, t.known = seen, true
	t.mu.Unlock()
	if !known {
		return ""
	}
	changed, another := seen.Since(last)
	switch {
	case !changed || tool == ToolOpen:
		// What open replaced is gone, and its answer is the new one.
		return ""
	case another && tool == ToolDescribe:
		return anotherNoticeDescribe
	case another:
		return anotherNotice
	case tool == ToolDescribe:
		return changedNoticeDescribe
	}
	return changedNotice
}

// noticed is res with the notice, if there is one, as its first item.
func noticed(res *mcp.CallToolResult, notice string) *mcp.CallToolResult {
	if notice != "" {
		res.Content = append([]mcp.Content{&mcp.TextContent{Text: notice}}, res.Content...)
	}
	return res
}
