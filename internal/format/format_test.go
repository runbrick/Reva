package format

import (
	"testing"
	"time"
)

func TestPrettyJSONPreservesOrderAndLexemes(t *testing.T) {
	input := []byte(` {"z":1e+09,"a":{"b":2,"a":3},"z":"\u0041","list":[true,null]} `)
	want := "{\n  \"z\": 1e+09,\n  \"a\": {\n    \"b\": 2,\n    \"a\": 3\n  },\n  \"z\": \"\\u0041\",\n  \"list\": [\n    true,\n    null\n  ]\n}"
	got, err := PrettyJSON(input)
	if err != nil || got != want {
		t.Fatalf("PrettyJSON = %q, %v; want %q", got, err, want)
	}
	stable, err := PrettyJSON([]byte(got))
	if err != nil || stable != got {
		t.Fatalf("formatting twice changed result: %q, %v", stable, err)
	}
}

func TestPrettyJSONRejectsInvalidInput(t *testing.T) {
	for _, input := range []string{"", "  ", `{"a":`, `{} {}`, `{"a":1,}`, "plain text"} {
		t.Run(input, func(t *testing.T) {
			if _, err := PrettyJSON([]byte(input)); err == nil {
				t.Fatal("expected invalid JSON error")
			}
		})
	}
}

func TestIsJSON(t *testing.T) {
	for _, tt := range []struct {
		name, contentType, body string
		want                    bool
	}{
		{"media type", "application/json; charset=utf-8", "incomplete", true},
		{"suffix", "application/problem+json", "incomplete", true},
		{"case", "Application/JSON", "incomplete", true},
		{"legacy", "text/json", "incomplete", true},
		{"sniff", "text/plain", `{"ok":true}`, true},
		{"missing type", "", " [1, 2] ", true},
		{"scalar", "", "42", true},
		{"malformed type", "invalid;", "null", true},
		{"html", "text/html", "<html></html>", false},
		{"empty", "", "", false},
		{"incomplete", "", `{"a":`, false},
		{"similar type", "application/jsonp", "text", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsJSON(tt.contentType, []byte(tt.body)); got != tt.want {
				t.Fatalf("IsJSON = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestByteSize(t *testing.T) {
	for _, tt := range []struct {
		n    int64
		want string
	}{{0, "0 B"}, {1, "1 B"}, {1023, "1023 B"}, {1024, "1.00 KB"}, {1536, "1.50 KB"}, {1048576, "1.00 MB"}, {2621440, "2.50 MB"}} {
		if got := ByteSize(tt.n); got != tt.want {
			t.Errorf("ByteSize(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestDuration(t *testing.T) {
	for _, tt := range []struct {
		d    time.Duration
		want string
	}{{0, "0 ms"}, {500 * time.Microsecond, "0 ms"}, {12 * time.Millisecond, "12 ms"}, {999 * time.Millisecond, "999 ms"}, {time.Second, "1.00 s"}, {1234 * time.Millisecond, "1.23 s"}, {1256 * time.Millisecond, "1.26 s"}} {
		if got := Duration(tt.d); got != tt.want {
			t.Errorf("Duration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}
