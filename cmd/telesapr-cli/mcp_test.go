package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/script"
	"github.com/telesapr/telesapr/pkg/script/agent"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// answer is a response of the server to a request, as much of it as
// the tests read.
type answer struct {
	ID     int `json:"id"`
	Result struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
		Content []struct {
			Type     string `json:"type"`
			Text     string `json:"text"`
			Data     string `json:"data"`
			MIMEType string `json:"mimeType"`
		} `json:"content"`
		IsError      bool   `json:"isError"`
		Instructions string `json:"instructions"`
	} `json:"result"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// text is the text of a tool's result.
func (a answer) text() string {
	var parts []string
	for _, c := range a.Result.Content {
		if c.Type == "text" {
			parts = append(parts, c.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// converse runs the mcp command and plays a client to it: each line is
// a message of the protocol, and one that is a request is answered
// before the next is sent. It returns the answers by request id, and
// the command's exit status once the client has hung up.
func converse(t *testing.T, args []string, lines []string) (answers map[int]answer, status int, stderr string) {
	t.Helper()
	toServer, clientWrites := io.Pipe()
	serverWrites, fromServer := newPipe()
	var errs bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- runMCP(context.Background(), append([]string{"telesapr-cli", "mcp"}, args...), toServer, serverWrites, &errs)
		_ = serverWrites.Close()
	}()

	answers = map[int]answer{}
	scan := bufio.NewScanner(fromServer)
	scan.Buffer(make([]byte, 0, 64<<10), 64<<20)
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var msg struct {
			ID *int `json:"id"`
		}
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			t.Fatalf("the recorded line %q: %v", line, err)
		}
		if _, err := io.WriteString(clientWrites, line+"\n"); err != nil {
			t.Fatalf("the server hung up: %v\n%s", err, errs.String())
		}
		for msg.ID != nil {
			if !scan.Scan() {
				t.Fatalf("no answer to request %d: %v\n%s", *msg.ID, scan.Err(), errs.String())
			}
			var a answer
			if err := json.Unmarshal(scan.Bytes(), &a); err != nil {
				t.Fatalf("the server wrote %q: %v", scan.Text(), err)
			}
			if a.ID == *msg.ID {
				answers[a.ID] = a
				break
			}
		}
	}
	_ = clientWrites.Close()
	go func() { _, _ = io.Copy(io.Discard, fromServer) }()
	return answers, <-done, errs.String()
}

// newPipe is a pipe with the writer first.
func newPipe() (*io.PipeWriter, *io.PipeReader) {
	r, w := io.Pipe()
	return w, r
}

// A client's recorded conversation, drawing the bracket of
// examples/scripts/bracket step by step with a mistake on the way,
// leaves the sketch the example's script makes: the tools reach the
// same engine as a script does, and a script that fails leaves no
// trace.
func TestMCPBracket(t *testing.T) {
	recorded, err := os.ReadFile(filepath.Join("..", "..", "testdata", "mcp", "bracket.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	example, err := filepath.Abs(filepath.Join("..", "..", "examples", "scripts", "bracket", "bracket.go"))
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())

	answers, status, stderr := converse(t, []string{"--headless"}, strings.Split(string(recorded), "\n"))
	if status != exitOK || stderr != "" {
		t.Fatalf("mcp --headless: status %d, stderr %q", status, stderr)
	}
	for id, a := range answers {
		if a.Error != nil {
			t.Errorf("request %d was answered with the error %q", id, a.Error.Message)
		}
	}

	if got := answers[1].Result.Instructions; got != agent.Instructions {
		t.Errorf("the instructions are %q", got)
	}
	if n := len(answers[2].Result.Tools); n != 8 {
		t.Errorf("%d tools are listed, want 8", n)
	}
	if got := answers[3].text(); got != agent.Primer() {
		t.Errorf("api_help with no name gave %d bytes that aren't the primer", len(got))
	}
	if got := answers[4].text(); !strings.HasPrefix(got, "sketch: unit mm\ngroups, in the order the model is built:\n    g002-sketch-in-plane, sketch-in-plane: solved, 0 DOF; active\n") {
		t.Errorf("the new drawing is described as\n%s", got)
	}
	if a := answers[5]; a.Result.IsError || !strings.Contains(a.text(), "solved, 0 DOF") {
		t.Errorf("the outline: error %v\n%s", a.Result.IsError, a.text())
	}
	if a := answers[6]; !a.Result.IsError || !strings.Contains(a.text(), "error: script:3:") || !strings.Contains(a.text(), "taken back") {
		t.Errorf("the mistake: error %v\n%s", a.Result.IsError, a.text())
	}
	if a := answers[7]; a.Result.IsError || !strings.HasPrefix(a.text(), "value: 0\n") {
		t.Errorf("the holes: error %v\n%s", a.Result.IsError, a.text())
	}
	if a := answers[8]; a.Result.IsError || !strings.HasPrefix(a.text(), "value: true\nstatus:\nactive group g003-plate, extrude: solved") {
		t.Errorf("the extrusion: error %v\n%s", a.Result.IsError, a.text())
	}

	picture := answers[9].Result.Content
	if len(picture) != 1 || picture[0].Type != "image" || picture[0].MIMEType != "image/png" {
		t.Fatalf("the picture is %+v", answers[9].Result)
	}
	data, err := base64.StdEncoding.DecodeString(picture[0].Data)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 320 || b.Dy() != 240 {
		t.Errorf("the picture is %d by %d, want 320 by 240", b.Dx(), b.Dy())
	}
	// Something is drawn in it: not every pixel is the corner's.
	varied := false
	corner := img.At(0, 0)
	for y := 0; y < 240 && !varied; y += 4 {
		for x := 0; x < 320; x += 4 {
			if img.At(x, y) != corner {
				varied = true
				break
			}
		}
	}
	if !varied {
		t.Errorf("the picture is all one colour")
	}

	if got := answers[10].text(); strings.Count(got, "-circle: circle at ") != 3 || strings.Contains(got, "radius 1\n") {
		t.Errorf("the sketch has other circles than the three holes:\n%s", got)
	}
	if a := answers[11]; a.Result.IsError {
		t.Fatalf("saving: %s", a.text())
	}

	// The example's script, with the same sizes, saves the same bytes.
	if status, _, stderr := runScripting(t, "", "run", example, "60", "3"); status != exitOK {
		t.Fatalf("the example: status %d, %s", status, stderr)
	}
	byScript, err := os.ReadFile("bracket.tlspr")
	if err != nil {
		t.Fatal(err)
	}
	byClient, err := os.ReadFile("mcp.tlspr")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(byScript, byClient) {
		t.Errorf("the client's sketch isn't the script's:\n%s\nthe script's:\n%s", byClient, byScript)
	}
}

// A sketch file is the drawing served, and what is wrong with the
// command line is said.
func TestMCPCommandLine(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	model := filepath.Join(dir, "part.tlspr")
	doc := api.New()
	doc.Sketch.Circle(api.V(0, 0, 0), 4)
	doc.Features.Extrude(2, "disc")
	if err := doc.File.Save(model); err != nil {
		t.Fatal(err)
	}

	answers, status, stderr := converse(t, []string{"--headless", model}, []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_drawing","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"run_script","arguments":{"code":"other := New()\nother.Sketch.Line(V(0, 0, 0), V(3, 4, 0)).Length()"}}}`,
	})
	if status != exitOK || stderr != "" {
		t.Fatalf("mcp --headless part.tlspr: status %d, stderr %q", status, stderr)
	}
	if got := answers[2].text(); !strings.Contains(got, "g003-disc, extrude: solved") {
		t.Errorf("the sketch file is described as\n%s", got)
	}
	if got := answers[3].text(); !strings.HasPrefix(got, "value: 5\n") {
		t.Errorf("a script on another new sketch gave\n%s", got)
	}

	for _, c := range []struct {
		args   []string
		status int
		stderr string
	}{
		{[]string{"--socket"}, exitUsage, "--socket needs the socket's path.\n"},
		{[]string{"--headless", "--socket", "x"}, exitUsage, "with --headless there is none"},
		{[]string{"--window"}, exitUsage, "Unrecognized option '--window'.\n"},
		{[]string{"--headless", "--lib"}, exitUsage, "--lib needs the library's folder.\n"},
		{[]string{"--lib", dir}, exitUsage, "are for --headless"},
		{[]string{"--unguarded"}, exitUsage, "are for --headless"},
		{[]string{"--headless", "--allow"}, exitUsage, "--allow needs a folder.\n"},
		{[]string{model}, exitUsage, "Unrecognized option"},
		{[]string{"--headless", filepath.Join(dir, "none.tlspr")}, exitLoad, "Cannot load"},
	} {
		var errs bytes.Buffer
		status := runMCP(context.Background(), append([]string{"telesapr-cli", "mcp"}, c.args...), strings.NewReader(""), io.Discard, &errs)
		if status != c.status || !strings.Contains(errs.String(), c.stderr) {
			t.Errorf("mcp %v: status %d, stderr %q; want %d, %q", c.args, status, errs.String(), c.status, c.stderr)
		}
	}
}

