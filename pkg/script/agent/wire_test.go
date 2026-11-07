//go:build !js

package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

// socketDir is a directory for sockets with a name short enough for
// one on every system; a test's own temporary directory may not be.
func socketDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "ss")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// serve listens in dir and serves b there; the returned function ends
// it, with its connections.
func serve(t *testing.T, dir string, b Backend) (stop func()) {
	t.Helper()
	l, _, err := Listen(dir)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		Accept(context.Background(), l, b)
	}()
	stopped := false
	stop = func() {
		if !stopped {
			stopped = true
			_ = l.Close()
			<-done
		}
	}
	t.Cleanup(stop)
	return stop
}

func remote(dir string) *Remote {
	return &Remote{Connect: func() (net.Conn, error) { return Connect(dir) }}
}

// A session reached through a socket does what it does when called.
func TestRemoteSession(t *testing.T) {
	t.Parallel()
	dir := socketDir(t)
	s := newSession(t)
	serve(t, dir, s)
	r := remote(dir)
	defer r.Close()
	ctx := context.Background()

	res, err := r.Run(ctx, "c := doc.Sketch.Circle(V(0, 0, 0), 5)\nfmt.Println(\"drawn\")\nc.Radius()")
	if err != nil {
		t.Fatal(err)
	}
	want := Result{Output: "drawn\n", Value: "5", Status: "active group g002-sketch-in-plane, sketch-in-plane: solved, 3 DOF",
		Groups: []GroupStatus{{Name: "sketch-in-plane", Kind: "sketch-in-plane", Active: true, Solved: true, DOF: 3}}}
	if !reflect.DeepEqual(res, want) {
		t.Errorf("the run gave %+v, want %+v", res, want)
	}
	if res, err = r.Run(ctx, "doc.Features.Extrude(2, \"disc\")\nc.Length()\nundefined()"); err != nil || !res.RolledBack || res.Error == "" {
		t.Errorf("a failing script gave %+v, %v", res, err)
	}

	text, err := describeText(r.Describe(ctx, Describing{}))
	if direct, _ := describeText(s.Describe(ctx, Describing{})); err != nil || text != direct || !strings.Contains(text, "r004-circle") {
		t.Errorf("the drawing is described as %q, %v; the session says %q", text, err, direct)
	}
	if text, err = describeText(r.Describe(ctx, Describing{Script: true})); err != nil || !strings.Contains(text, "package main") {
		t.Errorf("the drawing as a script through the wire is %q, %v", text, err)
	}
	if text, err = describeText(r.Describe(ctx, Describing{Solid: true})); err != nil || text != "no solid yet at g002-sketch-in-plane\n" {
		t.Errorf("the solid through the wire is %q, %v", text, err)
	}
	if _, err = r.Describe(ctx, Describing{Group: "nothing"}); err == nil || !strings.Contains(err.Error(), `no group is called "nothing"`) {
		t.Errorf("describing a group that isn't there: %v", err)
	}
	if _, err = r.Render(ctx, Picture{View: "top", Width: 40, Height: 30}); err == nil || !strings.Contains(err.Error(), "not available") {
		t.Errorf("a picture of a sketch without the application: %v", err)
	}
	if text, err = r.Help(ctx, "Group.Dof"); err != nil || !strings.Contains(text, "func (g Group) Dof() (dof int)") {
		t.Errorf("help of Group.Dof is %q, %v", text, err)
	}

	// Measuring, exporting, opening and listing go through as well.
	if res, err = r.Run(ctx, "doc.Features.Extrude(2, \"disc\")"); err != nil || res.Error != "" {
		t.Fatalf("the extrusion gave %+v, %v", res, err)
	}
	if text, err = measureText(r.Measure(ctx, Measuring{})); err != nil || !strings.Contains(text, "the model up to g003-disc") {
		t.Errorf("the disc measures as %q, %v", text, err)
	}
	if text, err = measureText(r.Measure(ctx, Measuring{Gaps: 1, Walls: 3})); err != nil || !strings.Contains(text, "No two parts are nearer than 1 mm.") ||
		!strings.Contains(text, "The thinnest wall is 2 mm") || !strings.Contains(text, "points measured are thinner than 3 mm") {
		t.Errorf("the disc's gaps and walls are %q, %v", text, err)
	}
	// The data crosses the wire as it left the session.
	for _, d := range []Describing{{}, {Solid: true}, {Group: "g002"}, {Script: true}} {
		far, err := r.Describe(ctx, d)
		near, _ := s.Describe(ctx, d)
		if err != nil || !reflect.DeepEqual(far, near) {
			t.Errorf("described %+v through the wire as %+v, %v; the session says %+v", d, far.Data, err, near.Data)
		}
	}
	if d, _ := r.Describe(ctx, Describing{Solid: true}); len(d.Data.Faces) != 3 || d.Data.Solid == nil || !d.Data.Solid.Sound {
		t.Errorf("the disc's solid through the wire is %+v", d.Data)
	}
	far, err := r.Measure(ctx, Measuring{Gaps: 1, Walls: 3})
	if near, _ := s.Measure(ctx, Measuring{Gaps: 1, Walls: 3}); err != nil || !reflect.DeepEqual(far, near) ||
		far.Data.Group != "g003-disc" || !far.Data.Sound || far.Data.Walls == nil || far.Data.Walls.Thinnest != 2 {
		t.Errorf("the disc measures through the wire as %+v, %v; the session says %+v", far.Data, err, near.Data)
	}
	if _, err = r.Measure(ctx, Measuring{Selected: true}); err == nil || !strings.Contains(err.Error(), "nothing is selected") {
		t.Errorf("measuring no selection: %v", err)
	}
	files := t.TempDir()
	stl := filepath.Join(files, "disc.stl")
	if text, err = r.Export(ctx, stl); err != nil || !strings.HasPrefix(text, "wrote "+stl) {
		t.Errorf("the export says %q, %v", text, err)
	}
	if _, err = r.Open(ctx, filepath.Join(files, "none.tlspr")); err == nil || err.Error() != ErrUnsaved.Error() {
		t.Errorf("opening over unsaved changes: %v", err)
	}
	l, err := r.Files(ctx, files)
	if err != nil || l.Dir != files || len(l.Entries) != 1 || l.Entries[0].Name != "disc.stl" || l.Entries[0].Size == 0 {
		t.Errorf("the directory is listed as %+v, %v", l, err)
	}
	if _, err = r.Files(ctx, filepath.Join(files, "none")); err == nil {
		t.Errorf("listing a directory that isn't there is no error")
	}
	if _, err = r.Run(ctx, ""); err == nil {
		t.Errorf("running nothing is no error")
	}
}

