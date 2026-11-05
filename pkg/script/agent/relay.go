//go:build !js

package agent

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
	"unicode/utf8"
)

// This file is the relay between telesapr-cli mcp and the browser
// version: a page has no socket, so a browser extension starts this
// program by native messaging (native.go), and the program listens on
// a socket as the window does. Each connection to the socket is
// carried to the extension, which carries it to the page's
// postMessage (socket_js.go). One native message is one of
//
//	{"conn": n, "op": "open"}                 a connection came (to the extension)
//	{"conn": n, "op": "ready"}                the page took it (from the extension)
//	{"conn": n, "op": "line", "text": t}      a line of the protocol, either way
//	{"conn": n, "op": "close", "error": e}    it ended, either way, and why if it failed
//
// A line to the extension longer than a native message may be is sent
// in parts, each but the last with "more": true. What the page answers
// is the protocol's; the relay reads only the ids of the requests sent
// before the page took the connection, to answer them with the error
// if it doesn't, and answers so those that come after, for a moment,
// before it closes the connection.

// The ops of a relay message.
const (
	relayOpen  = "open"
	relayReady = "ready"
	relayLine  = "line"
	relayClose = "close"
)

// relayPart is the most of a line one message carries: escaped as
// JSON, six bytes for one at worst, it stays under maxNativeOut.
const relayPart = 128 << 10

type relayMsg struct {
	Conn  int    `json:"conn"`
	Op    string `json:"op"`
	Text  string `json:"text,omitempty"`
	More  bool   `json:"more,omitempty"`
	Error string `json:"error,omitempty"`
}

// relayConn is a connection the relay carries.
type relayConn struct {
	conn net.Conn
	// ready is whether the page has taken it; until then, waiting are
	// the ids of the requests sent, which a refusal answers.
	ready   bool
	waiting []int
	// refusal is why the page didn't take it, once it said so.
	refusal string
	// part is a line that came in parts, so far.
	part []byte
	// wmu keeps the lines written to conn whole.
	wmu sync.Mutex
}

// write writes to the connection, a line or the refusals.
func (c *relayConn) write(p []byte) {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	_, _ = c.conn.Write(p)
}

// refusedFor is how long a connection the page refused is kept to
// answer the requests that still come with the refusal: a program
// waits for its first answer before it sends more.
const refusedFor = 2 * time.Second

// relay carries the connections of a listener to a browser extension.
type relay struct {
	// wmu keeps the messages to the extension whole; mu guards the rest.
	wmu   sync.Mutex
	out   io.Writer
	mu    sync.Mutex
	conns map[int]*relayConn
	next  int
	wg    sync.WaitGroup
}

// Relay carries each connection l takes to the browser extension that
// reads what is written to out and writes what is read from in, until
// in ends; it then closes l and the connections, and returns nil, or
// the error that ended in.
func Relay(l net.Listener, in io.Reader, out io.Writer) error {
	r := &relay{out: out, conns: map[int]*relayConn{}}
	r.wg.Go(func() { r.accept(l) })
	err := r.fromExtension(in)
	_ = l.Close()
	r.mu.Lock()
	for _, c := range r.conns {
		_ = c.conn.Close()
	}
	r.mu.Unlock()
	r.wg.Wait()
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

// send writes a message to the extension.
func (r *relay) send(m relayMsg) error {
	msg, err := json.Marshal(m)
	if err != nil {
		return err
	}
	r.wmu.Lock()
	defer r.wmu.Unlock()
	return writeNative(r.out, msg)
}

// accept takes connections until l is closed.
func (r *relay) accept(l net.Listener) {
	for {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		c := &relayConn{conn: conn}
		r.mu.Lock()
		r.next++
		id := r.next
		r.conns[id] = c
		r.mu.Unlock()
		if err := r.send(relayMsg{Conn: id, Op: relayOpen}); err != nil {
			_ = conn.Close()
			return
		}
		r.wg.Go(func() { r.toExtension(id, c) })
	}
}

// toExtension carries what comes on a connection to the extension.
func (r *relay) toExtension(id int, c *relayConn) {
	scan := bufio.NewScanner(c.conn)
	scan.Buffer(make([]byte, 0, 64<<10), maxLine)
	for scan.Scan() {
		line := scan.Bytes()
		if r.refused(c, line) {
			continue
		}
		if err := r.sendLine(id, line); err != nil {
			break
		}
	}
	r.mu.Lock()
	_, open := r.conns[id]
	delete(r.conns, id)
	told := c.refusal != ""
	r.mu.Unlock()
	_ = c.conn.Close()
	if open && !told {
		_ = r.send(relayMsg{Conn: id, Op: relayClose})
	}
}

// refused answers a request with the page's refusal if it refused the
// connection, and otherwise keeps its id while the page hasn't taken
// it yet.
func (r *relay) refused(c *relayConn, line []byte) bool {
	var req struct {
		ID int `json:"id"`
	}
	known := json.Unmarshal(line, &req) == nil
	r.mu.Lock()
	why := c.refusal
	if why == "" && !c.ready && known {
		c.waiting = append(c.waiting, req.ID)
	}
	r.mu.Unlock()
	if why == "" {
		return false
	}
	if known {
		refuse(c, []int{req.ID}, why)
	}
	return true
}

// sendLine sends a line, in parts if it is long, never splitting a
// character.
func (r *relay) sendLine(id int, line []byte) error {
	for {
		n := len(line)
		if n > relayPart {
			n = relayPart
			for n > 0 && !utf8.RuneStart(line[n]) {
				n--
			}
		}
		more := n < len(line)
		if err := r.send(relayMsg{Conn: id, Op: relayLine, Text: string(line[:n]), More: more}); err != nil {
			return err
		}
		if !more {
			return nil
		}
		line = line[n:]
	}
}

// fromExtension carries what the extension sends to the connections,
// until it ends.
func (r *relay) fromExtension(in io.Reader) error {
	for {
		raw, err := readNative(in)
		if err != nil {
			return err
		}
		var m relayMsg
		if err := json.Unmarshal(raw, &m); err != nil {
			return fmt.Errorf("not a relay message: %w", err)
		}
		r.mu.Lock()
		c := r.conns[m.Conn]
		if c == nil {
			r.mu.Unlock()
			continue
		}
		switch m.Op {
		case relayReady:
			c.ready, c.waiting = true, nil
			r.mu.Unlock()
		case relayLine:
			c.part = append(c.part, m.Text...)
			if m.More {
				r.mu.Unlock()
				continue
			}
			c.part = append(c.part, '\n')
			line := c.part
			c.part = nil
			r.mu.Unlock()
			c.write(line)
		case relayClose:
			if c.ready || c.refusal != "" {
				delete(r.conns, m.Conn)
				r.mu.Unlock()
				_ = c.conn.Close()
				continue
			}
			// Refused: the requests sent are answered, and those
			// still on their way when they come (refused).
			c.refusal = m.Error
			if c.refusal == "" {
				c.refusal = errBroken.Error()
			}
			waiting := c.waiting
			c.waiting = nil
			r.mu.Unlock()
			refuse(c, waiting, c.refusal)
			_ = c.conn.SetReadDeadline(time.Now().Add(refusedFor))
		default:
			r.mu.Unlock()
		}
	}
}

// refuse answers the requests sent before the page took the
// connection with why it didn't.
func refuse(c *relayConn, ids []int, why string) {
	for _, id := range ids {
		line, err := json.Marshal(response{ID: id, Error: why})
		if err == nil {
			c.write(append(line, '\n'))
		}
	}
}