// Without --headless the drawing is the one of the program that
// listens on the socket, which may not be there yet.
func TestMCPThroughSocket(t *testing.T) {
	t.Parallel()
	dir, err := os.MkdirTemp("", "ss")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	socket := filepath.Join(dir, "none.sock")
	call := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"run_script","arguments":{"code":"doc.Sketch.Circle(V(0, 0, 0), 2).Radius()"}}}`
	hello := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
	}

	// Nothing listens: the server starts all the same, help works, and
	// a tool that needs the drawing says what to do.
	answers, status, stderr := converse(t, []string{"--socket", socket}, append(hello, call,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"api_help","arguments":{"name":"Sketch.Circle"}}}`))
	if status != exitOK || stderr != "" {
		t.Fatalf("mcp --socket: status %d, stderr %q", status, stderr)
	}
	if a := answers[2]; !a.Result.IsError || !strings.Contains(a.text(), "Allow AI Connections") {
		t.Errorf("a script with no TeleSAPR running: error %v, %q", a.Result.IsError, a.text())
	}
	if a := answers[3]; a.Result.IsError || !strings.Contains(a.text(), "func (m SketchMenu) Circle(") {
		t.Errorf("help with no TeleSAPR running: error %v, %q", a.Result.IsError, a.text())
	}

	// A program that has a drawing listens.
	l, path, err := agent.Listen(dir)
	if err != nil {
		t.Fatal(err)
	}
	sk := sketch.NewSketch()
	sk.NewFile()
	session, _, err := agent.Open(script.Options{Host: api.NewSketchHost(sk, "")})
	if err != nil {
		t.Fatal(err)
	}
	served := make(chan struct{})
	go func() {
		defer close(served)
		agent.Accept(context.Background(), l, session)
	}()
	answers, status, stderr = converse(t, []string{"--socket", path}, append(hello, call))
	_ = l.Close()
	<-served
	if status != exitOK || stderr != "" {
		t.Fatalf("mcp --socket: status %d, stderr %q", status, stderr)
	}
	if a := answers[2]; a.Result.IsError || !strings.HasPrefix(a.text(), "value: 2\n") {
		t.Errorf("a script through the socket: error %v, %q", a.Result.IsError, a.text())
	}
	if n := len(session.Engine().Doc().Active().Requests()); n != 1 {
		t.Errorf("the program's drawing has %d requests after the script, want 1", n)
	}
}

