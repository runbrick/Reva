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
	view     components.CodeEditor
	editor   *text.Editor
	input    ui.Handlers
	native   app.TextInput
	snapshot components.CodeEditorSnapshot
	palette  Palette
	runs     [][]scriptRun
	synced   string
	paint    func(*ui.DrawContext)
	dragging bool
}

func (v *scriptCodeEditor) declare(c ui.C, source *string, label string, palette Palette) bool {
	if v.editor == nil {
		v.editor = text.NewEditor(*source)
		v.editor.SetCode(scriptCodeOptions)
		v.synced = *source
		v.input.Text = v.textInput
		v.input.Key = v.key
		v.input.Pointer = v.pointer
		v.input.Scroll = v.view.Scroll
		v.native.NavigationKeys = true
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
	box := ui.Control(c, label, &v.input).FillWidth().Height(Space6 * 6)
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
	}
	v.snapshot.CompositionStart, v.snapshot.CompositionEnd = 0, 0
	if marked, ok := v.editor.Composition(); ok {
		v.snapshot.CompositionStart = v.editor.ByteOffset(marked.Start)
		v.snapshot.CompositionEnd = v.editor.ByteOffset(marked.End)
	}
	theme := *d.Theme
	theme.MutedForeground = d.Fade(v.palette.CodeLineNo)
	v.view.Paint(d.Canvas, theme, d.Rect, v.snapshot)
}

func (v *scriptCodeEditor) textInput(context app.TextInputContext) *app.TextInput {
	v.native.Editor = v.editor
	v.native.HideNativeOverlays = true
	if caret, ok := v.view.CaretBounds(); ok {
		v.native.Bounds = caret
	} else {
		v.native.Bounds = context.Bounds
	}
	return &v.native
}

func (v *scriptCodeEditor) pointer(event gfx.PointerEvent) gfx.EventResult {
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
