package gui

import (
	"github.com/runbrick/reva/internal/format"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

// responseCodeView only connects the response's immutable text and cached tokens
// to Viva's virtualized editor. Geometry, scrolling, wrapping and visible-line
// painting remain in components.CodeEditor.
type responseCodeView struct {
	view     components.CodeEditor
	editor   *text.Editor
	input    ui.Handlers
	snapshot components.CodeEditorSnapshot
	tokens   [][]format.Token
	palette  Palette
	paint    func(*ui.DrawContext)
	dragging bool
}

func (v *responseCodeView) set(value string, tokens [][]format.Token) {
	if v.editor != nil && v.snapshot.Value == value {
		return
	}
	v.editor = text.NewEditor(value)
	v.editor.MoveTo(0, false)
	v.tokens = tokens
	v.view = components.CodeEditor{}
	if v.paint == nil {
		v.paint = v.draw
		v.input.Key = v.key
		v.input.Pointer = v.pointer
		v.input.Scroll = v.view.Scroll
		v.snapshot.HighlightLine = v.highlight
	}
	v.snapshot.Value = value
	v.snapshot.ReadOnly = true
	v.snapshot.LineNumbers = true
	v.snapshot.ShowIndentGuides = true
}

func (v *responseCodeView) declare(c ui.C, label string, palette Palette, wrap bool, marks []text.Span) {
	v.palette = palette
	v.snapshot.Wrap = wrap
	// Viva's Marks use fixed yellow colors. SelectionRanges uses the theme's
	// Selection instead, keeping search highlights correct in both themes.
	v.snapshot.SelectionRanges = marks
	b := ui.Control(c, label, &v.input).FillWidth().Grow(1).MinHeight(Space6 * 3)
	// These identities are allocated by the page, and routed back to this control.
	v.snapshot.ScrollbarID = v.input.Parts().At(0)
	v.snapshot.HorizontalScrollbarID = v.input.Parts().At(1)
	b.Draw(v.paint)
}

func (v *responseCodeView) draw(d *ui.DrawContext) {
	selection := v.editor.Selection()
	v.snapshot.SelectionStart = v.editor.ByteOffset(selection.Start)
	v.snapshot.SelectionEnd = v.editor.ByteOffset(selection.End)
	v.snapshot.Caret = v.editor.ByteOffset(v.editor.Caret())
	v.snapshot.Focused = d.State.Focused
	// The component uses MutedForeground for its line-number lane.
	theme := *d.Theme
	theme.MutedForeground = d.Fade(v.palette.CodeLineNo)
	v.view.Paint(d.Canvas, theme, d.Rect, v.snapshot)
}

func (v *responseCodeView) highlight(_ string, line int, _ string, dst []components.CodeEditorRun) []components.CodeEditorRun {
	if line >= len(v.tokens) {
		return dst
	}
	for _, token := range v.tokens[line] {
		var color ui.Color
		switch token.Kind {
		case format.Key:
			color = v.palette.CodeKey
		case format.String:
			color = v.palette.CodeString
		case format.Number:
			color = v.palette.CodeNumber
		case format.Literal:
			color = v.palette.CodeLiteral
		case format.Punctuation:
			color = v.palette.CodePunct
		default:
			continue
		}
		dst = append(dst, components.CodeEditorRun{Start: token.Start, End: token.End, Color: color})
	}
	return dst
}

func (v *responseCodeView) pointer(event gfx.PointerEvent) gfx.EventResult {
	if result := v.view.Pointer(event); result.Handled {
		return result
	}
	if event.Kind == gfx.PointerUp {
		v.dragging = false
		return gfx.EventResult{}
	}
	if event.Kind != gfx.PointerDown && (event.Kind != gfx.PointerMove || !v.dragging) {
		return gfx.EventResult{}
	}
	if event.Kind == gfx.PointerDown && event.Button != gfx.PointerButtonPrimary {
		return gfx.EventResult{}
	}
	offset, ok := v.view.OffsetAt(gfx.Point{X: event.X, Y: event.Y})
	if !ok {
		return gfx.EventResult{}
	}
	extend := v.dragging || event.Modifiers&gfx.ModifierShift != 0
	v.editor.MoveTo(v.editor.RuneIndex(offset), extend)
	v.dragging = true
	return gfx.EventResult{Handled: true, Repaint: true}
}

func (v *responseCodeView) key(event gfx.KeyEvent) gfx.EventResult {
	if event.Release {
		return gfx.EventResult{}
	}
	movement, extend := components.TextareaMovementForKey(event.Key, event.Modifiers)
	if movement == components.TextareaMoveNone {
		return gfx.EventResult{}
	}
	offset := v.view.Move(v.snapshot.Value, v.editor.ByteOffset(v.editor.Caret()), movement)
	v.editor.MoveTo(v.editor.RuneIndex(offset), extend)
	return gfx.EventResult{Handled: true, Repaint: true}
}

func (v *responseCodeView) selectMatch(match text.Span) {
	v.editor.SetSelection(text.Range{Start: v.editor.RuneIndex(match.Start), End: v.editor.RuneIndex(match.End)})
	v.view.Reveal(match.Start)
}

func (v *responseCodeView) selectedText() string {
	selection := v.editor.Selection()
	return v.snapshot.Value[v.editor.ByteOffset(selection.Start):v.editor.ByteOffset(selection.End)]
}