// A headless server's scripts write files in the sketch's folder and
// those --allow names, and nowhere else unless --unguarded.
func TestMCPGuardedFiles(t *testing.T) {
	t.Parallel()
	dir, other, outside := t.TempDir(), t.TempDir(), t.TempDir()
	model := filepath.Join(dir, "part.tlspr")
	if err := api.New().File.Save(model); err != nil {
		t.Fatal(err)
	}
	save := func(path string) string {
		code, err := json.Marshal("doc.File.Save(" + strconv.Quote(path) + ")")
		if err != nil {
			t.Fatal(err)
		}
		return `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"run_script","arguments":{"code":` + string(code) + `}}}`
	}
	start := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
	}
	for _, c := range []struct {
		args []string
		path string
		want string
	}{
		{[]string{"--headless", model}, filepath.Join(dir, "a.tlspr"), "value: nil"},
		{[]string{"--headless", model}, filepath.Join(outside, "a.tlspr"), "not allowed: a guarded script writes files only in"},
		{[]string{"--headless", "--allow", other, model}, filepath.Join(other, "a.tlspr"), "value: nil"},
		{[]string{"--headless", "--unguarded", model}, filepath.Join(outside, "a.tlspr"), "value: nil"},
	} {
		answers, status, stderr := converse(t, c.args, append(start, save(c.path)))
		if got := answers[3].text(); status != exitOK || stderr != "" || !strings.Contains(got, c.want) {
			t.Errorf("mcp %v saving %s: status %d, stderr %q, answer\n%s", c.args, c.path, status, stderr, got)
		}
	}
}
