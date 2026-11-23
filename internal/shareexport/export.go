package shareexport

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/runbrick/reva/internal/model"
)

type Format uint8

const (
	Native Format = iota
	Postman
	Hurl
	K6
)

func (format Format) Extension() string {
	switch format {
	case Hurl:
		return "hurl"
	case K6:
		return "js"
	default:
		return "json"
	}
}

func (format Format) Filename() string {
	return [...]string{"collection.share.json", "collection.postman_collection.json", "collection.hurl", "collection.k6.js"}[format]
}

type Issue string

const (
	Variables       Issue = "variables"
	Environments    Issue = "environments"
	Credentials     Issue = "credentials"
	Descriptions    Issue = "descriptions"
	URL             Issue = "url"
	QuerySyntax     Issue = "query_syntax"
	Values          Issue = "values"
	BodyValues      Issue = "body_values"
	ContentType     Issue = "content_type"
	Files           Issue = "files"
	Scripts         Issue = "scripts"
	Options         Issue = "options"
	Protocol        Issue = "protocol"
	BodyMode        Issue = "body_mode"
	Method          Issue = "method"
	RowName         Issue = "row_name"
	Disabled        Issue = "disabled"
	Folders         Issue = "folders"
	AuthOverride    Issue = "auth_override"
	UnsupportedAuth Issue = "unsupported_auth"
	DuplicateHeader Issue = "duplicate_header"
	ManagedHeader   Issue = "managed_header"
)

type Notice struct {
	Issue   Issue
	Request int
	Skipped bool
}

type Preview struct {
	Data     []byte
	Notices  []Notice
	Requests int
	Included int
}

const (
	maxNodes    = 512
	maxDepth    = 32
	maxRows     = 2048
	maxURLBytes = 16 << 10
	maxLabel    = 256
	maxOutput   = 512 << 10
)

var ErrLimit = errors.New("share export exceeds template limits")
var ErrInvalid = errors.New("invalid collection or export format")

type builder struct {
	format   Format
	preview  Preview
	notices  map[Notice]bool
	used     map[string]bool
	slots    []model.KV
	flow     []flowRequest
	position int
	nodes    int
}

func Build(col model.Collection, format Format) (Preview, error) {
	if format > K6 {
		return Preview{}, ErrInvalid
	}
	if err := checkBounds(col); err != nil {
		return Preview{}, err
	}
	build := builder{format: format, notices: make(map[Notice]bool), used: make(map[string]bool)}
	for _, row := range col.Variables {
		build.used[row.Key] = true
	}
	if scope := col.EnvironmentScope; scope != nil {
		for _, env := range scope.Local {
			for _, row := range env.Variables {
				build.used[row.Key] = true
			}
		}
	}
	clean := model.Collection{Version: model.SchemaVersion, ID: "collection", Name: col.Name}
	if len(col.Variables) > 0 {
		build.notice(Variables, false)
		if format <= Postman {
			clean.Variables = build.variableNames(col.Variables)
		}
	}
	if col.EnvironmentScope != nil {
		build.notice(Environments, false)
		if format == Native && len(col.EnvironmentScope.Local) > 0 {
			clean.EnvironmentScope = &model.CollectionEnvironmentScope{}
			for index, env := range col.EnvironmentScope.Local {
				clean.EnvironmentScope.Local = append(clean.EnvironmentScope.Local, model.Environment{
					Version: model.SchemaVersion, ID: fmt.Sprintf("environment-%d", index+1), Name: env.Name,
					Variables: build.variableNames(env.Variables),
				})
			}
		}
	}
	clean.Auth = build.auth(col.Auth, "COLLECTION")
	clean.Items = build.items(col.Items, clean.Auth)
	clean.Variables = append(clean.Variables, build.slots...)
	var data []byte
	var err error
	switch format {
	case Native:
		data, err = json.MarshalIndent(clean, "", "  ")
	case Postman:
		data, err = renderPostman(clean)
	case Hurl, K6:
		if len(build.flow) > 0 {
			if format == Hurl {
				data = renderHurl(build.flow)
			} else {
				data = renderK6(build.flow)
			}
		}
	}
	if err != nil {
		return Preview{}, ErrInvalid
	}
	if len(data) > maxOutput-1 {
		return Preview{}, ErrLimit
	}
	if len(data) != 0 {
		build.preview.Data = append(data, '\n')
	}
	return build.preview, nil
}

