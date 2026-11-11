package vars

import (
	"reflect"
	"testing"

	"github.com/runbrick/reqo/internal/model"
)

var env = []model.KV{
	{Key: "host", Value: "a.example"},
	{Key: "token", Value: "old"},
	{Key: "token", Value: "new"},
	{Key: "off", Value: "x", Disabled: true},
}

func TestExpand(t *testing.T) {
	cases := map[string]string{
		"https://{{host}}/v1":     "https://a.example/v1",
		"{{ host }}":              "a.example",
		"Bearer {{token}}":        "Bearer new",
		"{{missing}} and {{off}}": "{{missing}} and {{off}}",
		"no vars":                 "no vars",
		"{{unclosed":              "{{unclosed",
		"{{}}":                    "{{}}",
	}
	for in, want := range cases {
		if got := Expand(in, env); got != want {
			t.Errorf("Expand(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestUnresolved(t *testing.T) {
	got := Unresolved("{{host}}/{{a}}/{{b}}/{{a}}", env)
	if !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("got %v", got)
	}
}

func TestApplyDoesNotTouchOriginal(t *testing.T) {
	req := model.NewRequest("1")
	req.URL = "https://{{host}}"
	req.Headers = []model.KV{{Key: "Authorization", Value: "Bearer {{token}}"}}
	out := Apply(req, env)
	if out.URL != "https://a.example" || out.Headers[0].Value != "Bearer new" {
		t.Fatalf("unexpected %+v", out)
	}
	if req.Headers[0].Value != "Bearer {{token}}" {
		t.Fatal("Apply modified the original request")
	}
}
