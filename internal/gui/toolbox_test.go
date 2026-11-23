package gui

import (
	"context"
	"encoding/base64"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/service/fake"
	"github.com/runbrick/reva/internal/toolbox"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

func TestToolboxNavigationConversionAndDrafts(t *testing.T) {
	h := newHarness(t)
	labels := h.App.S.Toolbox
	request := h.App.Active()
	request.Request.URL = "https://example.test/draft"
	h.click(labels.Title)
	if h.App.Main != PageToolbox || !h.shows(labels.Encoding) || !h.shows(labels.Other) {
		t.Fatal("toolbox rail button did not open the grouped tools")
	}
	h.click(labels.Encoding)
	if slices.Contains(h.hitLabels(), "Base64") {
		t.Fatal("encoding group did not collapse")
	}
	h.click(labels.Encoding)
	h.typeInto(labels.Input, "你好")
	h.click(labels.Encode)
	want := base64.StdEncoding.EncodeToString([]byte("你好"))
	h.waitFor("Base64 output", func() bool { return h.shows(want) })
	h.click(labels.Copy)
	if h.D.Clipboard() != want {
		t.Fatalf("clipboard = %q, want %q", h.D.Clipboard(), want)
	}
	h.click("URL")
	h.typeInto(labels.Input, "%GG")
	h.click(labels.Decode)
	h.waitFor("invalid URL encoding message", func() bool { return h.shows(labels.Invalid) })
	h.click("Base64")
	if !h.shows(want) {
		t.Fatal("tool switch lost the Base64 result")
	}
	h.click(h.App.S.Shell.APIs)
	if h.App.Active() != request || request.Request.URL != "https://example.test/draft" {
		t.Fatal("toolbox changed the request draft")
	}
	h.click(labels.Title)
	h.click(labels.Clear)
	if h.shows(want) || h.App.toolbox.drafts[toolbox.Base64].input != "" {
		t.Fatal("clear left the previous input or result")
	}
}

func TestToolboxLayoutAtMinimumWindow(t *testing.T) {
	for _, language := range []string{"zh", "en"} {
		for _, mode := range []string{"light", "dark"} {
			t.Run(language+"/"+mode, func(t *testing.T) {
				h := newHarness(t)
				settings := model.DefaultSettings()
				settings.Language, settings.Theme = language, mode
				h.App.ApplySettings(settings)
				if err := h.D.Resize(960, 600); err != nil {
					t.Fatal(err)
				}
				labels := h.App.S.Toolbox
				h.click(labels.Title)
				h.click("JWT")
				if !h.shows(labels.JWTHint) {
					t.Fatal("JWT inspection did not show the unverified-signature warning")
				}
				h.click(labels.AES)
				for _, problem := range headless.CheckLayout(h.D.Snapshot()) {
					if problem.Bounds.Y >= TitlebarHeight && problem.Bounds.X >= RailWidth {
						t.Errorf("toolbox layout: %+v", problem)
					}
				}
				for _, name := range []string{labels.Input, labels.Encrypt, labels.Decrypt} {
					for _, hit := range h.D.Snapshot().Hits {
						if hit.Label != name {
							continue
						}
						bounds := hit.Bounds
						if bounds.Width <= 0 || bounds.Height <= 0 || bounds.X < 0 || bounds.Y < 0 || bounds.X+bounds.Width > 960 || bounds.Y+bounds.Height > 600 {
							t.Fatalf("control %q outside the window: %+v", name, bounds)
						}
					}
					if !slices.Contains(h.hitLabels(), name) {
						t.Fatalf("missing control %q", name)
					}
				}
				if dir := os.Getenv("REVA_THEME_CAPTURE_DIR"); dir != "" {
					h.App.toolbox.selected = toolbox.Base64
					page := ui.NewPage(ui.PageConfig{View: h.App.Page, Theme: &h.App.theme})
					session, err := headless.Open(ui.Configure(page, app.Config{Width: 1280, Height: 800}), nil, headless.Options{Scale: 1})
					if err != nil {
						t.Fatal(err)
					}
					defer session.Close()
					if err := session.Screenshot(filepath.Join(dir, "toolbox-"+language+"-"+mode+".png")); err != nil {
						t.Fatal(err)
					}
				}
			})
		}
	}
}

func TestToolboxPreviewsLocalImages(t *testing.T) {
	path := filepath.Join(t.TempDir(), "local.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, image.NewRGBA(image.Rect(0, 0, 12, 6))); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	job := toolbox.Job{Tool: toolbox.Image}
	local, err := runToolbox(context.Background(), &toolboxWork{job: job, path: path})
	if err != nil || local.image.Width != 12 || local.image.Height != 6 {
		t.Fatalf("local image: %dx%d, %v", local.image.Width, local.image.Height, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	job.Input = "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
	inline, err := runToolbox(context.Background(), &toolboxWork{job: job})
	if err != nil || inline.image.Width != 12 || inline.image.Height != 6 {
		t.Fatalf("inline image: %dx%d, %v", inline.image.Width, inline.image.Height, err)
	}
}

func BenchmarkToolboxPaint(b *testing.B) {
	a := New(fake.New())
	a.Main = PageToolbox
	page := ui.NewPage(ui.PageConfig{View: a.Page, Theme: &a.theme})
	driver, err := app.NewDriver(ui.Configure(page, app.Config{Width: 1280, Height: 800}), nil)
	if err != nil {
		b.Fatal(err)
	}
	defer driver.Close()
	b.ReportAllocs()
	for b.Loop() {
		if err := driver.Resize(1280, 800); err != nil {
			b.Fatal(err)
		}
	}
}
