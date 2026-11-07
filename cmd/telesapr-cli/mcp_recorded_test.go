package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Recorded sessions (mcp --record) replay against a headless server
// and leave the drawing they left. This is the model-free half of
// judging a model's session: doc/dev-notes/mcp.md.
//
// The test changes the working directory (the guarded files of a new
// sketch are there), so its subtests cannot be parallel.
func TestMCPRecorded(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "testdata", "mcp", "recorded", "*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no recorded sessions in testdata/mcp/recorded")
	}
	for _, file := range files {
		base := strings.TrimSuffix(file, ".jsonl")
		t.Run(filepath.Base(base), func(t *testing.T) {
			replayRecorded(t, base)
		})
	}
}

// Ids that follow a recording's own, for the calls made after it.
const (
	recordedDrawingID = 1 << 30
	recordedMeasureID = recordedDrawingID + 1
)

// replayRecorded replays base.jsonl, and checks the answers against
// base.errors and the final drawing against base.want.
func replayRecorded(t *testing.T, base string) {
	recorded, err := os.ReadFile(base + ".jsonl")
	if err != nil {
		t.Fatal(err)
	}
	abs, err := filepath.Abs(base)
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	if err := copyTree(abs+".files", "."); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}

	failing := map[int]bool{}
	if data, err := os.ReadFile(abs + ".errors"); err == nil {
		for f := range strings.FieldsSeq(string(data)) {
			var id int
			if _, err := fmt.Sscan(f, &id); err != nil {
				t.Fatalf("%s.errors: %q is not a request id", base, f)
			}
			failing[id] = true
		}
	}

	lines := strings.Split(string(recorded), "\n")
	lines = append(lines,
		fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"tools/call","params":{"name":"get_drawing","arguments":{}}}`, recordedDrawingID),
		fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"tools/call","params":{"name":"measure","arguments":{}}}`, recordedMeasureID))
	answers, status, stderr := converse(t, []string{"--headless"}, lines)
	if status != exitOK || stderr != "" {
		t.Fatalf("mcp --headless: status %d, stderr %q", status, stderr)
	}
	for id, a := range answers {
		if a.Error != nil {
			t.Errorf("request %d was answered with the error %q", id, a.Error.Message)
		}
	}
	for _, line := range lines {
		var msg struct {
			ID     *int   `json:"id"`
			Method string `json:"method"`
		}
		if err := json.Unmarshal([]byte(line), &msg); err != nil || msg.ID == nil || msg.Method != "tools/call" {
			continue
		}
		a := answers[*msg.ID]
		if a.Result.IsError != failing[*msg.ID] {
			t.Errorf("request %d: isError %v, want %v:\n%s", *msg.ID, a.Result.IsError, failing[*msg.ID], a.text())
		}
	}

	var got strings.Builder
	got.WriteString("# get_drawing: the groups\n")
	for l := range strings.SplitSeq(answers[recordedDrawingID].text(), "\n") {
		if strings.HasPrefix(l, "    ") && strings.Contains(l, ": ") {
			got.WriteString(l + "\n")
		}
	}
	got.WriteString("# measure\n")
	for l := range strings.SplitSeq(answers[recordedMeasureID].text(), "\n") {
		if strings.HasPrefix(l, "Volume:") || strings.HasPrefix(l, "Extent:") || strings.Contains(l, "solid is") {
			got.WriteString(l + "\n")
		}
	}
	checkRecordedWant(t, abs+".want", got.String())
}

// checkRecordedWant compares got with the file at path, or writes it
// there when TELESAPR_REGEN_GOLDEN is set, as the golden tests do.
func checkRecordedWant(t *testing.T, path, got string) {
	t.Helper()
	if os.Getenv("TELESAPR_REGEN_GOLDEN") != "" {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (record it with TELESAPR_REGEN_GOLDEN=1)", err)
	}
	if string(want) != got {
		t.Errorf("the drawing after the replay is\n%s\nwant\n%s", got, want)
	}
}

// copyTree copies the files under src into dst.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() { _ = in.Close() }()
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			_ = out.Close()
			return err
		}
		return out.Close()
	})
}

// --record keeps each message the client sent, and a file that can't
// be opened stops the command before it serves.
func TestMCPRecord(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "session.jsonl")
	lines := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"api_help","arguments":{"name":"Sketch.Circle"}}}`,
	}
	if _, status, stderr := converse(t, []string{"--headless", "--record", file}, lines); status != exitOK || stderr != "" {
		t.Fatalf("mcp --record: status %d, stderr %q", status, stderr)
	}
	got, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if want := strings.Join(lines, "\n") + "\n"; string(got) != want {
		t.Errorf("recorded\n%s\nwant\n%s", got, want)
	}

	for _, c := range []struct {
		args   []string
		status int
		stderr string
	}{
		{[]string{"--record"}, exitUsage, "--record needs a file.\n"},
		{[]string{"--headless", "--record", filepath.Join(t.TempDir(), "none", "x.jsonl")}, exitLoad, "Cannot record to"},
	} {
		var errs strings.Builder
		status := runMCP(context.Background(), append([]string{"telesapr-cli", "mcp"}, c.args...), strings.NewReader(""), io.Discard, &errs)
		if status != c.status || !strings.Contains(errs.String(), c.stderr) {
			t.Errorf("mcp %v: status %d, stderr %q; want %d, %q", c.args, status, errs.String(), c.status, c.stderr)
		}
	}
}
