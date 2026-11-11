package model

import "testing"

func TestTreeInsertFindRemove(t *testing.T) {
	var items []Item
	items, _ = InsertItem(items, "", Item{Type: ItemFolder, ID: "f", Name: "F"})
	items, ok := InsertItem(items, "f", Item{Type: ItemRequest, ID: "r", Request: &Request{ID: "r"}})
	if !ok {
		t.Fatal("insert into folder failed")
	}
	if _, ok := InsertItem(items, "r", Item{ID: "x"}); ok {
		t.Fatal("insert under a request must fail")
	}
	if FindItem(items, "r") == nil {
		t.Fatal("nested item not found")
	}
	n := 0
	WalkRequests(items, func(*Request) { n++ })
	if n != 1 {
		t.Fatalf("walked %d requests", n)
	}
	items, ok = RemoveItem(items, "r")
	if !ok || FindItem(items, "r") != nil || len(items) != 1 {
		t.Fatal("remove failed")
	}
}