func checkBounds(col model.Collection) error {
	if err := checkLabel(col.Name); err != nil {
		return err
	}
	rows, nodes := len(col.Variables), 0
	if scope := col.EnvironmentScope; scope != nil {
		rows += len(scope.Local) + len(scope.VisibleIDs)
		if rows > maxRows {
			return ErrLimit
		}
		for _, env := range scope.Local {
			if err := checkLabel(env.Name); err != nil {
				return err
			}
			rows += len(env.Variables)
			if rows > maxRows {
				return ErrLimit
			}
		}
	}
	var walk func([]model.Item, int) error
	walk = func(items []model.Item, depth int) error {
		if len(items) == 0 {
			return nil
		}
		if depth > maxDepth || len(items) > maxNodes-nodes {
			return ErrLimit
		}
		for _, item := range items {
			nodes++
			if nodes > maxNodes {
				return ErrLimit
			}
			if err := checkLabel(item.Name); err != nil {
				return err
			}
			switch {
			case item.Type == model.ItemFolder && item.Request == nil:
				if err := walk(item.Items, depth+1); err != nil {
					return err
				}
			case item.Type == model.ItemRequest && item.Request != nil && len(item.Items) == 0:
				req := item.Request
				if len(req.URL) > maxURLBytes {
					return ErrLimit
				}
				beforeFragment, _, _ := strings.Cut(req.URL, "#")
				_, query, hasQuery := strings.Cut(beforeFragment, "?")
				if hasQuery {
					rows += 1 + strings.Count(query, "&")
				}
				rows += len(req.Params) + len(req.Headers) + len(req.Body.Form) + len(req.Body.URLEncoded)
				if rows > maxRows {
					return ErrLimit
				}
			default:
				return ErrInvalid
			}
		}
		return nil
	}
	if rows > maxRows {
		return ErrLimit
	}
	return walk(col.Items, 1)
}

func checkLabel(label string) error {
	if len(label) > maxLabel {
		return ErrLimit
	}
	if strings.TrimSpace(label) == "" || !utf8.ValidString(label) || strings.ContainsFunc(label, unicode.IsControl) {
		return ErrInvalid
	}
	return nil
}

func (build *builder) notice(issue Issue, skipped bool) {
	notice := Notice{Issue: issue, Request: build.position, Skipped: skipped}
	if !build.notices[notice] {
		build.preview.Notices = append(build.preview.Notices, notice)
		build.notices[notice] = true
	}
}

func (build *builder) slot(name string) string {
	key := "REVA_" + name
	for build.used[key] {
		key += "_"
	}
	build.used[key] = true
	if build.format <= Postman {
		build.slots = append(build.slots, model.KV{Key: key})
	}
	return "{{" + key + "}}"
}

func (build *builder) items(source []model.Item, parent model.Auth) []model.Item {
	clean := make([]model.Item, 0, len(source))
	for _, item := range source {
		build.nodes++
		id := fmt.Sprintf("item-%d", build.nodes)
		if item.Type == model.ItemFolder {
			if build.format >= Hurl {
				build.notice(Folders, false)
			}
			own := build.auth(item.Auth, fmt.Sprintf("FOLDER_%d", build.nodes))
			effective := own
			if effective.Type == "" || effective.Type == model.AuthInherit {
				effective = parent
			}
			children := build.items(item.Items, effective)
			if build.format <= Postman {
				clean = append(clean, model.Item{Type: model.ItemFolder, ID: id, Name: item.Name, Auth: own, Items: children})
			}
			continue
		}
		build.preview.Requests++
		build.position = build.preview.Requests
		if request := build.request(*item.Request, id, item.Name, parent); request != nil && build.format <= Postman {
			clean = append(clean, model.Item{Type: model.ItemRequest, ID: id, Name: item.Name, Request: request})
		}
		build.position = 0
	}
	return clean
}

func (build *builder) request(source model.Request, id, name string, parent model.Auth) *model.Request {
	portable := build.format >= Hurl
	blocked := false
	if build.format != Native && (source.Method == "WS" || unsupportedURLScheme(source.URL) || source.Body.ContentType == "application/graphql+json") {
		build.notice(Protocol, true)
		blocked = true
	} else if !supportsMethod(build.format, source.Method) {
		build.notice(Method, true)
		blocked = true
	}
	if source.PreScript != "" || source.TestScript != "" {
		build.notice(Scripts, portable)
		blocked = blocked || portable
	}
	if source.Options != (model.RequestOptions{}) {
		build.notice(Options, portable)
		blocked = blocked || portable
	}
	if portable && !supportsFlowBody(build.format, source.Body.Mode) {
		if source.Body.Mode == model.BodyBinary || source.Body.Mode == model.BodyFormData {
			build.notice(Files, true)
		}
		build.notice(BodyMode, true)
		blocked = true
	}
	if blocked {
		return nil
	}
	prefix := fmt.Sprintf("REQUEST_%d", build.position)
	build.notice(URL, false)
	clean := model.Request{ID: id, Name: name, Method: source.Method, URL: build.slot(prefix + "_URL")}
	clean.Auth = build.auth(source.Auth, prefix)
	effective := clean.Auth
	if effective.Type == model.AuthInherit {
		effective = parent
	}
	clean.Params = build.redactRows(build.query(source), prefix+"_QUERY", false)
	clean.Headers = build.redactRows(source.Headers, prefix+"_HEADER", true)
	clean.Body = build.body(source.Body, prefix)
	if portable {
		step, ok := build.step(clean, effective)
		if !ok {
			return nil
		}
		build.flow = append(build.flow, step)
	} else if build.format == Postman && authCollides(clean, effective) {
		build.notice(AuthOverride, false)
	}
	build.preview.Included++
	return &clean
}

func supportsMethod(format Format, method model.Method) bool {
	switch format {
	case Native:
		return len(method) <= maxLabel && headerName(string(method))
	case K6:
		return method == model.MethodGet || method == model.MethodHead
	default:
		return slices.Contains(model.Methods, method)
	}
}
