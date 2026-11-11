// Package request sends HTTP requests using the standard HTTP transport.
package request

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/runbrick/reqo/internal/model"
	"github.com/runbrick/reqo/internal/service"
	"github.com/runbrick/reqo/internal/vars"
)

const (
	DefaultTimeout      = 60 * time.Second
	DefaultMaxBodyBytes = 10 << 20
)

// Service implements service.RequestService. Configure its fields before use;
// Send may then be called concurrently. Nonpositive limits use the defaults.
// A nil Transport uses http.DefaultTransport (including gzip decompression).
type Service struct {
	Timeout      time.Duration
	MaxBodyBytes int64
	Transport    http.RoundTripper
}

var _ service.RequestService = (*Service)(nil)

// New returns a service with a 60-second timeout and a 10 MiB response body cap.
func New() *Service {
	return &Service{Timeout: DefaultTimeout, MaxBodyBytes: DefaultMaxBodyBytes}
}

// Send resolves variables, sends req, and reads the complete response while
// retaining at most MaxBodyBytes bytes. Size includes bytes beyond that cap.
func (s *Service) Send(ctx context.Context, req model.Request, variables []model.KV) (model.Response, error) {
	if err := ctx.Err(); err != nil {
		return model.Response{}, err
	}
	req = vars.Apply(req, variables)
	u, err := requestURL(req.URL)
	if err != nil {
		return model.Response{}, err
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return model.Response{}, fmt.Errorf("invalid URL query: %w", err)
	}
	// The editor mirrors URL query entries in Params, while importers may
	// populate Params alone. Match each row against one original occurrence
	// so neither shape duplicates entries or loses intentional repetitions.
	remaining := make(map[model.KV]int)
	for key, values := range query {
		for _, value := range values {
			remaining[model.KV{Key: key, Value: value}]++
		}
	}
	for _, row := range req.Params {
		if row.Disabled {
			continue
		}
		if remaining[row] > 0 {
			remaining[row]--
		} else {
			query.Add(row.Key, row.Value)
		}
	}
	if req.Auth.Type == model.AuthAPIKey && req.Auth.In == model.APIKeyInQuery {
		query.Set(req.Auth.Key, req.Auth.Value)
	}
	u.RawQuery = query.Encode()

	// Use the same boundary when a 307/308 redirect replays a multipart upload.
	factory := newBodyFactory(req.Body)
	body, contentType, err := factory()
	if err != nil {
		return model.Response{}, err
	}
	if body != nil {
		defer body.Close()
	}
	httpReq, err := http.NewRequestWithContext(ctx, string(req.Method), u.String(), body)
	if err != nil {
		return model.Response{}, fmt.Errorf("build request: %w", err)
	}
	if body != nil {
		httpReq.GetBody = func() (io.ReadCloser, error) {
			r, _, err := factory()
			return r, err
		}
	}
	for _, row := range req.Headers {
		if !row.Disabled {
			httpReq.Header.Add(row.Key, row.Value)
		}
	}
	applyAuth(httpReq, req.Auth)
	setDefaultHeader(httpReq.Header, "User-Agent", "Reqo/0.1")
	if contentType != "" {
		setDefaultHeader(httpReq.Header, "Content-Type", contentType)
	}
	// Host is special in net/http: it lives on Request, not in Header.
	if values, ok := httpReq.Header["Host"]; ok && len(values) > 0 {
		httpReq.Host = values[0]
	}

	timeout := s.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	limit := s.MaxBodyBytes
	if limit <= 0 {
		limit = DefaultMaxBodyBytes
	}
	client := http.Client{
		Timeout:   timeout,
		Transport: s.Transport,
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) > 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			return nil
		},
	}
	start := time.Now()
	resp, err := client.Do(httpReq)
	if err != nil {
		return model.Response{}, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()
	kept, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	size := int64(len(kept))
	if err == nil {
		var discarded int64
		discarded, err = io.Copy(io.Discard, resp.Body)
		size += discarded
	}
	// A local round trip can complete within one Windows clock tick. Preserve
	// a positive measurement even when the clock cannot resolve the interval.
	duration := max(time.Since(start), time.Nanosecond)
	result := model.Response{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Proto:      resp.Proto,
		Headers:    responseHeaders(resp.Header),
		Body:       kept,
		Size:       size,
		Truncated:  size > int64(len(kept)),
		Duration:   duration,
		URL:        resp.Request.URL.String(),
	}
	if err != nil {
		return result, fmt.Errorf("read response body: %w", err)
	}
	return result, nil
}

func requestURL(raw string) (*url.URL, error) {
	if raw == "" {
		return nil, fmt.Errorf("invalid URL: URL is empty")
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.Opaque != "" {
		return nil, fmt.Errorf("invalid HTTP URL %q", raw)
	}
	return u, nil
}

func setDefaultHeader(headers http.Header, key, value string) {
	if _, exists := headers[http.CanonicalHeaderKey(key)]; !exists {
		headers.Set(key, value)
	}
}

func applyAuth(req *http.Request, auth model.Auth) {
	switch auth.Type {
	case model.AuthBearer:
		setDefaultHeader(req.Header, "Authorization", "Bearer "+auth.Token)
	case model.AuthBasic:
		if _, exists := req.Header["Authorization"]; !exists {
			req.SetBasicAuth(auth.Username, auth.Password)
		}
	case model.AuthAPIKey:
		if auth.In != model.APIKeyInQuery {
			setDefaultHeader(req.Header, auth.Key, auth.Value)
		}
	}
}

func responseHeaders(headers http.Header) []model.KV {
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var rows []model.KV
	for _, key := range keys {
		for _, value := range headers[key] {
			rows = append(rows, model.KV{Key: key, Value: value})
		}
	}
	return rows
}
