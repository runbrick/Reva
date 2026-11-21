package shareexport

import (
	"fmt"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/runbrick/reva/internal/model"
)

func (build *builder) variableNames(source []model.KV) []model.KV {
	var rows []model.KV
	for _, row := range source {
		if !rowName(row.Key) {
			build.notice(RowName, false)
			continue
		}
		if row.Description != "" {
			build.notice(Descriptions, false)
		}
		rows = append(rows, model.KV{Key: row.Key, Disabled: row.Disabled})
	}
	return rows
}

func (build *builder) query(request model.Request) []model.KV {
	beforeFragment, _, _ := strings.Cut(request.URL, "#")
	_, query, _ := strings.Cut(beforeFragment, "?")
	type pair struct{ key, value string }
	remaining := make(map[pair]int)
	var rows []model.KV
	for part := range strings.SplitSeq(query, "&") {
		if part == "" {
			continue
		}
		key, value, hasValue := strings.Cut(part, "=")
		decodedKey, keyErr := url.QueryUnescape(key)
		decodedValue, valueErr := url.QueryUnescape(value)
		if !hasValue || keyErr != nil || valueErr != nil || strings.ContainsRune(part, ';') {
			build.notice(QuerySyntax, false)
			continue
		}
		rows = append(rows, model.KV{Key: decodedKey, Value: decodedValue})
		remaining[pair{decodedKey, decodedValue}]++
	}
	for _, row := range request.Params {
		key := pair{row.Key, row.Value}
		if !row.Disabled && remaining[key] > 0 {
			remaining[key]--
			if row.Description != "" {
				build.notice(Descriptions, false)
			}
			continue
		}
		rows = append(rows, row)
	}
	return rows
}

func unsupportedURLScheme(raw string) bool {
	base, _, hasScheme := strings.Cut(strings.TrimSpace(raw), "://")
	if !hasScheme || base == "" || strings.ContainsAny(base, "{} /?#") {
		return false
	}
	return !strings.EqualFold(base, "http") && !strings.EqualFold(base, "https")
}

func (build *builder) redactRows(source []model.KV, prefix string, header bool) []model.KV {
	var rows []model.KV
	for index, row := range source {
		if row.Description != "" {
			build.notice(Descriptions, false)
		}
		if row.Disabled && build.format >= Hurl {
			build.notice(Disabled, false)
			continue
		}
		if !rowName(row.Key) || header && !headerName(row.Key) || build.format == Hurl && !hurlKey(row.Key) {
			build.notice(RowName, false)
			continue
		}
		value := ""
		if row.Value != "" {
			issue := Values
			if strings.HasSuffix(prefix, "_FORM") {
				issue = BodyValues
			}
			build.notice(issue, false)
			value = build.slot(fmt.Sprintf("%s_%d", prefix, index+1))
		}
		rows = append(rows, model.KV{Key: row.Key, Value: value, Disabled: row.Disabled})
	}
	return rows
}

func rowName(name string) bool {
	return len(name) <= maxLabel && utf8.ValidString(name) && strings.TrimSpace(name) != "" &&
		!strings.ContainsAny(name, "{}") && !strings.ContainsFunc(name, unicode.IsControl)
}

func headerName(name string) bool {
	return name != "" && !strings.ContainsFunc(name, func(char rune) bool {
		return !asciiAlphanumeric(char) && !strings.ContainsRune("!#$%&'*+-.^_`|~", char)
	})
}

func hurlKey(name string) bool {
	return name != "" && !strings.ContainsFunc(name, func(char rune) bool {
		return !asciiAlphanumeric(char) && !strings.ContainsRune("-_.~", char)
	})
}

func asciiAlphanumeric(char rune) bool {
	return char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z' || char >= '0' && char <= '9'
}

const invalidAuth model.AuthType = "invalid"

func (build *builder) auth(source model.Auth, prefix string) model.Auth {
	if source.Token != "" || source.Username != "" || source.Password != "" || source.Value != "" {
		build.notice(Credentials, false)
	}
	clean := model.Auth{Type: source.Type}
	switch source.Type {
	case "", model.AuthInherit, model.AuthNone:
		return clean
	case model.AuthBearer:
		build.notice(Credentials, false)
		clean.Token = build.slot(prefix + "_BEARER_TOKEN")
		return clean
	case model.AuthBasic:
		build.notice(Credentials, false)
		if build.format >= Hurl {
			clean.Value = build.slot(prefix + "_BASIC_AUTHORIZATION")
		} else {
			clean.Username = build.slot(prefix + "_BASIC_USERNAME")
			clean.Password = build.slot(prefix + "_BASIC_PASSWORD")
		}
		return clean
	case model.AuthAPIKey:
		in := source.In
		if in == "" {
			in = model.APIKeyInHeader
		}
		if rowName(source.Key) && (in == model.APIKeyInQuery || in == model.APIKeyInHeader && headerName(source.Key)) &&
			(build.format != Hurl || hurlKey(source.Key)) {
			build.notice(Credentials, false)
			return model.Auth{Type: source.Type, In: in, Key: source.Key, Value: build.slot(prefix + "_API_KEY")}
		}
	}
	build.notice(UnsupportedAuth, false)
	if build.format >= Hurl {
		return model.Auth{Type: invalidAuth}
	}
	return model.Auth{Type: model.AuthNone}
}

func supportsFlowBody(format Format, mode model.BodyMode) bool {
	return mode == "" || mode == model.BodyNone || format == Hurl && (mode == model.BodyJSON || mode == model.BodyRaw)
}

func (build *builder) body(source model.Body, prefix string) model.Body {
	clean := model.Body{Mode: source.Mode}
	if source.FilePath != "" {
		build.notice(Files, false)
	}
	if source.Text != "" || len(source.Form)+len(source.URLEncoded) > 0 {
		build.notice(BodyValues, false)
	}
	switch source.Mode {
	case "", model.BodyNone:
	case model.BodyJSON, model.BodyRaw:
		if source.Text != "" {
			clean.Text = build.slot(prefix + "_BODY")
		}
		if source.Mode == model.BodyRaw {
			switch source.ContentType {
			case "", "text/plain", "application/json", "application/xml", "text/html", "application/javascript":
				clean.ContentType = source.ContentType
			default:
				build.notice(ContentType, false)
			}
		} else if build.format == Native && source.ContentType == "application/graphql+json" {
			clean.ContentType = source.ContentType
		}
	case model.BodyURLEncoded:
		clean.URLEncoded = build.redactRows(source.URLEncoded, prefix+"_FORM", false)
	case model.BodyFormData:
		for index, field := range source.Form {
			if !rowName(field.Key) {
				build.notice(RowName, false)
				continue
			}
			if field.File {
				build.notice(Files, false)
				if build.format == Native {
					clean.Form = append(clean.Form, model.FormField{Key: field.Key, File: true, Disabled: true})
				}
				continue
			}
			value := ""
			if field.Value != "" {
				value = build.slot(fmt.Sprintf("%s_FORM_%d", prefix, index+1))
			}
			clean.Form = append(clean.Form, model.FormField{Key: field.Key, Value: value, Disabled: field.Disabled})
		}
	case model.BodyBinary:
		build.notice(Files, false)
		if build.format != Native {
			clean.Mode = model.BodyNone
		}
	default:
		clean.Mode = model.BodyNone
		build.notice(BodyMode, false)
	}
	return clean
}
