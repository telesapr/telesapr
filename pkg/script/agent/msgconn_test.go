package agent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

// describer is a Backend that answers Describe with what it was asked.
type describer struct{ Backend }

func (describer) Describe(_ context.Context, group string) (string, error) {
	return "group " + group, nil
}

// TestMessageConnServe serves a Backend over a MessageConn as the
// browser does: each request a message, each answer a message.
func TestMessageConnServe(t *testing.T) {
	t.Parallel()
	posted := make(chan string, 8)
	closed := make(chan struct{})
	conn := NewMessageConn(func(line string) error {
		posted <- line
		return nil
	}, func() { close(closed) })

	served := make(chan error, 1)
	go func() { served <- Serve(context.Background(), conn, describer{}) }()

	next := func() response {
		t.Helper()
		select {
		case line := <-posted:
			if strings.Contains(line, "\n") {
				t.Fatalf("a message holds a newline: %q", line)
			}
			var r response
			if err := json.Unmarshal([]byte(line), &r); err != nil {
				t.Fatalf("answer %q: %v", line, err)
			}
			return r
		case <-time.After(10 * time.Second):
			t.Fatal("no answer")
		}
		return response{}
	}

	conn.Deliver(`{"id":1,"op":"hello","protocol":2}`)
	if r := next(); r.ID != 1 || r.Protocol != Protocol {
		t.Fatalf("hello answered %+v", r)
	}
	conn.Deliver(`{"id":2,"op":"describe","group":"g001"}`)
	if r := next(); r.ID != 2 || r.Text != "group g001" || r.Error != "" {
		t.Fatalf("describe answered %+v", r)
	}
	conn.Deliver(`not json`)
	if r := next(); !strings.HasPrefix(r.Error, "not a request") {
		t.Fatalf("garbage answered %+v", r)
	}

	conn.End()
	select {
	case err := <-served:
		if err != nil {
			t.Fatalf("Serve ended with %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Serve didn't end when the other side went")
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	<-closed
	if err := conn.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

// TestMessageConnLines checks that what is written goes out a whole
// line at a time, whatever the writes' sizes, and that a closed
// connection reads EOF and refuses writes.
func TestMessageConnLines(t *testing.T) {
	t.Parallel()
	var got []string
	conn := NewMessageConn(func(line string) error {
		got = append(got, line)
		return nil
	}, nil)
	for _, part := range []string{`{"a":`, `1}` + "\n" + `{"b"`, `:2}` + "\n\n"} {
		if _, err := conn.Write([]byte(part)); err != nil {
			t.Fatal(err)
		}
	}
	if want := []string{`{"a":1}`, `{"b":2}`, ``}; strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("posted %q, want %q", got, want)
	}

	failing := NewMessageConn(func(string) error { return errors.New("port gone") }, nil)
	if _, err := failing.Write([]byte("x\n")); err == nil {
		t.Fatal("a failed post wasn't an error")
	}

	conn.Deliver("one")
	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	if err != nil || string(buf[:n]) != "one\n" {
		t.Fatalf("read %q, %v", buf[:n], err)
	}
	_ = conn.Close()
	if _, err := conn.Read(buf); !errors.Is(err, io.EOF) {
		t.Fatalf("read after Close: %v", err)
	}
	if _, err := conn.Write([]byte("x\n")); err == nil {
		t.Fatal("write after Close wasn't an error")
	}
	conn.Deliver("late")
	if _, err := conn.Read(buf); !errors.Is(err, io.EOF) {
		t.Fatalf("a message after Close was read: %v", err)
	}
}
