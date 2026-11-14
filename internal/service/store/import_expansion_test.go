package store

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/runbrick/reva/internal/importer"
	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/storage"
)

func TestImportExpansionFreshIDsCopyIsolationAndReload(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("REVA_HOME", dir)
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	existing, err := s.Collections().Create("Same name")
	if err != nil {
		t.Fatal(err)
	}
	input := `{"version":1,"id":"` + existing.ID + `","name":"Same name","auth":{"type":"bearer","token":"{{token}}"},"variables":[{"key":"token","value":"literal"}],"items":[{"type":"folder","id":"collision","name":"Folder","auth":{"type":"none"},"items":[{"type":"request","id":"collision","name":"R","request":{"id":"collision","name":"R","method":"POST","url":"{{base}}","params":[{"key":"q","value":"1","description":"kept"}],"headers":[{"key":"X","value":"original"}],"body":{"mode":"form-data","form":[{"key":"text","value":"original"}],"urlencoded":[{"key":"inactive","value":"original"}]},"auth":{"type":"inherit"},"preScript":"throw 'never run during import'","options":{"timeoutMs":1234,"verifyTls":"off"}}}]}]}`
	preview, err := importer.Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	before := snapshotImportFiles(t, dir)
	first, err := s.Collections().Import(*preview.Collection)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Collections().Import(*preview.Collection)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{existing.ID: true, preview.Collection.ID: true, "collision": true}
	for _, col := range []model.Collection{first, second} {
		for _, id := range []string{col.ID, col.Items[0].ID, col.Items[0].Items[0].ID} {
			if id == "" || seen[id] {
				t.Fatal("persisted import reused source, preview, or previous import identity")
			}
			seen[id] = true
		}
		if col.Items[0].Items[0].Request.ID != col.Items[0].Items[0].ID {
			t.Fatal("item/request identities diverged")
		}
	}
	// Neither the input, returned Import value, List nor Get may lend mutable
	// slices to storage. Exercise every nested mutable field from this export.
	mutate := func(col *model.Collection) {
		col.Variables[0].Value = "edited"
		r := col.Items[0].Items[0].Request
		r.Params[0].Description, r.Headers[0].Value = "edited", "edited"
		r.Body.Form[0].Value, r.Body.URLEncoded[0].Value = "edited", "edited"
		col.Items[0].Auth.Type = model.AuthInherit
	}
	want, _ := s.Collections().Get(first.ID)
	mutate(preview.Collection)
	mutate(&first)
	listed := s.Collections().List()
	for i := range listed {
		if listed[i].ID != existing.ID {
			mutate(&listed[i])
		}
	}
	got, _ := s.Collections().Get(want.ID)
	if !reflect.DeepEqual(got, want) {
		t.Fatal("imported store data aliases caller/returned/List slices")
	}
	mutate(&got)
	if live, _ := s.Collections().Get(want.ID); !reflect.DeepEqual(live, want) {
		t.Fatal("Get returned mutable storage-owned collection data")
	}
	reopened, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	reloaded, ok := reopened.Collections().Get(want.ID)
	if !ok || !reflect.DeepEqual(reloaded, want) || len(reopened.Collections().List()) != 3 || len(reopened.LoadErrors()) != 0 {
		t.Fatal("import did not survive store recreation exactly")
	}
	after := snapshotImportFiles(t, dir)
	for path, data := range before {
		if after[path] != data {
			t.Fatal("import changed existing on-disk bytes")
		}
	}
}

