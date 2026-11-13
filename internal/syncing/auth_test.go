package syncing

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/runbrick/reqo/internal/model"
	"github.com/runbrick/reqo/internal/service/fake"
)

func authSnapshotCopy(t *testing.T, s Snapshot) Snapshot {
	t.Helper()
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var copy Snapshot
	if err := json.Unmarshal(data, &copy); err != nil {
		t.Fatal(err)
	}
	return copy
}

func TestAuthSyncIndependentMetadataAndChildrenBothOrientations(t *testing.T) {
	base := Snapshot{Version: 1, Collections: []model.Collection{{Version: 1, ID: "col", Name: "API", Items: []model.Item{
		{ID: "folder", Name: "Shared", Type: model.ItemFolder, Items: []model.Item{{ID: "child", Name: "Original", Type: model.ItemRequest}}},
		{ID: "sibling", Name: "Sibling", Type: model.ItemRequest},
	}}}}
	for _, scope := range []string{"root", "folder"} {
		for _, reverse := range []bool{false, true} {
			t.Run(scope+map[bool]string{false: "/local-auth", true: "/remote-auth"}[reverse], func(t *testing.T) {
				metadata, children := authSnapshotCopy(t, base), authSnapshotCopy(t, base)
				cfg := model.Auth{Type: model.AuthBearer, Token: "{{shared}}"}
				if scope == "root" {
					metadata.Collections[0].Auth = cfg
					children.Collections[0].Items[0].Items[0].Name = "Edited child"
				} else {
					metadata.Collections[0].Items[0].Auth = cfg
					children.Collections[0].Items[1].Name = "Edited sibling"
				}
				local, remote := metadata, children
				if reverse {
					local, remote = remote, local
				}
				beforeLocal, beforeRemote := authSnapshotCopy(t, local), authSnapshotCopy(t, remote)
				merged := Merge(base, local, remote, time.Unix(1700000000, 0)).Collections[0]
				if scope == "root" && (merged.Auth != cfg || merged.Items[0].Items[0].Name != "Edited child") || scope == "folder" && (merged.Items[0].Auth != cfg || merged.Items[1].Name != "Edited sibling") {
					t.Fatalf("independent edits lost: %#v", merged)
				}
				if !reflect.DeepEqual(local, beforeLocal) || !reflect.DeepEqual(remote, beforeRemote) {
					t.Fatal("sync mutated input snapshots")
				}
			})
		}
	}
}

func TestAuthEncryptedSnapshotCarriesHierarchy(t *testing.T) {
	t.Setenv("REQO_HOME", t.TempDir())
	r := model.NewRequest("r")
	r.Name, r.Auth = "Private", model.Auth{Type: model.AuthInherit}
	col := model.Collection{ID: "col", Name: "API", Version: 1, Auth: model.Auth{Type: model.AuthBearer, Token: "root-private-marker"}, Items: []model.Item{{ID: "f", Name: "Shared", Type: model.ItemFolder, Auth: model.Auth{Type: model.AuthAPIKey, Key: "X-Key", Value: "{{folder_private}}"}, Items: []model.Item{{ID: r.ID, Name: r.Name, Type: model.ItemRequest, Request: &r}}}}}
	svc := fake.New()
	if err := svc.Collections.Save(col); err != nil {
		t.Fatal(err)
	}
	sealed, err := Seal(Capture(svc), "local-fixture-password")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte("root-private-marker")) || bytes.Contains(sealed, []byte("folder_private")) {
		t.Fatal("encrypted snapshot contains plaintext auth")
	}
	opened, err := Open(sealed, "local-fixture-password")
	if err != nil {
		t.Fatal(err)
	}
	restored := fake.New()
	if err := Apply(restored, opened); err != nil {
		t.Fatal(err)
	}
	got, ok := restored.Collections.Get(col.ID)
	if !ok || !reflect.DeepEqual(got, col) {
		t.Fatalf("encrypted auth restore mismatch: %#v", got)
	}
}
