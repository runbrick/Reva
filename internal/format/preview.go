package format

import (
	"html"
	"strings"
)

// HTMLText is a text-only preview, without loading resources or executing code.
// Tags, comments and script/style contents are omitted; entities are decoded.
func HTMLText(source string) string {
	var out strings.Builder
	lower := strings.ToLower(source)
	for i := 0; i < len(source); {
		if source[i] != '<' {
			out.WriteByte(source[i])
			i++
			continue
		}
		if strings.HasPrefix(lower[i:], "<!--") {
			end := strings.Index(lower[i+4:], "-->")
			if end < 0 {
				break
			}
			i += end + 7
			continue
		}
		end, quote := i+1, byte(0)
		for end < len(source) {
			b := source[end]
			if quote != 0 {
				if b == quote {
					quote = 0
				}
			} else if b == '\'' || b == '"' {
				quote = b
			} else if b == '>' {
				break
			}
			end++
		}
		if end == len(source) {
			break
		}
		tag := strings.TrimSpace(lower[i+1 : end])
		name := strings.TrimLeft(tag, "/")
		if at := strings.IndexAny(name, " \t\r\n/"); at >= 0 {
			name = name[:at]
		}
		i = end + 1
		if (name == "script" || name == "style") && !strings.HasPrefix(tag, "/") {
			if close := strings.Index(lower[i:], "</"+name); close >= 0 {
				i += close
			} else {
				break
			}
		}
		switch name {
		case "p", "div", "br", "li", "h1", "h2", "h3", "h4", "tr", "section", "article", "header", "footer":
			out.WriteByte('\n')
		}
	}
	return strings.TrimSpace(html.UnescapeString(out.String()))
}
