package shareexport

import (
	"encoding/json"
	"net/url"
	"strings"

	"github.com/runbrick/reva/internal/model"
)

const postmanSchema = "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"

type postmanItem struct {
	Name    string          `json:"name"`
	Items   *[]postmanItem  `json:"item,omitempty"`
	Auth    postmanAuth     `json:"auth,omitempty"`
	Request *postmanRequest `json:"request,omitempty"`
}

type postmanRequest struct {
	Method model.Method `json:"method"`
	URL    postmanURL   `json:"url"`
	Header []model.KV   `json:"header,omitempty"`
	Auth   postmanAuth  `json:"auth,omitempty"`
	Body   *postmanBody `json:"body,omitempty"`
}

type postmanURL struct {
	Raw   string     `json:"raw"`
	Host  []string   `json:"host"`
	Query []model.KV `json:"query,omitempty"`
}

type postmanBody struct {
	Mode       string          `json:"mode"`
	Raw        *string         `json:"raw,omitempty"`
	Options    *postmanOptions `json:"options,omitempty"`
	URLEncoded *[]model.KV     `json:"urlencoded,omitempty"`
	Form       *[]postmanField `json:"formdata,omitempty"`
}

type postmanField struct {
	model.KV
	Type string `json:"type"`
}

type postmanOptions struct {
	Raw struct {
		Language string `json:"language"`
	} `json:"raw"`
}

type postmanAuth map[string]any

func renderPostman(col model.Collection) ([]byte, error) {
	return json.MarshalIndent(struct {
		Info struct {
			Name   string `json:"name"`
			Schema string `json:"schema"`
		} `json:"info"`
		Items     []postmanItem `json:"item"`
		Auth      postmanAuth   `json:"auth,omitempty"`
		Variables []model.KV    `json:"variable,omitempty"`
	}{Info: struct {
		Name   string `json:"name"`
		Schema string `json:"schema"`
	}{col.Name, postmanSchema}, Items: postmanItems(col.Items), Auth: toPostmanAuth(col.Auth, false), Variables: col.Variables}, "", "  ")
}

func postmanItems(items []model.Item) []postmanItem {
	out := make([]postmanItem, 0, len(items))
	for _, item := range items {
		entry := postmanItem{Name: item.Name}
		if item.Type == model.ItemFolder {
			children := postmanItems(item.Items)
			entry.Items = &children
			entry.Auth = toPostmanAuth(item.Auth, false)
		} else {
			req := item.Request
			headers := req.Headers
			if req.Body.Mode == model.BodyRaw && req.Body.ContentType == "application/json" {
				present := false
				for _, header := range headers {
					present = present || !header.Disabled && strings.EqualFold(header.Key, "Content-Type")
				}
				if !present {
					headers = append(headers, model.KV{Key: "Content-Type", Value: "application/json"})
				}
			}
			entry.Request = &postmanRequest{Method: req.Method, Header: headers, Auth: toPostmanAuth(req.Auth, true),
				URL: toPostmanURL(*req), Body: toPostmanBody(req.Body)}
		}
		out = append(out, entry)
	}
	return out
}

func toPostmanURL(req model.Request) postmanURL {
	var raw strings.Builder
	raw.WriteString(req.URL)
	separator := "?"
	for _, row := range req.Params {
		if row.Disabled {
			continue
		}
		raw.WriteString(separator)
		raw.WriteString(url.QueryEscape(row.Key))
		raw.WriteByte('=')
		raw.WriteString(row.Value)
		separator = "&"
	}
	return postmanURL{Raw: raw.String(), Host: []string{req.URL}, Query: req.Params}
}

func toPostmanAuth(auth model.Auth, request bool) postmanAuth {
	switch auth.Type {
	case model.AuthInherit:
		return nil
	case "":
		if !request {
			return nil
		}
		return postmanAuth{"type": "noauth"}
	case model.AuthNone:
		return postmanAuth{"type": "noauth"}
	}
	var fields []model.KV
	switch auth.Type {
	case model.AuthBearer:
		fields = []model.KV{{Key: "token", Value: auth.Token}}
	case model.AuthBasic:
		fields = []model.KV{{Key: "username", Value: auth.Username}, {Key: "password", Value: auth.Password}}
	case model.AuthAPIKey:
		fields = []model.KV{{Key: "key", Value: auth.Key}, {Key: "value", Value: auth.Value}, {Key: "in", Value: string(auth.In)}}
	}
	return postmanAuth{"type": auth.Type, string(auth.Type): fields}
}

func toPostmanBody(body model.Body) *postmanBody {
	switch body.Mode {
	case model.BodyJSON, model.BodyRaw:
		out := &postmanBody{Mode: "raw", Raw: &body.Text, Options: &postmanOptions{}}
		out.Options.Raw.Language = "text"
		if body.Mode == model.BodyJSON {
			out.Options.Raw.Language = "json"
		} else {
			switch body.ContentType {
			case "application/xml":
				out.Options.Raw.Language = "xml"
			case "text/html":
				out.Options.Raw.Language = "html"
			case "application/javascript":
				out.Options.Raw.Language = "javascript"
			}
		}
		return out
	case model.BodyURLEncoded:
		rows := body.URLEncoded
		if rows == nil {
			rows = []model.KV{}
		}
		return &postmanBody{Mode: "urlencoded", URLEncoded: &rows}
	case model.BodyFormData:
		rows := make([]postmanField, 0, len(body.Form))
		for _, field := range body.Form {
			rows = append(rows, postmanField{KV: model.KV{Key: field.Key, Value: field.Value, Disabled: field.Disabled}, Type: "text"})
		}
		return &postmanBody{Mode: "formdata", Form: &rows}
	default:
		return nil
	}
}
