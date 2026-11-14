package gui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/runbrick/reva/internal/i18n"
	"github.com/runbrick/reva/internal/importer"
	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/service"
	"github.com/runbrick/reva/internal/service/fake"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

const smokeCleanCollection = `{"info":{"name":"Reviewed"},"item":[{"name":"Empty folder","item":[]}],"variable":[{"key":"host","value":"example.test"}]}`
const smokeWarnedCollection = `{"info":{"name":"Reviewed"},"event":[{"listen":"prerequest","script":{"exec":["SECRET_SCRIPT_PAYLOAD"]}}],"item":[]}`

func importSmokeHit(t *testing.T, h *harness, label string) gfx.Hit {
	t.Helper()
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label == label {
			return hit
		}
	}
	t.Fatalf("missing control %q in %q", label, h.hitLabels())
	return gfx.Hit{}
}

func TestImportSmokeReviewGateAndEdit(t *testing.T) {
	for _, warned := range []bool{false, true} {
		t.Run(fmt.Sprint(warned), func(t *testing.T) {
			h := newHarness(t)
			input := smokeCleanCollection
			if warned {
				input = smokeWarnedCollection
			}
			h.click(h.App.S.Import.Title)
			importTypeInto(h, input)
			reviewImportForTest(h)
			if len(h.Svc.Collections.List()) != 0 || len(h.Svc.Environments.List()) != 0 || len(h.App.tabs) != 1 {
				t.Fatal("review mutated data")
			}
			if h.App.importer.preview == nil {
				t.Fatalf("review failed: %s", h.App.importer.err)
			}
			submit := importSmokeHit(t, h, h.App.S.Import.Submit)
			if submit.Disabled != warned {
				t.Fatalf("confirmation disabled=%t, warned=%t", submit.Disabled, warned)
			}
			if warned {
				if h.shows("SECRET_SCRIPT_PAYLOAD") {
					t.Fatal("preview exposed script payload")
				}
				h.click(h.App.S.Import.Submit)
				if len(h.Svc.Collections.List()) != 0 {
					t.Fatal("unacknowledged import wrote data")
				}
				h.click(h.App.S.Import.Acknowledge)
				if !h.App.importer.acknowledged || importSmokeHit(t, h, h.App.S.Import.Submit).Disabled {
					t.Fatal("acknowledgement did not enable confirmation")
				}
			}
			h.click(h.App.S.Import.Back)
			if h.App.importer.preview != nil || h.App.importer.acknowledged || h.App.importer.input != input {
				t.Fatal("back did not invalidate review and preserve input")
			}
			importTypeInto(h, " ")
			reviewImportForTest(h)
			if warned {
				if h.App.importer.acknowledged || !importSmokeHit(t, h, h.App.S.Import.Submit).Disabled {
					t.Fatal("edited input reused acknowledgement")
				}
				h.click(h.App.S.Import.Acknowledge)
			}
			h.click(h.App.S.Import.Submit)
			if len(h.Svc.Collections.List()) != 1 || h.App.importer.open {
				t.Fatalf("confirmed import failed: open=%t preview=%#v error=%q input=%q", h.App.importer.open, h.App.importer.preview, h.App.importer.err, h.App.importer.input)
			}
			// A duplicate queued invocation after success has no authority to write.
			h.App.performImport()
			if len(h.Svc.Collections.List()) != 1 {
				t.Fatal("duplicate commit")
			}
		})
	}
}

