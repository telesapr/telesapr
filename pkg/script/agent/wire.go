package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// This file is a Backend through a connection: the program that has
// the drawing serves its Backend with Serve, and another reaches it
// with a Remote. Each side writes one JSON value to a line: a request
// with an id, and the response with that id. A request may be sent
// before the last was answered, which is how one that runs is
// cancelled.

// Protocol is the version of what goes over the connection; both
// sides must have the same. Version 2 added measure, export, open and
// files, and the section of a picture.
const Protocol = 2

// The operations of a request.
const (
	opHello    = "hello"
	opRun      = "run"
	opDescribe = "describe"
	opRender   = "render"
	opHelp     = "help"
	opMeasure  = "measure"
	opExport   = "export"
	opOpen     = "open"
	opFiles    = "files"
	opCancel   = "cancel"
)

type request struct {
	ID int    `json:"id"`
	Op string `json:"op"`
	// Protocol is the caller's, in a hello.
	Protocol int `json:"protocol,omitempty"`
	// Code is what run runs.
	Code string `json:"code,omitempty"`
	// Group is what describe describes, the whole drawing for "", and
	// the group measure measures up to.
	Group string `json:"group,omitempty"`
	// Picture is what render draws.
	Picture *Picture `json:"picture,omitempty"`
	// Selected has measure measure the selection.
	Selected bool `json:"selected,omitempty"`
	// Gaps, Walls and Within are what else measure measures
	// (Measuring).
	Gaps   float64 `json:"gaps,omitempty"`
	Walls  float64 `json:"walls,omitempty"`
	Within bool    `json:"within,omitempty"`
	// Path is the file export writes and open reads, or the directory
	// files lists.
	Path string `json:"path,omitempty"`
	// Name is what help is asked about.
	Name string `json:"name,omitempty"`
	// Target is the id of the request cancel stops.
	Target int `json:"target,omitempty"`
}

type response struct {
	ID int `json:"id"`
	// Error is why the request couldn't be done.
	Error string `json:"error,omitempty"`
	// Protocol answers a hello.
	Protocol int `json:"protocol,omitempty"`
	// Result answers run; Text describe, help, measure, export and
	// open; PNG render; Listing files.
	Result  *Result  `json:"result,omitempty"`
	Text    string   `json:"text,omitempty"`
	PNG     []byte   `json:"png,omitempty"`
	Listing *Listing `json:"listing,omitempty"`
}

// maxLine is the longest line either side reads: a picture of
// MaxSide squared, in base64, fits with room to spare.
const maxLine = 64 << 20

// Serve answers the requests that come over conn with b until the
// connection ends, and then stops what still runs. It returns the
// error that ended it, nil when the other side closed.
func Serve(ctx context.Context, conn io.ReadWriter, b Backend) error {
	ctx, cancelAll := context.WithCancel(ctx)
	defer cancelAll()

	var (
		mu      sync.Mutex
		running = map[int]context.CancelFunc{}
		wg      sync.WaitGroup
	)
	defer wg.Wait()
	enc := json.NewEncoder(conn)
	reply := func(r response) {
		mu.Lock()
		defer mu.Unlock()
		_ = enc.Encode(r)
	}

	scan := bufio.NewScanner(conn)
	scan.Buffer(make([]byte, 0, 64<<10), maxLine)
	for scan.Scan() {
		var req request
		if err := json.Unmarshal(scan.Bytes(), &req); err != nil {
			reply(response{Error: "not a request: " + err.Error()})
			continue
		}
		switch req.Op {
		case opHello:
			reply(response{ID: req.ID, Protocol: Protocol})
			continue
		case opCancel:
			mu.Lock()
			if cancel := running[req.Target]; cancel != nil {
				cancel()
			}
			mu.Unlock()
			continue
		}

		rctx, cancel := context.WithCancel(ctx)
		mu.Lock()
		running[req.ID] = cancel
		mu.Unlock()
		wg.Go(func() {
			res := answer(rctx, b, req)
			mu.Lock()
			delete(running, req.ID)
			mu.Unlock()
			cancel()
			reply(res)
		})
	}
	cancelAll()
	return scan.Err()
}

