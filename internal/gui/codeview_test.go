package gui

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"

	"github.com/runbrick/reva/internal/model"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func TestResponseCodeViewReadOnlySelectionAndCopy(t *testing.T) {
	h := responseHarness(t, model.Response{Body: []byte("immutable response")}, nil)
	h.click(h.App.S.Response.CodeBody)
	if _, err := h.D.ShortcutChord(ui.Primary, 'a'); err != nil {
		t.Fatal(err)
	}
	if _, err := h.D.ShortcutChord(ui.Primary, 'c'); err != nil {
		t.Fatal(err)
	}
	if got := h.D.Clipboard(); got != "immutable response" {
		t.Fatalf("selected copy = %q", got)
	}
	// The response control is not a native editable field.
	_ = h.D.Type("overwrite")
	v := h.App.response.views[h.App.Active().ID]
	if v.code.editor.String() != "immutable response" || !h.shows("immutable response") {
		t.Fatal("typing changed read-only response")
	}
}

func TestResponseCodeViewVirtualizesAndWraps(t *testing.T) {
	body := "top\n" + strings.Repeat("middle\n", 4998) + "bottom"
	h := responseHarness(t, model.Response{Body: []byte(body)}, nil)
	v := h.App.response.views[h.App.Active().ID]
	if v.code.view.LineCount() != 5000 || h.shows("bottom") {
		t.Fatalf("code view did not virtualize its lines: count=%d bottom=%v", v.code.view.LineCount(), h.shows("bottom"))
	}
	h.click(h.App.S.Response.CodeBody)
	var bounds gfx.Rect
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label == h.App.S.Response.CodeBody {
			bounds = hit.Bounds
		}
	}
	if err := h.D.Scroll(gfx.ScrollEvent{X: bounds.X + bounds.Width/2, Y: bounds.Y + bounds.Height/2, DeltaY: 500, Precise: true}); err != nil {
		t.Fatal(err)
	}
	if v.code.view.ScrollY() <= 0 || h.shows("top") {
		t.Fatal("wheel did not scroll code view")
	}
	h.App.Active().Response = &model.Response{Body: []byte(strings.Repeat("word ", 200))}
	responsePaint(h)
	h.click(h.App.S.Response.Wrap)
	if !v.wrap || !v.code.snapshot.Wrap || v.code.view.MaxScrollX() != 0 {
		t.Fatal("wrap toggle did not wrap the long line")
	}
}

func TestResponseImagePreviewStandardFormats(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 2, 1))
	source.Set(0, 0, color.RGBA{R: 255, A: 255})
	source.Set(1, 0, color.RGBA{B: 255, A: 255})
	for _, kind := range []string{"png", "jpeg", "gif"} {
		t.Run(kind, func(t *testing.T) {
			var encoded bytes.Buffer
			var err error
			switch kind {
			case "png":
				err = png.Encode(&encoded, source)
			case "jpeg":
				err = jpeg.Encode(&encoded, source, nil)
			case "gif":
				err = gif.Encode(&encoded, source, nil)
			}
			if err != nil {
				t.Fatal(err)
			}
			h := responseHarness(t, model.Response{Body: encoded.Bytes(), Headers: []model.KV{{Key: "Content-Type", Value: "image/" + kind}}}, nil)
			h.click(h.App.S.Response.Preview)
			bitmap := h.App.response.views[h.App.Active().ID].previewImage
			if bitmap.Width != 2 || bitmap.Height != 1 || len(bitmap.Pix) != 8 || h.shows(h.App.S.Response.PreviewUnavailable) {
				t.Fatalf("missing %s preview: %+v", kind, bitmap)
			}
			if kind == "png" && (bitmap.Pix[2] != 255 || bitmap.Pix[4] != 255) {
				t.Fatal("preview pixels are not BGRA")
			}
		})
	}
}

func TestResponseCodeColorsAndSearchHighlightInBothThemes(t *testing.T) {
	h := responseHarness(t, model.Response{Body: []byte(`{"key":"value","number":42,"literal":true}`)}, nil)
	for _, mode := range []string{"light", "dark"} {
		h.App.ApplySettings(model.Settings{Language: "en", Theme: mode})
		responsePaint(h)
		palette := h.App.palette
		for _, want := range []gfx.Color{palette.CodeKey, palette.CodeString, palette.CodeNumber, palette.CodeLiteral, palette.CodePunct} {
			found := false
			for _, label := range h.D.Snapshot().Labels {
				for _, run := range label.Runs {
					found = found || run.Color == want
				}
			}
			if !found {
				t.Errorf("%s code view missing syntax color %+v", mode, want)
			}
		}
		h.click(h.App.S.Response.Search)
		h.typeInto(h.App.S.Response.SearchHint, "value")
		found := false
		for _, primitive := range h.D.Snapshot().Primitives {
			found = found || primitive.Color == h.App.theme.Selection
		}
		if !found {
			t.Errorf("%s search did not highlight with theme.Selection", mode)
		}
		h.click(h.App.S.Response.CloseSearch)
		// Clear the query and selection before checking the next theme's syntax.
		v := h.App.response.views[h.App.Active().ID]
		v.query = ""
		v.code.editor.MoveTo(0, false)
	}
}