func TestImportSmokeNativeEnvironmentAndHAR(t *testing.T) {
	native := model.Collection{Version: 1, ID: "source", Name: "Native", Items: []model.Item{{Type: model.ItemFolder, ID: "folder", Name: "Empty"}}}
	encoded, err := json.Marshal(native)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, input string
		env, warn   bool
	}{
		{"native", string(encoded), false, false},
		{"native environment", `{"version":1,"id":"source","name":"Imported env","variables":[{"key":"token","value":"literal","disabled":true,"description":"kept"}]}`, true, false},
		{"postman environment", `{"_postman_variable_scope":"environment","name":"Imported env","values":[{"key":"host","value":"example.test","enabled":false}]}`, true, false},
		{"HAR", `{"log":{"version":"1.2","entries":[{"request":{"method":"GET","url":"https://example.test/path","headers":[],"queryString":[]}}]}}`, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := fake.New()
			existing, err := svc.Environments.Create("Active")
			if err != nil {
				t.Fatal(err)
			}
			settings := svc.Settings.Get()
			settings.ActiveEnvID = existing.ID
			if err := svc.Settings.Save(settings); err != nil {
				t.Fatal(err)
			}
			h := newHarness(t, svc)
			before := h.App.Active().Request.Clone()
			h.click(h.App.S.Import.Title)
			importTypeInto(h, tc.input)
			reviewImportForTest(h)
			if h.App.importer.preview == nil {
				t.Fatalf("no preview: %s", h.App.importer.err)
			}
			if len(svc.Collections.List()) != 0 || len(svc.Environments.List()) != 1 {
				t.Fatal("review saved prematurely")
			}
			if tc.warn {
				h.click(h.App.S.Import.Acknowledge)
			}
			h.click(h.App.S.Import.Submit)
			if h.App.importer.open {
				t.Fatalf("commit failed: %s", h.App.importer.err)
			}
			if svc.Settings.Get() != settings || h.App.Active().Request.URL != before.URL || len(h.App.tabs) != 1 || len(svc.History.List()) != 0 {
				t.Fatal("import changed settings, active draft, tabs or history")
			}
			if tc.env {
				envs := svc.Environments.List()
				if len(envs) != 2 {
					t.Fatalf("environments=%#v", envs)
				}
				var imported model.Environment
				for _, env := range envs {
					if env.ID != existing.ID {
						imported = env
					}
				}
				if imported.ID == "source" || imported.ID == "" || imported.Version != 1 || len(imported.Variables) != 1 || !imported.Variables[0].Disabled {
					t.Fatalf("imported env=%#v", imported)
				}
			} else if len(svc.Collections.List()) != 1 {
				t.Fatal("collection missing")
			}
		})
	}
}

type smokeRetryEnvironments struct {
	service.EnvironmentService
	fail           bool
	saves, creates int
}

func (s *smokeRetryEnvironments) Create(string) (model.Environment, error) {
	s.creates++
	return model.Environment{}, errors.New("must not create")
}
func (s *smokeRetryEnvironments) Save(env model.Environment) error {
	s.saves++
	if s.fail {
		return errors.New("PRIVATE_DISK_FAILURE")
	}
	return s.EnvironmentService.Save(env)
}

type smokeRetryCollections struct {
	service.CollectionService
	fail    bool
	imports int
}

func (s *smokeRetryCollections) Import(col model.Collection) (model.Collection, error) {
	s.imports++
	if s.fail {
		return model.Collection{}, errors.New("PRIVATE_DISK_FAILURE")
	}
	return s.CollectionService.Import(col)
}
func TestImportSmokeSaveRetryAndCancel(t *testing.T) {
	for _, environment := range []bool{false, true} {
		t.Run(fmt.Sprint(environment), func(t *testing.T) {
			svc := fake.New()
			env := &smokeRetryEnvironments{EnvironmentService: svc.Environments, fail: true}
			cols := &smokeRetryCollections{CollectionService: svc.Collections, fail: true}
			svc.Environments, svc.Collections = env, cols
			h := newHarness(t, svc)
			input := smokeCleanCollection
			if environment {
				input = `{"version":1,"name":"Retry","variables":[]}`
			}
			h.click(h.App.S.Import.Title)
			importTypeInto(h, input)
			reviewImportForTest(h)
			preview := h.App.importer.preview
			h.click(h.App.S.Import.Submit)
			if !h.App.importer.open || h.App.importer.preview != preview || h.App.importer.input != input || !h.shows(h.App.S.Import.SaveError) || h.shows("PRIVATE_DISK_FAILURE") {
				t.Fatal("save failure discarded review or exposed raw error")
			}
			if len(svc.Environments.List()) != 0 || len(svc.Collections.List()) != 0 || env.creates != 0 {
				t.Fatal("failed save left an entity")
			}
			env.fail, cols.fail = false, false
			h.click(h.App.S.Import.Submit)
			if h.App.importer.open || len(svc.Environments.List())+len(svc.Collections.List()) != 1 {
				t.Fatal("retry failed")
			}
			if environment && env.saves != 2 || !environment && cols.imports != 2 {
				t.Fatal("unexpected number of writes")
			}
			h.click(h.App.S.Import.Title)
			importTypeInto(h, input)
			reviewImportForTest(h)
			h.click(h.App.S.Import.Cancel)
			if len(svc.Environments.List())+len(svc.Collections.List()) != 1 {
				t.Fatal("cancel wrote data")
			}
		})
	}
}

