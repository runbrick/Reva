package gui

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/runbrick/reva/internal/service/fake"
	"github.com/runbrick/reva/internal/syncing"
	"github.com/vivaui/viva/gfx"
)

func TestSettingsEncryptedExportAndRestore(t *testing.T) {
	t.Setenv("REVA_HOME", t.TempDir())
	svc := fake.New()
	col, err := svc.Collections.Create("Saved API")
	if err != nil {
		t.Fatal(err)
	}
	h := newHarness(t, svc)
	h.click(h.App.S.Settings.Title)
	if err := h.D.Scroll(gfx.ScrollEvent{X: 800, Y: 580, DeltaY: 180, Precise: true}); err != nil {
		t.Fatal(err)
	}
	h.click(h.App.S.Settings.Export)
	h.typeInto(h.App.S.Settings.BackupPassword, "abcdefgh")
	backup := filepath.Join(t.TempDir(), "backup.revabak")
	if err := h.D.QueueSaveFile(backup, nil); err != nil {
		t.Fatal(err)
	}
	h.click(h.App.S.Settings.Save)
	h.waitFor("encrypted export", func() bool { _, err := os.Stat(backup); return err == nil && !h.App.settings.sync.running })
	if err := svc.Collections.Delete(col.ID); err != nil {
		t.Fatal(err)
	}
	h.click(h.App.S.Settings.Restore)
	h.typeInto(h.App.S.Settings.BackupPassword, "abcdefgh")
	if err := h.D.QueueOpenFiles([]string{backup}, nil); err != nil {
		t.Fatal(err)
	}
	h.click(h.App.S.Settings.Save)
	h.waitFor("restore", func() bool { _, ok := svc.Collections.Get(col.ID); return ok && !h.App.settings.sync.running })
	if _, err := os.Stat(filepath.Join(h.App.settings.dir, "before-restore.revabak")); err != nil {
		t.Fatal(err)
	}
}

func TestSettingsWebDAVSyncUploadsAndRestores(t *testing.T) {
	t.Setenv("REVA_HOME", t.TempDir())
	var saved []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "MKCOL" {
			w.WriteHeader(http.StatusCreated)
			return
		}
		if r.Method == http.MethodGet {
			if saved == nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("ETag", "\"one\"")
			_, _ = w.Write(saved)
			return
		}
		if r.Method == http.MethodPut {
			if r.Header.Get("If-None-Match") != "*" {
				t.Error("initial upload not conditional")
			}
			saved, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusCreated)
		}
	}))
	defer server.Close()
	svc := fake.New()
	col, err := svc.Collections.Create("Sync me")
	if err != nil {
		t.Fatal(err)
	}
	h := newHarness(t, svc)
	h.click(h.App.S.Settings.Title)
	s := &h.App.settings.sync
	s.config = syncConfig{Provider: "webdav", URL: server.URL, File: "reva/backup.revabak"}
	password := "abcdefgh"
	s.backupPassword = password
	h.App.startSync("sync", "")
	settingsRepaint(t, h)
	h.waitFor("sync upload", func() bool { return !s.running })
	if len(saved) == 0 {
		t.Fatalf("upload failed: %s", s.errorText)
	}
	if _, err := syncing.Open(saved, password); err != nil {
		t.Fatal(err)
	}
	if err := svc.Collections.Delete(col.ID); err != nil {
		t.Fatal(err)
	}
	h.App.startSync("remote-restore", "")
	settingsRepaint(t, h)
	h.waitFor("remote restore", func() bool { _, ok := svc.Collections.Get(col.ID); return !s.running && ok })
}
