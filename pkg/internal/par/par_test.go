package par

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestForVisitsEveryIndexOnce(t *testing.T) {
	defer SetWorkers(SetWorkers(8))
	for _, n := range []int{0, 1, 2, 7, 1000} {
		seen := make([]int32, n)
		For(n, func(i int) { atomic.AddInt32(&seen[i], 1) })
		for i, c := range seen {
			if c != 1 {
				t.Fatalf("n=%d: index %d ran %d times", n, i, c)
			}
		}
	}
}

func TestForOneWorkerRunsInOrder(t *testing.T) {
	defer SetWorkers(SetWorkers(1))
	var order []int
	For(5, func(i int) { order = append(order, i) })
	for i, v := range order {
		if i != v {
			t.Fatalf("order %v", order)
		}
	}
}

// TestForNestedStaysWithinLimit checks that loops inside loops share
// the goroutines instead of each taking its own.
func TestForNestedStaysWithinLimit(t *testing.T) {
	const limit = 4
	defer SetWorkers(SetWorkers(limit))
	var running, peak atomic.Int32
	var total atomic.Int32
	For(8, func(int) {
		For(8, func(int) {
			cur := running.Add(1)
			for {
				p := peak.Load()
				if cur <= p || peak.CompareAndSwap(p, cur) {
					break
				}
			}
			// Let the others get in before this one leaves.
			var wg sync.WaitGroup
			wg.Add(1)
			go wg.Done()
			wg.Wait()
			total.Add(1)
			running.Add(-1)
		})
	})
	if total.Load() != 64 {
		t.Fatalf("ran %d iterations, want 64", total.Load())
	}
	if p := peak.Load(); p > limit {
		t.Fatalf("%d iterations at once, limit %d", p, limit)
	}
	if extra.Load() != 0 {
		t.Fatalf("%d goroutines still counted", extra.Load())
	}
}

func TestForPanicsWithLowestIndex(t *testing.T) {
	defer SetWorkers(SetWorkers(8))
	for range 50 {
		func() {
			defer func() {
				if rec := recover(); rec != 3 {
					t.Fatalf("recovered %v, want 3", rec)
				}
			}()
			For(100, func(i int) {
				if i >= 3 && i%2 == 1 {
					panic(i)
				}
			})
			t.Fatal("no panic")
		}()
	}
	if extra.Load() != 0 {
		t.Fatalf("%d goroutines still counted", extra.Load())
	}
}