func TestImportSmokeCancelLateParseAndReopen(t *testing.T) {
	for _, dismiss := range []string{"cancel", "escape", "outside"} {
		t.Run(dismiss, func(t *testing.T) {
			h := newHarness(t)
			started, finished := make(chan struct{}), make(chan struct{})
			h.App.importer.parse = ui.NewEffectKeyed(func(ctx context.Context, job *importParseJob) (importer.Result, error) {
				close(started)
				<-ctx.Done()
				defer close(finished)
				// Deliberately return a successful stale result after cancellation.
				return importer.Parse(job.input)
			})
			h.click(h.App.S.Import.Title)
			importTypeInto(h, smokeWarnedCollection)
			h.click(h.App.S.Import.Review)
			h.waitFor("parse started", func() bool {
				select {
				case <-started:
					return true
				default:
					return false
				}
			})
			if !importSmokeHit(t, h, h.App.S.Import.LoadFile).Disabled {
				t.Fatal("file action enabled while parsing")
			}
			switch dismiss {
			case "cancel":
				h.click(h.App.S.Import.Cancel)
			case "escape":
				if err := h.D.Key(gfx.KeyEscape, 0); err != nil {
					t.Fatal(err)
				}
			case "outside":
				if err := h.D.Click(5, 5); err != nil {
					t.Fatal(err)
				}
			}
			h.waitFor("parse cancelled", func() bool {
				select {
				case <-finished:
					return true
				default:
					return false
				}
			})
			h.pump()
			if h.App.importer.open || h.App.importer.preview != nil || len(h.Svc.Collections.List()) != 0 {
				t.Fatal("late result reopened or saved import")
			}
			h.App.importer.parse = ui.NewEffectKeyed(parseImport)
			h.click(h.App.S.Import.Title)
			reviewImportForTest(h)
			if h.App.importer.preview == nil || h.App.importer.acknowledged || !importSmokeHit(t, h, h.App.S.Import.Submit).Disabled {
				t.Fatal("reopen reused acceptance or stale work")
			}
		})
	}
}

