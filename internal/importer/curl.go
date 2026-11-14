package importer

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/storage"
)

// Curl parses one curl command, without running a shell, expanding environment
// variables, or opening files. Unsupported options are errors rather than being
// silently reinterpreted as URLs.
func Curl(cmd string) (model.Request, error) {
	r := model.NewRequest(storage.NewID())
	words, err := shellWords(strings.TrimSpace(strings.TrimPrefix(cmd, "\ufeff")))
	if err != nil {
		return r, err
	}
	if len(words) == 0 || !isCurl(words[0]) {
		return r, errors.New("expected a curl command")
	}
	var target, explicitMethod string
	var data []string
	var binaryPath string
	var hasData, get, head, positional bool
	for i := 1; i < len(words); i++ {
		arg := words[i]
		if arg == "--" && !positional {
			positional = true
			continue
		}
		if positional || !strings.HasPrefix(arg, "-") || arg == "-" {
			if target != "" {
				return r, errors.New("only one curl URL can be imported")
			}
			target = arg
			continue
		}
		option, inline, attached := arg, "", false
		if strings.HasPrefix(arg, "--") {
			option, inline, attached = strings.Cut(arg, "=")
		} else if len(arg) > 2 {
			if strings.ContainsAny(arg[1:2], "XHdFuAeb") {
				option, inline, attached = arg[:2], arg[2:], true
			} else {
				// Curl commonly combines switches, e.g. -sSILk.
				for _, flag := range arg[1:] {
					switch flag {
					case 'G':
						get = true
					case 'I':
						head = true
					case 'k', 'L', 's', 'S', 'v', 'i':
					default:
						return r, fmt.Errorf("unsupported curl option -%c", flag)
					}
				}
				continue
			}
		}
		switch option {
		case "-G", "--get":
			get = true
			continue
		case "-I", "--head":
			head = true
			continue
		case "--compressed", "-k", "--insecure", "-L", "--location", "-s", "--silent", "-S", "--show-error", "-v", "--verbose", "-i", "--include", "--globoff", "-g":
			continue
		case "-X", "--request", "--url", "-H", "--header", "-d", "--data", "--data-raw", "--data-binary", "--data-ascii", "--data-urlencode", "-F", "--form", "-u", "--user", "-A", "--user-agent", "-e", "--referer", "-b", "--cookie":
		default:
			return r, fmt.Errorf("unsupported curl option %s", option)
		}
		value := inline
		if !attached {
			if i+1 >= len(words) {
				return r, fmt.Errorf("curl option %s requires a value", option)
			}
			i++
			value = words[i]
		}
		switch option {
		case "-X", "--request":
			if value == "" {
				return r, errors.New("curl request method is empty")
			}
			explicitMethod = strings.ToUpper(value)
		case "--url":
			if target != "" {
				return r, errors.New("only one curl URL can be imported")
			}
			target = value
		case "-H", "--header":
			if row, ok := headerRow(value); ok {
				r.Headers = append(r.Headers, row)
			} else {
				return r, errors.New("curl header requires a name and colon")
			}
		case "-u", "--user":
			r.Auth.Type = model.AuthBasic
			r.Auth.Username, r.Auth.Password, _ = strings.Cut(value, ":")
		case "-A", "--user-agent":
			r.Headers = append(r.Headers, model.KV{Key: "User-Agent", Value: value})
		case "-e", "--referer":
			r.Headers = append(r.Headers, model.KV{Key: "Referer", Value: value})
		case "-b", "--cookie":
			r.Headers = append(r.Headers, model.KV{Key: "Cookie", Value: value})
		case "-F", "--form":
			key, fieldValue, ok := strings.Cut(value, "=")
			if !ok {
				return r, errors.New("curl form requires name=value")
			}
			field := model.FormField{Key: key, Value: fieldValue}
			if strings.HasPrefix(fieldValue, "@") {
				field.File = true
				field.Value, _, _ = strings.Cut(strings.TrimPrefix(fieldValue, "@"), ";")
				field.Value = strings.Trim(field.Value, "\"")
			}
			r.Body.Form = append(r.Body.Form, field)
		default:
			hasData = true
			if option == "--data-urlencode" {
				if key, val, ok := strings.Cut(value, "="); ok {
					value = key + "=" + url.QueryEscape(val)
				} else if strings.Contains(value, "@") {
					return r, errors.New("curl --data-urlencode from a file is unsupported")
				} else {
					value = url.QueryEscape(value)
				}
			} else if option != "--data-raw" && strings.HasPrefix(value, "@") {
				binaryPath = strings.TrimPrefix(value, "@")
				if binaryPath == "" || binaryPath == "-" {
					return r, errors.New("curl file data requires a local file path")
				}
			}
			data = append(data, value)
		}
	}
	if strings.TrimSpace(target) == "" {
		return r, errors.New("curl URL is missing")
	}
	r.URL, r.Params = splitURL(target)
	if len(r.Body.Form) > 0 && hasData {
		return r, errors.New("curl form and data options cannot be combined")
	}
	if get {
		if binaryPath != "" || len(r.Body.Form) > 0 {
			return r, errors.New("curl --get cannot import file or multipart data")
		}
		if hasData {
			r.Params = append(r.Params, queryRows(strings.Join(data, "&"))...)
		}
	} else if len(r.Body.Form) > 0 {
		r.Body.Mode = model.BodyFormData
		r.Method = model.MethodPost
	} else if hasData {
		r.Method = model.MethodPost
		text := strings.Join(data, "&")
		switch {
		case binaryPath != "":
			if len(data) != 1 {
				return r, errors.New("curl file data requires one local file path")
			}
			r.Body.Mode, r.Body.FilePath = model.BodyBinary, binaryPath
		case json.Valid([]byte(text)):
			r.Body.Mode, r.Body.Text = model.BodyJSON, text
		case looksURLEncoded(text):
			r.Body.Mode, r.Body.URLEncoded = model.BodyURLEncoded, queryRows(text)
		default:
			r.Body.Mode, r.Body.Text = model.BodyRaw, text
		}
	}
	if head {
		r.Method = model.MethodHead
	}
	if explicitMethod != "" {
		r.Method = model.Method(explicitMethod)
	}
	for _, header := range r.Headers {
		if strings.EqualFold(header.Key, "Content-Type") && r.Body.Mode == model.BodyRaw {
			r.Body.ContentType = header.Value
		}
	}
	return r, nil
}