// The other program may start later, end, and start again; help
// needs none.
func TestRemoteConnectsWhenNeeded(t *testing.T) {
	t.Parallel()
	dir := socketDir(t)
	r := remote(dir)
	defer r.Close()
	ctx := context.Background()

	if _, err := r.Describe(ctx, Describing{}); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("with nothing listening: %v, want ErrNotRunning", err)
	}
	if text, err := r.Help(ctx, ""); err != nil || text != Primer() {
		t.Errorf("help with nothing listening: %v", err)
	}

	stop := serve(t, dir, newSession(t))
	if res, err := r.Run(ctx, "x := 1\nx"); err != nil || res.Value != "1" {
		t.Fatalf("once something listens: %+v, %v", res, err)
	}
	stop()
	if _, err := r.Run(ctx, "x"); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("after it ended: %v, want ErrNotRunning", err)
	}

	// Another program, with another drawing and nothing defined.
	serve(t, dir, newSession(t))
	if res, err := r.Run(ctx, "x"); err != nil || res.Error == "" {
		t.Errorf("x is known to a program started anew: %+v, %v", res, err)
	}
	if res, err := r.Run(ctx, "2 + 2"); err != nil || res.Value != "4" {
		t.Errorf("the program started anew gave %+v, %v", res, err)
	}
}

