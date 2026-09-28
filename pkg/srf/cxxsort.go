package srf

import "math/bits"

// CxxSort sorts s as libstdc++'s std::sort does, so that elements that
// compare equal end up in the same order as in C++ (std::sort isn't
// stable, and a mesh's order is what gets exported): introsort with a
// median-of-three pivot, heapsort past the depth limit, then insertion
// sort (bits/stl_algo.h, bits/stl_heap.h).
func CxxSort[T any](s []T, less func(a, b *T) bool) {
	if len(s) == 0 {
		return
	}
	cxxIntrosortLoop(s, 0, len(s), 2*(bits.Len(uint(len(s)))-1), less)
	cxxFinalInsertionSort(s, 0, len(s), less)
}

const cxxSortThreshold = 16

func cxxIntrosortLoop[T any](s []T, first, last, depthLimit int, less func(a, b *T) bool) {
	for last-first > cxxSortThreshold {
		if depthLimit == 0 {
			cxxPartialSort(s, first, last, last, less)
			return
		}
		depthLimit--
		cut := cxxUnguardedPartitionPivot(s, first, last, less)
		cxxIntrosortLoop(s, cut, last, depthLimit, less)
		last = cut
	}
}

func cxxUnguardedPartitionPivot[T any](s []T, first, last int, less func(a, b *T) bool) int {
	mid := first + (last-first)/2
	cxxMoveMedianToFirst(s, first, first+1, mid, last-1, less)
	return cxxUnguardedPartition(s, first+1, last, first, less)
}

func cxxMoveMedianToFirst[T any](s []T, result, a, b, c int, less func(a, b *T) bool) {
	if less(&s[a], &s[b]) {
		if less(&s[b], &s[c]) {
			s[result], s[b] = s[b], s[result]
		} else if less(&s[a], &s[c]) {
			s[result], s[c] = s[c], s[result]
		} else {
			s[result], s[a] = s[a], s[result]
		}
	} else if less(&s[a], &s[c]) {
		s[result], s[a] = s[a], s[result]
	} else if less(&s[b], &s[c]) {
		s[result], s[c] = s[c], s[result]
	} else {
		s[result], s[b] = s[b], s[result]
	}
}

func cxxUnguardedPartition[T any](s []T, first, last, pivot int, less func(a, b *T) bool) int {
	for {
		for less(&s[first], &s[pivot]) {
			first++
		}
		last--
		for less(&s[pivot], &s[last]) {
			last--
		}
		if !(first < last) {
			return first
		}
		s[first], s[last] = s[last], s[first]
		first++
	}
}

func cxxFinalInsertionSort[T any](s []T, first, last int, less func(a, b *T) bool) {
	if last-first > cxxSortThreshold {
		cxxInsertionSort(s, first, first+cxxSortThreshold, less)
		for i := first + cxxSortThreshold; i != last; i++ {
			cxxUnguardedLinearInsert(s, i, less)
		}
	} else {
		cxxInsertionSort(s, first, last, less)
	}
}

func cxxInsertionSort[T any](s []T, first, last int, less func(a, b *T) bool) {
	if first == last {
		return
	}
	for i := first + 1; i != last; i++ {
		if less(&s[i], &s[first]) {
			val := s[i]
			copy(s[first+1:i+1], s[first:i])
			s[first] = val
		} else {
			cxxUnguardedLinearInsert(s, i, less)
		}
	}
}

func cxxUnguardedLinearInsert[T any](s []T, last int, less func(a, b *T) bool) {
	val := s[last]
	next := last - 1
	for less(&val, &s[next]) {
		s[last] = s[next]
		last = next
		next--
	}
	s[last] = val
}

func cxxPartialSort[T any](s []T, first, middle, last int, less func(a, b *T) bool) {
	cxxHeapSelect(s, first, middle, last, less)
	cxxSortHeap(s, first, middle, less)
}

func cxxHeapSelect[T any](s []T, first, middle, last int, less func(a, b *T) bool) {
	cxxMakeHeap(s, first, middle, less)
	for i := middle; i < last; i++ {
		if less(&s[i], &s[first]) {
			cxxPopHeap(s, first, middle, i, less)
		}
	}
}

func cxxMakeHeap[T any](s []T, first, last int, less func(a, b *T) bool) {
	n := last - first
	if n < 2 {
		return
	}
	parent := (n - 2) / 2
	for {
		value := s[first+parent]
		cxxAdjustHeap(s, first, parent, n, value, less)
		if parent == 0 {
			return
		}
		parent--
	}
}

func cxxSortHeap[T any](s []T, first, last int, less func(a, b *T) bool) {
	for last-first > 1 {
		last--
		cxxPopHeap(s, first, last, last, less)
	}
}

func cxxPopHeap[T any](s []T, first, last, result int, less func(a, b *T) bool) {
	value := s[result]
	s[result] = s[first]
	cxxAdjustHeap(s, first, 0, last-first, value, less)
}

func cxxAdjustHeap[T any](s []T, first, holeIndex, n int, value T, less func(a, b *T) bool) {
	topIndex := holeIndex
	secondChild := holeIndex
	for secondChild < (n-1)/2 {
		secondChild = 2 * (secondChild + 1)
		if less(&s[first+secondChild], &s[first+secondChild-1]) {
			secondChild--
		}
		s[first+holeIndex] = s[first+secondChild]
		holeIndex = secondChild
	}
	if n&1 == 0 && secondChild == (n-2)/2 {
		secondChild = 2 * (secondChild + 1)
		s[first+holeIndex] = s[first+secondChild-1]
		holeIndex = secondChild - 1
	}
	// __push_heap
	parent := (holeIndex - 1) / 2
	for holeIndex > topIndex && less(&s[first+parent], &value) {
		s[first+holeIndex] = s[first+parent]
		holeIndex = parent
		parent = (holeIndex - 1) / 2
	}
	s[first+holeIndex] = value
}
