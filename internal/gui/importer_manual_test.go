package gui

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/runbrick/reva/internal/i18n"
	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/service"
	"github.com/runbrick/reva/internal/service/fake"
	"github.com/vivaui/viva/ui"
)

func TestImportManualSourcesAndInputModes(t *testing.T) {
	for _, lang := range []i18n.Lang{i18n.Zh, i18n.En} {
		t.Run(string(lang), func(t *testing.T) {
			svc := fake.New()
			settings := svc.Settings.Get()
			settings.Language = string(lang)
			if err := svc.Settings.Save(settings); err != nil {
				t.Fatal(err)
			}
			h := newHarness(t, svc)
			h.click(h.App.S.Import.Title)
			text := h.App.S.Import
			for _, label := range []string{text.SwaggerSource, text.SwaggerURLSource, text.YAPISource, text.MarkdownSource, text.InsomniaSource, text.APIDocSource, text.ApizzaSource, text.EolinkSource, text.CurrentCollection} {
				if !importSmokeHit(t, h, label).Disabled {
					t.Fatalf("unavailable control is enabled: %s", label)
				}
			}
			h.click(text.ApifoxSource)
			if !h.shows(text.ApifoxHelp) {
				t.Fatal("Apifox export instructions are missing")
			}
			h.click(text.ApiPostSource)
			if !h.shows(text.ApiPostHelp) {
				t.Fatal("ApiPost export instructions are missing")
			}
			h.click(text.CurlSource)
			if !h.App.importer.paste {
				t.Fatal("cURL did not switch to the command editor")
			}
			input := "curl https://example.test/kept"
			importTypeInto(h, input)
			h.click(text.SelectFile)
			h.click(text.PasteContent)
			if h.App.importer.input != input {
				t.Fatal("switching input modes discarded content")
			}
			h.click(text.AutoSource)
			if h.App.importer.source != importAuto || h.App.importer.input != input || len(h.Svc.Collections.List()) != 0 {
				t.Fatal("switching sources discarded input or saved data")
			}
			h.click(text.Details)
			if !h.App.importer.details || !h.shows(string([]rune(text.Help)[:24])) {
				t.Fatal("detailed instructions are missing")
			}
		})
	}
}

func TestImportManualSourceValidation(t *testing.T) {
	for _, source := range []importSource{importNative, importPostman, importApifox, importApiPost, importCurl, importHAR} {
		t.Run(fmt.Sprint(source), func(t *testing.T) {
			h := newHarness(t)
			h.click(h.App.S.Import.Title)
			text := h.App.S.Import
			labels := map[importSource]string{importNative: text.NativeSource, importPostman: text.PostmanSource, importApifox: text.ApifoxSource, importApiPost: text.ApiPostSource, importCurl: text.CurlSource, importHAR: text.HARSource}
			h.click(labels[source])
			input := "curl https://example.test/wrong-source"
			if source == importCurl {
				input = smokeCleanCollection
			}
			importTypeInto(h, input)
			reviewImportForTest(h)
			if h.App.importer.preview != nil || h.App.importer.err != text.SourceMismatch || h.App.importer.input != input || len(h.Svc.Collections.List()) != 0 {
				t.Fatal("a mismatched source reached confirmation, lost input, or wrote data")
			}
			h.click(text.AutoSource)
			reviewImportForTest(h)
			if h.App.importer.preview == nil || h.App.importer.err != "" {
				t.Fatal("auto detection could not review retained input")
			}
			h.click(text.Back)
			h.click(labels[source])
			valid := smokeCleanCollection
			switch source {
			case importNative:
				valid = `{"version":1,"name":"Native","items":[]}`
			case importCurl:
				valid = "curl https://example.test/matching-source"
			case importHAR:
				valid = `{"log":{"version":"1.2","entries":[{"request":{"method":"GET","url":"https://example.test/har","headers":[],"queryString":[]}}]}}`
			}
			h.click(text.Input)
			if _, err := h.D.ShortcutChord(ui.Primary, 'a'); err != nil {
				t.Fatal(err)
			}
			if err := h.D.Type(valid); err != nil {
				t.Fatal(err)
			}
			reviewImportForTest(h)
			if h.App.importer.preview == nil {
				t.Fatal("matching source did not reach review: " + h.App.importer.err)
			}
			if len(h.App.importer.preview.Warnings) > 0 {
				h.click(text.Acknowledge)
			}
			h.click(text.Submit)
			if h.App.importer.open || source != importCurl && len(h.Svc.Collections.List()) != 1 || source == importCurl && h.App.Active().Request.URL != "https://example.test/matching-source" {
				t.Fatal("matching source was not imported")
			}
		})
	}
}