// answer does what a request asks of a Backend.
func answer(ctx context.Context, b Backend, req request) response {
	res := response{ID: req.ID}
	var err error
	switch req.Op {
	case opRun:
		var r Result
		if r, err = b.Run(ctx, req.Code); err == nil {
			res.Result = &r
		}
	case opDescribe:
		res.Text, err = b.Describe(ctx, req.Group)
	case opRender:
		var p Picture
		if req.Picture != nil {
			p = *req.Picture
		}
		res.PNG, err = b.Render(ctx, p)
	case opHelp:
		res.Text, err = b.Help(ctx, req.Name)
	case opMeasure:
		res.Text, err = b.Measure(ctx, Measuring{Group: req.Group, Selected: req.Selected,
			Gaps: req.Gaps, Walls: req.Walls, Within: req.Within})
	case opExport:
		res.Text, err = b.Export(ctx, req.Path)
	case opOpen:
		res.Text, err = b.Open(ctx, req.Path)
	case opFiles:
		var l Listing
		if l, err = b.Files(ctx, req.Path); err == nil {
			res.Listing = &l
		}
	default:
		err = fmt.Errorf("no operation %q", req.Op)
	}
	if err != nil {
		res.Error = err.Error()
	}
	return res
}

// ErrNotRunning is the error of a Remote that finds no program to
// talk to.
var ErrNotRunning = errors.New("TeleSAPR isn't running, or Script > Allow AI Connections is off in it; ask the user to start TeleSAPR and turn that on")

// Remote is the Backend of another program, reached through a
// connection that is made when first needed and again when it broke:
// the other program may start after this one, and be started anew.
type Remote struct {
	// Connect makes a connection to the program; its error should be,
	// or wrap, ErrNotRunning when there is none to connect to.
	Connect func() (net.Conn, error)
	// CancelWait is how long a cancelled request waits for the
	// program to say what came of it; a second if zero.
	CancelWait time.Duration

	mu      sync.Mutex
	conn    net.Conn
	enc     *json.Encoder
	next    int
	pending map[int]chan response
}

var _ Backend = (*Remote)(nil)

// link is the connection, made if there is none.
func (r *Remote) link() (net.Conn, error) {
	r.mu.Lock()
	if r.conn != nil {
		defer r.mu.Unlock()
		return r.conn, nil
	}
	r.mu.Unlock()

	conn, err := r.Connect()
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	if r.conn != nil {
		// Another call connected meanwhile.
		r.mu.Unlock()
		_ = conn.Close()
		return r.link()
	}
	r.conn, r.enc, r.pending = conn, json.NewEncoder(conn), map[int]chan response{}
	r.mu.Unlock()
	go r.read(conn)

	// The two programs are installed together, but one may be older.
	res, err := r.roundTrip(context.Background(), conn, request{Op: opHello, Protocol: Protocol})
	if err == nil && res.Protocol != Protocol {
		err = fmt.Errorf("TeleSAPR speaks version %d of the protocol and telesapr-cli version %d; install the same version of both", res.Protocol, Protocol)
	}
	if err != nil {
		r.drop(conn)
		return nil, err
	}
	return conn, nil
}

// read gives each response to the call that waits for it, and when
// the connection ends, ends them all.
func (r *Remote) read(conn net.Conn) {
	scan := bufio.NewScanner(conn)
	scan.Buffer(make([]byte, 0, 64<<10), maxLine)
	for scan.Scan() {
		var res response
		if err := json.Unmarshal(scan.Bytes(), &res); err != nil {
			continue
		}
		r.mu.Lock()
		ch := r.pending[res.ID]
		delete(r.pending, res.ID)
		r.mu.Unlock()
		if ch != nil {
			ch <- res
		}
	}
	r.drop(conn)
}

// drop forgets a connection that ended, and fails the calls that wait
// on it.
func (r *Remote) drop(conn net.Conn) {
	_ = conn.Close()
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.conn != conn {
		return
	}
	for id, ch := range r.pending {
		close(ch)
		delete(r.pending, id)
	}
	r.conn, r.enc = nil, nil
}

// errBroken is the error of a request whose connection ended before
// the answer came.
var errBroken = errors.New("the connection to TeleSAPR ended")

