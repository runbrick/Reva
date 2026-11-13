package model

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestAuthAdditiveJSONAndLegacyRequest(t *testing.T) {
	var legacy Collection
	if err := json.Unmarshal([]byte(`{"version":1,"id":"old","name":"Legacy","items":[{"type":"folder","id":"folder","items":[{"type":"request","id":"r","request":{"id":"r","url":"https://example.test"}}]}]}`), &legacy); err != nil {
		t.Fatal(err)
	}
	if legacy.Auth != (Auth{}) || legacy.Items[0].Auth != (Auth{}) || legacy.Items[0].Items[0].Request.Auth != (Auth{}) || NewRequest("new").Auth.Type != AuthNone {
		t.Fatal("old/standalone requests acquired inheritance")
	}
	data, err := json.Marshal(legacy)
	if err != nil || strings.Count(string(data), `"auth"`) != 1 {
		t.Fatalf("zero root/folder auth must be omitted, request auth retained: %s %v", data, err)
	}
	legacy.Auth = Auth{Type: AuthBearer, Token: "{{root}}"}
	legacy.Items[0].Auth = Auth{Type: AuthBasic, Username: "{{user}}"}
	legacy.Items[0].Items[0].Request.Auth = Auth{Type: AuthInherit}
	data, err = json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	var restored Collection
	if err := json.Unmarshal(data, &restored); err != nil || !reflect.DeepEqual(restored, legacy) || restored.Version != 1 || SchemaVersion != 1 {
		t.Fatalf("additive auth roundtrip: %#v, %v", restored, err)
	}
}