// A request that is cancelled stops the script, which is rolled back.
func TestRemoteCancel(t *testing.T) {
	t.Parallel()
	dir := socketDir(t)
	s := newSession(t)
	serve(t, dir, s)
	r := remote(dir)
	r.CancelWait = 10 * time.Second
	defer r.Close()

	drawn := filepath.Join(dir, "drawn")
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		for {
			if _, err := os.Stat(drawn); err == nil {
				break
			}
			time.Sleep(time.Millisecond)
		}
		cancel()
	}()
	res, err := r.Run(ctx, fmt.Sprintf("doc.Sketch.Circle(V(0, 0, 0), 1)\nos.WriteFile(%q, nil, 0o644)\nfor {\n\ttime.Sleep(time.Millisecond)\n}", drawn))
	if err != nil || res.Error != "the script was stopped" || !res.RolledBack {
		t.Errorf("the cancelled run gave %+v, %v", res, err)
	}
	if res, err = r.Run(context.Background(), "len(doc.Active().Requests())"); err != nil || res.Value != "0" {
		t.Errorf("after the cancelled run: %+v, %v", res, err)
	}
}

// slow is a Backend whose Run doesn't end until it is cancelled, and
// then takes its time.
type slow struct{ Backend }

func (slow) Run(ctx context.Context, _ string) (Result, error) {
	<-ctx.Done()
	time.Sleep(300 * time.Millisecond)
	return Result{Error: "stopped late"}, nil
}

// A cancelled request doesn't wait for ever for what came of it.
func TestRemoteCancelGivesUp(t *testing.T) {
	t.Parallel()
	dir := socketDir(t)
	serve(t, dir, slow{})
	r := remote(dir)
	r.CancelWait = 20 * time.Millisecond
	defer r.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := r.Run(ctx, "x"); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("the cancelled run gave %v, want the deadline", err)
	}
}

func TestSockets(t *testing.T) {
	dir := filepath.Join(socketDir(t), "made")
	l, path, err := Listen(dir)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		for name, want := range map[string]os.FileMode{dir: 0o700, path: 0o600} {
			info, err := os.Stat(name)
			if err != nil {
				t.Fatal(err)
			}
			if got := info.Mode().Perm(); got != want {
				t.Errorf("%s has mode %o, want %o", name, got, want)
			}
		}
	}

	// An older socket that is listened on, and a newer one that
	// isn't: the first is connected to, the second removed.
	stale := filepath.Join(dir, socketPrefix+"stale"+socketSuffix)
	sl, err := net.Listen("unix", stale)
	if err != nil {
		t.Fatal(err)
	}
	sl.(*net.UnixListener).SetUnlinkOnClose(false)
	_ = sl.Close()
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(stale, later, later); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	conn, err := Connect(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("the socket nothing listens on is still there: %v", err)
	}

	if conn, err = ConnectTo(path); err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
	_ = l.Close()
	if _, err := Connect(dir); !errors.Is(err, ErrNotRunning) {
		t.Errorf("connecting after the listener closed: %v, want ErrNotRunning", err)
	}
	if _, err := ConnectTo(path); !errors.Is(err, ErrNotRunning) {
		t.Errorf("connecting to the closed socket: %v, want ErrNotRunning", err)
	}
	if _, err := Connect(filepath.Join(dir, "nowhere")); !errors.Is(err, ErrNotRunning) {
		t.Errorf("connecting in a directory that isn't there: %v, want ErrNotRunning", err)
	}

	long := filepath.Join(dir, strings.Repeat("d", maxSocketPath))
	if _, _, err := Listen(long); err == nil || !strings.Contains(err.Error(), "bytes long") {
		t.Errorf("listening on a name too long: %v", err)
	}

	if got, err := SocketDir(); err != nil || !strings.Contains(got, "telesapr") {
		t.Errorf("the sockets' directory is %q, %v", got, err)
	}
	if runtime.GOOS == "linux" {
		t.Setenv("XDG_RUNTIME_DIR", "/run/user/1")
		if got, _ := SocketDir(); got != "/run/user/1/telesapr" {
			t.Errorf("with XDG_RUNTIME_DIR the sockets' directory is %q", got)
		}
		t.Setenv("XDG_RUNTIME_DIR", "")
		if got, _ := SocketDir(); !strings.HasPrefix(filepath.Base(got), "telesapr-") {
			t.Errorf("without XDG_RUNTIME_DIR the sockets' directory is %q", got)
		}
	}
}

