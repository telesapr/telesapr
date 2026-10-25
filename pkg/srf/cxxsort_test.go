package srf

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"testing"
)

// TestCxxSortPermutations sorts 3000 random arrays with few distinct
// keys, half with CxxSort and half with the heapsort cxxPartialSort,
// and pins the order equal keys come out in by a hash: that order is
// the order of a mesh's triangles, which the goldens and the exports
// record. It matched libstdc++ 16's std::sort and
// std::partial_sort(first, last, last) when the hash was taken.
func TestCxxSortPermutations(t *testing.T) {
	t.Parallel()
	const want = "098d058e2fdca9b54ecfb3dc96f884eff33b6f7b3f54465f8c98562a023e2aa4"
	type e struct{ key, id int32 }
	s := uint64(12345)
	rnd := func() uint64 {
		s ^= s << 13
		s ^= s >> 7
		s ^= s << 17
		return s
	}
	var out []byte
	put := func(v int32) { out = binary.LittleEndian.AppendUint32(out, uint32(v)) }
	less := func(a, b *e) bool { return a.key < b.key }
	for c := range 3000 {
		n := int(rnd() % 300)
		alpha := 1 + int(rnd()%4)
		mode := c % 2
		v := make([]e, n)
		for i := range v {
			v[i] = e{int32(rnd() % uint64(alpha)), int32(i)}
		}
		if mode == 0 {
			CxxSort(v, less)
		} else {
			cxxPartialSort(v, 0, n, n, less)
		}
		put(int32(n))
		put(int32(alpha))
		put(int32(mode))
		for _, x := range v {
			put(x.key)
			put(x.id)
		}
	}
	sum := sha256.Sum256(out)
	if got := hex.EncodeToString(sum[:]); got != want {
		t.Errorf("hash %s, want %s", got, want)
	}
}
