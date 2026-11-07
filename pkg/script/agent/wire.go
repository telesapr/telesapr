package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

// This file is a Backend through a connection: the program that has
// the drawing serves its Backend with Serve, and another reaches it
// with a Remote. Each side writes one JSON value to a line: a request
// with an id, and the response with that id. A request may be sent
// before the last was answered, which is how one that runs is
// cancelled. Before the response to a run, lines of op "progress" with
// the run's id may come, each with a Progress, which the side that
// asked passes on or, for an id it doesn't know, drops.

// Protocol is the version of what goes over the connection; both
// sides must have the same. Version 2 added measure, export, open and
// files, and the section of a picture; version 3 the script option of
// describe; version 4 the caption of a picture; version 5 the solid
// option of describe; version 6 the progress of a run and the data of
// describe and measure.
const Protocol = 6

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
	// opProgress is not a request but a line that comes before the
	// response to a run.
	opProgress = "progress"
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
	// Script has describe return the drawing as a script.
	Script bool `json:"script,omitempty"`
	// Solid has describe list the faces and edges of the solid.
	Solid bool `json:"solid,omitempty"`
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

	// report, if not nil, is given the progress of a run that comes
	// before its response; it is the caller's and isn't sent.
	report func(Progress)
}

type response struct {
	ID int `json:"id"`
	// Op is "progress" for a line that tells how far a run is, with
	// Progress, and "" for the response.
	Op       string    `json:"op,omitempty"`
	Progress *Progress `json:"progress,omitempty"`
	// Error is why the request couldn't be done.
	Error string `json:"error,omitempty"`
	// Protocol answers a hello.
	Protocol int `json:"protocol,omitempty"`
	// Result answers run; Text describe, help, measure, export and
	// open; PNG and Caption render; Listing files. Data is describe's
	// text as a Description, and measure's as a Measurement.
	Result  *Result         `json:"result,omitempty"`
	Text    string          `json:"text,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
	PNG     []byte          `json:"png,omitempty"`
	Caption string          `json:"caption,omitempty"`
	Listing *Listing        `json:"listing,omitempty"`
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
			report := func(p Progress) { reply(response{ID: req.ID, Op: opProgress, Progress: &p}) }
			res := answer(rctx, b, req, report)
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

// answer does what a request asks of a Backend; report tells the
// progress of a run, if b says it (ProgressRunner).
func answer(ctx context.Context, b Backend, req request, report func(Progress)) response {
	res := response{ID: req.ID}
	var err error
	switch req.Op {
	case opRun:
		var r Result
		if pr, ok := b.(ProgressRunner); ok {
			r, err = pr.RunWithProgress(ctx, req.Code, report)
		} else {
			r, err = b.Run(ctx, req.Code)
		}
		if err == nil {
			res.Result = &r
		}
	case opDescribe:
		var d Described
		if d, err = b.Describe(ctx, Describing{Group: req.Group, Script: req.Script, Solid: req.Solid}); err == nil {
			res.Text = d.Text
			res.Data, err = json.Marshal(d.Data)
		}
	case opRender:
		var p Picture
		if req.Picture != nil {
			p = *req.Picture
		}
		var r Rendered
		if r, err = b.Render(ctx, p); err == nil {
			res.PNG, res.Caption = r.PNG, r.Caption
		}
	case opHelp:
		res.Text, err = b.Help(ctx, req.Name)
	case opMeasure:
		var m Measured
		if m, err = b.Measure(ctx, Measuring{Group: req.Group, Selected: req.Selected,
			Gaps: req.Gaps, Walls: req.Walls, Within: req.Within}); err == nil {
			res.Text = m.Text
			res.Data, err = json.Marshal(m.Data)
		}
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
	Connect func() (Conn, error)
	// CancelWait is how long a cancelled request waits for the
	// program to say what came of it; a second if zero.
	CancelWait time.Duration

	mu      sync.Mutex
	conn    Conn
	enc     *json.Encoder
	next    int
	pending map[int]chan response
	// reports are the callers' progress callbacks, by request id.
	reports map[int]func(Progress)
}

var (
	_ Backend        = (*Remote)(nil)
	_ ProgressRunner = (*Remote)(nil)
)

// link is the connection, made if there is none.
func (r *Remote) link() (Conn, error) {
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
	r.conn, r.enc, r.pending, r.reports = conn, json.NewEncoder(conn), map[int]chan response{}, map[int]func(Progress){}
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
func (r *Remote) read(conn Conn) {
	scan := bufio.NewScanner(conn)
	scan.Buffer(make([]byte, 0, 64<<10), maxLine)
	for scan.Scan() {
		var res response
		if err := json.Unmarshal(scan.Bytes(), &res); err != nil {
			continue
		}
		if res.Op == opProgress {
			// Passed on here, before the response that follows it is.
			r.mu.Lock()
			report := r.reports[res.ID]
			r.mu.Unlock()
			if report != nil && res.Progress != nil {
				report(*res.Progress)
			}
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
func (r *Remote) drop(conn Conn) {
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
	clear(r.reports)
	r.conn, r.enc = nil, nil
}

// errBroken is the error of a request whose connection ended before
// the answer came.
var errBroken = errors.New("the connection to TeleSAPR ended")

// roundTrip sends a request over conn and waits for its response.
func (r *Remote) roundTrip(ctx context.Context, conn Conn, req request) (response, error) {
	r.mu.Lock()
	if r.conn != conn {
		r.mu.Unlock()
		return response{}, errBroken
	}
	r.next++
	req.ID = r.next
	ch := make(chan response, 1)
	r.pending[req.ID] = ch
	if req.report != nil {
		r.reports[req.ID] = req.report
		defer func() {
			r.mu.Lock()
			delete(r.reports, req.ID)
			r.mu.Unlock()
		}()
	}
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
	return r.RunWithProgress(ctx, code, nil)
}

// RunWithProgress is Run that passes report what the program says of
// how far the script is, on the goroutine that reads the connection.
func (r *Remote) RunWithProgress(ctx context.Context, code string, report func(Progress)) (Result, error) {
	req := request{Op: opRun, Code: code}
	if report != nil {
		// A run that was cancelled may return while a report is read:
		// that one, and any after, is dropped.
		var (
			mu    sync.Mutex
			ended bool
		)
		req.report = func(p Progress) {
			mu.Lock()
			defer mu.Unlock()
			if !ended {
				report(p)
			}
		}
		defer func() {
			mu.Lock()
			ended = true
			mu.Unlock()
		}()
	}
	res, err := r.call(ctx, req)
	if err != nil {
		return Result{}, err
	}
	if res.Result == nil {
		return Result{}, errors.New("TeleSAPR answered a run with no result")
	}
	return *res.Result, nil
}

func (r *Remote) Describe(ctx context.Context, d Describing) (Described, error) {
	res, err := r.call(ctx, request{Op: opDescribe, Group: d.Group, Script: d.Script, Solid: d.Solid})
	if err != nil {
		return Described{}, err
	}
	out := Described{Text: res.Text}
	if err := unpack(res.Data, &out.Data); err != nil {
		return Described{}, err
	}
	return out, nil
}

func (r *Remote) Render(ctx context.Context, p Picture) (Rendered, error) {
	res, err := r.call(ctx, request{Op: opRender, Picture: &p})
	return Rendered{PNG: res.PNG, Caption: res.Caption}, err
}

func (r *Remote) Measure(ctx context.Context, m Measuring) (Measured, error) {
	res, err := r.call(ctx, request{Op: opMeasure, Group: m.Group, Selected: m.Selected,
		Gaps: m.Gaps, Walls: m.Walls, Within: m.Within})
	if err != nil {
		return Measured{}, err
	}
	out := Measured{Text: res.Text}
	if err := unpack(res.Data, &out.Data); err != nil {
		return Measured{}, err
	}
	return out, nil
}

// unpack reads a response's data into v; a response with none leaves
// v as it is.
func unpack(data json.RawMessage, v any) error {
	if len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("reading what TeleSAPR answered: %w", err)
	}
	return nil
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
