package responseexample

import (
	"encoding/json"
	"errors"
	"mime"
	"net/url"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/runbrick/reva/internal/model"
)

const MaxBodyBytes = 256 << 10

var ErrInvalid = errors.New("invalid response example or mapping")

type Sample struct {
	Method      model.Method `json:"method"`
	Path        string       `json:"path,omitempty"`
	Status      int          `json:"status"`
	ContentType string       `json:"contentType,omitempty"`
	Body        string       `json:"body,omitempty"`
}

func ValidMethod(method model.Method) bool {
	return slices.Contains(model.Methods, method)
}

func ValidPath(path string) bool {
	if len(path) > 2048 || !strings.HasPrefix(path, "/") || strings.ContainsAny(path, "?#") {
		return false
	}
	decoded, err := url.PathUnescape(path)
	if err != nil || !utf8.ValidString(decoded) || strings.HasPrefix(decoded, "//") || strings.Contains(decoded, "://") || strings.ContainsAny(decoded, "?#;\\{}%") || strings.ContainsFunc(decoded, unicode.IsControl) {
		return false
	}
	parsed, err := url.ParseRequestURI(path)
	return err == nil && parsed.EscapedPath() == path
}

func HTTPPath(raw string) string {
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.Opaque != "" {
		return ""
	}
	path := parsed.EscapedPath()
	if path == "" {
		path = "/"
	}
	if !ValidPath(path) {
		return ""
	}
	return strings.Clone(path)
}

func FromResponse(method model.Method, path string, response *model.Response) Sample {
	contentType := response.Header("Content-Type")
	if len(contentType) > 256 {
		return Sample{Method: method, Path: path, Status: response.StatusCode}
	}
	mediaType, params, _ := mime.ParseMediaType(contentType)
	sample := Sample{Method: method, Path: path, Status: response.StatusCode, ContentType: safeContentType(mediaType)}
	charset := strings.ToLower(params["charset"])
	if !response.Truncated && response.Size <= int64(len(response.Body)) && len(response.Body) <= MaxBodyBytes &&
		(charset == "" || charset == "utf-8" || charset == "us-ascii") &&
		(contentType == "" || strings.HasPrefix(mediaType, "text/") || sample.ContentType != "" || strings.HasSuffix(mediaType, "+json") || strings.HasSuffix(mediaType, "+xml")) &&
		utf8.Valid(response.Body) {
		body := string(response.Body)
		if validBody(body) {
			sample.Body = body
		}
	}
	return sample
}

func safeContentType(mediaType string) string {
	switch mediaType {
	case "application/json", "application/problem+json", "application/xml", "text/xml", "text/plain", "text/csv":
		return mediaType
	default:
		return ""
	}
}

func validBody(body string) bool {
	return len(body) <= MaxBodyBytes && utf8.ValidString(body) && !strings.ContainsFunc(body, func(char rune) bool {
		return unicode.IsControl(char) && char != '\t' && char != '\r' && char != '\n'
	})
}

func (sample Sample) valid() bool {
	return ValidMethod(sample.Method) && (sample.Path == "" || ValidPath(sample.Path)) && sample.Status >= 100 && sample.Status <= 599 &&
		safeContentType(sample.ContentType) == sample.ContentType && validBody(sample.Body)
}

func (sample Sample) Mapping(includeBody bool) ([]byte, error) {
	if sample.Path == "" || !sample.valid() || (includeBody && !sample.CanExportBody()) {
		return nil, ErrInvalid
	}
	type requestPattern struct {
		Method  model.Method `json:"method"`
		URLPath string       `json:"urlPath"`
	}
	type responseDefinition struct {
		Status  int               `json:"status"`
		Headers map[string]string `json:"headers,omitempty"`
		Body    string            `json:"body,omitempty"`
	}
	mapping := struct {
		Request  requestPattern     `json:"request"`
		Response responseDefinition `json:"response"`
	}{
		Request:  requestPattern{Method: sample.Method, URLPath: sample.Path},
		Response: responseDefinition{Status: sample.Status},
	}
	if sample.ContentType != "" {
		mapping.Response.Headers = map[string]string{"Content-Type": sample.ContentType}
	}
	if includeBody {
		mapping.Response.Body = sample.Body
	}
	data, err := json.MarshalIndent(mapping, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func (sample Sample) CanExportBody() bool {
	return sample.Body != "" && !strings.Contains(sample.Body, "{{")
}