// exchange sends lines to a Serve of b and returns the responses.
func exchange(t *testing.T, b Backend, lines ...string) []response {
	t.Helper()
	client, server := net.Pipe()
	done := make(chan error, 1)
	go func() { done <- Serve(context.Background(), server, b) }()
	var out []response
	scan := bufio.NewScanner(client)
	for _, line := range lines {
		if _, err := fmt.Fprintln(client, line); err != nil {
			t.Fatal(err)
		}
		if !scan.Scan() {
			t.Fatalf("no response to %s", line)
		}
		var res response
		if err := json.Unmarshal(scan.Bytes(), &res); err != nil {
			t.Fatal(err)
		}
		out = append(out, res)
	}
	_ = client.Close()
	_ = server.Close()
	<-done
	return out
}

// What isn't a request is answered with an error, and the next one
// is still served.
func TestServeBadRequests(t *testing.T) {
	t.Parallel()
	got := exchange(t, newSession(t),
		`{"id":1,"op":"hello","protocol":1}`,
		`not json`,
		`{"id":3,"op":"dance"}`,
		`{"id":4,"op":"help","name":"Sketch.Line"}`,
	)
	if got[0].ID != 1 || got[0].Protocol != Protocol {
		t.Errorf("hello was answered %+v", got[0])
	}
	if !strings.HasPrefix(got[1].Error, "not a request") {
		t.Errorf("a line that isn't JSON was answered %+v", got[1])
	}
	if got[2].ID != 3 || got[2].Error != `no operation "dance"` {
		t.Errorf("an operation there isn't was answered %+v", got[2])
	}
	if got[3].ID != 4 || !strings.Contains(got[3].Text, "func (m SketchMenu) Line(a, b Vec) Entity") {
		t.Errorf("help was answered %+v", got[3])
	}
}

// A program that speaks another version of the protocol is told so.
func TestRemoteProtocolMismatch(t *testing.T) {
	t.Parallel()
	dir := socketDir(t)
	l, _, err := Listen(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = conn.Close() }()
				scan := bufio.NewScanner(conn)
				for scan.Scan() {
					var req request
					_ = json.Unmarshal(scan.Bytes(), &req)
					_ = json.NewEncoder(conn).Encode(response{ID: req.ID, Protocol: Protocol + 1})
				}
			}()
		}
	}()
	r := remote(dir)
	defer r.Close()
	if _, err := r.Describe(context.Background(), Describing{}); err == nil || !strings.Contains(err.Error(), "install the same version") {
		t.Errorf("a program of another version: %v", err)
	}
}

// twice is a Backend whose run reports twice before it ends.
type twice struct{ Backend }

func (twice) Run(ctx context.Context, code string) (Result, error) {
	return twice{}.RunWithProgress(ctx, code, nil)
}

func (twice) RunWithProgress(_ context.Context, code string, report func(Progress)) (Result, error) {
	if report != nil {
		report(Progress{Message: "regenerating g002-a, 1 of 2", Done: 1, Total: 2})
		report(Progress{Message: "printed", Output: "printed\n"})
	}
	return Result{Output: "printed\n", Value: code}, nil
}

// The reports of a run come through the connection to the caller, in
// order and before the result; a caller with no callback, or a
// program that sends progress for a request it didn't make, has them
// dropped.
func TestRemoteProgress(t *testing.T) {
	t.Parallel()
	dir := socketDir(t)
	serve(t, dir, twice{})
	r := remote(dir)
	defer r.Close()
	ctx := context.Background()

	var got []Progress
	res, err := r.RunWithProgress(ctx, "x", func(p Progress) { got = append(got, p) })
	if err != nil || res.Value != "x" {
		t.Fatalf("the run gave %+v, %v", res, err)
	}
	want := []Progress{{Message: "regenerating g002-a, 1 of 2", Done: 1, Total: 2}, {Message: "printed", Output: "printed\n"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("the reports were %+v, want %+v", got, want)
	}
	if res, err := r.Run(ctx, "y"); err != nil || res.Value != "y" {
		t.Errorf("a run with no callback gave %+v, %v", res, err)
	}

	// On the wire: progress lines with the run's id, then the response.
	lines := exchange(t, twice{}, `{"id":7,"op":"run","code":"z"}`)
	if len(lines) != 1 || lines[0].Op != opProgress || lines[0].ID != 7 || lines[0].Progress == nil || lines[0].Progress.Done != 1 {
		t.Errorf("the first line of a run is %+v, want its progress", lines)
	}
}