func TestImportExpansionActualWriteFailureRepairRetry(t *testing.T) {
	for _, environment := range []bool{false, true} {
		name := "collection"
		if environment {
			name = "environment"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("REVA_HOME", dir)
			s, err := Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.Collections().Create("Existing collection"); err != nil {
				t.Fatal(err)
			}
			existing, err := s.Environments().Create("Existing environment")
			if err != nil {
				t.Fatal(err)
			}
			settings := s.Settings().Get()
			settings.ActiveEnvID = existing.ID
			if err := s.Settings().Save(settings); err != nil {
				t.Fatal(err)
			}
			if err := s.History().Add(model.HistoryEntry{ID: "prior-history", Request: model.NewRequest("prior-request")}); err != nil {
				t.Fatal(err)
			}
			before := snapshotImportFiles(t, dir)
			cols, envs := s.Collections().List(), s.Environments().List()
			preview, err := importer.Parse(`{"version":1,"name":"Imported","variables":[{"key":"v","value":"full payload","description":"kept"}]}`)
			if err != nil {
				t.Fatal(err)
			}
			env := *preview.Environment
			env.ID = storage.NewID()
			col := model.Collection{Version: 1, Name: "Imported", Variables: env.Variables, Items: []model.Item{{Type: model.ItemFolder, ID: "source", Name: "Empty"}}}
			sub := "collections"
			if environment {
				sub = "environments"
			}
			path, backup := filepath.Join(dir, sub), filepath.Join(dir, sub+"-retained")
			if err := os.Rename(path, backup); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("intentional local failure"), 0600); err != nil {
				t.Fatal(err)
			}
			attempt := func() error {
				if environment {
					return s.Environments().Save(env)
				}
				_, err := s.Collections().Import(col)
				return err
			}
			if attempt() == nil {
				t.Fatal("real blocked storage path unexpectedly accepted a write")
			}
			if !reflect.DeepEqual(cols, s.Collections().List()) || !reflect.DeepEqual(envs, s.Environments().List()) || s.Settings().Get() != settings || len(s.History().List()) != 1 {
				t.Fatal("failed write published an empty/partial entity or collateral state")
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(backup, path); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(snapshotImportFiles(t, dir), before) {
				t.Fatal("failed import left orphan/temp data or changed existing bytes")
			}
			if err := attempt(); err != nil {
				t.Fatal(err)
			}
			reloaded, err := Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(reloaded.LoadErrors()) != 0 || len(reloaded.Collections().List())+len(reloaded.Environments().List()) != 3 || reloaded.Settings().Get() != settings || len(reloaded.History().List()) != 1 {
				t.Fatal("repaired retry did not durably save exactly one full entity")
			}
			after := snapshotImportFiles(t, dir)
			if len(after) != len(before)+1 {
				t.Fatal("retry left an extra empty or orphan file")
			}
			for path, data := range before {
				if after[path] != data {
					t.Fatal("retry changed collateral data")
				}
			}
			if environment {
				got, ok := reloaded.Environments().Get(env.ID)
				if !ok || !reflect.DeepEqual(got, env) {
					t.Fatal("environment retry persisted incomplete content")
				}
			}
		})
	}
}

func snapshotImportFiles(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		if strings.HasSuffix(path, ".tmp") {
			t.Error("orphan temporary storage file")
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		files[rel] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestImportExpansionEnvironmentRenameFailureCleanupAndIsolation(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("REVA_HOME", dir)
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := importer.Parse(`{"version":1,"id":"source","name":"Environment","variables":[{"key":"token","value":"literal","disabled":true,"description":"kept"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	env := *parsed.Environment
	env.ID = storage.NewID()
	target := filepath.Join(dir, "environments", env.ID+".json")
	// A directory at the destination allows temporary-file creation and write,
	// then forces the final atomic rename to fail. This exercises cleanup after
	// bytes have actually been written, as well as service publication order.
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	if err := s.Environments().Save(env); err == nil {
		t.Fatal("atomic rename unexpectedly replaced the blocking directory")
	}
	if len(s.Environments().List()) != 0 || len(snapshotImportFiles(t, dir)) != 0 {
		t.Fatal("failed final rename published an environment or leaked a temp file")
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if err := s.Environments().Save(env); err != nil {
		t.Fatal(err)
	}
	want, _ := s.Environments().Get(env.ID)
	env.Variables[0].Value = "caller edit"
	got, _ := s.Environments().Get(env.ID)
	got.Variables[0].Description = "Get edit"
	listed := s.Environments().List()
	listed[0].Variables[0].Disabled = false
	if live, _ := s.Environments().Get(want.ID); !reflect.DeepEqual(live, want) {
		t.Fatal("environment Save/Get/List lend mutable variable rows to callers")
	}
	reloaded, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := reloaded.Environments().Get(want.ID)
	if !ok || !reflect.DeepEqual(got, want) || len(reloaded.LoadErrors()) != 0 || len(snapshotImportFiles(t, dir)) != 1 {
		t.Fatal("repaired environment lost content, aliases mutable rows, or left orphan data")
	}
}