func TestImportSmokeSameFileAndSizeGuard(t *testing.T) {
	h := newHarness(t)
	path := filepath.Join(t.TempDir(), "input.har")
	for _, input := range []string{smokeCleanCollection, smokeWarnedCollection} {
		if err := os.WriteFile(path, []byte(input), 0600); err != nil {
			t.Fatal(err)
		}
		h.click(h.App.S.Import.Title)
		if err := h.D.QueueOpenFiles([]string{path}, nil); err != nil {
			t.Fatal(err)
		}
		h.click(h.App.S.Import.LoadFile)
		h.waitFor("same file loaded", func() bool { return !h.App.importer.loading && h.App.importer.input == input })
		reviewImportForTest(h)
		if h.App.importer.preview == nil {
			t.Fatal(h.App.importer.err)
		}
		h.click(h.App.S.Import.Cancel)
	}
	h.click(h.App.S.Import.Title)
	// Exercise the guard without asking a native editor to shape 32 MiB of text.
	h.App.importer.input = strings.Repeat(" ", importer.MaxInputBytes+1)
	h.App.reviewImport()
	if h.App.importer.parsing || h.App.importer.err != h.App.S.Import.InputTooLarge {
		t.Fatal("paste size was not guarded")
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(importer.MaxInputBytes + 1); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := readImportFile(context.Background(), &importFile{path: path}); err == nil {
		t.Fatal("large file accepted")
	} else {
		var failure *importer.ParseError
		if !errors.As(err, &failure) || failure.Code != "input_too_large" {
			t.Fatalf("file size error=%v", err)
		}
	}
}

func TestImportSmokeCancelLateFile(t *testing.T) {
	h := newHarness(t)
	started, finished := make(chan struct{}), make(chan struct{})
	h.App.importer.read = ui.NewEffectKeyed(func(ctx context.Context, _ *importFile) (string, error) {
		close(started)
		<-ctx.Done()
		defer close(finished)
		return smokeWarnedCollection, nil
	})
	h.click(h.App.S.Import.Title)
	importTypeInto(h, "keep original input")
	if err := h.D.QueueOpenFiles([]string{"unused.json"}, nil); err != nil {
		t.Fatal(err)
	}
	h.click(h.App.S.Import.LoadFile)
	h.waitFor("file read started", func() bool {
		select {
		case <-started:
			return true
		default:
			return false
		}
	})
	if !importSmokeHit(t, h, h.App.S.Import.Review).Disabled {
		t.Fatal("review enabled during file read")
	}
	h.click(h.App.S.Import.Cancel)
	h.waitFor("file read cancelled", func() bool {
		select {
		case <-finished:
			return true
		default:
			return false
		}
	})
	h.click(h.App.S.Import.Title)
	h.pump()
	if h.App.importer.input != "keep original input" || h.App.importer.loading || h.App.importer.preview != nil || len(h.Svc.Collections.List()) != 0 {
		t.Fatal("late file result changed reopened dialog")
	}
}

func TestImportSmokeLanguagesThemesAndLayout(t *testing.T) {
	for _, lang := range []i18n.Lang{i18n.Zh, i18n.En} {
		for _, theme := range []string{"light", "dark"} {
			for _, size := range [][2]int{{960, 600}, {1280, 800}} {
				t.Run(fmt.Sprintf("%s/%s/%dx%d", lang, theme, size[0], size[1]), func(t *testing.T) {
					svc := fake.New()
					settings := svc.Settings.Get()
					settings.Language, settings.Theme = string(lang), theme
					if err := svc.Settings.Save(settings); err != nil {
						t.Fatal(err)
					}
					h := newHarness(t, svc)
					if err := h.D.Resize(size[0], size[1]); err != nil {
						t.Fatal(err)
					}
					// Enough structural warnings to require scrolling, with no payload in preview.
					var items []string
					for i := 0; i < 30; i++ {
						items = append(items, fmt.Sprintf(`{"name":"Request %d","description":"omitted","request":{"method":"GET","url":"https://example.test"}}`, i))
					}
					input := `{"info":{"name":"Layout"},"item":[` + strings.Join(items, ",") + `]}`
					h.click(h.App.S.Import.Title)
					importSmokeLayout(t, h)
					importTypeInto(h, input)
					reviewImportForTest(h)
					if h.App.importer.preview == nil {
						t.Fatal(h.App.importer.err)
					}
					importSmokeLayout(t, h)
					ack := importSmokeHit(t, h, h.App.S.Import.Acknowledge)
					submit := importSmokeHit(t, h, h.App.S.Import.Submit)
					if !submit.Disabled {
						t.Fatal("warned confirmation enabled")
					}
					for _, hit := range []gfx.Hit{ack, submit, importSmokeHit(t, h, h.App.S.Import.Back), importSmokeHit(t, h, h.App.S.Import.Cancel)} {
						b := hit.Bounds
						if b.X < 0 || b.Y < 0 || b.Width <= 0 || b.Height <= 0 || b.X+b.Width > float32(size[0]) || b.Y+b.Height > float32(size[1]) {
							t.Fatalf("unreachable control %q: %#v", hit.Label, b)
						}
					}
					lastPath := h.App.importer.preview.Warnings[len(h.App.importer.preview.Warnings)-1].Path
					before := h.shows(lastPath)
					// The warning viewport is between summary and acknowledgement.
					for attempt := 0; attempt < 5 && !h.shows(lastPath); attempt++ {
						if err := h.D.Scroll(gfx.ScrollEvent{X: ack.Bounds.X + ack.Bounds.Width/2, Y: ack.Bounds.Y - Space6, DeltaY: 10000, Precise: true}); err != nil {
							t.Fatal(err)
						}
						h.pump()
					}
					importSmokeLayout(t, h)
					if before || !h.shows(lastPath) {
						t.Fatalf("warning list did not scroll to final structural path %s (before=%t)", lastPath, before)
					}
					h.click(h.App.S.Import.Acknowledge)
					if importSmokeHit(t, h, h.App.S.Import.Submit).Disabled {
						t.Fatal("confirmation unreachable after scrolling")
					}
					// A blocking localized error also leaves all input actions reachable.
					h.click(h.App.S.Import.Back)
					h.App.importer.input = `{"info":{"name":"Bad"},"item":42}`
					h.pump()
					reviewImportForTest(h)
					importSmokeLayout(t, h)
					// Deep structural diagnostics must scroll within the body rather
					// than pushing input actions or the footer out of the modal.
					bad := `{"name":"Bad","request":{"method":"GET","url":"https://example.test","auth":{"type":"oauth2"}}}`
					for range 100 {
						bad = `{"name":"Folder","item":[` + bad + `]}`
					}
					h.App.importer.input = `{"info":{"name":"Deep"},"item":[` + bad + `]}`
					h.pump()
					reviewImportForTest(h)
					if len(h.App.importer.err) < 800 {
						t.Fatal("expected long structural diagnostic")
					}
					importSmokeLayout(t, h)
					if b := importSmokeHit(t, h, h.App.S.Import.Input).Bounds; b.Height < h.App.theme.Metrics.ControlHeight {
						t.Fatalf("error collapsed input: %#v", b)
					}
				})
			}
		}
	}
}

func importSmokeLayout(t *testing.T, h *harness) {
	t.Helper()
	snapshot := h.D.Snapshot()
	// Validate only the owned overlay; the shell is separately covered.
	var texts []gfx.Text
	var widths []float32
	var hits []gfx.Hit
	for i, text := range snapshot.Texts {
		if text.Layer >= 2 {
			texts = append(texts, text)
			widths = append(widths, snapshot.TextWidths[i])
		}
	}
	for _, hit := range snapshot.Hits {
		if hit.Layer >= 2 {
			hits = append(hits, hit)
		}
	}
	snapshot.Texts, snapshot.TextWidths, snapshot.Hits = texts, widths, hits
	for _, problem := range headless.CheckLayout(snapshot) {
		t.Errorf("import layout: %s %q: %s", problem.Kind, problem.Text, problem.Detail)
	}
}
