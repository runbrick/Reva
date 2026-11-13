package importer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/runbrick/reqo/internal/model"
	"github.com/runbrick/reqo/internal/storage"
)

type postmanCollection struct {
	Info struct {
		Name   string `json:"name"`
		Schema string `json:"schema"`
	} `json:"info"`
	Item     []postmanItem   `json:"item"`
	Variable []postmanKV     `json:"variable"`
	Auth     json.RawMessage `json:"auth"`
}

type postmanItem struct {
	Name    string          `json:"name"`
	Item    []postmanItem   `json:"item"`
	Request json.RawMessage `json:"request"`
	Auth    json.RawMessage `json:"auth"`
}

type postmanKV struct {
	Key      string          `json:"key"`
	Value    json.RawMessage `json:"value"`
	Disabled bool            `json:"disabled"`
}

func (kv postmanKV) row() model.KV {
	return model.KV{Key: kv.Key, Value: jsonText(kv.Value), Disabled: kv.Disabled}
}

func jsonText(raw json.RawMessage) string {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return ""
	}
	return string(raw)
}

// Postman imports a Postman Collection v2.0 or v2.1 export. Authentication is
// inherited from collections and folders when a request has no auth of its own.
func Postman(data []byte) (model.Collection, error) {
	var source postmanCollection
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	if err := json.Unmarshal(data, &source); err != nil {
		return model.Collection{}, fmt.Errorf("invalid Postman JSON: %w", err)
	}
	if strings.TrimSpace(source.Info.Name) == "" || source.Item == nil {
		return model.Collection{}, errors.New("Postman collection requires info.name and an item array")
	}
	if schema := source.Info.Schema; schema != "" && !strings.Contains(schema, "/v2.0.") && !strings.Contains(schema, "/v2.1.") {
		return model.Collection{}, errors.New("unsupported Postman collection schema")
	}
	auth, err := postmanAuth(source.Auth, model.Auth{Type: model.AuthNone})
	if err != nil {
		return model.Collection{}, err
	}
	items, err := postmanItems(source.Item, 0)
	if err != nil {
		return model.Collection{}, err
	}
	col := model.Collection{Version: model.SchemaVersion, ID: storage.NewID(), Name: source.Info.Name, Auth: auth, Items: items}
	for _, kv := range source.Variable {
		col.Variables = append(col.Variables, kv.row())
	}
	return col, nil
}

func postmanItems(source []postmanItem, depth int) ([]model.Item, error) {
	if depth > 256 {
		return nil, errors.New("Postman folders are nested too deeply")
	}
	items := make([]model.Item, 0, len(source))
	for _, item := range source {
		var err error
		out := model.Item{ID: storage.NewID(), Name: item.Name}
		if item.Item != nil {
			out.Type = model.ItemFolder
			out.Auth, err = postmanAuth(item.Auth, model.Auth{Type: model.AuthInherit})
			if err == nil {
				out.Items, err = postmanItems(item.Item, depth+1)
			}
		} else if len(item.Request) > 0 && string(item.Request) != "null" {
			out.Type = model.ItemRequest
			req, reqErr := postmanRequest(item.Request)
			err = reqErr
			req.Name, req.ID = item.Name, out.ID
			out.Request = &req
		} else {
			continue // Unknown item types are not requests.
		}
		if err != nil {
			return nil, fmt.Errorf("item %q: %w", item.Name, err)
		}
		items = append(items, out)
	}
	return items, nil
}

func postmanRequest(raw json.RawMessage) (model.Request, error) {
	r := model.NewRequest(storage.NewID())
	r.Auth = model.Auth{Type: model.AuthInherit}
	var urlOnly string
	if json.Unmarshal(raw, &urlOnly) == nil {
		r.URL, r.Params = splitURL(urlOnly)
		return r, nil
	}
	var source struct {
		Method string          `json:"method"`
		URL    json.RawMessage `json:"url"`
		Header json.RawMessage `json:"header"`
		Body   postmanBody     `json:"body"`
		Auth   json.RawMessage `json:"auth"`
	}
	if err := json.Unmarshal(raw, &source); err != nil {
		return r, err
	}
	if source.Method != "" {
		r.Method = model.Method(strings.ToUpper(source.Method))
	}
	var err error
	if r.URL, r.Params, err = postmanURL(source.URL); err != nil {
		return r, err
	}
	if len(source.Header) > 0 && string(source.Header) != "null" {
		var rows []postmanKV
		if err := json.Unmarshal(source.Header, &rows); err == nil {
			for _, row := range rows {
				r.Headers = append(r.Headers, row.row())
			}
		} else {
			var text string
			if err := json.Unmarshal(source.Header, &text); err != nil {
				return r, fmt.Errorf("invalid headers: %w", err)
			}
			for _, line := range strings.Split(text, "\n") {
				if h, ok := headerRow(strings.TrimSpace(line)); ok {
					r.Headers = append(r.Headers, h)
				}
			}
		}
	}
	r.Body = source.Body.model()
	r.Auth, err = postmanAuth(source.Auth, model.Auth{Type: model.AuthInherit})
	return r, err
}

