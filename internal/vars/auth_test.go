package vars

import (
	"reflect"
	"testing"

	"github.com/runbrick/reqo/internal/model"
)

func TestApplyAuthExpandsCopyAndKeepsUnknownPlaceholders(t *testing.T) {
	r := model.NewRequest("r")
	r.Auth = model.Auth{Type: model.AuthAPIKey, Token: "{{value}}", Username: "{{value}}", Password: "{{missing}}", Key: "X-{{value}}", Value: "{{value}}", In: model.APIKeyInHeader}
	r.Headers = []model.KV{{Key: "X-Test", Value: "{{value}}"}}
	before := r.Clone()
	out := Apply(r, []model.KV{{Key: "value", Value: "collection"}, {Key: "value", Value: "environment"}, {Key: "value", Value: "disabled", Disabled: true}, {Key: "value", Value: "script"}})
	want := model.Auth{Type: model.AuthAPIKey, Token: "script", Username: "script", Password: "{{missing}}", Key: "X-script", Value: "script", In: model.APIKeyInHeader}
	if out.Auth != want || !reflect.DeepEqual(r, before) {
		t.Fatalf("expanded auth/caller copy: %#v, %#v", out.Auth, r)
	}
	out.Headers[0].Value = "edited"
	if !reflect.DeepEqual(r, before) {
		t.Fatal("expanded copy aliases caller")
	}
}
