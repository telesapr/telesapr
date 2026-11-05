//go:build !js

package agent

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
)

// pictured is a Backend whose answers are long: a picture of n bytes,
// and a description of many characters of two bytes each.
type pictured struct {
	Backend
	png []byte
}

func (p pictured) Render(context.Context, Picture) ([]byte, error) { return p.png, nil }

func (pictured) Describe(_ context.Context, group string) (string, error) {
	return strings.Repeat("é", 100_000) + group, nil
}

// extension is a browser extension's end of the relay, with the page
// behind it: each connection the relay opens is served by b over a
// MessageConn, as the page serves one that comes by postMessage. With
// refusal set, the page refuses every connection with it.
type extension struct {
	b       Backend
	refusal string

	mu      sync.Mutex
	longest int
}

// run reads the relay's messages from from and answers on to until
// from ends.
func (e *extension) run(t *testing.T, from io.Reader, to io.Writer) {
	var wmu sync.Mutex
	send := func(m relayMsg) {
		msg, err := json.Marshal(m)
		if err != nil {
			t.Error(err)
			return
		}
		wmu.Lock()
		defer wmu.Unlock()
		// The browser's limit the other way is far higher; this side
		// only needs to frame it.
		buf := append([]byte{0, 0, 0, 0}, msg...)
		putNativeLen(buf, len(msg))
		_, _ = to.Write(buf)
	}
	pages := map[int]*MessageConn{}
	parts := map[int]string{}
	for {
		raw, err := readNative(from)
		if err != nil {
			return
		}
		e.mu.Lock()
		e.longest = max(e.longest, len(raw))
		e.mu.Unlock()
		var m relayMsg
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Errorf("not a relay message: %v", err)
			return
		}
		switch m.Op {
		case relayOpen:
			if e.refusal != "" {
				send(relayMsg{Conn: m.Conn, Op: relayClose, Error: e.refusal})
				continue
			}
			id := m.Conn
			page := NewMessageConn(func(line string) error {
				send(relayMsg{Conn: id, Op: relayLine, Text: line})
				return nil
			}, nil)
			pages[id] = page
			send(relayMsg{Conn: id, Op: relayReady})
			go func() { _ = Serve(context.Background(), page, e.b) }()
		case relayLine:
			parts[m.Conn] += m.Text
			if m.More {
				continue
			}
			if page := pages[m.Conn]; page != nil {
				page.Deliver(parts[m.Conn])
			}
			delete(parts, m.Conn)
		case relayClose:
			if page := pages[m.Conn]; page != nil {
				_ = page.Close()
				delete(pages, m.Conn)
			}
		}
	}
}

// putNativeLen writes n as a native message's length at the start of
// buf.
func putNativeLen(buf []byte, n int) {
	binary.NativeEndian.PutUint32(buf, uint32(n))
}

// startRelay runs a relay in dir to e, and returns the function that
// ends the extension's side, which waits for the relay to return.
func startRelay(t *testing.T, dir string, e *extension) (end func() error) {
	t.Helper()
	l, _, err := Listen(dir)
	if err != nil {
		t.Fatal(err)
	}
	toRelay, fromExt := io.Pipe()
	toExt, fromRelay := io.Pipe()
	done := make(chan error, 1)
	go func() {
		done <- Relay(l, toRelay, fromRelay)
		_ = fromRelay.Close()
	}()
	go e.run(t, toExt, fromExt)
	ended := false
	var result error
	end = func() error {
		if !ended {
			ended = true
			_ = fromExt.Close()
			result = <-done
		}
		return result
	}
	t.Cleanup(func() { _ = end() })
	return end
}

// A program reaches the page through the relay as it reaches the
// window through its socket, answers longer than a native message
// coming whole.
func TestRelay(t *testing.T) {
	t.Parallel()
	dir := socketDir(t)
	png := bytes.Repeat([]byte{0x89, 'P', 'N', 'G', 0, 0xff}, 600_000)
	e := &extension{b: pictured{png: png}}
	end := startRelay(t, dir, e)

	r := remote(dir)
	defer r.Close()
	ctx := context.Background()
	text, err := r.Describe(ctx, "bracket")
	if err != nil {
		t.Fatal(err)
	}
	if want, _ := (pictured{}).Describe(ctx, "bracket"); text != want {
		t.Errorf("the description came back %d bytes long, want %d", len(text), len(want))
	}
	got, err := r.Render(ctx, Picture{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, png) {
		t.Errorf("the picture came back %d bytes long, want %d", len(got), len(png))
	}
	e.mu.Lock()
	longest := e.longest
	e.mu.Unlock()
	if longest > maxNativeOut {
		t.Errorf("the relay sent a message of %d bytes; the browser takes %d at most", longest, maxNativeOut)
	}

	// The extension gone, the relay ends, and its socket with it.
	if err := end(); err != nil {
		t.Fatalf("Relay = %v, want nil when the extension ends", err)
	}
	if _, err := Connect(dir); !errors.Is(err, ErrNotRunning) {
		t.Errorf("Connect after the relay ended = %v, want ErrNotRunning", err)
	}
}

// A page that doesn't take the connection says why, and the program
// is told that.
func TestRelayRefused(t *testing.T) {
	t.Parallel()
	dir := socketDir(t)
	const why = "the page didn't answer: turn on Script > Allow AI Connections in it"
	startRelay(t, dir, &extension{b: describer{}, refusal: why})

	r := remote(dir)
	defer r.Close()
	if _, err := r.Describe(context.Background(), ""); err == nil || err.Error() != why {
		t.Errorf("Describe through a refused relay = %v, want %q", err, why)
	}
}

// Native messages are framed by their length, and one too long either
// way is refused.
func TestNativeFraming(t *testing.T) {
	t.Parallel()
	var b bytes.Buffer
	if err := writeNative(&b, []byte(`{"a":1}`)); err != nil {
		t.Fatal(err)
	}
	if b.Len() != 4+7 {
		t.Fatalf("a message of 7 bytes was written as %d", b.Len())
	}
	msg, err := readNative(&b)
	if err != nil || string(msg) != `{"a":1}` {
		t.Fatalf("readNative = %q, %v", msg, err)
	}
	if err := writeNative(io.Discard, make([]byte, maxNativeOut+1)); !errors.Is(err, errNativeTooLong) {
		t.Errorf("writeNative of a message too long = %v", err)
	}
	var long bytes.Buffer
	putLen := make([]byte, 4)
	putNativeLen(putLen, maxNativeIn+1)
	long.Write(putLen)
	if _, err := readNative(&long); !errors.Is(err, errNativeTooLong) {
		t.Errorf("readNative of a message too long = %v", err)
	}
	if _, err := readNative(bytes.NewReader([]byte{5, 0, 0, 0, 'a'})); err == nil {
		t.Error("readNative of a message cut short gave no error")
	}
	if _, err := readNative(bytes.NewReader(nil)); !errors.Is(err, io.EOF) {
		t.Errorf("readNative at the end = %v, want io.EOF", err)
	}
}
