package gui

import (
	"github.com/runbrick/reva/internal/format"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

type requestJSONEditor struct {
	view      components.CodeEditor
	native    app.TextInput
	input     ui.Handlers
	snapshot  components.CodeEditorSnapshot
	tokens    [][]format.Token
	palette   Palette
	paint     func(*ui.DrawContext)
	synced    string
	committed string
	dragging  bool
}

func (editor *requestJSONEditor) declare(c ui.C, source *string, label string, palette Palette) bool {
	if editor.native.Editor == nil {
		editor.native = app.TextInput{Editor: text.NewEditor(*source), HideNativeOverlays: true, NavigationKeys: true}
		editor.native.Editor.MoveTo(0, false)
		editor.input = ui.Handlers{Role: gfx.RoleTextField, Text: editor.textInput, Pointer: editor.pointer, Key: editor.key, Scroll: editor.view.Scroll}
		editor.snapshot = components.CodeEditorSnapshot{LineNumbers: true, CurrentLine: true, Wrap: true, HighlightLine: editor.highlight}
		editor.synced, editor.committed = *source, editor.native.Editor.CommittedString()
		editor.paint = editor.draw
	}
	doc := editor.native.Editor
	if *source != editor.synced {
		doc.SetSelection(text.Range{End: doc.RuneCount()})
		doc.Paste(*source)
		editor.synced, editor.committed = *source, doc.CommittedString()
	}
	value := doc.CommittedString()
	changed := value != editor.committed
	if changed {
		*source, editor.synced, editor.committed = value, value, value
	}
	if display := doc.String(); display != editor.snapshot.Value {
		editor.tokens = format.TokenizeJSONDraft(display)
		editor.snapshot.Value = display
	}
	editor.palette = palette
	id := ui.ID(c.Part("json-body"))
	box := ui.Control(c.Part("json-body"), label, &editor.input).
		HitWith(id, label, gfx.HitOptions{
			Role: gfx.RoleTextField, Cursor: gfx.PointerCursorText, Value: editor.snapshot.Value,
			Multiline: true, Placement: &editor.view, HideNativeOverlays: true,
		}).FillWidth().Height(Space6 * 6)
	editor.snapshot.ScrollbarID = editor.input.Parts().At(0)
	box.Draw(editor.paint)
	return changed
}

func (editor *requestJSONEditor) draw(d *ui.DrawContext) {
	doc := editor.native.Editor
	selection := doc.Selection()
	editor.snapshot.Caret = doc.ByteOffset(doc.Caret())
	editor.snapshot.SelectionStart = doc.ByteOffset(selection.Start)
	editor.snapshot.SelectionEnd = doc.ByteOffset(selection.End)
	editor.snapshot.Focused, editor.snapshot.ShowCaret = d.State.Focused, d.State.Focused
	editor.snapshot.CompositionStart, editor.snapshot.CompositionEnd = 0, 0
	if marked, ok := doc.Composition(); ok {
		editor.snapshot.CompositionStart = doc.ByteOffset(marked.Start)
		editor.snapshot.CompositionEnd = doc.ByteOffset(marked.End)
	}
	theme := *d.Theme
	theme.MutedForeground = d.Fade(editor.palette.CodeLineNo)
	editor.view.Paint(d.Canvas, theme, d.Rect, editor.snapshot)
}

func (editor *requestJSONEditor) textInput(context app.TextInputContext) *app.TextInput {
	editor.native.Bounds = context.Bounds
	if caret, ok := editor.view.CaretBounds(); ok {
		editor.native.Bounds = caret
	}
	return &editor.native
}

func (editor *requestJSONEditor) highlight(_ string, line int, _ string, dst []components.CodeEditorRun) []components.CodeEditorRun {
	if line >= len(editor.tokens) {
		return dst
	}
	colors := [...]ui.Color{
		format.Key: editor.palette.CodeKey, format.String: editor.palette.CodeString,
		format.Number: editor.palette.CodeNumber, format.Literal: editor.palette.CodeLiteral,
		format.Punctuation: editor.palette.CodePunct,
	}
	for _, token := range editor.tokens[line] {
		if token.Kind != format.Plain {
			dst = append(dst, components.CodeEditorRun{Start: token.Start, End: token.End, Color: colors[token.Kind]})
		}
	}
	return dst
}

func (editor *requestJSONEditor) pointer(event gfx.PointerEvent) gfx.EventResult {
	if result := editor.view.Pointer(event); result.Handled {
		return result
	}
	switch event.Kind {
	case gfx.PointerUp:
		editor.dragging = false
		return gfx.EventResult{}
	case gfx.PointerDown:
		if event.Button != gfx.PointerButtonPrimary {
			return gfx.EventResult{}
		}
	case gfx.PointerMove:
		if !editor.dragging {
			return gfx.EventResult{}
		}
	default:
		return gfx.EventResult{}
	}
	offset, ok := editor.view.OffsetAt(gfx.Point{X: event.X, Y: event.Y})
	if !ok {
		return gfx.EventResult{}
	}
	doc := editor.native.Editor
	doc.MoveTo(doc.RuneIndex(offset), editor.dragging || event.Modifiers&gfx.ModifierShift != 0)
	editor.dragging = true
	return gfx.EventResult{Handled: true, Repaint: true}
}

func (editor *requestJSONEditor) key(event gfx.KeyEvent) gfx.EventResult {
	if event.Release {
		return gfx.EventResult{}
	}
	movement, extend := components.TextareaMovementForKey(event.Key, event.Modifiers)
	if movement == components.TextareaMoveNone {
		return gfx.EventResult{}
	}
	doc := editor.native.Editor
	offset := editor.view.Move(doc.String(), doc.ByteOffset(doc.Caret()), movement)
	doc.MoveTo(doc.RuneIndex(offset), extend)
	return gfx.EventResult{Handled: true, Repaint: true}
}