func TestImportAppendChoosesCollection(t *testing.T) {
	svc := fake.New()
	first, err := svc.Collections.Create("Import destination A")
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Collections.Create("Import destination B")
	if err != nil {
		t.Fatal(err)
	}
	h := newHarness(t, svc)
	h.click(h.App.S.Import.Title)
	h.click(h.App.S.Import.CurrentCollection)
	h.click(h.App.S.Import.SelectCollection)
	collectionsOption(h, second.Name)
	if h.App.importer.collectionID != second.ID {
		t.Fatal("destination picker did not select the second collection")
	}
	importTypeInto(h, smokeCleanCollection)
	reviewImportForTest(h)
	h.click(h.App.S.Import.Submit)
	one, _ := svc.Collections.Get(first.ID)
	two, _ := svc.Collections.Get(second.ID)
	if len(one.Items) != 0 || len(two.Items) != 1 || h.App.importer.open {
		t.Fatal("import did not append exclusively to the chosen collection")
	}
}

func TestImportAppendCollectionPreservesScopesAndIdentity(t *testing.T) {
	for _, sharedAuth := range []bool{false, true} {
		t.Run(fmt.Sprint(sharedAuth), func(t *testing.T) {
			svc, dir := importRegressionServices(t)
			col, err := svc.Collections.Create("Destination")
			if err != nil {
				t.Fatal(err)
			}
			col.Auth = model.Auth{Type: model.AuthBearer, Token: "destination-token"}
			col.Variables = []model.KV{{Key: "host", Value: "example.test"}}
			req := model.NewRequest("existing")
			req.Name, req.URL = "Keep", "https://example.test/keep"
			col.Items = []model.Item{{ID: req.ID, Type: model.ItemRequest, Name: req.Name, Request: &req}}
			if err := svc.Collections.Save(col); err != nil {
				t.Fatal(err)
			}
			before := importRegressionDisk(t, dir)
			h := newHarness(t, svc)
			h.click(h.App.S.Import.Title)
			h.click(h.App.S.Import.CurrentCollection)
			auth := ""
			if sharedAuth {
				auth = `"auth":{"type":"bearer","bearer":[{"key":"token","value":"imported-token"}]},`
			}
			input := `{"info":{"name":"Imported folder"},` + auth + `"variable":[{"key":"host","value":"example.test"},{"key":"new","value":"retained"}],"item":[{"name":"Nested","item":[{"name":"Imported","request":{"url":"https://{{host}}/new"}}]}]}`
			importTypeInto(h, input)
			reviewImportForTest(h)
			if h.App.importer.preview == nil || !reflect.DeepEqual(importRegressionDisk(t, dir), before) || !h.shows(fmt.Sprintf(h.App.S.Import.AppendDestination, col.Name)) {
				t.Fatal("append review wrote files or failed to explain its destination")
			}
			sourceID := h.App.importer.preview.Collection.Items[0].Items[0].ID
			h.click(h.App.S.Import.Submit)
			stored, ok := svc.Collections.Get(col.ID)
			if !ok || h.App.importer.open || len(svc.Collections.List()) != 1 || len(stored.Items) != 2 || !reflect.DeepEqual(stored.Items[0], col.Items[0]) || stored.Auth != col.Auth || len(stored.Variables) != 3 {
				t.Fatal("append replaced existing content, lost variables, or created a second collection")
			}
			folder := stored.Items[1]
			imported := folder.Items[0].Items[0]
			if folder.Type != model.ItemFolder || folder.Name != "Imported folder" || imported.ID == sourceID || imported.Request.ID != imported.ID || imported.Request.Auth.Type != model.AuthInherit {
				t.Fatal("append lost folder structure, inheritance, or fresh identities")
			}
			if sharedAuth && (folder.Auth.Type != model.AuthBearer || folder.Auth.Token != "imported-token") || !sharedAuth && folder.Auth.Type != model.AuthNone {
				t.Fatal("destination credentials changed the imported collection's auth scope")
			}
		})
	}
}

func TestImportAppendRechecksDestinationBeforeSaving(t *testing.T) {
	for _, change := range []string{"variable", "delete"} {
		t.Run(change, func(t *testing.T) {
			svc := fake.New()
			col, err := svc.Collections.Create("Destination")
			if err != nil {
				t.Fatal(err)
			}
			h := newHarness(t, svc)
			h.click(h.App.S.Import.Title)
			h.click(h.App.S.Import.CurrentCollection)
			input := `{"info":{"name":"Append"},"variable":[{"key":"host","value":"imported"}],"item":[]}`
			importTypeInto(h, input)
			reviewImportForTest(h)
			preview := h.App.importer.preview
			if preview == nil {
				t.Fatal(h.App.importer.err)
			}
			want := h.App.S.Import.VariableConflict
			if change == "delete" {
				if err := svc.Collections.Delete(col.ID); err != nil {
					t.Fatal(err)
				}
				want = h.App.S.Import.MissingDestination
			} else {
				col.Variables = []model.KV{{Key: "host", Value: "changed"}}
				if err := svc.Collections.Save(col); err != nil {
					t.Fatal(err)
				}
			}
			before := svc.Collections.List()
			h.click(h.App.S.Import.Submit)
			if !h.App.importer.open || h.App.importer.preview != preview || h.App.importer.err != want || !reflect.DeepEqual(svc.Collections.List(), before) {
				t.Fatal("stale destination was overwritten or review was discarded")
			}
		})
	}
}

