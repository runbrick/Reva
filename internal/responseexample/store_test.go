package responseexample

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/storage"
)

func TestExampleStorePersistsAndIsolatesRequests(t *testing.T) {
	home := t.TempDir()
	t.Setenv(storage.EnvHome, home)
	key := Key{CollectionID: "../collection", RequestID: "same-request-id"}
	sample := Sample{Method: model.MethodGet, Path: "/items", Status: 200, ContentType: "text/plain", Body: "reviewed text\n"}
	if err := Save(key, sample); err != nil {
		t.Fatal(err)
	}
	path, err := key.path()
	if err != nil || filepath.Dir(path) != filepath.Join(home, "examples") {
		t.Fatalf("example escaped its directory: %q, %v", path, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), `"version": 1`) || strings.Contains(string(data), key.CollectionID) || strings.Contains(string(data), key.RequestID) {
		t.Fatalf("example record: %v\n%s", err, data)
	}
	if got, found, err := Load(key); err != nil || !found || got != sample {
		t.Fatalf("reopen: %#v, %v, %v", got, found, err)
	}
	for _, other := range []Key{{CollectionID: "other", RequestID: key.RequestID}, {CollectionID: key.CollectionID, RequestID: "other"}} {
		if _, found, err := Load(other); err != nil || found {
			t.Fatalf("example accessible by another request: %v, %v", found, err)
		}
	}
	sample.Body, sample.Path = "", ""
	if err := Save(key, sample); err != nil {
		t.Fatal(err)
	}
	if got, found, err := Load(key); err != nil || !found || got != sample {
		t.Fatalf("replacement: %#v, %v, %v", got, found, err)
	}
	if err := Delete(key); err != nil {
		t.Fatal(err)
	}
	if _, found, err := Load(key); err != nil || found {
		t.Fatalf("example still exists: %v, %v", found, err)
	}
}

func TestExampleStoreRejectsInvalidRecords(t *testing.T) {
	t.Setenv(storage.EnvHome, t.TempDir())
	key := Key{CollectionID: "collection", RequestID: "request"}
	path, err := key.path()
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{
		strings.Repeat("x", maxFileBytes+1),
		`{"version":2,"method":"GET","status":200}`,
		`{"version":1,"method":"GET","status":200,"headers":{"Set-Cookie":"secret"}}`,
		`{"version":1,"method":"GET","status":200,"path":"/a?secret=123"}`,
		`{"version":1,"method":"GET","status":200} {}`,
	} {
		if err := storage.WriteFileAtomic(path, []byte(invalid)); err != nil {
			t.Fatal(err)
		}
		if _, found, err := Load(key); err == nil || found {
			t.Fatal("accepted invalid or oversized example file")
		}
	}
}