// roundTrip sends a request over conn and waits for its response.
func (r *Remote) roundTrip(ctx context.Context, conn net.Conn, req request) (response, error) {
	r.mu.Lock()
	if r.conn != conn {
		r.mu.Unlock()
		return response{}, errBroken
	}
	r.next++
	req.ID = r.next
	ch := make(chan response, 1)
	r.pending[req.ID] = ch
	err := r.enc.Encode(req)
	r.mu.Unlock()
	if err != nil {
		r.drop(conn)
		return response{}, errBroken
	}

	select {
	case res, ok := <-ch:
		if !ok {
			return response{}, errBroken
		}
		return res, nil
	case <-ctx.Done():
	}
	// The program is told to stop, and has a moment to say what came
	// of it: a script that was stopped is rolled back there.
	r.mu.Lock()
	if r.conn == conn {
		_ = r.enc.Encode(request{Op: opCancel, Target: req.ID})
	}
	r.mu.Unlock()
	wait := r.CancelWait
	if wait == 0 {
		wait = time.Second
	}
	select {
	case res, ok := <-ch:
		if ok {
			return res, nil
		}
		return response{}, errBroken
	case <-time.After(wait):
		r.mu.Lock()
		delete(r.pending, req.ID)
		r.mu.Unlock()
		return response{}, ctx.Err()
	}
}

// call does a request, on a new connection if the one there turns out
// to have ended: the program may have been started anew since the
// last call.
func (r *Remote) call(ctx context.Context, req request) (response, error) {
	for attempt := 0; ; attempt++ {
		conn, err := r.link()
		if err != nil {
			return response{}, err
		}
		res, err := r.roundTrip(ctx, conn, req)
		switch {
		case errors.Is(err, errBroken) && attempt == 0:
			continue
		case err != nil:
			return response{}, err
		case res.Error != "":
			return response{}, errors.New(res.Error)
		}
		return res, nil
	}
}

// Close ends the connection, if there is one; the next call makes
// another.
func (r *Remote) Close() {
	r.mu.Lock()
	conn := r.conn
	r.mu.Unlock()
	if conn != nil {
		r.drop(conn)
	}
}

func (r *Remote) Run(ctx context.Context, code string) (Result, error) {
	res, err := r.call(ctx, request{Op: opRun, Code: code})
	if err != nil {
		return Result{}, err
	}
	if res.Result == nil {
		return Result{}, errors.New("TeleSAPR answered a run with no result")
	}
	return *res.Result, nil
}

func (r *Remote) Describe(ctx context.Context, group string) (string, error) {
	res, err := r.call(ctx, request{Op: opDescribe, Group: group})
	return res.Text, err
}

func (r *Remote) Render(ctx context.Context, p Picture) ([]byte, error) {
	res, err := r.call(ctx, request{Op: opRender, Picture: &p})
	return res.PNG, err
}

func (r *Remote) Measure(ctx context.Context, m Measuring) (string, error) {
	res, err := r.call(ctx, request{Op: opMeasure, Group: m.Group, Selected: m.Selected,
		Gaps: m.Gaps, Walls: m.Walls, Within: m.Within})
	return res.Text, err
}

func (r *Remote) Export(ctx context.Context, path string) (string, error) {
	res, err := r.call(ctx, request{Op: opExport, Path: path})
	return res.Text, err
}

func (r *Remote) Open(ctx context.Context, path string) (string, error) {
	res, err := r.call(ctx, request{Op: opOpen, Path: path})
	return res.Text, err
}

// Files is listed by the program with the drawing, so that a relative
// directory is taken from where Export and Open take a relative path.
func (r *Remote) Files(ctx context.Context, dir string) (Listing, error) {
	res, err := r.call(ctx, request{Op: opFiles, Path: dir})
	if err != nil {
		return Listing{}, err
	}
	if res.Listing == nil {
		return Listing{}, errors.New("TeleSAPR answered a listing with none")
	}
	return *res.Listing, nil
}

// Help needs no drawing, so it is answered here: it works before the
// program with the drawing is started.
func (r *Remote) Help(_ context.Context, name string) (string, error) {
	return Help(name)
}