func postmanURL(raw json.RawMessage) (string, []model.KV, error) {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		base, params := splitURL(text)
		return base, params, nil
	}
	var source struct {
		Raw      string          `json:"raw"`
		Protocol string          `json:"protocol"`
		Host     json.RawMessage `json:"host"`
		Port     string          `json:"port"`
		Path     json.RawMessage `json:"path"`
		Query    []postmanKV     `json:"query"`
		Hash     string          `json:"hash"`
	}
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil, nil
	}
	if err := json.Unmarshal(raw, &source); err != nil {
		return "", nil, fmt.Errorf("invalid URL: %w", err)
	}
	text = source.Raw
	if text == "" {
		text = joinedText(source.Host, ".")
		if source.Protocol != "" {
			text = source.Protocol + "://" + text
		}
		if source.Port != "" {
			text += ":" + source.Port
		}
		if path := joinedText(source.Path, "/"); path != "" {
			text += "/" + strings.TrimPrefix(path, "/")
		}
		if source.Hash != "" {
			text += "#" + strings.TrimPrefix(source.Hash, "#")
		}
	}
	base, params := splitURL(text)
	if source.Query != nil {
		params = nil
		for _, row := range source.Query {
			params = append(params, row.row())
		}
	}
	return base, params, nil
}

func joinedText(raw json.RawMessage, separator string) string {
	var parts []string
	if json.Unmarshal(raw, &parts) == nil {
		return strings.Join(parts, separator)
	}
	var text string
	_ = json.Unmarshal(raw, &text)
	return text
}

type postmanBody struct {
	Mode    string `json:"mode"`
	Raw     string `json:"raw"`
	Options struct {
		Raw struct {
			Language string `json:"language"`
		} `json:"raw"`
	} `json:"options"`
	URLEncoded []postmanKV `json:"urlencoded"`
	FormData   []struct {
		Key      string          `json:"key"`
		Value    string          `json:"value"`
		Type     string          `json:"type"`
		Src      json.RawMessage `json:"src"`
		Disabled bool            `json:"disabled"`
	} `json:"formdata"`
	File struct {
		Src string `json:"src"`
	} `json:"file"`
}

func (b postmanBody) model() model.Body {
	out := model.Body{Mode: model.BodyNone}
	switch b.Mode {
	case "raw":
		out.Mode, out.Text = model.BodyRaw, b.Raw
		if strings.EqualFold(b.Options.Raw.Language, "json") {
			out.Mode = model.BodyJSON
		}
	case "urlencoded":
		out.Mode = model.BodyURLEncoded
		for _, row := range b.URLEncoded {
			out.URLEncoded = append(out.URLEncoded, row.row())
		}
	case "formdata":
		out.Mode = model.BodyFormData
		for _, row := range b.FormData {
			field := model.FormField{Key: row.Key, Value: row.Value, File: row.Type == "file", Disabled: row.Disabled}
			if field.File {
				var paths []string
				if json.Unmarshal(row.Src, &paths) == nil {
					for _, path := range paths {
						field.Value = path
						out.Form = append(out.Form, field)
					}
					continue
				}
				field.Value = jsonText(row.Src)
			}
			out.Form = append(out.Form, field)
		}
	case "file":
		out.Mode, out.FilePath = model.BodyBinary, b.File.Src
	}
	return out
}

func postmanAuth(raw json.RawMessage, inherited model.Auth) (model.Auth, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return inherited, nil
	}
	var source struct {
		Type   string          `json:"type"`
		Bearer json.RawMessage `json:"bearer"`
		Basic  json.RawMessage `json:"basic"`
		APIKey json.RawMessage `json:"apikey"`
	}
	if err := json.Unmarshal(raw, &source); err != nil {
		return model.Auth{}, fmt.Errorf("invalid auth: %w", err)
	}
	var fields json.RawMessage
	out := model.Auth{Type: model.AuthNone}
	switch source.Type {
	case "bearer":
		out.Type, fields = model.AuthBearer, source.Bearer
	case "basic":
		out.Type, fields = model.AuthBasic, source.Basic
	case "apikey":
		out.Type, fields = model.AuthAPIKey, source.APIKey
	default:
		return out, nil
	}
	values := make(map[string]string)
	if len(fields) > 0 && string(fields) != "null" {
		var rows []postmanKV
		if err := json.Unmarshal(fields, &rows); err == nil {
			for _, row := range rows {
				values[row.Key] = jsonText(row.Value)
			}
		} else {
			var object map[string]json.RawMessage
			if err := json.Unmarshal(fields, &object); err != nil {
				return out, fmt.Errorf("invalid auth fields: %w", err)
			}
			for key, value := range object {
				values[key] = jsonText(value)
			}
		}
	}
	out.Token, out.Username, out.Password = values["token"], values["username"], values["password"]
	out.Key, out.Value = values["key"], values["value"]
	if out.Type == model.AuthAPIKey {
		out.In = model.APIKeyIn(values["in"])
		if out.In == "" {
			out.In = model.APIKeyInHeader
		}
	}
	return out, nil
}
