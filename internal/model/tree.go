package model

// FindItem returns a pointer to the item with id anywhere in items, or nil.
// The pointer is valid until items is next modified.
func FindItem(items []Item, id string) *Item {
	for i := range items {
		if items[i].ID == id {
			return &items[i]
		}
		if found := FindItem(items[i].Items, id); found != nil {
			return found
		}
	}
	return nil
}

// InsertItem appends item under the folder parentID ("" = top level) and
// returns the new slice. It reports false when parentID names no folder.
func InsertItem(items []Item, parentID string, item Item) ([]Item, bool) {
	if parentID == "" {
		return append(items, item), true
	}
	parent := FindItem(items, parentID)
	if parent == nil || parent.Type != ItemFolder {
		return items, false
	}
	parent.Items = append(parent.Items, item)
	return items, true
}

// RemoveItem removes the item with id (and its children) and returns the
// new slice. It reports false when no item has that id.
func RemoveItem(items []Item, id string) ([]Item, bool) {
	for i := range items {
		if items[i].ID == id {
			return append(items[:i:i], items[i+1:]...), true
		}
		if sub, ok := RemoveItem(items[i].Items, id); ok {
			items[i].Items = sub
			return items, true
		}
	}
	return items, false
}

// WalkRequests calls fn for every request in items, depth first.
func WalkRequests(items []Item, fn func(*Request)) {
	for i := range items {
		if items[i].Type == ItemRequest && items[i].Request != nil {
			fn(items[i].Request)
		}
		WalkRequests(items[i].Items, fn)
	}
}
