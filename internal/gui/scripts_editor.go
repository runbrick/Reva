package gui

import (
	"strings"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

var scriptCodeOptions = text.CodeOptions{
	TabWidth: 4, AutoIndent: true, AutoClose: true, IndentAfter: "{[(", LineComment: "//",
}

// Each script owns its caret, scroll position and undo history independently.
type scriptCodeEditor struct {
	view        components.CodeEditor
	editor      *text.Editor
	input       ui.Handlers
	native      app.TextInput
	snapshot    components.CodeEditorSnapshot
	palette     Palette
	runs        [][]scriptRun
	synced      string
	paint       func(*ui.DrawContext)
	dragging    bool
	height      float32
	resizeLabel string
	resizing    bool
	startY      float32
	startH      float32

	completions []scriptCompletion
	start       int
	selected    int
	query       string
	dismissed   bool
	accepted    bool
	popup       gfx.Rect
	controlID   gfx.ID
	refocus     bool
}

func (v *scriptCodeEditor) declare(c ui.C, source *string, label, resizeLabel string, palette Palette, tests bool) bool {
	if v.editor == nil {
		v.height = Space6 * 6
		v.editor = text.NewEditor(*source)
		v.editor.SetCode(scriptCodeOptions)
		v.synced = *source
		v.input.Text = v.textInput
		v.input.Key = v.key
		v.input.Pointer = v.pointer
		v.input.Scroll = v.view.Scroll
		v.native.NavigationKeys = true
		v.native.OnEvent = v.onText
		v.paint = v.draw
		v.snapshot.LineNumbers = true
		v.snapshot.CurrentLine = true
		v.snapshot.MatchBrackets = true
		v.snapshot.ShowIndentGuides = true
		v.snapshot.HighlightLine = v.highlight
	}
	if *source != v.synced {
		v.editor.SetSelection(text.Range{Start: 0, End: v.editor.RuneCount()})
		v.editor.Insert(*source)
		v.synced = *source
	}
	changed := v.editor.String() != *source
	if changed {
		*source = v.editor.String()
		v.synced = *source
	}
	if v.snapshot.Value != *source {
		v.snapshot.Value = *source
		v.runs = lexScript(*source)
	}
	v.palette = palette
	v.snapshot.Label = label
	v.resizeLabel = resizeLabel
	v.updateCompletions(tests)
	box := ui.Control(c, label, &v.input).FillWidth().Height(v.height)
	v.snapshot.ScrollbarID = v.input.Parts().At(0)
	v.snapshot.HorizontalScrollbarID = v.input.Parts().At(1)
	box.Draw(v.paint)
	return changed
}

func (v *scriptCodeEditor) draw(d *ui.DrawContext) {
	selection := v.editor.Selection()
	v.snapshot.Caret = v.editor.ByteOffset(v.editor.Caret())
	v.snapshot.SelectionStart = v.editor.ByteOffset(selection.Start)
	v.snapshot.SelectionEnd = v.editor.ByteOffset(selection.End)
	v.snapshot.Focused = d.State.Focused
	v.snapshot.ShowCaret = d.State.Focused && selection.Start == selection.End
	v.snapshot.ID = gfx.ID{}
	if d.State.Focused {
		v.snapshot.ID = d.Canvas.Focused()
		v.controlID = v.snapshot.ID
	}
	if v.refocus && !v.controlID.IsZero() {
		d.Canvas.RequestFocus(v.controlID)
		v.refocus = false
	}
	v.snapshot.CompositionStart, v.snapshot.CompositionEnd = 0, 0
	if marked, ok := v.editor.Composition(); ok {
		v.snapshot.CompositionStart = v.editor.ByteOffset(marked.Start)
		v.snapshot.CompositionEnd = v.editor.ByteOffset(marked.End)
	}
	theme := *d.Theme
	theme.MutedForeground = d.Fade(v.palette.CodeLineNo)
	v.view.Paint(d.Canvas, theme, d.Rect, v.snapshot)
	v.drawGrip(d)
	if d.State.Focused && len(v.completions) > 0 && !v.dismissed {
		v.drawCompletions(d)
	}
}

func (v *scriptCodeEditor) textInput(context app.TextInputContext) *app.TextInput {
	v.native.State = app.TextStateOf(v.editor)
	v.native.HideNativeOverlays = true
	if caret, ok := v.view.CaretBounds(); ok {
		v.native.Bounds = caret
	} else {
		v.native.Bounds = context.Bounds
	}
	return &v.native
}

func (v *scriptCodeEditor) pointer(event gfx.PointerEvent) gfx.EventResult {
	if part, ok := event.Target.In(v.input.Parts()); ok && part == 2 {
		return v.resizePointer(event)
	} else if ok && part >= 10 && part < 10+len(v.completions) {
		if event.Kind == gfx.PointerDown && event.Button == gfx.PointerButtonPrimary {
			v.selected = part - 10
			v.acceptCompletion()
			v.refocus = true
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		return gfx.EventResult{}
	}
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
	v.editor.MoveTo(v.editor.RuneIndex(offset), v.dragging || event.Modifiers&gfx.ModifierShift != 0)
	v.dragging = true
	return gfx.EventResult{Handled: true, Repaint: true}
}

func (v *scriptCodeEditor) key(event gfx.KeyEvent) gfx.EventResult {
	if part, ok := event.Target.In(v.input.Parts()); ok && part == 2 {
		return v.resizeKey(event)
	}
	if !event.Release && len(v.completions) > 0 && !v.dismissed {
		switch event.Key {
		case gfx.KeyDown:
			v.selected = (v.selected + 1) % len(v.completions)
			return gfx.EventResult{Handled: true, Repaint: true}
		case gfx.KeyUp:
			v.selected = (v.selected + len(v.completions) - 1) % len(v.completions)
			return gfx.EventResult{Handled: true, Repaint: true}
		case gfx.KeyTab:
			v.acceptCompletion()
			return gfx.EventResult{Handled: true, Repaint: true}
		case gfx.KeyEscape:
			v.dismissed = true
			return gfx.EventResult{Handled: true, Repaint: true}
		}
	}
	if command, ok := components.CodeEditorCommandForKey(event); ok && command.Apply(v.editor) {
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	movement, extend := components.TextareaMovementForKey(event.Key, event.Modifiers)
	if movement == components.TextareaMoveNone {
		return gfx.EventResult{}
	}
	offset := v.view.Move(v.editor.String(), v.editor.ByteOffset(v.editor.Caret()), movement)
	v.editor.MoveTo(v.editor.RuneIndex(offset), extend)
	return gfx.EventResult{Handled: true, Repaint: true}
}

func (v *scriptCodeEditor) onText(event app.TextEvent) (app.TextState, error) {
	if event.Kind == app.TextCommandEvent && (event.Command == app.TextInsertNewline || event.Command == app.TextInsertLineBreak) &&
		len(v.completions) > 0 && !v.dismissed {
		v.acceptCompletion()
	} else {
		event.Apply(v.editor)
	}
	return app.TextStateOf(v.editor), nil
}

func (v *scriptCodeEditor) resizePointer(event gfx.PointerEvent) gfx.EventResult {
	switch event.Kind {
	case gfx.PointerDown:
		if event.Button != gfx.PointerButtonPrimary {
			return gfx.EventResult{}
		}
		v.resizing, v.startY, v.startH = true, event.Y, v.height
	case gfx.PointerMove:
		if !v.resizing {
			return gfx.EventResult{}
		}
		v.height = min(max(v.startH+event.Y-v.startY, Space6*5), Space6*34)
	case gfx.PointerUp:
		if !v.resizing {
			return gfx.EventResult{}
		}
		v.resizing = false
	default:
		return gfx.EventResult{}
	}
	return gfx.EventResult{Handled: true, Repaint: true}
}

func (v *scriptCodeEditor) resizeKey(event gfx.KeyEvent) gfx.EventResult {
	if event.Release {
		return gfx.EventResult{}
	}
	switch event.Key {
	case gfx.KeyUp:
		v.height = max(Space6*5, v.height-Space4)
	case gfx.KeyDown:
		v.height = min(Space6*34, v.height+Space4)
	default:
		return gfx.EventResult{}
	}
	return gfx.EventResult{Handled: true, Repaint: true}
}

func (v *scriptCodeEditor) drawGrip(d *ui.DrawContext) {
	grip := gfx.Rect{X: d.Rect.X + Space2, Y: d.Rect.Y + d.Rect.Height - Space3, Width: d.Rect.Width - Space4, Height: Space3}
	d.Canvas.Fill(grip, d.Theme.Background)
	for i := float32(0); i < 2; i++ {
		d.Canvas.Rounded(gfx.Rect{X: grip.X + grip.Width/2 - Space4, Y: grip.Y + Space1/2 + i*Space1, Width: Space4 * 2, Height: 1}, 1, d.Theme.MutedForeground)
	}
	d.Canvas.HitWithOptions(v.input.Parts().At(2), grip, v.resizeLabel, gfx.HitOptions{Role: gfx.RoleSlider, Cursor: gfx.PointerCursorResizeVertical})
}

type scriptRun struct {
	start, end int
	kind       scriptTokenKind
}

type scriptTokenKind uint8

const (
	scriptKeyword scriptTokenKind = iota
	scriptString
	scriptNumber
	scriptComment
	scriptAPI
	scriptPunctuation
)

func (v *scriptCodeEditor) highlight(_ string, index int, _ string, dst []components.CodeEditorRun) []components.CodeEditorRun {
	if index >= len(v.runs) {
		return dst
	}
	for _, run := range v.runs[index] {
		color := v.palette.CodePunct
		switch run.kind {
		case scriptKeyword:
			color = v.palette.CodeLiteral
		case scriptString:
			color = v.palette.CodeString
		case scriptNumber:
			color = v.palette.CodeNumber
		case scriptComment:
			color = v.palette.CodeLineNo
		case scriptAPI:
			color = v.palette.CodeKey
		}
		dst = append(dst, components.CodeEditorRun{Start: run.start, End: run.end, Color: color})
	}
	return dst
}

// Tokenize whole source on edits so a block comment keeps its color across lines.
func lexScript(source string) [][]scriptRun {
	lines := strings.Split(source, "\n")
	result := make([][]scriptRun, len(lines))
	comment := false
	for n, line := range lines {
		for i := 0; i < len(line); {
			start := i
			switch {
			case comment:
				if end := strings.Index(line[i:], "*/"); end >= 0 {
					i += end + 2
					comment = false
				} else {
					i = len(line)
				}
				result[n] = append(result[n], scriptRun{start, i, scriptComment})
			case strings.HasPrefix(line[i:], "//"):
				result[n] = append(result[n], scriptRun{start, len(line), scriptComment})
				i = len(line)
			case strings.HasPrefix(line[i:], "/*"):
				comment = true
				i += 2
				if end := strings.Index(line[i:], "*/"); end >= 0 {
					i += end + 2
					comment = false
				} else {
					i = len(line)
				}
				result[n] = append(result[n], scriptRun{start, i, scriptComment})
			case line[i] == '\'' || line[i] == '"':
				quote := line[i]
				i++
				for i < len(line) {
					if line[i] == '\\' && i+1 < len(line) {
						i += 2
					} else if line[i] == quote {
						i++
						break
					} else {
						i++
					}
				}
				result[n] = append(result[n], scriptRun{start, i, scriptString})
			case scriptDigit(line[i]):
				i++
				for i < len(line) && (scriptWord(line[i]) || line[i] == '.') {
					i++
				}
				result[n] = append(result[n], scriptRun{start, i, scriptNumber})
			case scriptWordStart(line[i]):
				i++
				for i < len(line) && scriptWord(line[i]) {
					i++
				}
				word := line[start:i]
				if scriptKeywords[word] && (start == 0 || line[start-1] != '.') {
					result[n] = append(result[n], scriptRun{start, i, scriptKeyword})
				} else if word == "pm" || word == "console" {
					result[n] = append(result[n], scriptRun{start, i, scriptAPI})
				}
			default:
				i++
				if strings.ContainsAny(line[start:i], "{}[]().,;:+-*/=<>!&|?") {
					result[n] = append(result[n], scriptRun{start, i, scriptPunctuation})
				}
			}
		}
	}
	return result
}

var scriptKeywords = map[string]bool{
	"break": true, "case": true, "catch": true, "continue": true, "default": true,
	"delete": true, "do": true, "else": true, "false": true, "finally": true,
	"for": true, "function": true, "if": true, "in": true, "instanceof": true,
	"new": true, "null": true, "return": true, "switch": true, "this": true,
	"throw": true, "true": true, "try": true, "typeof": true, "undefined": true,
	"var": true, "void": true, "while": true, "with": true,
}

func scriptDigit(b byte) bool { return b >= '0' && b <= '9' }
func scriptWordStart(b byte) bool {
	return b == '_' || b == '$' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}
func scriptWord(b byte) bool { return scriptWordStart(b) || scriptDigit(b) }