type failedAppendCollections struct{ service.CollectionService }

func (s failedAppendCollections) Save(model.Collection) error { return errors.New("disk full") }

func TestImportAppendFailureAndVariableConflict(t *testing.T) {
	for _, conflict := range []bool{false, true} {
		t.Run(fmt.Sprint(conflict), func(t *testing.T) {
			svc := fake.New()
			col, err := svc.Collections.Create("Destination")
			if err != nil {
				t.Fatal(err)
			}
			col.Items = []model.Item{{ID: "keep", Type: model.ItemFolder, Name: "Keep"}}
			if conflict {
				col.Variables = []model.KV{{Key: "host", Value: "destination"}}
			}
			if err := svc.Collections.Save(col); err != nil {
				t.Fatal(err)
			}
			svc.Collections = failedAppendCollections{svc.Collections}
			before := svc.Collections.List()
			h := newHarness(t, svc)
			h.click(h.App.S.Import.Title)
			h.click(h.App.S.Import.CurrentCollection)
			input := `{"info":{"name":"Append"},"variable":[{"key":"host","value":"imported"}],"item":[]}`
			importTypeInto(h, input)
			reviewImportForTest(h)
			if conflict {
				if h.App.importer.preview != nil || h.App.importer.err != h.App.S.Import.VariableConflict {
					t.Fatal("conflicting variables reached confirmation")
				}
				h.click(h.App.S.Import.NewCollection)
				reviewImportForTest(h)
				h.click(h.App.S.Import.Submit)
				if len(svc.Collections.List()) != 2 || h.App.importer.open {
					t.Fatal("new-project fallback did not import independently")
				}
			} else {
				h.click(h.App.S.Import.Submit)
				if !h.App.importer.open || h.App.importer.preview == nil || h.App.importer.err != h.App.S.Import.SaveError || !reflect.DeepEqual(before, svc.Collections.List()) {
					t.Fatal("failed append changed cached content or discarded review")
				}
				svc.Collections = svc.Collections.(failedAppendCollections).CollectionService
				h.App.Svc.Collections = svc.Collections
				h.click(h.App.S.Import.Submit)
				stored, _ := svc.Collections.Get(col.ID)
				if h.App.importer.open || len(stored.Items) != 2 || len(stored.Variables) != 1 {
					t.Fatal("append retry did not save exactly once")
				}
			}
		})
	}
}

func TestImportAppendCurlAndEnvironment(t *testing.T) {
	for _, environment := range []bool{false, true} {
		t.Run(fmt.Sprint(environment), func(t *testing.T) {
			svc := fake.New()
			col, err := svc.Collections.Create("Destination")
			if err != nil {
				t.Fatal(err)
			}
			h := newHarness(t, svc)
			h.click(h.App.S.Import.Title)
			h.click(h.App.S.Import.CurrentCollection)
			input := "curl https://example.test/saved"
			if environment {
				input = `{"version":1,"name":"Environment","variables":[]}`
			}
			importTypeInto(h, input)
			reviewImportForTest(h)
			if environment {
				if h.App.importer.preview != nil || h.App.importer.err != h.App.S.Import.EnvironmentDestination || len(svc.Environments.List()) != 0 {
					t.Fatal("environment was offered a collection destination")
				}
				h.click(h.App.S.Import.NewCollection)
				reviewImportForTest(h)
			}
			h.click(h.App.S.Import.Submit)
			stored, _ := svc.Collections.Get(col.ID)
			if h.App.importer.open || environment && (len(stored.Items) != 0 || len(svc.Environments.List()) != 1) || !environment && (len(stored.Items) != 1 || h.App.Active().CollectionID != col.ID || h.App.Active().Request.URL != "https://example.test/saved") {
				t.Fatal("import did not use the chosen destination")
			}
		})
	}
}

func TestImportManualLayout(t *testing.T) {
	for _, lang := range []i18n.Lang{i18n.Zh, i18n.En} {
		for _, theme := range []string{"light", "dark"} {
			for _, size := range [][2]int{{960, 600}, {1280, 720}, {1280, 800}, {1440, 900}} {
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
					h.click(h.App.S.Import.Title)
					importSmokeLayout(t, h)
					h.click(h.App.S.Import.Details)
					importSmokeLayout(t, h)
					h.click(h.App.S.Import.PasteContent)
					importSmokeLayout(t, h)
					importTypeInto(h, "invalid content")
					reviewImportForTest(h)
					importSmokeLayout(t, h)
					for _, label := range []string{h.App.S.Import.Input, h.App.S.Import.Review, h.App.S.Import.LoadFile, h.App.S.Import.Cancel} {
						hit := importSmokeHit(t, h, label)
						if hit.Bounds.Height < h.App.theme.Metrics.ControlHeight-Space2 || hit.Bounds.Y+hit.Bounds.Height > float32(size[1]) {
							t.Fatalf("control is unreachable: %s: %#v", label, hit.Bounds)
						}
					}
				})
			}
		}
	}
}
