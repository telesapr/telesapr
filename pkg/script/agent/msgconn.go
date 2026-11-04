package agent

import (
	"bytes"
	"io"
	"sync"
)

// This file is a connection made of whole messages, as a browser's
// MessagePort carries them: each line of the protocol, one JSON value,
// goes as one message, and each message that comes is read as a line.
// It is the browser's connection (socket_js.go), and has no part of the
// browser in it, so the desktop's tests try it too.

// MessageConn is a connection over something that carries whole
// messages: Deliver gives it what came, and what is written goes out
// through post a line at a time.
type MessageConn struct {
	post    func(string) error
	onClose func()

	mu     sync.Mutex
	more   *sync.Cond
	in     bytes.Buffer
	ended  bool
	out    bytes.Buffer
	closed bool
}

// NewMessageConn is a connection that sends each line written to it
// with post, without its newline, and calls onClose, if not nil, once
// when it is closed.
func NewMessageConn(post func(string) error, onClose func()) *MessageConn {
	c := &MessageConn{post: post, onClose: onClose}
	c.more = sync.NewCond(&c.mu)
	return c
}

// Deliver gives the connection a message that came: Read reads it as
// a line. It doesn't wait, so a browser's event handler may call it.
func (c *MessageConn) Deliver(msg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ended {
		return
	}
	c.in.WriteString(msg)
	c.in.WriteByte('\n')
	c.more.Broadcast()
}

// End says the other side has gone: Read returns io.EOF once what
// came before is read.
func (c *MessageConn) End() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ended = true
	c.more.Broadcast()
}

// Read reads what came, waiting for a message when there is none.
func (c *MessageConn) Read(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for c.in.Len() == 0 && !c.ended {
		c.more.Wait()
	}
	if c.in.Len() == 0 {
		return 0, io.EOF
	}
	return c.in.Read(p)
}

// Write sends each whole line of p as a message, and keeps what
// follows the last newline for the next Write.
func (c *MessageConn) Write(p []byte) (int, error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return 0, io.ErrClosedPipe
	}
	c.out.Write(p)
	var lines []string
	for {
		i := bytes.IndexByte(c.out.Bytes(), '\n')
		if i < 0 {
			break
		}
		lines = append(lines, string(c.out.Next(i)))
		c.out.Next(1)
	}
	c.mu.Unlock()
	// Not under the lock: post may call back into a browser.
	for _, line := range lines {
		if err := c.post(line); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

// Close ends the connection both ways: Read returns io.EOF, and Write
// an error.
func (c *MessageConn) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.ended = true
	c.in.Reset()
	c.more.Broadcast()
	c.mu.Unlock()
	if c.onClose != nil {
		c.onClose()
	}
	return nil
}
