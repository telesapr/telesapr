package core

import "testing"

func TestIdListBasicAddFind(t *testing.T) {
	type item struct{ V int }
	var list IDList[HEntity, item]
	list.Add(HEntity{V: 1}, item{V: 10})
	list.Add(HEntity{V: 2}, item{V: 20})

	got, ok := list.FindByID(HEntity{V: 1})
	if !ok || got.V != 10 {
		t.Fatalf("FindById(1) = %v,%v; want {10},true", got, ok)
	}
	_, ok = list.FindByID(HEntity{V: 99})
	if ok {
		t.Fatalf("FindById(99) should miss")
	}
}

func TestIdListMoveSelfInto(t *testing.T) {
	type item struct{ V int }
	src := NewIDList[HEntity, item](2)
	src.Add(HEntity{V: 1}, item{V: 10})
	src.Add(HEntity{V: 2}, item{V: 20})

	var dst IDList[HEntity, item]
	src.MoveSelfInto(&dst)

	if dst.Len() != 2 {
		t.Fatalf("dst should have 2 items, has %d", dst.Len())
	}
	if src.Len() != 0 {
		t.Fatalf("src should be empty, has %d", src.Len())
	}
	if got, _ := dst.FindByID(HEntity{V: 1}); got.V != 10 {
		t.Fatalf("dst.FindById(1) = %v", got)
	}
}

func TestIdListIDsInInsertionOrder(t *testing.T) {
	type item struct{ V int }
	var list IDList[HEntity, item]
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

// TestIdListClear verifies Clear empties the list.
func TestIdListClear(t *testing.T) {
	list := NewIDList[int, string](0)
	list.Add(1, "one")
	list.Add(2, "two")
	list.Clear()
	if list.Len() != 0 {
		t.Errorf("Len after clear = %d, want 0", list.Len())
	}
}

// TestIdListEmptyAdd verifies adding to empty list works.
func TestIdListEmptyAdd(t *testing.T) {
	list := NewIDList[int, string](0)
	idx := list.Add(42, "hello")
	if idx != 0 {
		t.Errorf("first Add idx = %d, want 0", idx)
	}
	if list.Len() != 1 {
		t.Errorf("Len = %d, want 1", list.Len())
	}
}

// TestIdListFindById verifies FindById.
func TestIdListFindById(t *testing.T) {
	list := NewIDList[int, string](0)
	list.Add(1, "one")
	list.Add(2, "two")
	v, ok := list.FindByID(1)
	if !ok || v != "one" {
		t.Errorf("FindById(1) = (%v, %v), want (one, true)", v, ok)
	}
	_, ok = list.FindByID(99)
	if ok {
		t.Errorf("FindById(99) should not find")
	}
}

// TestIdListSortedHandles verifies SortedHandles ordering.
func TestIdListSortedHandles(t *testing.T) {
	list := NewIDList[int, string](0)
	list.Add(3, "three")
	list.Add(1, "one")
	list.Add(2, "two")
	ids := list.SortedHandles(func(a, b int) bool { return a < b })
	if len(ids) != 3 || ids[0] != 1 || ids[1] != 2 || ids[2] != 3 {
		t.Errorf("SortedHandles = %v, want [1, 2, 3]", ids)
	}
}

// TestIdListIDs verifies the IDs accessor returns every handle that
// was added. IDs is map-iteration-ordered so we only check membership.
func TestIdListIDs(t *testing.T) {
	list := NewIDList[int, string](0)
	list.Add(10, "ten")
	list.Add(20, "twenty")
	ids := list.IDs()
	if len(ids) != 2 {
		t.Fatalf("IDs len = %d, want 2", len(ids))
	}
	seen := map[int]bool{10: false, 20: false}
	for _, h := range ids {
		if _, ok := seen[h]; !ok {
			t.Errorf("unexpected handle in IDs: %v", h)
		}
		seen[h] = true
	}
	for h, ok := range seen {
		if !ok {
			t.Errorf("missing handle in IDs: %v", h)
		}
	}
}

// TestIdListReverseOrder verifies items preserve insertion order.
func TestIdListReverseOrder(t *testing.T) {
	list := NewIDList[int, string](0)
	list.Add(1, "first")
	list.Add(2, "second")
	list.Add(3, "third")
	if list.Items[0] != "first" || list.Items[1] != "second" || list.Items[2] != "third" {
		t.Errorf("insertion order not preserved: %v", list.Items)
	}
}

// TestIdListReserve verifies Reserve doesn't break length.
func TestIdListReserve(t *testing.T) {
	list := NewIDList[int, string](0)
	list.Reserve(100)
	if list.Len() != 0 {
		t.Errorf("Reserve shouldn't add items, Len = %d", list.Len())
	}
}
