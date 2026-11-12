package gui

import (
	"strings"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

type scriptCompletion struct {
	name string
	call bool
}

var scriptMemberCompletions = map[string][]scriptCompletion{
	"pm":                  {{name: "request"}, {name: "response"}, {name: "variables"}, {name: "environment"}, {name: "test", call: true}, {name: "expect", call: true}},
	"pm.request":          {{name: "url"}, {name: "method"}, {name: "headers"}},
	"pm.request.headers":  {{name: "add", call: true}, {name: "upsert", call: true}, {name: "remove", call: true}},
	"pm.variables":        {{name: "get", call: true}, {name: "set", call: true}},
	"pm.environment":      {{name: "get", call: true}, {name: "set", call: true}},
	"pm.response":         {{name: "code"}, {name: "status"}, {name: "responseTime"}, {name: "headers"}, {name: "text", call: true}, {name: "json", call: true}, {name: "to"}},
	"pm.response.headers": {{name: "get", call: true}},
	"pm.response.to":      {{name: "have"}, {name: "be"}, {name: "equal", call: true}},
	"pm.response.to.have": {{name: "status", call: true}},
	"console":             {{name: "log", call: true}},
	"Math":                {{name: "floor", call: true}, {name: "ceil", call: true}, {name: "round", call: true}, {name: "abs", call: true}, {name: "max", call: true}, {name: "min", call: true}, {name: "random", call: true}},
	"Date":                {{name: "now", call: true}, {name: "parse", call: true}},
	"JSON":                {{name: "parse", call: true}, {name: "stringify", call: true}},
	"Object":              {{name: "keys", call: true}, {name: "create", call: true}},
	"Array":               {{name: "isArray", call: true}},
}

var scriptRootCompletions = []scriptCompletion{
	{name: "pm"}, {name: "console"}, {name: "Math"}, {name: "Date"},
	{name: "JSON"}, {name: "Object"}, {name: "Array"}, {name: "parseInt", call: true},
	{name: "parseFloat", call: true}, {name: "var"}, {name: "function"},
	{name: "return"}, {name: "if"}, {name: "true"}, {name: "false"},
}

func (v *scriptCodeEditor) updateCompletions(tests bool) {
	value := v.editor.String()
	selection := v.editor.Selection()
	v.completions = v.completions[:0]
	if selection.Start != selection.End {
		return
	}
	caret := v.editor.ByteOffset(v.editor.Caret())
	start := caret
	for start > 0 && (scriptWord(value[start-1]) || value[start-1] == '.') {
		start--
	}
	query := value[start:caret]
	if query != v.query || start != v.start {
		v.dismissed, v.selected = false, 0
	}
	v.query, v.start = query, start
	if v.accepted {
		v.dismissed, v.accepted = true, false
	}
	if query == "" || v.inScriptCommentOrString(value, caret) {
		return
	}
	qualifier, prefix, member := "", query, false
	if dot := strings.LastIndexByte(query, '.'); dot >= 0 {
		qualifier, prefix, member = query[:dot], query[dot+1:], true
	}
	var choices []scriptCompletion
	if member {
		choices = scriptMemberCompletions[qualifier]
		if !tests && strings.HasPrefix(qualifier, "pm.response") {
			return
		}
	} else if len(prefix) >= 2 {
		choices = scriptRootCompletions
	}
	for _, choice := range choices {
		if !tests && qualifier == "pm" && choice.name == "response" {
			continue
		}
		if strings.HasPrefix(choice.name, prefix) && choice.name != prefix && len(v.completions) < 6 {
			v.completions = append(v.completions, choice)
		} else if prefix == "" && len(v.completions) < 6 {
			v.completions = append(v.completions, choice)
		}
	}
	if v.selected >= len(v.completions) {
		v.selected = 0
	}
}

func (v *scriptCodeEditor) inScriptCommentOrString(value string, caret int) bool {
	line := strings.Count(value[:caret], "\n")
	if line >= len(v.runs) {
		return false
	}
	start := strings.LastIndexByte(value[:caret], '\n') + 1
	for _, run := range v.runs[line] {
		if run.start <= caret-start-1 && caret-start-1 < run.end && (run.kind == scriptComment || run.kind == scriptString) {
			return true
		}
	}
	return false
}

func (v *scriptCodeEditor) acceptCompletion() {
	if len(v.completions) == 0 || v.selected >= len(v.completions) {
		return
	}
	choice := v.completions[v.selected]
	caret := v.editor.ByteOffset(v.editor.Caret())
	start := caret - len(v.query)
	if dot := strings.LastIndexByte(v.query, '.'); dot >= 0 {
		start += dot + 1
	}
	insert := choice.name
	if choice.call {
		insert += "()"
	}
	v.editor.ReplaceRange(text.Range{Start: v.editor.RuneIndex(start), End: v.editor.Caret()}, insert)
	if choice.call {
		v.editor.MoveTo(v.editor.Caret()-1, false)
	}
	v.dismissed, v.accepted = true, true
}

func (v *scriptCodeEditor) drawCompletions(d *ui.DrawContext) {
	caret, ok := v.view.CaretBounds()
	if !ok {
		return
	}
	const rowHeight float32 = Space6
	width := min(Space6*12, d.Rect.Width-Space4)
	height := float32(len(v.completions))*rowHeight + Space1
	x := min(max(caret.X, d.Rect.X+Space2), d.Rect.X+d.Rect.Width-width-Space2)
	y := caret.Y + caret.Height
	if y+height > d.Rect.Y+d.Rect.Height-Space1 {
		y = caret.Y - height
	}
	y = max(d.Rect.Y+Space1, y)
	v.popup = gfx.Rect{X: x, Y: y, Width: width, Height: height}
	d.Canvas.RoundedBorder(v.popup, d.Theme.Metrics.RadiusControl, 1, d.Theme.Border, d.Theme.Popover)
	for i, option := range v.completions {
		row := gfx.Rect{X: x + 1, Y: y + Space1/2 + float32(i)*rowHeight, Width: width - 2, Height: rowHeight}
		color := d.Theme.Foreground
		if i == v.selected {
			d.Canvas.Fill(row, d.Theme.Accent)
			color = d.Theme.AccentForeground
		}
		label := option.name
		if option.call {
			label += "()"
		}
		d.Canvas.Label(label, row.Inset(gfx.Insets{Left: Space2}), gfx.TextStyle{Size: d.Theme.Metrics.FontSizeS, Color: color, Mono: true})
		d.Canvas.HitWithOptions(v.input.Parts().At(10+i), row, label, gfx.HitOptions{Role: gfx.RoleButton, Cursor: gfx.PointerCursorHand})
	}
}
