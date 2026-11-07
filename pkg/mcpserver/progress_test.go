package mcpserver

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/telesapr/telesapr/pkg/script"
	"github.com/telesapr/telesapr/pkg/script/agent"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// The progress sent only grows, though a report's count starts again
// at each pass and a line has none, and the total is where the pass
// the progress is in ends.
func TestProgressCount(t *testing.T) {
	t.Parallel()
	var c progressCount
	type sent struct{ progress, total float64 }
	var got []sent
	for _, p := range []agent.Progress{
		{Message: "line"},
		{Done: 1, Total: 3}, {Done: 3, Total: 3},
		{Done: 1, Total: 3}, {Message: "line"}, {Done: 2, Total: 3},
		{Done: 2, Total: 3},
	} {
		progress, total := c.next(p)
		got = append(got, sent{progress, total})
	}
	want := []sent{{1, 0}, {2, 4}, {4, 4}, {5, 7}, {6, 8}, {7, 8}, {8, 9}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("the counts went %v, want %v", got, want)
	}
}

// A client that asks for progress, and has set a log level, is sent
// the lines a script prints while run_script runs, before its result.
func TestRunScriptProgress(t *testing.T) {
	t.Parallel()
	sk := sketch.NewSketch()
	sk.NewFile()
	session, _, err := agent.Open(script.Options{Host: api.NewSketchHost(sk, "")})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	clientSide, serverSide := mcp.NewInMemoryTransports()
	served, err := New(session, "1.2").Connect(ctx, serverSide, nil)
	if err != nil {
		t.Fatal(err)
	}
	var (
		mu       sync.Mutex
		progress []*mcp.ProgressNotificationParams
		logs     []string
	)
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, &mcp.ClientOptions{
		ProgressNotificationHandler: func(_ context.Context, r *mcp.ProgressNotificationClientRequest) {
			mu.Lock()
			defer mu.Unlock()
			progress = append(progress, r.Params)
		},
		//nolint:staticcheck // the server still logs, deprecated as logging is
		LoggingMessageHandler: func(_ context.Context, r *mcp.LoggingMessageRequest) {
			mu.Lock()
			defer mu.Unlock()
			if r.Params.Logger == logger && r.Params.Level == "info" {
				logs = append(logs, r.Params.Data.(string))
			}
		},
	})
	cs, err := client.Connect(ctx, clientSide, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = cs.Close()
		_ = served.Wait()
	}()
	// The client's log level goes with the request, as the protocol
	// since 2026-07-28 has it.
	//nolint:staticcheck // deprecated with logging, which the server still does
	params := &mcp.CallToolParams{Meta: mcp.Meta{mcp.MetaKeyLogLevel: "info"}, Name: ToolRun, Arguments: map[string]any{
		"code": "fmt.Println(\"first\")\ntime.Sleep(300 * time.Millisecond)\nfmt.Println(\"second\")"}}
	params.SetProgressToken("run-1")
	res, err := cs.CallTool(ctx, params)
	if err != nil || res.IsError {
		t.Fatalf("run_script gave %+v, %v", res, err)
	}
	// The client handles the notifications in the order they came,
	// before the result.
	mu.Lock()
	defer mu.Unlock()
	if strings.Join(logs, "\n") != "first\nsecond" {
		t.Errorf("the client was logged %q, want the two lines", logs)
	}
	if len(progress) != 2 {
		t.Fatalf("the client was sent %d progress notifications, want 2", len(progress))
	}
	for i, p := range progress {
		if p.ProgressToken != "run-1" || p.Progress != float64(i+1) {
			t.Errorf("notification %d is %+v", i, p)
		}
	}
	if progress[0].Message != "first" || progress[1].Message != "second" {
		t.Errorf("the notifications say %q and %q", progress[0].Message, progress[1].Message)
	}
}
