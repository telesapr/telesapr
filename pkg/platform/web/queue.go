package web

import (
	"log"
	"runtime/debug"
	"sync"
)

// eventQueue holds what the page and other goroutines ask the event
// loop to run. Any goroutine posts; only the loop's goroutine takes.
type eventQueue struct {
	mu     sync.Mutex
	items  []func()
	closed bool
	// wake has room for one token: a post leaves one for the loop if it
	// sleeps.
	wake chan struct{}
	// after, if set, is called after each function the loop runs, on
	// its goroutine: the platform ends a busy spell there.
	after func()
}

func newEventQueue() *eventQueue {
	return &eventQueue{wake: make(chan struct{}, 1)}
}

// post has fn run by the loop, after what is queued already. It never
// blocks.
func (q *eventQueue) post(fn func()) {
	q.mu.Lock()
	if !q.closed {
		q.items = append(q.items, fn)
	}
	q.mu.Unlock()
	q.signal()
}

func (q *eventQueue) signal() {
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

// close ends every loop that runs the queue; what is posted later is
// dropped.
func (q *eventQueue) close() {
	q.mu.Lock()
	q.closed = true
	q.items = nil
	q.mu.Unlock()
	q.signal()
}

// next waits for the next function; false once the queue is closed.
func (q *eventQueue) next() (func(), bool) {
	for {
		q.mu.Lock()
		if q.closed {
			q.mu.Unlock()
			return nil, false
		}
		if len(q.items) > 0 {
			fn := q.items[0]
			q.items[0] = nil
			q.items = q.items[1:]
			q.mu.Unlock()
			return fn, true
		}
		q.mu.Unlock()
		<-q.wake
	}
}

// run takes functions and runs them until stop says so, which it is
// asked before each wait and so sees what the last function did; false
// if the queue was closed first. A modal dialog calls it again from
// inside a function, which is the nested loop. A function that panics
// is logged and the loop goes on: a page that died on a bug in one
// handler would lose the user's model.
func (q *eventQueue) run(stop func() bool) bool {
	for !stop() {
		fn, ok := q.next()
		if !ok {
			return false
		}
		runGuarded(fn)
		if q.after != nil {
			q.after()
		}
	}
	return true
}

// pending is how many functions wait.
func (q *eventQueue) pending() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

func runGuarded(fn func()) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("panic in an event handler: %v\n%s", r, debug.Stack())
		}
	}()
	fn()
}
