package syncing

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/runbrick/reqo/internal/model"
	"github.com/runbrick/reqo/internal/service/fake"
)

func TestEncryptedBackupRoundTripAndWrongPassword(t *testing.T) {
	t.Setenv("REQO_HOME", t.TempDir())
	svc := fake.New()
	col, err := svc.Collections.Create("Secrets")
	if err != nil {
		t.Fatal(err)
	}
	col.Variables = []model.KV{{Key: "token", Value: "super secret"}}
	if err := svc.Collections.Save(col); err != nil {
		t.Fatal(err)
	}
	snapshot := Capture(svc)
	data, err := Seal(snapshot, "correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == "" || bytes.Contains(data, []byte("super secret")) {
		t.Fatal("backup contains plaintext")
	}
	if _, err := Open(data, "incorrect password"); !errors.Is(err, ErrPassword) {
		t.Fatalf("wrong password: %v", err)
	}
	opened, err := Open(data, "correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	other := fake.New()
	if err := Apply(other, opened); err != nil {
		t.Fatal(err)
	}
	if got := Capture(other).Collections; len(got) != 1 || got[0].Variables[0].Value != "super secret" {
		t.Fatalf("restored collections: %+v", got)
	}
}

func TestThreeWayMergePropagatesEditsAndDeletion(t *testing.T) {
	now := time.Now()
	col := model.Collection{Version: 1, ID: "a", Name: "original"}
	base := Snapshot{Version: 1, Collections: []model.Collection{col}}
	local := base
	remote := base
	remote.Collections = []model.Collection{{Version: 1, ID: "a", Name: "changed remotely"}, {Version: 1, ID: "b", Name: "new"}}
	merged := Merge(base, local, remote, now)
	if len(merged.Collections) != 2 || merged.Collections[0].Name != "changed remotely" {
		t.Fatalf("remote edits lost: %+v", merged.Collections)
	}
	local.Collections = nil
	deleted := Merge(base, local, base, now)
	if len(deleted.Collections) != 0 || deleted.Deleted["c:a"] == 0 {
		t.Fatalf("deletion not recorded: %+v", deleted)
	}
	stale := Merge(Snapshot{Version: 1}, base, deleted, now)
	if len(stale.Collections) != 0 {
		t.Fatalf("stale device resurrected deleted collection: %+v", stale.Collections)
	}
}

func TestThreeWayMergePreservesIndependentRequestsWithinCollection(t *testing.T) {
	now := time.Now()
	first := model.Item{Type: model.ItemRequest, ID: "one", Name: "first"}
	second := model.Item{Type: model.ItemRequest, ID: "two", Name: "second"}
	base := Snapshot{Version: 1, Collections: []model.Collection{{Version: 1, ID: "col", Name: "API", Items: []model.Item{first, second}}}}
	local := Snapshot{Version: 1, Collections: []model.Collection{{Version: 1, ID: "col", Name: "API", Items: []model.Item{{Type: model.ItemRequest, ID: "one", Name: "edited locally"}, second}}}}
	remote := Snapshot{Version: 1, Collections: []model.Collection{{Version: 1, ID: "col", Name: "API", Items: []model.Item{first, {Type: model.ItemRequest, ID: "two", Name: "edited remotely"}}}}}
	merged := Merge(base, local, remote, now)
	items := merged.Collections[0].Items
	if len(items) != 2 || items[0].Name != "edited locally" || items[1].Name != "edited remotely" {
		t.Fatalf("independent edits lost: %+v", items)
	}
	local.Collections[0].Items = []model.Item{second}
	deleted := Merge(base, local, base, now)
	if len(deleted.Collections[0].Items) != 1 || deleted.Deleted["i:col:one"] == 0 {
		t.Fatalf("item deletion lost: %+v", deleted)
	}
	stale := Merge(Snapshot{Version: 1}, base, deleted, now)
	if len(stale.Collections[0].Items) != 1 {
		t.Fatalf("stale item resurrected: %+v", stale.Collections[0].Items)
	}
}

func TestWebDAVConditionalSyncAndNoCredentialRedirect(t *testing.T) {
	var data []byte
	version := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user, pass, ok := r.BasicAuth(); !ok || user != "u" || pass != "p" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Path == "/reqo" && r.Method == "MKCOL" {
			w.WriteHeader(http.StatusCreated)
			return
		}
		if r.URL.Path != "/reqo/backup.reqobak" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method == http.MethodGet {
			if version == 0 {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("ETag", fmt.Sprintf("\"v%d\"", version))
			_, _ = w.Write(data)
			return
		}
		if version == 0 && r.Header.Get("If-None-Match") != "*" || version > 0 && r.Header.Get("If-Match") != fmt.Sprintf("\"v%d\"", version) {
			w.WriteHeader(http.StatusPreconditionFailed)
			return
		}
		data, _ = io.ReadAll(r.Body)
		version++
		w.Header().Set("ETag", fmt.Sprintf("\"v%d\"", version))
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()
	client, err := NewWebDAV(WebDAVConfig{URL: server.URL, User: "u", Password: "p", File: "reqo/backup.reqobak"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	_, tag, missing, err := Download(ctx, client, "abcdefgh")
	if err != nil || !missing || tag != "" {
		t.Fatalf("missing backup: %q %t %v", tag, missing, err)
	}
	snapshot := Snapshot{Version: 1}
	if err := Upload(ctx, client, snapshot, "abcdefgh", tag); err != nil {
		t.Fatal(err)
	}
	if err := Upload(ctx, client, snapshot, "abcdefgh", tag); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale create: %v", err)
	}
	_, tag, missing, err = Download(ctx, client, "abcdefgh")
	if err != nil || missing || tag != "\"v1\"" {
		t.Fatalf("read after create: %q %t %v", tag, missing, err)
	}
	if err := Upload(ctx, client, snapshot, "abcdefgh", tag); err != nil {
		t.Fatal(err)
	}
	if err := Upload(ctx, client, snapshot, "abcdefgh", tag); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale update: %v", err)
	}
	if _, err := NewWebDAV(WebDAVConfig{URL: "http://example.com/dav", File: "reqo/backup.reqobak"}); err == nil {
		t.Fatal("HTTP outside loopback allowed credentials")
	}
	if _, err := NewWebDAV(WebDAVConfig{URL: server.URL, File: "../outside"}); err == nil {
		t.Fatal("path escape allowed")
	}
}