func headerRow(value string) (model.KV, bool) {
	key, val, ok := strings.Cut(value, ":")
	if !ok && strings.HasSuffix(value, ";") {
		key, val, ok = strings.TrimSuffix(value, ";"), "", true
	}
	key = strings.TrimSpace(key)
	return model.KV{Key: key, Value: strings.TrimSpace(val)}, ok && key != ""
}

func looksURLEncoded(text string) bool {
	if text == "" {
		return false
	}
	for _, part := range strings.Split(text, "&") {
		key, _, ok := strings.Cut(part, "=")
		if !ok || key == "" || strings.ContainsAny(key, " \r\n\t") {
			return false
		}
	}
	return true
}

// shellWords supports bash quoting (including ANSI-C strings emitted by some
// browsers) and cmd caret escaping. It only tokenizes: substitutions are literal.
func shellWords(input string) ([]string, error) {
	cmdMode := false
	var probeQuote byte
	for i := 0; i < len(input); i++ {
		ch := input[i]
		if ch == '\\' && probeQuote != '\'' && i+1 < len(input) {
			i++
			continue
		}
		if ch == '^' && probeQuote != '\'' && i+1 < len(input) && strings.ContainsRune(" \t\r\n&|<>()^\"\\%{}", rune(input[i+1])) {
			cmdMode = true
			break
		}
		if ch == probeQuote {
			probeQuote = 0
		} else if probeQuote == 0 && (ch == '\'' || ch == '"') {
			probeQuote = ch
		}
	}
	var words []string
	var word strings.Builder
	var quote byte
	var active, ansi bool
	flush := func() {
		if active {
			words = append(words, word.String())
			word.Reset()
			active = false
		}
	}
	for i := 0; i < len(input); i++ {
		ch := input[i]
		// Chrome's cmd export escapes a quote for both cmd and the Windows
		// argument decoder: ^\^". The resulting argument contains one quote.
		if cmdMode && ch == '^' && i+3 < len(input) && input[i+1:i+4] == "\\^\"" {
			word.WriteByte('"')
			active = true
			i += 3
			continue
		}
		if cmdMode && ch == '^' && i+1 < len(input) {
			i++
			if input[i] == '\r' && i+1 < len(input) && input[i+1] == '\n' {
				i++
				continue
			}
			if input[i] == '\n' {
				continue
			}
			word.WriteByte(input[i])
			active = true
			continue
		}
		if quote == '\'' {
			if ch == '\'' {
				quote, ansi = 0, false
			} else if ansi && ch == '\\' {
				decoded, _, tail, err := strconv.UnquoteChar(input[i:], '\'')
				if err != nil {
					return nil, errors.New("invalid escape in curl ANSI-C string")
				}
				word.WriteRune(decoded)
				i = len(input) - len(tail) - 1
			} else {
				word.WriteByte(ch)
			}
			continue
		}
		if !cmdMode && ch == '\\' {
			if i+1 == len(input) {
				return nil, errors.New("unfinished curl backslash escape")
			}
			next := input[i+1]
			if next == '\r' && i+2 < len(input) && input[i+2] == '\n' {
				i += 2
				continue
			}
			if next == '\n' {
				i++
				continue
			}
			if quote == 0 || strings.ContainsRune("\\\"$`", rune(next)) {
				i++
				word.WriteByte(next)
			} else {
				word.WriteByte(ch)
			}
			active = true
			continue
		}
		if cmdMode && ch == '\\' && quote == '"' && i+1 < len(input) && input[i+1] == '"' {
			i++
			word.WriteByte('"')
			continue
		}
		if quote == '"' {
			if ch == '"' {
				quote = 0
			} else {
				word.WriteByte(ch)
			}
			continue
		}
		if ch == '$' && !cmdMode && i+1 < len(input) && input[i+1] == '\'' {
			i++
			quote, ansi, active = '\'', true, true
		} else if ch == '"' || ch == '\'' && !cmdMode {
			quote, active = ch, true
		} else if unicode.IsSpace(rune(ch)) {
			flush()
		} else if strings.ContainsRune(";|<>", rune(ch)) {
			return nil, errors.New("expected one curl command without shell operators")
		} else {
			word.WriteByte(ch)
			active = true
		}
	}
	if quote != 0 {
		return nil, errors.New("unclosed quote in curl command")
	}
	if cmdMode && strings.HasSuffix(input, "^") {
		return nil, errors.New("unfinished curl caret escape")
	}
	flush()
	return words, nil
}
