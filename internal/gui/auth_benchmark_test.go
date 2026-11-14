package gui

import (
	"testing"

	"github.com/runbrick/reva/internal/i18n"
	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/service/fake"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

// These benchmarks repaint the actual shared editor and dialog after warming
// their state. Inherit includes the latest-collection lookup and source preview.
func BenchmarkAuthEditorPaint(b *testing.B) {
	for _, kind := range []model.AuthType{model.AuthInherit, model.AuthNone, model.AuthBearer, model.AuthBasic, model.AuthAPIKey} {
		b.Run(string(kind), func(b *testing.B) {
			a, _ := authBenchmarkApp(b)
			tab := a.Active()
			tab.Request.Auth = authBenchmarkConfig(kind)
			authBenchmarkPaint(b, a, func(c ui.C) {
				ui.Column(c).Fill().Pad(Space4).Gap(Space2).Children(func() { a.requestAuthSection(c, tab) })
			}, 800, 400)
		})
	}
}

func BenchmarkAuthDialogPaint(b *testing.B) {
	for _, scope := range []string{"root", "folder"} {
		kinds := model.AuthTypes
		if scope == "folder" {
			kinds = inheritedAuthTypes
		}
		for _, kind := range kinds {
			b.Run(scope+"/"+string(kind), func(b *testing.B) {
				a, col := authBenchmarkApp(b)
				node := collectionNode{collectionID: col.ID, name: col.Name}
				if scope == "folder" {
					node.id, node.name = "folder", "Shared folder"
				}
				a.collectionAuthDialog(node)
				a.collections.authDraft = authBenchmarkConfig(kind)
				authBenchmarkPaint(b, a, a.collectionsDialogs, 1280, 800)
			})
		}
	}
}

func authBenchmarkConfig(kind model.AuthType) model.Auth {
	return model.Auth{Type: kind, Token: "{{token}}", Username: "user", Password: "{{password}}", Key: "X-Key", Value: "{{key}}", In: model.APIKeyInHeader}
}

func authBenchmarkApp(b *testing.B) (*App, model.Collection) {
	b.Helper()
	svc := fake.New()
	r := model.NewRequest("request")
	r.Name, r.Auth = "Get items", model.Auth{Type: model.AuthInherit}
	col := model.Collection{Version: 1, ID: "collection", Name: "Private API", Auth: authBenchmarkConfig(model.AuthBearer), Items: []model.Item{
		{ID: "folder", Name: "Shared folder", Type: model.ItemFolder, Items: []model.Item{{ID: r.ID, Name: r.Name, Type: model.ItemRequest, Request: &r}}},
	}}
	if err := svc.Collections.Save(col); err != nil {
		b.Fatal(err)
	}
	a := New(svc)
	a.S = i18n.For(i18n.En)
	a.Active().Request, a.Active().CollectionID = r, col.ID
	return a, col
}

func authBenchmarkPaint(b *testing.B, a *App, view func(ui.C), width, height int) {
	b.Helper()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &a.theme})
	d, err := app.NewDriver(page.Config(app.Config{Width: width, Height: height}), nil)
	if err != nil {
		b.Fatal(err)
	}
	defer d.Close()
	for range 5 {
		if err := d.Resize(width, height); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := d.Resize(width, height); err != nil {
			b.Fatal(err)
		}
	}
}
