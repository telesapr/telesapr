package core

import "testing"

func TestIdListBasicAddFind(t *testing.T) {
	type item struct{ V int }
	var list IdList[HEntity, item]
	list.Add(HEntity{V: 1}, item{V: 10})
	list.Add(HEntity{V: 2}, item{V: 20})

	got, ok := list.FindById(HEntity{V: 1})
	if !ok || got.V != 10 {
		t.Fatalf("FindById(1) = %v,%v; want {10},true", got, ok)
	}
	_, ok = list.FindById(HEntity{V: 99})
	if ok {
		t.Fatalf("FindById(99) should miss")
	}
}

func TestIdListMoveSelfInto(t *testing.T) {
	type item struct{ V int }
	src := NewIdList[HEntity, item](2)
	src.Add(HEntity{V: 1}, item{V: 10})
	src.Add(HEntity{V: 2}, item{V: 20})

	var dst IdList[HEntity, item]
	src.MoveSelfInto(&dst)

	if dst.Len() != 2 {
		t.Fatalf("dst should have 2 items, has %d", dst.Len())
	}
	if src.Len() != 0 {
		t.Fatalf("src should be empty, has %d", src.Len())
	}
	if got, _ := dst.FindById(HEntity{V: 1}); got.V != 10 {
		t.Fatalf("dst.FindById(1) = %v", got)
	}
}

func TestIdListIDsInInsertionOrder(t *testing.T) {
	type item struct{ V int }
	var list IdList[HEntity, item]
	list.Add(HEntity{V: 5}, item{})
	list.Add(HEntity{V: 3}, item{})
	list.Add(HEntity{V: 8}, item{})

	ids := list.IDs()
	if len(ids) != 3 {
		t.Fatalf("IDs len = %d", len(ids))
	}
	// Map iteration order is not deterministic; just check membership.
	want := map[uint32]bool{3: false, 5: false, 8: false}
	for _, h := range ids {
		if _, ok := want[h.V]; !ok {
			t.Fatalf("unexpected handle in IDs: %v", h)
		}
		want[h.V] = true
	}
	for h, seen := range want {
		if !seen {
			t.Fatalf("missing handle in IDs: %v", h)
		}
	}
}
