package gui

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/service/fake"
	"github.com/runbrick/reva/internal/shareexport"
	"github.com/vivaui/viva/app"
)

func TestCollectionShareExportPreviewsAndSavesAllFormats(t *testing.T) {
	h, col := collectionLayoutFixture(t)
	var sends atomic.Int32
	h.Svc.Requests.(*fake.Requests).Fn = func(context.Context, model.Request) (model.Response, error) {
		sends.Add(1)
		return model.Response{}, nil
	}
	labels := &h.App.S.Collections
	h.click(fmt.Sprintf(labels.SettingsFor, "Invoices"))
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label == labels.ShareExport {
			t.Fatal("folder settings offered a collection-only export")
		}
	}
	h.click(fmt.Sprintf(labels.SettingsFor, col.Name))
	collectionsReplace(h, labels.Name, "Unsaved share name")
	for _, format := range collectionShareFormats {
		h.click(labels.ShareExport)
		state := &settingsDraft(h).shareExport
		if !state.open || state.format != shareexport.Native || !h.shows(labels.ShareNotices) || !h.shows(labels.ShareFilePreview) {
			t.Fatalf("missing default preview, notices, or file region; hits: %v", h.hitLabels())
		}
		if format != shareexport.Native {
			h.click(labels.ShareConfirm)
			h.click(labels.ShareFormat)
			h.click(shareFormatLabel(labels, format))
			h.pump()
			if state.format != format || state.confirmed {
				t.Fatalf("selection did not rebuild and reset confirmation: %v %t", state.format, state.confirmed)
			}
		}
		preview, err := shareexport.Build(col, format)
		authMust(t, err)
		if !bytes.Equal(state.preview.Data, preview.Data) || !h.shows(fmt.Sprintf(labels.ShareCounts, preview.Included, preview.Requests)) {
			t.Fatalf("incorrect preview/count for %v", format)
		}
		path := filepath.Join(t.TempDir(), format.Filename())
		authMust(t, h.D.QueueSaveFile(path, nil))
		h.click(labels.ShareSave)
		if _, err := os.Stat(path); !os.IsNotExist(err) || state.job != nil {
			t.Fatal("save started before confirmation")
		}
		h.click(labels.ShareConfirm)
		h.click(labels.ShareSave)
		h.waitFor("share export written", func() bool { return !state.open })
		data, err := os.ReadFile(path)
		authMust(t, err)
		if !bytes.Equal(data, preview.Data) || bytes.Contains(data, []byte("saved.test")) {
			t.Fatalf("saved file does not match reviewed bytes for %v", format)
		}
	}
	stored, _ := h.Svc.Collections.Get(col.ID)
	if !reflect.DeepEqual(stored, col) || !h.App.Active().Dirty || len(h.Svc.History.List()) > 0 || sends.Load() != 0 {
		t.Fatal("export changed the saved collection or draft, or executed requests")
	}
}

func TestCollectionShareExportCancelAndSaveFailureKeepCollection(t *testing.T) {
	h, col := collectionLayoutFixture(t)
	labels := &h.App.S.Collections
	h.click(fmt.Sprintf(labels.SettingsFor, col.Name))
	h.click(labels.ShareExport)
	state := &settingsDraft(h).shareExport
	h.click(labels.Cancel)
	if state.open || len(state.preview.Data) != 0 {
		t.Fatal("closing review left share state open")
	}
	h.click(labels.ShareExport)
	h.click(labels.ShareConfirm)
	preview := bytes.Clone(state.preview.Data)
	authMust(t, h.D.QueueSaveFile("", app.ErrCanceled))
	h.click(labels.ShareSave)
	if !state.open || state.job != nil || !bytes.Equal(state.preview.Data, preview) {
		t.Fatal("cancelled picker changed the review")
	}
	blocked := filepath.Join(t.TempDir(), "existing-file")
	authMust(t, os.WriteFile(blocked, []byte("original"), 0600))
	authMust(t, h.D.QueueSaveFile(filepath.Join(blocked, "template.json"), nil))
	h.click(labels.ShareSave)
	h.waitFor("localized save failure", func() bool { return state.problem == labels.ShareSaveFailed })
	if !h.shows(labels.ShareSaveFailed) || !state.open || !bytes.Equal(state.preview.Data, preview) {
		t.Fatal("failed save did not keep the exact review ready for retry")
	}
	data, err := os.ReadFile(blocked)
	authMust(t, err)
	stored, _ := h.Svc.Collections.Get(col.ID)
	if string(data) != "original" || !reflect.DeepEqual(stored, col) {
		t.Fatal("failed save touched existing data")
	}
}
