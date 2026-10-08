// Package par runs the iterations of a loop on several goroutines.
// Callers keep their results the same as one thread's by writing only
// to what an iteration owns and committing in index order afterwards.
package par

import (
	"runtime"
	"sync"
	"sync/atomic"
)

var (
	// forced is the number of goroutines a loop may use, the caller
	// included, or 0 for GOMAXPROCS.
	forced atomic.Int32
	// extra counts the goroutines running loops besides their callers,
	// over all the loops in progress, so that nested loops share the
	// processors rather than multiply them.
	extra atomic.Int32
)

// SetWorkers sets how many goroutines a loop may use, its caller
// included, and returns the previous setting; 0 means GOMAXPROCS, and
// 1 runs every loop on its caller.
func SetWorkers(n int) int {
	return int(forced.Swap(int32(n)))
}

// Workers returns how many goroutines a loop may use.
func Workers() int {
	if n := int(forced.Load()); n > 0 {
		return n
	}
	return runtime.GOMAXPROCS(0)
}

// acquire takes up to want of the goroutines that are free.
func acquire(want int) int {
	limit := int32(Workers() - 1)
	for {
		cur := extra.Load()
		got := min(int32(want), limit-cur)
		if got <= 0 {
			return 0
		}
		if extra.CompareAndSwap(cur, cur+got) {
			return int(got)
		}
	}
}

// For calls fn(i) for every i in [0, n), on the caller's goroutine and
// on as many others as are free, and returns when all are done. With
// none free the loop runs in order on the caller, so a loop inside
// another neither waits for it nor adds goroutines beyond the limit.
//
// If iterations panic, For panics on the caller's goroutine with the
// value of the lowest one that did, as the loop on one goroutine
// would; the iterations after it may or may not have run.
func For(n int, fn func(i int)) {
	if n < 2 {
		if n == 1 {
			fn(0)
		}
		return
	}
	helpers := acquire(n - 1)
	if helpers == 0 {
		for i := range n {
			fn(i)
		}
		return
	}

	var (
		next     atomic.Int64
		mu       sync.Mutex
		panicked = int64(n) // the lowest iteration that panicked
		value    any
	)
	failedAt := func() int64 {
		mu.Lock()
		defer mu.Unlock()
		return panicked
	}
	run := func(i int64) {
		defer func() {
			if rec := recover(); rec != nil {
				mu.Lock()
				if i < panicked {
					panicked, value = i, rec
				}
				mu.Unlock()
			}
		}()
		fn(int(i))
	}
	work := func() {
		for {
			i := next.Add(1) - 1
			if i >= int64(n) {
				return
			}
			// Iterations before one that panicked still run: one of
			// them may be the first to panic.
			if i > failedAt() {
				continue
			}
			run(i)
		}
	}

	var wg sync.WaitGroup
	wg.Add(helpers)
	for range helpers {
		go func() {
			defer wg.Done()
			defer extra.Add(-1)
			work()
		}()
	}
	work()
	wg.Wait()
	if panicked < int64(n) {
		panic(value)
	}
}
