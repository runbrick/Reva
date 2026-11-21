package responsejson

import (
	"strconv"
	"strings"
)

type ChangeKind int

const (
	Added ChangeKind = iota
	Removed
	TypeChanged
	ValueChanged
)

type Change struct {
	Pointer       string
	Kind          ChangeKind
	Before, After *Value
}

func IgnoredPaths(lines string) (map[string]bool, error) {
	if len(lines) > 16*1024 {
		return nil, ErrInvalid
	}
	ignored := make(map[string]bool)
	for line := range strings.SplitSeq(lines, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if line == "" {
			continue
		}
		if _, ok := pointerSegments(line); !ok || len(ignored) == 256 && !ignored[line] {
			return nil, ErrInvalid
		}
		ignored[line] = true
	}
	return ignored, nil
}

func Diff(before, after *Document, ignored map[string]bool, limit int) (changes []Change, more bool) {
	var visit func(string, *Value, *Value) bool
	visit = func(path string, left, right *Value) bool {
		if ignored[path] {
			return true
		}
		var kind ChangeKind
		switch {
		case left == nil:
			kind = Added
		case right == nil:
			kind = Removed
		case left.kind != right.kind:
			kind = TypeChanged
		case left.kind == Object:
			for _, key := range keys(left.object, right.object) {
				if !visit(path+"/"+encodeSegment.Replace(key), left.object[key], right.object[key]) {
					return false
				}
			}
			return true
		case left.kind == Array:
			for index := 0; index < max(len(left.array), len(right.array)); index++ {
				var previous, current *Value
				if index < len(left.array) {
					previous = left.array[index]
				}
				if index < len(right.array) {
					current = right.array[index]
				}
				if !visit(path+"/"+strconv.Itoa(index), previous, current) {
					return false
				}
			}
			return true
		case equalScalar(left, right):
			return true
		default:
			kind = ValueChanged
		}
		if len(changes) == limit {
			return false
		}
		changes = append(changes, Change{Pointer: path, Kind: kind, Before: left, After: right})
		return true
	}
	more = !visit("", before.root, after.root)
	return changes, more
}
