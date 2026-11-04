//go:build js

package agent

import (
	"context"
	"errors"
	"io"
	"sync"
	"syscall/js"
)

// A browser has no sockets (leaving package net out keeps it out of the
// download, doc/dev-notes/browser.md). A program reaches the page
// instead through window.postMessage: it sends
//
//	{type: "telesapr-agent-connect"}
//
// with a MessagePort it keeps the other end of, and the page answers on
// that port {type: "telesapr-agent-ready", protocol} or
// {type: "telesapr-agent-refused", error}. After the ready, each message
// either way is one line of the protocol (wire.go) as a string, and
// {type: "telesapr-agent-close"} from the program ends the connection.
// A program of the page's own origin, such as an extension's script in
// the page, is let in; one of another origin only if the user says so.

// The types of the messages that aren't lines of the protocol.
const (
	msgConnect = "telesapr-agent-connect"
	msgReady   = "telesapr-agent-ready"
	msgRefused = "telesapr-agent-refused"
	msgClose   = "telesapr-agent-close"
)

// errNotAllowed is what a program of a site the user didn't let in is
// told.
var errNotAllowed = errors.New("the user didn't let this site connect")

// Listener is what Listen returns and Accept serves.
type Listener interface{ Close() error }

// Conn is a connection to the program with the drawing.
type Conn = io.ReadWriteCloser

// pageListener takes the connections that come to the page.
type pageListener struct {
	onMessage js.Func
	conns     chan *MessageConn
	done      chan struct{}

	mu sync.Mutex
	// allow asks the user whether a site may connect; nil lets in only
	// the page's own origin.
	allow func(origin string) bool
	// allowed are the origins let in, and refused those the user said
	// no to, so that each is asked once while connections are allowed.
	allowed, refused map[string]bool
	// asking are the origins the user is being asked about, each
	// closed when answered, so that a program that asks again while
	// the question is up waits for the one answer.
	asking map[string]chan struct{}
	// open are the connections there are, closed with the listener.
	open   map[*MessageConn]bool
	closed bool
}

// Listen listens for programs on the page's postMessage; the directory
// means nothing in a browser. What it returns as the socket's path
// says where programs connect.
func Listen(string) (Listener, string, error) {
	l := &pageListener{
		conns:   make(chan *MessageConn),
		done:    make(chan struct{}),
		allowed: map[string]bool{js.Global().Get("location").Get("origin").String(): true},
		refused: map[string]bool{},
		asking:  map[string]chan struct{}{},
		open:    map[*MessageConn]bool{},
	}
	l.onMessage = js.FuncOf(func(_ js.Value, args []js.Value) any {
		l.message(args[0])
		return nil
	})
	js.Global().Call("addEventListener", "message", l.onMessage)
	return l, "this page's window.postMessage", nil
}

// AllowOrigins has the listener ask allow whether a site other than
// the page's own may connect.
func (l *pageListener) AllowOrigins(allow func(origin string) bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.allow = allow
}

// message takes a connection that a message asks for. It runs in the
// browser's event handler, so it doesn't wait: asking the user is left
// to a goroutine.
func (l *pageListener) message(ev js.Value) {
	data := ev.Get("data")
	if data.Type() != js.TypeObject || data.Get("type").Type() != js.TypeString ||
		data.Get("type").String() != msgConnect {
		return
	}
	ports := ev.Get("ports")
	if ports.Type() != js.TypeObject || ports.Length() < 1 {
		return
	}
	port := ports.Index(0)
	origin := ev.Get("origin").String()
	go func() {
		if !l.let(origin) {
			reply(port, msgRefused, "error", errNotAllowed.Error())
			port.Call("close")
			return
		}
		l.connect(port)
	}()
}

// let is whether a program of origin may connect, asking the user once.
func (l *pageListener) let(origin string) bool {
	l.mu.Lock()
	for {
		if l.closed || l.refused[origin] {
			l.mu.Unlock()
			return false
		}
		if l.allowed[origin] {
			l.mu.Unlock()
			return true
		}
		wait, ok := l.asking[origin]
		if !ok {
			break
		}
		l.mu.Unlock()
		<-wait
		l.mu.Lock()
	}
	answered := make(chan struct{})
	l.asking[origin] = answered
	allow := l.allow
	l.mu.Unlock()
	ok := allow != nil && allow(origin)
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.asking, origin)
	close(answered)
	if ok {
		l.allowed[origin] = true
	} else {
		l.refused[origin] = true
	}
	return ok && !l.closed
}

// connect makes the connection through port and hands it to Accept.
func (l *pageListener) connect(port js.Value) {
	var onMessage js.Func
	conn := NewMessageConn(func(line string) error {
		port.Call("postMessage", line)
		return nil
	}, func() {
		port.Call("close")
		onMessage.Release()
	})
	onMessage = js.FuncOf(func(_ js.Value, args []js.Value) any {
		data := args[0].Get("data")
		switch data.Type() {
		case js.TypeString:
			conn.Deliver(data.String())
		case js.TypeObject:
			if t := data.Get("type"); t.Type() == js.TypeString && t.String() == msgClose {
				conn.End()
			}
		}
		return nil
	})
	port.Set("onmessage", onMessage)

	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		_ = conn.Close()
		return
	}
	l.open[conn] = true
	l.mu.Unlock()
	reply(port, msgReady, "protocol", Protocol)
	select {
	case l.conns <- conn:
	case <-l.done:
		_ = conn.Close()
	}
}

// reply posts on port a message of type typ with one more field.
func reply(port js.Value, typ, key string, value any) {
	msg := js.Global().Get("Object").New()
	msg.Set("type", typ)
	msg.Set(key, value)
	port.Call("postMessage", msg)
}

// Close stops taking connections and ends those there are.
func (l *pageListener) Close() error {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return nil
	}
	l.closed = true
	open := l.open
	l.open = nil
	l.mu.Unlock()
	js.Global().Call("removeEventListener", "message", l.onMessage)
	l.onMessage.Release()
	close(l.done)
	for conn := range open {
		_ = conn.Close()
	}
	return nil
}

// forget drops a connection that ended from those Close ends.
func (l *pageListener) forget(conn *MessageConn) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.open, conn)
}

// Accept serves b on every connection the page takes, until the
// listener is closed; it then ends the connections it has, and returns
// when they have ended.
func Accept(ctx context.Context, l Listener, b Backend) {
	pl, ok := l.(*pageListener)
	if !ok {
		return
	}
	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	defer func() {
		cancel()
		wg.Wait()
	}()
	for {
		select {
		case <-pl.done:
			return
		case conn := <-pl.conns:
			wg.Go(func() {
				stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
				defer stop()
				_ = Serve(ctx, conn, b)
				_ = conn.Close()
				pl.forget(conn)
			})
		}
	}
}

// Connect returns ErrNotRunning: nothing can be reached from a browser.
func Connect(string) (Conn, error) { return nil, ErrNotRunning }

// ConnectTo returns ErrNotRunning: nothing can be reached from a browser.
func ConnectTo(string) (Conn, error) { return nil, ErrNotRunning }
