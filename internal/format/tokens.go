package format

import (
	"encoding/json"
	"strings"
)

// TokenKind is a JSON presentation category, independent of the active theme.
type TokenKind uint8

const (
	Plain TokenKind = iota
	Key
	String
	Number
	Literal
	Punctuation
)

// Token identifies a byte range within one source line.
type Token struct {
	Start, End int
	Kind       TokenKind
}

// TokenizeJSON preserves every source byte and returns tokens per logical line.
// Invalid or incomplete JSON is deliberately plain, with no misleading colors.
// Call once when installing a response, never from a painter.
func TokenizeJSON(value string) [][]Token {
	lines := strings.Split(value, "\n")
	out := make([][]Token, len(lines))
	valid := json.Valid([]byte(value))
	for n, line := range lines {
		if !valid {
			out[n] = []Token{{End: len(line), Kind: Plain}}
			continue
		}
		for i := 0; i < len(line); {
			start, kind := i, Plain
			switch line[i] {
			case ' ', '\t', '\r':
				i++
				for i < len(line) && (line[i] == ' ' || line[i] == '\t' || line[i] == '\r') {
					i++
				}
			case '"':
				kind = String
				i++
				for i < len(line) {
					b := line[i]
					i++
					if b == '\\' {
						i++
					} else if b == '"' {
						break
					}
				}
				j := i
				for j < len(line) && (line[j] == ' ' || line[j] == '\t' || line[j] == '\r') {
					j++
				}
				// A key's colon can be on the following line in valid JSON.
				if j == len(line) {
					for k := n + 1; k < len(lines); k++ {
						next := strings.TrimSpace(lines[k])
						if next != "" {
							if next[0] == ':' {
								kind = Key
							}
							break
						}
					}
				} else if line[j] == ':' {
					kind = Key
				}
			case '{', '}', '[', ']', ':', ',':
				kind = Punctuation
				i++
			default:
				kind = Literal
				if line[i] == '-' || line[i] >= '0' && line[i] <= '9' {
					kind = Number
				}
				for i < len(line) && !strings.ContainsRune(" \t\r{}[]:,", rune(line[i])) {
					i++
				}
			}
			out[n] = append(out[n], Token{Start: start, End: i, Kind: kind})
		}
	}
	return out
}
