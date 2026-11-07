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
)

// listed is a list_files result's structured content.
type listed struct {
	Result struct {
		Structured struct {
			Dir     string `json:"dir"`
			Entries []struct {
				Name string `json:"name"`
				Dir  bool   `json:"dir"`
				Size int64  `json:"size"`
			} `json:"entries"`
		} `json:"structuredContent"`
	} `json:"result"`
}

// converseRaw is converse, but it returns each answer as the server
// wrote it, for the tests that read more of it than answer holds.
func converseRaw(t *testing.T, args []string, lines []string) (raw map[int][]byte, status int, stderr string) {
	t.Helper()
	toServer, clientWrites := io.Pipe()
	fromServer, serverWrites := io.Pipe()
	var errs bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- runMCP(context.Background(), append([]string{"telesapr-cli", "mcp"}, args...), toServer, serverWrites, &errs)
		_ = serverWrites.Close()
	}()
	raw = map[int][]byte{}
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
			var a struct {
				ID int `json:"id"`
			}
			if err := json.Unmarshal(scan.Bytes(), &a); err != nil {
				t.Fatalf("the server wrote %q: %v", scan.Text(), err)
			}
			if a.ID == *msg.ID {
				raw[a.ID] = append([]byte(nil), scan.Bytes()...)
				break
			}
		}
	}
	_ = clientWrites.Close()
	go func() { _, _ = io.Copy(io.Discard, fromServer) }()
	return raw, <-done, errs.String()
}

// A client's recorded conversation with the tools the bracket's does
// not use: open (refused while the drawing has unsaved changes),
// measure and its options, list_files, export to three formats, and
// render and get_drawing with theirs, over the real protocol against a
// headless server in a folder of its own.
func TestMCPTools(t *testing.T) {
	recorded, err := os.ReadFile(filepath.Join("..", "..", "testdata", "mcp", "tools.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	other, err := os.ReadFile(filepath.Join("..", "..", "testdata", "newfile.tlspr"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "other.tlspr"), other, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	raw, status, stderr := converseRaw(t, []string{"--headless"}, strings.Split(string(recorded), "\n"))
	if status != exitOK || stderr != "" {
		t.Fatalf("mcp --headless: status %d, stderr %q", status, stderr)
	}
	answers := map[int]answer{}
	for id, b := range raw {
		var a answer
		if err := json.Unmarshal(b, &a); err != nil {
			t.Fatal(err)
		}
		answers[id] = a
		if a.Error != nil {
			t.Errorf("request %d was answered with the error %q", id, a.Error.Message)
		}
		// The refusal of open is the one deliberate error.
		if a.Result.IsError != (id == 3) {
			t.Errorf("request %d: isError is %v:\n%s", id, a.Result.IsError, a.text())
		}
	}
	has := func(id int, want ...string) {
		t.Helper()
		got := answers[id].text()
		for _, w := range want {
			if !strings.Contains(got, w) {
				t.Errorf("request %d: want %q in\n%s", id, w, got)
			}
		}
	}
	value := func(id int) string {
		t.Helper()
		first, _, _ := strings.Cut(answers[id].text(), "\n")
		return strings.TrimPrefix(first, "value: ")
	}

	// The extrusion is active; open refuses to replace the drawing the
	// script changed.
	extrusion := value(2)
	if !strings.Contains(extrusion, "disc") {
		t.Errorf("the active group after the extrusion is %q", extrusion)
	}
	has(3, "changes that aren't saved")

	has(4, "Volume: ", "Extent: 10 x 10 x 10 mm")
	has(5, "wall")
	has(6, "Volume: ", "No two parts are nearer than 1 mm")

	// list_files: text and structured content agree.
	for _, id := range []int{7, 11} {
		var l listed
		if err := json.Unmarshal(raw[id], &l); err != nil {
			t.Fatal(err)
		}
		if l.Result.Structured.Dir == "" || l.Result.Structured.Entries == nil {
			t.Errorf("request %d has no structured listing: %s", id, raw[id])
		}
		names := map[string]int64{}
		for _, e := range l.Result.Structured.Entries {
			names[e.Name] = e.Size
		}
		if _, found := names["other.tlspr"]; !found {
			t.Errorf("request %d lists %v, without other.tlspr", id, names)
		}
		has(id, "other.tlspr")
		if id == 11 {
			for _, f := range []string{"disc.stl", "disc.step", "disc.go"} {
				if names[f] == 0 {
					t.Errorf("the listing after the exports has %s of %d bytes", f, names[f])
				}
			}
		}
	}

	// export: where, and how many bytes, which are the file's.
	for id, f := range map[int]string{8: "disc.stl", 9: "disc.step", 10: "disc.go"} {
		fi, err := os.Stat(f)
		if err != nil {
			t.Errorf("export %s: %v", f, err)
			continue
		}
		has(id, "wrote ", f, strconv.FormatInt(fi.Size(), 10)+" bytes")
	}

	// render with a group leaves the active group as it was.
	picture(t, answers[12], 320, 240)
	if got := value(13); got != extrusion {
		t.Errorf("after render of g002 the active group is %q, was %q", got, extrusion)
	}

	// render with a section leaves the user's section view as it was.
	before := value(14)
	if !strings.HasPrefix(before, "\"true") {
		t.Errorf("the section view was set to %q", before)
	}
	pic := picture(t, answers[15], 320, 240)
	if !strings.Contains(pic, "cut by a plane parallel to ZX") {
		t.Errorf("the caption of the cut is %q", pic)
	}
	if got := value(16); got != before {
		t.Errorf("after render with a section the section view is %q, was %q", got, before)
	}

	// Only the width: the height follows at 4:3.
	if pic := picture(t, answers[17], 160, 120); !strings.Contains(pic, "view") || !strings.Contains(pic, "pixels") {
		t.Errorf("the caption is %q", pic)
	}

	// The drawing as a script.
	if got := answers[18].text(); !strings.HasPrefix(got, "// The model, written by") || !strings.Contains(got, "package main") {
		t.Errorf("the drawing as a script starts %q", got[:min(len(got), 60)])
	}

	// Once the changes are let go, open works.
	has(19)
	has(20, "opened ", "other.tlspr")
}

// picture checks that a render's result is a PNG of w by h pixels
// with something drawn in it, and returns its caption.
func picture(t *testing.T, a answer, w, h int) string {
	t.Helper()
	c := a.Result.Content
	if len(c) != 2 || c[0].Type != "image" || c[0].MIMEType != "image/png" || c[1].Type != "text" {
		t.Fatalf("the picture has %d parts: %+v", len(c), a.Result.Content)
	}
	data, err := base64.StdEncoding.DecodeString(c[0].Data)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != w || b.Dy() != h {
		t.Errorf("the picture is %d by %d, want %d by %d", b.Dx(), b.Dy(), w, h)
	}
	corner := img.At(0, 0)
	for y := 0; y < h; y += 2 {
		for x := 0; x < w; x += 2 {
			if img.At(x, y) != corner {
				return c[1].Text
			}
		}
	}
	t.Errorf("the picture is all one colour")
	return c[1].Text
}
