package format

import (
	"strings"
	"testing"
)

func TestTokenizeJSONNestedAndEscapedStrings(t *testing.T) {
	source := "{\n  \"nested\": [{\"say\\\"hi\": \"a\\\\b\\\"c\"}],\n  \"number\": -12.50e+3, \"yes\": true, \"no\": false, \"nil\": null\n}"
	lines, tokens := strings.Split(source, "\n"), TokenizeJSON(source)
	seen := make(map[string]TokenKind)
	for i, line := range tokens {
		end := 0
		for _, token := range line {
			if token.Start != end || token.End > len(lines[i]) {
				t.Fatalf("tokens do not preserve source line %d: %+v", i, line)
			}
			seen[lines[i][token.Start:token.End]] = token.Kind
			end = token.End
		}
		if end != len(lines[i]) {
			t.Fatalf("tokens omitted bytes in line %d", i)
		}
	}
	for value, kind := range map[string]TokenKind{
		`"nested"`: Key, `"say\"hi"`: Key, `"a\\b\"c"`: String,
		"-12.50e+3": Number, "true": Literal, "false": Literal, "null": Literal,
		"{": Punctuation, "[": Punctuation, "]": Punctuation,
	} {
		if got, ok := seen[value]; !ok || got != kind {
			t.Errorf("%q token = %v (present %v), want %v", value, got, ok, kind)
		}
	}
}

func TestTokenizeJSONInvalidFallsBackToPlain(t *testing.T) {
	for _, source := range []string{`{"broken":`, "{\n\"x\": truth\n}", `{"x":"bad\q"}`, "plain text", ""} {
		for _, line := range TokenizeJSON(source) {
			for _, token := range line {
				if token.Kind != Plain {
					t.Errorf("invalid JSON %q received syntax colors: %+v", source, token)
				}
			}
		}
	}
}

func TestTokenizeJSONKeyColonOnNextLine(t *testing.T) {
	tokens := TokenizeJSON("{\"key\"\n\n: \"value\"}")
	if tokens[0][1].Kind != Key || tokens[2][2].Kind != String {
		t.Fatalf("multiline key/value = %+v", tokens)
	}
}

func TestTokenizeJSONDraftPreservesIncompleteTokens(t *testing.T) {
	source := "{\n  \"名称\": \"带转义的值\\\n  \"enabled\": true, \"limit\": -2, \"unknown\": truth"
	want := map[string]TokenKind{
		"{": Punctuation, `"名称"`: Key, `"带转义的值\`: String,
		`"enabled"`: Key, "true": Literal, "-2": Number, "truth": Plain,
	}
	lines := strings.Split(source, "\n")
	for index, tokens := range TokenizeJSONDraft(source) {
		line := lines[index]
		end := 0
		for _, token := range tokens {
			if token.Start != end || token.End > len(line) || token.End <= end {
				t.Fatalf("token %+v does not match source line %q", token, line)
			}
			value := line[token.Start:token.End]
			if kind, ok := want[value]; ok {
				if token.Kind != kind {
					t.Errorf("%q token = %v, want %v", value, token.Kind, kind)
				}
				delete(want, value)
			}
			end = token.End
		}
		if end != len(line) {
			t.Fatalf("tokens omitted bytes in line %q", line)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing tokens: %v", want)
	}
}

func TestHTMLTextRemovesTagsAndHiddenCode(t *testing.T) {
	source := `<h1 title="a > b">Hello &amp; welcome</h1><p>World <b>today</b>.</p><!-- hidden --><script>alert("bad")</script><style>p{color:red}</style>`
	got := HTMLText(source)
	if !strings.Contains(got, "Hello & welcome") || !strings.Contains(got, "World today.") || strings.ContainsAny(got, "<>") || strings.Contains(got, "hidden") || strings.Contains(got, "bad") || strings.Contains(got, "color") {
		t.Fatalf("HTML preview = %q", got)
	}
}
