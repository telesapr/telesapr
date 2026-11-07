package mcpserver

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/telesapr/telesapr/pkg/script/agent"
)

// This file tells the client how far a run_script is while it runs: a
// script that regenerates a large model takes minutes, and a client
// told nothing may give up on it. A client that asked for progress (a
// progress token) is sent each report as a progress notification; one
// that shows logs is sent what the script prints as it prints it.

// logger is the name the server's log messages go by.
const logger = "telesapr"

// progressCount makes the reports' counts a progress that only grows,
// as MCP wants: a report's Done starts again at each pass over the
// groups and each regeneration, and a line printed has no count.
type progressCount struct {
	// base is what is added to a report's Done; total the Total of the
	// last report with one, and last the progress last sent.
	base, total, last float64
}

// next is the progress and total, 0 for unknown, to send for p.
func (c *progressCount) next(p agent.Progress) (progress, total float64) {
	if p.Total > 0 {
		c.total = float64(p.Total)
		progress = c.base + float64(p.Done)
		if progress <= c.last {
			// A pass begun anew: counted on from where the last ended.
			c.base += c.last + 1 - progress
			progress = c.last + 1
		}
	} else {
		// A line: one step more, and one more to the end.
		c.base++
		progress = c.last + 1
	}
	c.last = progress
	if c.total > 0 {
		total = max(c.base+c.total, progress)
	}
	return progress, total
}

// runReporter is what passes a run's reports to the client of req, nil
// when there is no session to send them in. Failing to send is not the
// script's failure: the report is dropped.
func runReporter(ctx context.Context, req *mcp.CallToolRequest) func(agent.Progress) {
	if req == nil || req.Session == nil {
		return nil
	}
	ss := req.Session
	var token any
	if req.Params != nil {
		token = req.Params.GetProgressToken()
	}
	var count progressCount
	return func(p agent.Progress) {
		if token != nil {
			progress, total := count.next(p)
			_ = ss.NotifyProgress(ctx, &mcp.ProgressNotificationParams{ProgressToken: token,
				Progress: progress, Total: total, Message: p.Message})
		}
		if out := strings.TrimSuffix(p.Output, "\n"); out != "" {
			// Logging is deprecated since protocol 2026-07-28 but kept for
			// a year at least, and the clients of today show it.
			_ = ss.Log(ctx, &mcp.LoggingMessageParams{Level: "info", Logger: logger, Data: out}) //nolint:staticcheck // see above
		}
	}
}
